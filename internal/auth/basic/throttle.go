package basic

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"slices"
	"sync"
	"time"
)

const (
	maxFailures = 5
	window      = 15 * time.Minute
	lockout     = 15 * time.Minute
)

// throttle blocks logins after too many failures. Failures are counted per
// username and, independently, per client address, so many usernames from
// one address lock the address and one username from many addresses locks
// the username. Addresses are only kept as keyed hash under a secret that
// changes every UTC day; the change drops the address counters.
type throttle struct {
	mu     sync.Mutex
	day    string
	secret []byte
	users  map[string]*counter
	addrs  map[string]*counter
}

type counter struct {
	failures []time.Time
	until    time.Time
}

func newThrottle() *throttle {
	return &throttle{
		users: map[string]*counter{},
		addrs: map[string]*counter{},
	}
}

// locked reports whether c is locked out at now. c may be nil.
func (c *counter) locked(now time.Time) bool {
	return c != nil && now.Before(c.until)
}

// addrKey returns the keyed hash of ip. The caller holds t.mu.
func (t *throttle) addrKey(ip string, now time.Time) string {
	if day := now.UTC().Format(time.DateOnly); day != t.day {
		t.day = day
		t.secret = make([]byte, 32)
		_, _ = rand.Read(t.secret)
		clear(t.addrs)
	}
	mac := hmac.New(sha256.New, t.secret)
	mac.Write([]byte(ip))
	return string(mac.Sum(nil))
}

// blocked reports whether attempts for username or from ip are locked out.
func (t *throttle) blocked(username, ip string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.users[username].locked(now) || t.addrs[t.addrKey(ip, now)].locked(now)
}

// fail records a failed attempt for username and for ip.
func (t *throttle) fail(username, ip string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	addr := t.addrKey(ip, now)
	prune(t.users, username, now)
	prune(t.addrs, addr, now)
	count(t.users, username, now)
	count(t.addrs, addr, now)
}

// prune drops failures outside the window, and counters without failures
// and lockout except the one for keep.
func prune(counters map[string]*counter, keep string, now time.Time) {
	for key, c := range counters {
		outside := func(at time.Time) bool { return now.Sub(at) >= window }
		c.failures = slices.DeleteFunc(c.failures, outside)
		if len(c.failures) == 0 && !c.locked(now) && key != keep {
			delete(counters, key)
		}
	}
}

// count adds a failure to the counter for key. The last allowed failure
// starts a lockout.
func count(counters map[string]*counter, key string, now time.Time) {
	c := counters[key]
	if c == nil {
		c = &counter{}
		counters[key] = c
	}

	c.failures = append(c.failures, now)
	if len(c.failures) >= maxFailures {
		c.failures = nil
		c.until = now.Add(lockout)
	}
}

// reset drops the counters of username and ip after a successful login.
func (t *throttle) reset(username, ip string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.users, username)
	delete(t.addrs, t.addrKey(ip, now))
}
