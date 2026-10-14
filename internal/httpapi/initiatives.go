package httpapi

import (
	"github.com/DengNaichen/inftask/internal/app"
	"net/http"
	"strconv"
)

func (s *server) listInitiatives(w http.ResponseWriter, r *http.Request) {
	items, err := s.service.ListInitiatives(r.Context(), r.URL.Query().Get("include_archived") == "true")
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"initiatives": items})
}
func (s *server) getInitiative(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, app.ErrInitiativeNotFound)
		return
	}
	item, err := s.service.GetInitiative(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (s *server) createInitiative(w http.ResponseWriter, r *http.Request) {
	var input app.CreateInitiative
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	item, err := s.service.CreateInitiative(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (s *server) updateInitiative(w http.ResponseWriter, r *http.Request) {
	var input app.UpdateInitiative
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || (input.ID != 0 && input.ID != id) {
		writeError(w, app.ErrInitiativeNotFound)
		return
	}
	input.ID = id
	item, err := s.service.UpdateInitiative(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
