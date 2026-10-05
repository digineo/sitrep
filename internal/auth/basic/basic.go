// Package basic signs admins in with username and password from an
// htpasswd-style file with bcrypt or argon2id hashes.
package basic

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/digineo/xlog"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/auth"
	"github.com/digineo/sitrep/internal/config"
	"github.com/digineo/sitrep/internal/httpx"
)

func init() {
	auth.Register("basic", New)
}

// Bounds of the unauthenticated login endpoint. Each argon2id verification
// allocates its memory parameter, so only a few may run at a time.
const (
	maxUsername   = 200 // bytes
	verifications = 4
	verifyWait    = 5 * time.Second
)

type provider struct {
	path       string
	trustProxy bool
	throttle   *throttle
	// verified for unknown users, so timing reveals nothing
	dummy    string
	slots    chan struct{} // one per running verification
	slotWait time.Duration // how long an attempt waits for a slot

	mu    sync.Mutex
	users map[string]string
	mtime time.Time
}

// New reads SITREP_BASIC_USERS_FILE and loads the users file.
func New(env *config.Env, cfg config.Config) auth.Provider {
	const name = "SITREP_BASIC_USERS_FILE"
	p := &provider{
		path:       env.Required(name),
		trustProxy: cfg.TrustProxy,
		throttle:   newThrottle(),
		dummy:      Hash(""),
		slots:      make(chan struct{}, verifications),
		slotWait:   verifyWait,
	}
	if p.path != "" {
		if err := p.load(); err != nil {
			env.Errorf(name, "%v", err)
		}
	}
	return p
}

func (p *provider) Method() auth.Method { return auth.MethodCredentials }

func (p *provider) Available() bool { return true }

func (p *provider) Routes(mux *http.ServeMux, core *auth.Core) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		p.login(w, r, core)
	}
	mux.HandleFunc("POST /auth/basic/login", handler)
}

// Users returns the usernames of the users file, sorted. If the file
// fails to reload, the previous users are returned with the error.
func (p *provider) Users() ([]string, error) {
	err := p.load()
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Sorted(maps.Keys(p.users)), err
}

// load reads the users file, unless it is unchanged since the last load.
// A file that fails validation leaves the users unchanged.
func (p *provider) load() error {
	fi, err := os.Stat(p.path)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if fi.ModTime().Equal(p.mtime) {
		return nil
	}

	p.mtime = fi.ModTime()
	data, err := os.ReadFile(p.path)
	if err != nil {
		return err
	}

	users, err := parseUsers(string(data))
	if err != nil {
		return fmt.Errorf("%s: %w", p.path, err)
	}

	p.users = users
	return nil
}

// parseUsers parses lines of "username:hash". Blank lines and lines starting
// with # are skipped. Errors name the line, never its content.
func parseUsers(data string) (map[string]string, error) {
	users := map[string]string{}
	for i, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}

		name, hash, ok := strings.Cut(line, ":")
		if !ok || name == "" {
			return nil, fmt.Errorf("line %d: expected username:hash", i+1)
		}

		if len(name) > maxUsername {
			return nil, fmt.Errorf(
				"line %d: username longer than %d bytes",
				i+1,
				maxUsername,
			)
		}

		if _, dup := users[name]; dup {
			return nil, fmt.Errorf("line %d: duplicate username", i+1)
		}

		if err := checkHash(hash); err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}

		users[name] = hash
	}
	return users, nil
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (p *provider) login(w http.ResponseWriter, r *http.Request, core *auth.Core) {
	var c credentials
	if err := httpx.ReadJSON(w, r, &c); err != nil {
		httpx.WriteError(w, r, core.Log, err)
		return
	}

	if len(c.Username) > maxUsername {
		core.Log.Info("failed login with an overlong username")
		e := apierr.New(http.StatusUnauthorized, apierr.InvalidCredentials)
		httpx.WriteError(w, r, core.Log, e)
		return
	}

	// The throttle is checked while holding a slot, so parallel attempts
	// cannot all pass it before the first failure is recorded.
	ip := httpx.Effective(r, p.trustProxy).IP
	if !p.acquire(r.Context()) {
		e := apierr.New(http.StatusTooManyRequests, apierr.Throttled)
		httpx.WriteError(w, r, core.Log, e)
		return
	}

	ok, throttled := p.verify(core, c, ip)
	<-p.slots
	switch {
	case throttled:
		e := apierr.New(http.StatusTooManyRequests, apierr.Throttled)
		httpx.WriteError(w, r, core.Log, e)
	case !ok:
		core.Log.Info("failed login",
			slog.String("username", c.Username))
		e := apierr.New(http.StatusUnauthorized, apierr.InvalidCredentials)
		httpx.WriteError(w, r, core.Log, e)
	default:
		id := auth.Identity{
			Subject:     c.Username,
			DisplayName: c.Username,
		}
		if err := core.Login(w, r, id); err != nil {
			httpx.WriteError(w, r, core.Log, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// acquire takes a verification slot. It gives up after p.slotWait or when
// ctx ends.
func (p *provider) acquire(ctx context.Context) bool {
	select {
	case p.slots <- struct{}{}:
		return true
	case <-time.After(p.slotWait):
	case <-ctx.Done():
	}
	return false
}

// verify checks the credentials unless the throttle blocks them, and
// records the outcome with the throttle. Unknown users are verified against
// a dummy hash. The caller holds a slot.
func (p *provider) verify(
	core *auth.Core,
	c credentials,
	ip string,
) (ok, throttled bool) {
	if p.throttle.blocked(c.Username, ip, time.Now()) {
		return false, true
	}

	if err := p.load(); err != nil {
		core.Log.Error("reloading the users file failed, keeping the previous users",
			xlog.Error(err))
	}

	p.mu.Lock()
	hash, known := p.users[c.Username]
	p.mu.Unlock()
	if !known {
		hash = p.dummy
	}
	if !Verify(hash, c.Password) || !known {
		p.throttle.fail(c.Username, ip, time.Now())
		return false, false
	}

	p.throttle.reset(c.Username, ip, time.Now())
	return true, false
}
