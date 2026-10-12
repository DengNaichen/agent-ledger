package httpapi

import (
	"github.com/DengNaichen/inftask/internal/app"
	"net/http"
)

func (s *server) accessRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/w/{workspace}/access", s.in((*server).access))
	mux.HandleFunc("GET /api/w/{workspace}/members", s.in((*server).members))
	mux.HandleFunc("PATCH /api/w/{workspace}/members", s.in((*server).updateMember))
	mux.HandleFunc("POST /api/w/{workspace}/invite", s.in((*server).resetInvite))
	mux.HandleFunc("POST /api/join", s.joinWorkspace)
}
func (s *server) access(w http.ResponseWriter, r *http.Request) {
	result, err := s.service.Access(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s *server) members(w http.ResponseWriter, r *http.Request) {
	result, err := s.service.ListMembers(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": result})
}
func (s *server) updateMember(w http.ResponseWriter, r *http.Request) {
	var input app.UpdateMember
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if err := s.service.UpdateMember(r.Context(), input); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"updated": true})
}
func (s *server) resetInvite(w http.ResponseWriter, r *http.Request) {
	token, err := s.service.ResetInvite(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}
func (s *server) joinWorkspace(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	workspace, err := s.service.JoinWorkspace(r.Context(), input.Token)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, workspace)
}
