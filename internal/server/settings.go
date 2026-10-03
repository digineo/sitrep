package server

import (
	"net/http"

	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/model"
)

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.db.Settings()
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}
	settings.Languages = settings.Languages.Effective()
	httpx.WriteJSON(w, http.StatusOK, settings)
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var settings model.Settings
	if err := httpx.ReadJSON(w, r, &settings); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	settings.Normalize()
	if err := settings.Validate(); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	if err := s.db.PutSettings(settings); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	s.poller.SettingsChanged()
	httpx.WriteJSON(w, http.StatusOK, settings)
}
