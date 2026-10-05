package httpx

import (
	"context"
	"crypto/rand"
	"log/slog"
	"net/http"
	"time"

	"github.com/digineo/xlog"
)

type requestIDKey struct{}

// RequestID returns the ID that AccessLog assigned to the request.
func RequestID(r *http.Request) string {
	id, _ := r.Context().Value(requestIDKey{}).(string)
	return id
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}

// AccessLog assigns each request an ID, sends it as X-Request-Id, and logs
// one line per request, at debug level for successful health checks. The
// line never contains the client address or the query string.
func AccessLog(log xlog.Logger, proxies Proxies, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := rand.Text()
		w.Header().Set("X-Request-Id", id)
		rec := &statusRecorder{
			ResponseWriter: w,
			status:         http.StatusOK,
		}
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(rec, r.WithContext(ctx))
		logf := log.Info
		if r.URL.Path == "/healthz" && rec.status == http.StatusOK {
			logf = log.Debug
		}
		logf("request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("host", Effective(r, proxies).Host),
			slog.Int("status", rec.status),
			slog.Duration("duration", time.Since(start)),
			slog.String("request_id", id))
	})
}
