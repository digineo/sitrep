package config

import (
	"log/slog"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/model"
)

// Config is the core server configuration. The auth provider reads its own
// variables.
type Config struct {
	Listen         string
	DB             string
	SecretKey      []byte
	DefaultRefresh time.Duration
	BaseDomains    []string
	TrustProxy     httpx.Proxies
	Auth           string
	SessionTTL     time.Duration
	LogLevel       slog.Level
	LogFormat      string
}

// Load reads the core configuration from env. Errors are recorded in env.
func Load(env *Env) Config {
	c := Config{
		Listen:    env.String("SITREP_LISTEN", ":2607"),
		DB:        env.String("SITREP_DB", "sitrep.db"),
		SecretKey: env.Key("SITREP_SECRET_KEY"),
		DefaultRefresh: env.Duration(
			"SITREP_DEFAULT_REFRESH",
			30*time.Second,
			5*time.Second,
			24*time.Hour,
		),
		BaseDomains: baseDomains(env),
		TrustProxy:  trustProxy(env),
		Auth:        env.String("SITREP_AUTH", "oidc"),
		SessionTTL: env.Duration(
			"SITREP_SESSION_TTL",
			12*time.Hour,
			5*time.Minute,
			30*24*time.Hour,
		),
		LogFormat: env.Enum(
			"SITREP_LOG_FORMAT",
			"text",
			"text",
			"json",
			"pretty",
		),
	}

	level := env.Enum(
		"SITREP_LOG_LEVEL",
		"info",
		"debug",
		"info",
		"warn",
		"error",
	)
	switch level {
	case "debug":
		c.LogLevel = slog.LevelDebug
	case "warn":
		c.LogLevel = slog.LevelWarn
	case "error":
		c.LogLevel = slog.LevelError
	}
	return c
}

func baseDomains(env *Env) []string {
	const name = "SITREP_BASE_DOMAINS"
	v := env.Required(name)
	if v == "" {
		return nil
	}

	var domains []string
	for d := range strings.SplitSeq(v, ",") {
		d = strings.TrimSpace(d)
		switch {
		case !model.ValidDomain(d):
			env.Errorf(
				name,
				"%q is not a valid lowercase hostname with at least two labels",
				d,
			)
		case slices.Contains(domains, d):
			env.Errorf(name, "%q is listed twice", d)
		default:
			domains = append(domains, d)
		}
	}
	return domains
}

// trustProxy reads the comma-separated addresses and networks of trusted
// proxies, or the deprecated boolean, of which true trusts every peer.
func trustProxy(env *Env) httpx.Proxies {
	const name = "SITREP_TRUST_PROXY"
	v, _ := env.lookup(name)
	env.log(name, v)

	var p httpx.Proxies
	switch strings.ToLower(v) {
	case "", "false", "0", "no", "off":
	case "true", "1", "yes", "on":
		p.All = true
	default:
		for s := range strings.SplitSeq(v, ",") {
			s = strings.TrimSpace(s)
			n, err := netip.ParsePrefix(s)
			if a, aerr := netip.ParseAddr(s); aerr == nil {
				n, err = a.Prefix(a.BitLen())
			}
			if err != nil {
				env.Errorf(name, "%q is neither an IP address nor a network in CIDR notation", s)
				continue
			}
			p.Prefixes = append(p.Prefixes, n)
		}
	}
	return p
}
