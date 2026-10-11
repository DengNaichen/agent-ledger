package httpapi

import (
	"net/http"
	"strconv"

	"github.com/DengNaichen/inftask/internal/app"
)

func (s *server) labelRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/w/{workspace}/labels", s.in(func(s *server, w http.ResponseWriter, r *http.Request) {
		labels, err := s.service.ListLabels(r.Context(), r.URL.Query().Get("include_archived") == "true")
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"labels": labels})
	}))
	mux.HandleFunc("GET /api/w/{workspace}/label-groups", s.in(func(s *server, w http.ResponseWriter, r *http.Request) {
		groups, err := s.service.ListLabelGroups(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"groups": groups})
	}))
	mux.HandleFunc("POST /api/w/{workspace}/label-groups", s.in(func(s *server, w http.ResponseWriter, r *http.Request) {
		var input app.SaveLabelGroup
		if !labelInput(w, r, &input) {
			return
		}
		group, err := s.service.SaveLabelGroup(r.Context(), input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, group)
	}))
	mux.HandleFunc("POST /api/w/{workspace}/labels", s.in(func(s *server, w http.ResponseWriter, r *http.Request) {
		var input app.CreateLabel
		if !labelInput(w, r, &input) {
			return
		}
		label, err := s.service.CreateLabel(r.Context(), input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, label)
	}))
	mux.HandleFunc("PATCH /api/w/{workspace}/labels/{id}", s.in(func(s *server, w http.ResponseWriter, r *http.Request) {
		var input app.UpdateLabel
		if !labelInput(w, r, &input) {
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, app.ErrInvalidLabel)
			return
		}
		input.ID = id
		label, err := s.service.UpdateLabel(r.Context(), input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, label)
	}))
	mux.HandleFunc("POST /api/w/{workspace}/labels/merge", s.in(func(s *server, w http.ResponseWriter, r *http.Request) {
		var input app.MergeLabels
		if !labelInput(w, r, &input) {
			return
		}
		if err := s.service.MergeLabels(r.Context(), input); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, struct{}{})
	}))
	mux.HandleFunc("POST /api/w/{workspace}/issue-labels", s.in(func(s *server, w http.ResponseWriter, r *http.Request) {
		var input app.ChangeIssueLabels
		if !labelInput(w, r, &input) {
			return
		}
		if err := s.service.ChangeIssueLabels(r.Context(), input); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, struct{}{})
	}))
}
func labelInput(w http.ResponseWriter, r *http.Request, input any) bool {
	if err := decodeJSON(w, r, input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return false
	}
	return true
}
func labelQuery(r *http.Request, input *app.ListIssues) error {
	input.LabelMode = r.URL.Query().Get("label_mode")
	for _, field := range []struct {
		key    string
		target *[]int64
	}{{"label_id", &input.LabelIDs}, {"label_group_id", &input.LabelGroupIDs}} {
		for _, value := range r.URL.Query()[field.key] {
			id, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return app.ErrInvalidLabel
			}
			*field.target = append(*field.target, id)
		}
	}
	return nil
}
