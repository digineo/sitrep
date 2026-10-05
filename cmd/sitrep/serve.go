package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/digineo/xlog"
	"github.com/digineo/xlog/slogor"

	"github.com/digineo/sitrep/internal/auth"
	"github.com/digineo/sitrep/internal/buildinfo"
	"github.com/digineo/sitrep/internal/config"
	"github.com/digineo/sitrep/internal/i18n"
	"github.com/digineo/sitrep/internal/model"
	"github.com/digineo/sitrep/internal/poller"
	"github.com/digineo/sitrep/internal/server"
	"github.com/digineo/sitrep/internal/store"
)

// loadConfig reads the configuration and creates the auth provider. It
// reports errors to stderr.
func loadConfig(
	stderr io.Writer,
) (*config.Env, config.Config, auth.Provider, bool) {
	dotenv, err := config.ReadDotenv(".env.local", ".env")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil, config.Config{}, nil, false
	}

	env := config.NewEnv(config.Lookup(dotenv))
	cfg := config.Load(env)
	provider := auth.NewProvider(env, cfg)
	if err := env.Err(); err != nil {
		fmt.Fprintf(stderr, "invalid configuration:\n%v\n", err)
		return nil, config.Config{}, nil, false
	}
	return env, cfg, provider, true
}

// serve runs the server until SIGINT or SIGTERM.
func serve(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if fs.NArg() > 0 {
		fmt.Fprintln(stderr, "serve takes no arguments")
		return 2
	}

	env, cfg, provider, ok := loadConfig(stderr)
	if !ok {
		return 1
	}

	log, err := newLogger(cfg, os.Stdout)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	log.Info("build",
		slog.String("version", buildinfo.Version),
		slog.String("commit", buildinfo.Commit),
		slog.String("date", buildinfo.Date))
	log.Info("configuration",
		env.LogAttrs()...)
	if cfg.TrustProxy.All {
		log.Warn("SITREP_TRUST_PROXY=true is deprecated: it trusts the X-Forwarded-* headers of every client; list the proxies' addresses instead")
	}

	db, err := store.Open(cfg.DB)
	if err != nil {
		log.Error("startup failed",
			xlog.Error(err))
		return 1
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error("closing the database failed",
				xlog.Error(err))
		}
	}()

	if err := warnStartup(log, db); err != nil {
		log.Error("startup failed",
			xlog.Error(err))
		return 1
	}

	core := auth.NewCore(
		log,
		db,
		cfg.Auth,
		provider,
		cfg.SessionTTL,
		cfg.TrustProxy,
	)
	polls := poller.New(log, db, cfg.SecretKey, cfg.DefaultRefresh)
	srv, err := server.New(log, cfg, db, core, polls)
	if err != nil {
		log.Error("startup failed",
			xlog.Error(err))
		return 1
	}

	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		log.Error("startup failed",
			xlog.Error(err))
		return 1
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	jobsCtx, stopJobs := context.WithCancel(context.Background())
	var jobs sync.WaitGroup
	jobs.Go(func() { runJobs(jobsCtx, log, db, polls) })
	jobs.Go(func() { polls.Run(jobsCtx) })

	hs := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	failed := make(chan error, 1)
	go func() { failed <- hs.Serve(ln) }()
	log.Info("listening",
		slog.String("address", ln.Addr().String()))

	code := 0
	select {
	case <-ctx.Done():
		log.Info("shutting down")
	case err := <-failed:
		log.Error("server failed",
			xlog.Error(err))
		code = 1
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := hs.Shutdown(shutdownCtx); err != nil {
		log.Warn("draining requests failed",
			xlog.Error(err))
		_ = hs.Close()
	}

	stopJobs()
	jobs.Wait()
	return code
}

func newLogger(cfg config.Config, w io.Writer) (xlog.Logger, error) {
	format := xlog.AsText()
	switch cfg.LogFormat {
	case "json":
		format = xlog.AsJSON()
	case "pretty":
		format = slogor.Colorized()
	}
	return xlog.New(format, xlog.Leveled(cfg.LogLevel), xlog.WriteTo(w))
}

// warnStartup warns about languages without a catalog that were primary,
// and about path-mode sites whose slug became a legal slug.
func warnStartup(log xlog.Logger, db *store.DB) error {
	settings, err := db.Settings()
	if err != nil {
		return err
	}

	if p := settings.Languages.Primary; !i18n.IsSupported(p) {
		log.Warn("the instance's primary language has no catalog, using another one",
			slog.String("language", p),
			slog.String("primary", settings.Languages.Effective().Primary))
	}

	sites, err := db.Sites()
	if err != nil {
		return err
	}

	for _, site := range sites {
		if p := site.Languages.Primary; !i18n.IsSupported(p) {
			log.Warn("a site's primary language has no catalog, using another one",
				slog.String("site", site.ID),
				slog.String("language", p),
				slog.String("primary", site.Languages.Effective().Primary))
		}

		if site.Route.Mode != model.RoutePath {
			continue
		}

		for _, lang := range i18n.Supported() {
			c := i18n.Get(lang)
			if site.Route.Slug == c.Imprint || site.Route.Slug == c.Privacy {
				log.Warn("a path-mode site's slug is also a legal page slug; the site takes precedence",
					slog.String("site", site.ID),
					slog.String("slug", site.Route.Slug),
					slog.String("language", lang))
			}
		}
	}
	return nil
}

// runJobs purges expired sessions and applies incident retention at
// startup and hourly, until ctx ends.
func runJobs(
	ctx context.Context,
	log xlog.Logger,
	db *store.DB,
	polls *poller.Poller,
) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()

	for {
		n, err := db.PurgeSessions(time.Now())
		if err != nil {
			log.Error("purging expired sessions failed",
				xlog.Error(err))
		} else if n > 0 {
			log.Info("purged expired sessions",
				slog.Int("count", n))
		}

		purged, err := db.PurgeIncidents(time.Now())
		if err != nil {
			log.Error("applying incident retention failed",
				xlog.Error(err))
		}

		for site, n := range purged {
			polls.IncidentsChanged(site)
			log.Info("deleted incidents after their retention",
				slog.String("site", site),
				slog.Int("count", n))
		}

		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
