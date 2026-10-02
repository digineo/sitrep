// Package httpx holds HTTP helpers shared by the server and the auth
// providers.
package httpx

import (
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
)

// Info describes a request as the client sent it, taking trusted proxy
// headers into account.
type Info struct {
	Host   string // lowercase, without port and trailing dot
	Origin string // scheme://host[:port], without default port
	Scheme string // http or https
	IP     string // client address
}

// Effective returns the request's effective host, origin, scheme and client
// address. With trustProxy, X-Forwarded-Host, X-Forwarded-Proto and
// X-Forwarded-For are honored.
func Effective(r *http.Request, trustProxy bool) Info {
	raw := r.Host
	scheme := "http"
	ip := r.RemoteAddr
	if h, _, err := net.SplitHostPort(ip); err == nil {
		ip = h
	}

	if r.TLS != nil {
		scheme = "https"
	}

	if trustProxy {
		first, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Host"), ",")
		if first = strings.TrimSpace(first); validHostPort(first) {
			raw = first
		}

		if r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}

		if xff := r.Header.Values("X-Forwarded-For"); len(xff) > 0 {
			last := xff[len(xff)-1]
			addr := strings.TrimSpace(last[strings.LastIndex(last, ",")+1:])
			if a, err := netip.ParseAddr(addr); err == nil {
				ip = a.String()
			}
		}
	}

	host, port := splitHostPort(strings.ToLower(raw))
	host = strings.TrimSuffix(host, ".")
	origin := scheme + "://" + host
	if strings.Contains(host, ":") {
		origin = scheme + "://[" + host + "]"
	}

	defaultPort := (scheme == "http" && port == "80") ||
		(scheme == "https" && port == "443")
	if port != "" && !defaultPort {
		origin += ":" + port
	}
	return Info{
		Host:   host,
		Origin: origin,
		Scheme: scheme,
		IP:     ip,
	}
}

func splitHostPort(s string) (string, string) {
	if h, p, err := net.SplitHostPort(s); err == nil {
		return h, p
	}
	return strings.Trim(s, "[]"), ""
}

// validHostPort reports whether s is a DNS name or IP address, optionally
// followed by a port.
func validHostPort(s string) bool {
	host, port := splitHostPort(s)
	if port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return false
		}
	}

	if _, err := netip.ParseAddr(host); err == nil {
		return true
	}

	host = strings.TrimSuffix(host, ".")
	if host == "" || len(host) > 253 {
		return false
	}

	for label := range strings.SplitSeq(strings.ToLower(host), ".") {
		if label == "" || len(label) > 63 ||
			label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}

		for _, c := range label {
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
				return false
			}
		}
	}
	return true
}
