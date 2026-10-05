package basic

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/digineo/xlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/auth"
	"github.com/digineo/sitrep/internal/config"
	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/store"
)

// Test vectors computed with independent implementations: Node.js's
// crypto.argon2Sync (OpenSSL) and bcryptjs.
const (
	argonCorrectHorse  = "$argon2id$v=19$m=19456,t=2,p=1$cHJvbXRpbWUtdGVzdC1zYWx0LTE2Yg$6gqTvPutwbWqr+zCExGT5EJ0/yeWpnl1UMXCOzpoua8"
	argonBatteryStaple = "$argon2id$v=19$m=32768,t=3,p=2$cHJvbXRpbWUtdGVzdC1zYWx0LTE2Yg$2bAx5ed9nA7XB57QVgm70hdF6jE9bBVI5t2dDFVQQNfyRUkSVPh2cA"
	bcryptCorrectHorse = "$2b$10$WJf1hT2iQBLDFiChwU0FweHVd1yu6FSGe9KOIX1l15IKeKU879dy2"
	bcryptCost4        = "$2a$04$C6snK0B5jw1aTz0TKF4Il.V1IQHgilgJ1wrCSammMXD4W9r/ILB/a"
)

func TestVerifyVectors(t *testing.T) {
	assert := assert.New(t)

	assert.True(Verify(argonCorrectHorse, "correct horse"))
	assert.False(Verify(argonCorrectHorse, "correct horse "))
	assert.True(Verify(argonBatteryStaple, "battery staple"), "40-byte hash, p=2")
	assert.False(Verify(argonBatteryStaple, "correct horse"))

	for _, minor := range []string{"a", "b", "y"} {
		h := "$2" + minor + strings.TrimPrefix(bcryptCorrectHorse, "$2b")
		assert.True(Verify(h, "correct horse"), minor)
		assert.False(Verify(h, "wrong"), minor)
	}
}

func TestHash(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	h := Hash("s3cret")
	require.NoError(checkHash(h))
	assert.True(Verify(h, "s3cret"))
	assert.False(Verify(h, "s3cret!"))
	assert.NotEqual(h, Hash("s3cret"), "salted")

	p, err := parseArgon2id(h)
	require.NoError(err)
	want := &argon2idHash{
		memory:  65536,
		time:    3,
		threads: 4,
		salt:    p.salt,
		key:     p.key,
	}
	assert.Equal(want, p)
	assert.Len(p.salt, 16)
	assert.Len(p.key, 32)
}

func TestCheckHash(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	require.NoError(checkHash(argonCorrectHorse))
	require.NoError(checkHash(bcryptCorrectHorse))

	salt := base64.RawStdEncoding.EncodeToString([]byte("0123456789abcdef"))
	key := base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	argon := func(params string) string {
		return "$argon2id$v=19$" + params + "$" + salt + "$" + key
	}

	require.NoError(checkHash(argon("m=1048576,t=10,p=16")), "upper bounds")

	for name, h := range map[string]string{
		"bcrypt cost below 10": bcryptCost4,
		"truncated bcrypt":     bcryptCorrectHorse[:50],
		"argon2i":              strings.Replace(argonCorrectHorse, "argon2id", "argon2i", 1),
		"argon2d":              strings.Replace(argonCorrectHorse, "argon2id", "argon2d", 1),
		"version 16":           strings.Replace(argonCorrectHorse, "v=19", "v=16", 1),
		"memory too low":       argon("m=19455,t=2,p=1"),
		"memory too high":      argon("m=1048577,t=2,p=1"),
		"too few iterations":   argon("m=19456,t=1,p=1"),
		"too many iterations":  argon("m=19456,t=11,p=1"),
		"no parallelism":       argon("m=19456,t=2,p=0"),
		"too much parallelism": argon("m=19456,t=2,p=17"),
		"reordered parameters": argon("t=2,m=19456,p=1"),
		"trailing garbage":     argon("m=19456,t=2,p=1x"),
		"short salt":           "$argon2id$v=19$m=19456,t=2,p=1$" + base64.RawStdEncoding.EncodeToString([]byte("short")) + "$" + key,
		"short hash":           "$argon2id$v=19$m=19456,t=2,p=1$" + salt + "$" + base64.RawStdEncoding.EncodeToString([]byte("short")),
		"padded base64":        "$argon2id$v=19$m=19456,t=2,p=1$" + base64.StdEncoding.EncodeToString([]byte("0123456789abcdefg")) + "$" + key,
		"missing part":         "$argon2id$v=19$m=19456,t=2,p=1$" + salt,
		"sha1":                 "{SHA}fEqNCco3Yq9h5ZUglD3CZJT4lBs=",
		"md5":                  "$apr1$abc$def",
		"plain text":           "password",
	} {
		err := checkHash(h)
		if assert.Error(err, name) {
			assert.NotContains(err.Error(), h, name)
		}
	}
}

func TestParseUsers(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	data := "# admins\n\nann:" + argonCorrectHorse +
		"\r\nbob:" + bcryptCorrectHorse + "\n"
	users, err := parseUsers(data)
	require.NoError(err)
	want := map[string]string{
		"ann": argonCorrectHorse,
		"bob": bcryptCorrectHorse,
	}
	assert.Equal(want, users)

	for data, want := range map[string]string{
		"ann:" + argonCorrectHorse + "\n# x\nbob":                  "line 3: expected username:hash",
		":" + argonCorrectHorse:                                    "line 1: expected username:hash",
		"ann:" + argonCorrectHorse + "\nann:" + bcryptCorrectHorse: "line 2: duplicate username",
		"ann:" + argonCorrectHorse + "\n\nbob:plaintext-secret":    "line 3: unsupported hash format",
	} {
		_, err := parseUsers(data)
		require.Error(err, data)
		assert.Contains(err.Error(), want)
		assert.NotContains(err.Error(), "plaintext-secret")
	}
}

func TestThrottle(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	th := newThrottle()
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	for i := range maxFailures - 1 {
		th.fail("ann", "192.0.2.1", now.Add(time.Duration(i)*time.Minute))
	}

	blocked := th.blocked("ann", "192.0.2.1", now.Add(5*time.Minute))
	assert.False(blocked, "four failures are allowed")

	th.fail("ann", "192.0.2.1", now.Add(5*time.Minute))
	assert.True(th.blocked("ann", "192.0.2.1", now.Add(5*time.Minute)))
	assert.True(th.blocked("ann", "192.0.2.1", now.Add(20*time.Minute-time.Second)))
	blocked = th.blocked("ann", "192.0.2.1", now.Add(20*time.Minute))
	assert.False(blocked, "the lockout lasts 15 minutes")
	blocked = th.blocked("ann", "192.0.2.2", now.Add(5*time.Minute))
	assert.True(blocked, "the username is locked from every address")
	blocked = th.blocked("bob", "192.0.2.1", now.Add(5*time.Minute))
	assert.True(blocked, "the address is locked for every username")
	assert.False(th.blocked("bob", "192.0.2.2", now.Add(5*time.Minute)))

	for i := range maxFailures - 1 {
		th.fail("bob", "192.0.2.3", now.Add(time.Duration(i)*5*time.Minute))
	}

	th.fail("bob", "192.0.2.3", now.Add(20*time.Minute))
	blocked = th.blocked("bob", "192.0.2.3", now.Add(20*time.Minute))
	assert.False(blocked, "failures older than 15 minutes expire")

	for range maxFailures - 1 {
		th.fail("eve", "192.0.2.4", now)
	}

	th.reset("eve", "192.0.2.4", now)
	th.fail("eve", "192.0.2.4", now)
	assert.False(th.blocked("eve", "192.0.2.4", now), "success drops the counters")

	late := time.Date(2026, 10, 2, 23, 55, 0, 0, time.UTC)
	for i := range maxFailures {
		th.fail("mallory", fmt.Sprintf("198.51.100.%d", i+1), late)
		th.fail(fmt.Sprintf("user%d", i), "192.0.2.9", late)
	}

	require.True(th.blocked("trent", "192.0.2.9", late))
	nextDay := late.Add(10 * time.Minute)
	blocked = th.blocked("trent", "192.0.2.9", nextDay)
	assert.False(blocked, "the daily secret change drops the address counters")
	blocked = th.blocked("mallory", "203.0.113.1", nextDay)
	assert.True(blocked, "username counters survive the secret change")
	for key := range th.addrs {
		assert.NotContains(key, "192.0.2", "addresses are only kept as keyed hash")
	}
}

func TestThrottleUsernameFromManyAddresses(t *testing.T) {
	th := newThrottle()
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	for i := range maxFailures {
		ip := fmt.Sprintf("192.0.2.%d", i+1)
		assert.False(t, th.blocked("ann", ip, now), ip)
		th.fail("ann", ip, now)
	}

	blocked := th.blocked("ann", "198.51.100.1", now)
	assert.True(t, blocked, "a fresh address cannot try the username")
	blocked = th.blocked("bob", "192.0.2.1", now)
	assert.False(t, blocked, "the addresses each failed only once")
}

func TestThrottleAddressWithManyUsernames(t *testing.T) {
	assert := assert.New(t)

	th := newThrottle()
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	for i := range maxFailures {
		user := fmt.Sprintf("user%d", i)
		assert.False(th.blocked(user, "192.0.2.1", now), user)
		th.fail(user, "192.0.2.1", now)
	}

	blocked := th.blocked("fresh", "192.0.2.1", now)
	assert.True(blocked, "the address cannot try another username")
	blocked = th.blocked("user0", "192.0.2.2", now)
	assert.False(blocked, "the usernames each failed only once")

	th.reset("fresh", "192.0.2.1", now)
	blocked = th.blocked("fresh", "192.0.2.1", now)
	assert.False(blocked, "success drops the address counter")
}

func TestThrottleIPv6Prefix(t *testing.T) {
	assert := assert.New(t)

	th := newThrottle()
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	for i := range maxFailures {
		ip := fmt.Sprintf("2001:db8:1:2::%x", i+1)
		assert.False(th.blocked("fresh", ip, now), ip)
		th.fail(fmt.Sprintf("user%d", i), ip, now)
	}

	assert.Len(th.addrs, 1, "the /64 shares one counter")
	blocked := th.blocked("fresh", "2001:db8:1:2:ffff:ffff:ffff:ffff", now)
	assert.True(blocked, "another address of the /64 is locked")
	blocked = th.blocked("fresh", "2001:db8:1:3::1", now)
	assert.False(blocked, "another /64 is not")

	for i := range maxFailures {
		th.fail(fmt.Sprintf("user%d", i), fmt.Sprintf("::ffff:192.0.2.%d", i+1), now)
	}
	blocked = th.blocked("fresh", "192.0.2.9", now)
	assert.False(blocked, "IPv4-mapped addresses count as IPv4")
}

func TestThrottleCap(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	th := newThrottle()
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	for i := range maxCounters / 2 {
		ip := fmt.Sprintf("10.0.%d.%d", i/256, i%256)
		th.fail(fmt.Sprintf("user%d", i), ip, now)
	}

	require.Equal(maxCounters, len(th.users)+len(th.addrs))
	blocked := th.blocked("user1", "10.0.0.1", now)
	assert.False(blocked, "known keys are counted as usual")
	blocked = th.blocked("fresh", "10.0.0.1", now)
	assert.True(blocked, "a new username is locked while the counters are full")
	blocked = th.blocked("user1", "192.0.2.1", now)
	assert.True(blocked, "a new address is locked while the counters are full")

	later := now.Add(window)
	blocked = th.blocked("fresh", "192.0.2.1", later)
	assert.False(blocked, "expired counters make room")
	assert.Empty(th.users)
}

func TestThrottlePrunesPeriodically(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	th := newThrottle()
	t0 := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	th.fail("ann", "192.0.2.1", t0)
	th.fail("bob", "192.0.2.2", t0.Add(50*time.Second))
	require.Len(th.users, 2)

	th.blocked("eve", "192.0.2.3", t0.Add(window+10*time.Second))
	users := slices.Collect(maps.Keys(th.users))
	assert.Equal([]string{"bob"}, users, "ann expired")

	th.blocked("eve", "192.0.2.3", t0.Add(window+time.Minute))
	assert.Len(
		th.users,
		1,
		"bob expired, but the last pruning was less than a minute ago",
	)

	th.blocked("eve", "192.0.2.3", t0.Add(window+70*time.Second))
	assert.Empty(th.users)
	assert.Empty(th.addrs)
}

type fixture struct {
	provider *provider
	core     *auth.Core
	path     string
	login    http.Handler
	attempts atomic.Int32
}

func newFixture(t *testing.T, users string) *fixture {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "users")
	require.NoError(t, os.WriteFile(path, []byte(users), 0o600))
	env := config.NewEnv(func(name string) (string, bool) {
		return path, name == "SITREP_BASIC_USERS_FILE"
	})

	p := New(env, config.Config{})
	require.NoError(t, env.Err())

	db, err := store.Open(filepath.Join(dir, "sitrep.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	core := auth.NewCore(xlog.NewDiscard(), db, "basic", p, time.Hour, httpx.Proxies{})
	return &fixture{
		provider: p.(*provider),
		core:     core,
		path:     path,
		login:    core.Handler(),
	}
}

// attempt posts a login, each one from another client address.
func (f *fixture) attempt(username, password string) *httptest.ResponseRecorder {
	n := f.attempts.Add(1)
	target := "http://status.example.com/auth/basic/login"
	body := `{"username":"` + username + `","password":"` + password + `"}`
	r := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	r.RemoteAddr = fmt.Sprintf("10.0.%d.%d:1234", n/256, n%256)
	r.Header.Set("Origin", "http://status.example.com")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	f.login.ServeHTTP(w, r)
	return w
}

func TestNewValidatesUsersFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users")
	require.NoError(t, os.WriteFile(path, []byte("ann:plaintext"), 0o600))
	for _, value := range []string{
		path,
		filepath.Join(t.TempDir(), "missing"),
		"",
	} {
		env := config.NewEnv(func(name string) (string, bool) {
			return value, name == "SITREP_BASIC_USERS_FILE"
		})

		New(env, config.Config{})
		assert.ErrorContains(t, env.Err(), "SITREP_BASIC_USERS_FILE", value)
	}
}

func TestLogin(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t, "ann:"+argonCorrectHorse+"\n")
	w := f.attempt("ann", "correct horse")
	assert.Equal(http.StatusNoContent, w.Code)
	require.Len(w.Result().Cookies(), 1)

	for _, c := range [][2]string{
		{"ann", "wrong"},
		{"nobody", "correct horse"},
		{"", ""},
	} {
		w := f.attempt(c[0], c[1])
		assert.Equal(http.StatusUnauthorized, w.Code, c)
		assert.Contains(w.Body.String(), `"code":"invalid_credentials"`, c)
		assert.Empty(w.Result().Cookies(), c)
	}
}

func TestLoginThrottled(t *testing.T) {
	assert := assert.New(t)

	f := newFixture(t, "ann:"+argonCorrectHorse+"\nbob:"+argonCorrectHorse+"\n")
	for range maxFailures {
		assert.Equal(http.StatusUnauthorized, f.attempt("ann", "wrong").Code)
	}

	w := f.attempt("ann", "correct horse")
	assert.Equal(
		http.StatusTooManyRequests,
		w.Code,
		"even the right password is refused",
	)
	assert.Contains(w.Body.String(), `"code":"throttled"`)

	w = f.attempt("bob", "correct horse")
	assert.Equal(http.StatusNoContent, w.Code, "the addresses failed only once")
}

func TestLoginUnknownUserTakesComparableTime(t *testing.T) {
	f := newFixture(t, "ann:"+Hash("correct horse")+"\n")
	measure := func(username string) time.Duration {
		start := time.Now()
		for range 3 {
			code := f.attempt(username, "wrong").Code
			require.Equal(t, http.StatusUnauthorized, code)
		}
		return time.Since(start)
	}

	known := measure("ann")
	unknown := measure("nobody")
	assert.Greater(
		t,
		unknown,
		known/2,
		"unknown users go through a dummy verification",
	)
}

func TestReload(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t, "ann:"+argonCorrectHorse+"\n")
	require.Equal(http.StatusNoContent, f.attempt("ann", "correct horse").Code)

	later := time.Now().Add(time.Minute)
	data := []byte("bob:" + bcryptCorrectHorse + "\n")
	require.NoError(os.WriteFile(f.path, data, 0o600))
	require.NoError(os.Chtimes(f.path, later, later))
	assert.Equal(http.StatusNoContent, f.attempt("bob", "correct horse").Code)
	assert.Equal(http.StatusUnauthorized, f.attempt("ann", "correct horse").Code)

	later = later.Add(time.Minute)
	require.NoError(os.WriteFile(f.path, []byte("broken"), 0o600))
	require.NoError(os.Chtimes(f.path, later, later))
	assert.Equal(
		http.StatusNoContent,
		f.attempt("bob", "correct horse").Code,
		"a broken file keeps the previous users",
	)
}

func TestRemovalEndsSessions(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t, "ann:"+argonCorrectHorse+"\nbob:"+bcryptCorrectHorse+"\n")
	w := f.attempt("ann", "correct horse")
	require.Equal(http.StatusNoContent, w.Code)
	r := httptest.NewRequest(http.MethodGet, "http://status.example.com/auth/session", nil)
	r.AddCookie(w.Result().Cookies()[0])
	_, ok, err := f.core.User(r)
	require.NoError(err)
	require.True(ok)

	later := time.Now().Add(time.Minute)
	data := []byte("bob:" + bcryptCorrectHorse + "\n")
	require.NoError(os.WriteFile(f.path, data, 0o600))
	require.NoError(os.Chtimes(f.path, later, later))
	_, ok, err = f.core.User(r)
	require.NoError(err)
	assert.False(ok, "the session ends with the removal from the users file")
}

func TestUsers(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t, "bob:"+bcryptCorrectHorse+"\nann:"+argonCorrectHorse+"\n")
	var dir auth.Directory = f.provider
	users, err := dir.Users()
	require.NoError(err)
	assert.Equal([]string{"ann", "bob"}, users)

	later := time.Now().Add(time.Minute)
	data := []byte("cat:" + bcryptCorrectHorse + "\n")
	require.NoError(os.WriteFile(f.path, data, 0o600))
	require.NoError(os.Chtimes(f.path, later, later))
	users, err = dir.Users()
	require.NoError(err)
	assert.Equal([]string{"cat"}, users, "the file is reloaded")

	later = later.Add(time.Minute)
	require.NoError(os.WriteFile(f.path, []byte("broken"), 0o600))
	require.NoError(os.Chtimes(f.path, later, later))
	users, err = dir.Users()
	assert.Error(err)
	assert.Equal([]string{"cat"}, users, "a broken file keeps the previous users")
}

// occupySlots takes every verification slot until the test ends, and makes
// attempts give up waiting for one quickly.
func (f *fixture) occupySlots(t *testing.T) {
	t.Helper()
	f.provider.slotWait = 10 * time.Millisecond
	for range verifications {
		f.provider.slots <- struct{}{}
	}
	t.Cleanup(func() {
		for range verifications {
			<-f.provider.slots
		}
	})
}

func TestLoginWaitsForAVerificationSlot(t *testing.T) {
	f := newFixture(t, "ann:"+argonCorrectHorse+"\n")
	f.occupySlots(t)
	w := f.attempt("ann", "correct horse")
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"throttled"`)
	users := f.provider.throttle.users
	assert.Empty(t, users, "no verification ran, so no failure is counted")
}

func TestLoginRejectsOverlongUsernames(t *testing.T) {
	assert := assert.New(t)

	f := newFixture(t, "ann:"+argonCorrectHorse+"\n")
	f.occupySlots(t)
	w := f.attempt(strings.Repeat("a", maxUsername+1), "correct horse")
	assert.Equal(
		http.StatusUnauthorized,
		w.Code,
		"answered without a verification slot",
	)
	assert.Contains(w.Body.String(), `"code":"invalid_credentials"`)
	assert.Empty(f.provider.throttle.users)
	assert.Empty(f.provider.throttle.addrs)

	w = f.attempt(strings.Repeat("a", maxUsername), "correct horse")
	assert.Equal(
		http.StatusTooManyRequests,
		w.Code,
		"a username at the limit is verified",
	)

	line := strings.Repeat("a", maxUsername+1) + ":" + argonCorrectHorse
	_, err := parseUsers(line)
	assert.ErrorContains(err, "line 1: username longer than 200 bytes")
}

func TestLoginParallelAttemptsAreThrottled(t *testing.T) {
	f := newFixture(t, "ann:"+argonCorrectHorse+"\n")
	var mu sync.Mutex
	codes := map[int]int{}
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			code := f.attempt("ann", "wrong").Code
			mu.Lock()
			codes[code]++
			mu.Unlock()
		})
	}

	wg.Wait()
	total := codes[http.StatusUnauthorized] + codes[http.StatusTooManyRequests]
	assert.Equal(t, 20, total, codes)
	assert.GreaterOrEqual(t, codes[http.StatusUnauthorized], maxFailures, codes)
	assert.LessOrEqual(
		t,
		codes[http.StatusUnauthorized],
		maxFailures+verifications-1,
		"only attempts holding a slot can pass the throttle together",
	)
}
