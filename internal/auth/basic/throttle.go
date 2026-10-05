package basic

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"maps"
	"net/netip"
	"slices"
	"sync"
	"time"
)

const (
	maxFailures = 5
	window      = 15 * time.Minute
	lockout     = 15 * time.Minute
	maxCounters = 10000
	pruneEvery  = time.Minute
)

// throttle blocks logins after too many failures. Failures are counted per
// username and, independently, per client address, so many usernames from
// one address lock the address and one username from many addresses locks
// the username. IPv6 addresses are counted per /64, which clients often
// get as a whole. Addresses are only kept as keyed hash under a secret that
// changes every UTC day; the change drops the address counters.
//
// The number of counters is capped. Expired counters are pruned at most
// once per pruneEvery, so failures do not scan all counters.
type throttle struct {
	mu     sync.Mutex
	day    string
	secret []byte
	users  map[string]*counter
	addrs  map[string]*counter
	pruned time.Time
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

// expired reports whether c neither locks nor holds failures within the
// window at now.
func (c *counter) expired(now time.Time) bool {
	return !c.locked(now) &&
		(len(c.failures) == 0 || now.Sub(c.failures[len(c.failures)-1]) >= window)
}

// addrKey returns the keyed hash of ip, or of its /64 for IPv6. The caller
// holds t.mu.
func (t *throttle) addrKey(ip string, now time.Time) string {
	if a, err := netip.ParseAddr(ip); err == nil {
		if a = a.Unmap(); a.Is6() {
			p, _ := a.Prefix(64)
			ip = p.String()
		}
	}
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
// While the counters are at their cap, attempts that would need a new
// counter are locked out, too.
func (t *throttle) blocked(username, ip string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.prune(now)

	user := t.users[username]
	addr := t.addrs[t.addrKey(ip, now)]
	if (user == nil || addr == nil) && len(t.users)+len(t.addrs) >= maxCounters {
		return true
	}
	return user.locked(now) || addr.locked(now)
}

// fail records a failed attempt for username and for ip.
func (t *throttle) fail(username, ip string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.prune(now)
	count(t.users, username, now)
	count(t.addrs, t.addrKey(ip, now), now)
}

// prune drops expired counters, unless it ran less than pruneEvery ago.
// The caller holds t.mu.
func (t *throttle) prune(now time.Time) {
	if now.Sub(t.pruned) < pruneEvery {
		return
	}
	t.pruned = now
	for _, counters := range []map[string]*counter{t.users, t.addrs} {
		maps.DeleteFunc(counters, func(_ string, c *counter) bool {
			return c.expired(now)
		})
	}
}

// count adds a failure to the counter for key, dropping its failures
// outside the window. The last allowed failure starts a lockout.
func count(counters map[string]*counter, key string, now time.Time) {
	c := counters[key]
	if c == nil {
		c = &counter{}
		counters[key] = c
	}

	outside := func(at time.Time) bool { return now.Sub(at) >= window }
	c.failures = slices.DeleteFunc(c.failures, outside)
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
