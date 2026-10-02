//go:build dev

package server

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// viteServer is the Vite dev server started by make dev.
var viteServer = &url.URL{
	Scheme: "http",
	Host:   "localhost:5173",
}

// assets proxies /assets/ to the Vite dev server, including its hot module
// replacement websocket, so the developer browses a single URL.
type assets struct {
	proxy *httputil.ReverseProxy
}

func newAssets() (*assets, error) {
	return &assets{proxy: &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) { r.SetURL(viteServer) },
	}}, nil
}

func (a *assets) entry(name string) shellAssets {
	return shellAssets{
		Scripts: []string{
			"/assets/@vite/client",
			"/assets/src/" + name + "/main.ts",
		},
	}
}

func (a *assets) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.proxy.ServeHTTP(w, r)
}
