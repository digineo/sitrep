//go:build testauth

package bypass

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/digineo/xlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/auth"
	"github.com/digineo/sitrep/internal/config"
	"github.com/digineo/sitrep/internal/store"
)

func TestLoginAndLogout(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, err := store.Open(filepath.Join(t.TempDir(), "sitrep.db"))
	require.NoError(err)
	defer func() { _ = db.Close() }()

	env := config.NewEnv(func(string) (string, bool) { return "", false })
	p := auth.NewProvider(env, config.Config{Auth: "bypass"})
	require.NoError(env.Err())

	core := auth.NewCore(xlog.NewDiscard(), db, "bypass", p, time.Hour, false)
	h := core.Handler()

	target := "http://status.example.com/auth/bypass/login?return=/admin/settings"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
	assert.Equal(http.StatusSeeOther, w.Code)
	assert.Equal("/admin/settings", w.Header().Get("Location"))
	cookies := w.Result().Cookies()
	require.Len(cookies, 1)

	target = "http://status.example.com/auth/session"
	r := httptest.NewRequest(http.MethodGet, target, nil)
	r.AddCookie(cookies[0])
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Contains(w.Body.String(), `"displayName":"Test Admin"`)

	target = "http://status.example.com/auth/logout"
	r = httptest.NewRequest(http.MethodPost, target, strings.NewReader("{}"))
	r.Header.Set("Origin", "http://status.example.com")
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(cookies[0])
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(http.StatusSeeOther, w.Code)

	target = "http://status.example.com/auth/session"
	r = httptest.NewRequest(http.MethodGet, target, nil)
	r.AddCookie(cookies[0])
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	body := w.Body.String()
	assert.Contains(body, `"user":null`, "the session is gone after logout")
}
