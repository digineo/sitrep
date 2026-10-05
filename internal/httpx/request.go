// Package httpx holds HTTP helpers shared by the server and the auth
// providers.
package httpx

import (
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strconv"
	"strings"
)

// Proxies are the peers whose X-Forwarded-* headers are trusted.
type Proxies struct {
	// All trusts every peer, and only the last X-Forwarded-For entry, for
	// the deprecated SITREP_TRUST_PROXY=true.
	All      bool
	Prefixes []netip.Prefix
}

func (p Proxies) trusts(a netip.Addr) bool {
	a = a.Unmap()
	return slices.ContainsFunc(p.Prefixes, func(n netip.Prefix) bool {
		return n.Contains(a)
	})
}

// client returns the client address of X-Forwarded-For entries: the last
// one that is not a trusted proxy, or, with All, the last one. An invalid
// entry ends the search, and peer is the fallback.
func (p Proxies) client(xff []string, peer string) string {
	entries := strings.Split(strings.Join(xff, ","), ",")
	for _, e := range slices.Backward(entries) {
		a, err := netip.ParseAddr(strings.TrimSpace(e))
		if err != nil {
			break
		}
		peer = a.String()
		if p.All || !p.trusts(a) {
			break
		}
	}
	return peer
}

// Info describes a request as the client sent it, taking trusted proxy
// headers into account.
type Info struct {
	Host   string // lowercase, without port and trailing dot
	Origin string // scheme://host[:port], without default port
	Scheme string // http or https
	IP     string // client address
}

// Effective returns the request's effective host, origin, scheme and client
// address. Requests from trusted proxies have their X-Forwarded-Host,
// X-Forwarded-Proto and X-Forwarded-For honored.
func Effective(r *http.Request, proxies Proxies) Info {
	raw := r.Host
	scheme := "http"
	ip := r.RemoteAddr
	if h, _, err := net.SplitHostPort(ip); err == nil {
		ip = h
	}

	if r.TLS != nil {
		scheme = "https"
	}

	if peer, _ := netip.ParseAddr(ip); proxies.All || proxies.trusts(peer) {
		first, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Host"), ",")
		if first = strings.TrimSpace(first); validHostPort(first) {
			raw = first
		}

		if r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}

		ip = proxies.client(r.Header.Values("X-Forwarded-For"), ip)
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

// NormalizeHost returns s lowercased, without port and one trailing dot.
// It reports false if s is not a DNS name or IP address with an optional
// port.
func NormalizeHost(s string) (string, bool) {
	if !validHostPort(s) {
		return "", false
	}
	host, _ := splitHostPort(strings.ToLower(s))
	return strings.TrimSuffix(host, "."), true
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
