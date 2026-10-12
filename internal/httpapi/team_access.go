package httpapi

import (
	"github.com/DengNaichen/inftask/internal/app"
	"net/http"
)

func (s *server) teamAccessRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/w/{workspace}/team-directory", s.in((*server).teamDirectory))
	mux.HandleFunc("GET /api/w/{workspace}/teams/{key}/members", s.in((*server).teamMembers))
	mux.HandleFunc("PATCH /api/w/{workspace}/teams/{key}/members", s.in((*server).changeTeamMember))
	mux.HandleFunc("POST /api/w/{workspace}/teams/{key}/membership", s.in((*server).joinTeam))
}
func (s *server) teamDirectory(w http.ResponseWriter, r *http.Request) {
	teams, err := s.service.TeamDirectory(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"teams": teams})
}
func (s *server) teamMembers(w http.ResponseWriter, r *http.Request) {
	members, err := s.service.ListTeamMembers(r.Context(), r.PathValue("key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"members": members})
}
func (s *server) changeTeamMember(w http.ResponseWriter, r *http.Request) {
	var input app.ChangeTeamMember
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	input.Key = r.PathValue("key")
	if err := s.service.ChangeTeamMember(r.Context(), input); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"updated": true})
}
func (s *server) joinTeam(w http.ResponseWriter, r *http.Request) {
	var input app.JoinTeam
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	input.Key = r.PathValue("key")
	team, err := s.service.JoinTeam(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, team)
}
