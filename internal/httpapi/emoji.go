package httpapi

import (
	"github.com/DengNaichen/inftask/internal/app"
	"net/http"
	"strconv"
)

func projectReference(r *http.Request) (app.ProjectRef, error) {
	ref := app.ProjectRef{Name: r.PathValue("name")}
	if r.PathValue("id") != "" {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			return ref, app.ErrProjectReference
		}
		ref.ID = id
		ref.Name = r.URL.Query().Get("name")
	}
	return ref, nil
}
func (s *server) getProject(w http.ResponseWriter, r *http.Request) {
	ref, err := projectReference(r)
	if err != nil {
		writeError(w, err)
		return
	}
	p, err := s.service.GetProject(r.Context(), ref)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}
func (s *server) createProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Teams       []string `json:"teams"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	p, err := s.service.CreateProject(r.Context(), body.Name, body.Description, body.Teams...)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}
func (s *server) updateProject(w http.ResponseWriter, r *http.Request) {
	var body app.UpdateProject
	if err := decodeJSON(w, r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	ref, err := projectReference(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if (body.ID != 0 && ref.ID != 0 && body.ID != ref.ID) || (body.Name != "" && ref.Name != "" && body.Name != ref.Name) {
		writeError(w, app.ErrProjectReference)
		return
	}
	if ref.ID != 0 {
		body.ID = ref.ID
	}
	if ref.Name != "" {
		body.Name = ref.Name
	}
	p, err := s.service.UpdateProject(r.Context(), body)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
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
