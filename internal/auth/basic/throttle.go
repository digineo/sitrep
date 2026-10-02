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
// username and client address. The address is only kept as keyed hash under
// a secret that changes every UTC day; the change drops all counters.
type throttle struct {
	mu      sync.Mutex
	day     string
	secret  []byte
	entries map[string]*throttleEntry
}

type throttleEntry struct {
	failures []time.Time
	until    time.Time
}

func newThrottle() *throttle {
	return &throttle{entries: map[string]*throttleEntry{}}
}

// key returns the counter key. The caller holds t.mu.
func (t *throttle) key(username, ip string, now time.Time) string {
	if day := now.UTC().Format(time.DateOnly); day != t.day {
		t.day = day
		t.secret = make([]byte, 32)
		_, _ = rand.Read(t.secret)
		clear(t.entries)
	}
	mac := hmac.New(sha256.New, t.secret)
	mac.Write([]byte(ip))
	return username + "\x00" + string(mac.Sum(nil))
}

// blocked reports whether attempts for username from ip are locked out.
func (t *throttle) blocked(username, ip string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.entries[t.key(username, ip, now)]
	return e != nil && now.Before(e.until)
}

// fail records a failed attempt. The last allowed failure starts a lockout.
func (t *throttle) fail(username, ip string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := t.key(username, ip, now)
	for key, e := range t.entries {
		outside := func(at time.Time) bool { return now.Sub(at) >= window }
		e.failures = slices.DeleteFunc(e.failures, outside)
		if len(e.failures) == 0 && !now.Before(e.until) && key != k {
			delete(t.entries, key)
		}
	}

	e := t.entries[k]
	if e == nil {
		e = &throttleEntry{}
		t.entries[k] = e
	}

	e.failures = append(e.failures, now)
	if len(e.failures) >= maxFailures {
		e.failures = nil
		e.until = now.Add(lockout)
	}
}

// reset drops the counter after a successful login.
func (t *throttle) reset(username, ip string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, t.key(username, ip, now))
}
