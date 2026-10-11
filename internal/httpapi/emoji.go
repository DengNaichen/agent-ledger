package httpapi

import (
	"github.com/DengNaichen/inftask/internal/app"
	"net/http"
)

func (s *server) updateProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Emoji *string `json:"emoji"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	project, err := s.service.UpdateProject(r.Context(), app.UpdateProject{Name: r.PathValue("name"), Emoji: body.Emoji})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (s *server) updateTeam(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Emoji *string `json:"emoji"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	team, err := s.service.UpdateTeam(r.Context(), app.UpdateTeam{Key: r.PathValue("key"), Emoji: body.Emoji})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, team)
}
