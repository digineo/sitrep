package server

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/model"
)

// logoFile returns the file name of a site's logo, which changes with its
// content, or "" if the site has none.
func logoFile(site *model.Site) string {
	if site.Logo == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(site.Logo))
	return hex.EncodeToString(sum[:8]) + ".svg"
}

// logoURL returns the URL of a site's logo, or "" if it has none.
func logoURL(site *model.Site) string {
	if f := logoFile(site); f != "" {
		return "/api/public/sites/" + site.ID + "/logo/" + f
	}
	return ""
}

// publicLogo answers a site's logo in every availability state. Its URL
// changes with the logo, so it is cached for a year.
func (s *Server) publicLogo(w http.ResponseWriter, r *http.Request) {
	site, err := s.publicSiteOf(r)
	if err == nil && (site.Logo == "" || r.PathValue("file") != logoFile(site)) {
		err = apierr.New(http.StatusNotFound, apierr.NotFound)
	}
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	csp := "default-src 'none'; style-src 'unsafe-inline'; sandbox"
	h := w.Header()
	h.Set("Content-Type", "image/svg+xml")
	h.Set("Content-Security-Policy", csp)
	h.Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write([]byte(site.Logo))
}

// sanitizeSVG answers an uploaded logo sanitized, so that the console can
// show what will be saved.
func (s *Server) sanitizeSVG(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SVG string `json:"svg"`
	}
	if err := httpx.ReadJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	logo, code := model.SanitizeLogo(req.SVG)
	if code != "" {
		err := apierr.Fields{{
			Path: "svg",
			Code: code,
		}}.Err()
		httpx.WriteError(w, r, s.log, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{"svg": logo})
}
