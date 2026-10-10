package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/DengNaichen/inftask/internal/app"
)

type server struct{ service *app.Service }

// New serves the REST API, the MCP endpoint, and the web UI. Browsers may only
// reach them from the same origin; MCP clients send no Origin and pass through.
func New(service *app.Service, web, mcp http.Handler) http.Handler {
	s := &server{service: service}
	mux := http.NewServeMux()
	s.labelRoutes(mux)
	mux.HandleFunc("GET /auth/session", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
	})
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/issues", s.listIssues)
	mux.HandleFunc("POST /api/issues", s.createIssue)
	mux.HandleFunc("PATCH /api/issues/{identifier}/status", s.moveIssue)
	mux.HandleFunc("PATCH /api/issues/{identifier}", s.updateIssue)
	mux.HandleFunc("GET /api/issues/{identifier}", s.getIssue)
	mux.HandleFunc("POST /api/issues/{identifier}/comments", s.addComment)
	mux.HandleFunc("GET /api/projects", s.listProjects)
	for _, method := range []string{"GET", "POST", "DELETE"} {
		mux.Handle(method+" /mcp", mcp)
	}
	mux.Handle("GET /", web)
	return http.NewCrossOriginProtection().Handler(mux)
}

func (s *server) listIssues(w http.ResponseWriter, r *http.Request) {
	status := app.Status(r.URL.Query().Get("status"))
	issueType := r.URL.Query().Get("type")
	limit := -1
	input := app.ListIssues{Project: r.URL.Query().Get("project"), Query: r.URL.Query().Get("query"), Limit: &limit}
	if r.URL.Query().Has("status") {
		input.Status = &status
	}
	if r.URL.Query().Has("type") {
		input.Type = &issueType
	}
	if err := labelQuery(r, &input); err != nil {
		writeError(w, err)
		return
	}
	issues, err := s.service.List(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"issues": issues})
}

func (s *server) createIssue(w http.ResponseWriter, r *http.Request) {
	var input app.CreateIssue
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	issue, err := s.service.Create(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, issue)
}

func (s *server) moveIssue(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Status app.Status `json:"status"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	issue, err := s.service.Update(r.Context(), app.UpdateIssue{Identifier: r.PathValue("identifier"), Status: &input.Status})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, issue)
}

func (s *server) updateIssue(w http.ResponseWriter, r *http.Request) {
	var input app.UpdateIssue
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	input.Identifier = r.PathValue("identifier")
	issue, err := s.service.Update(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, issue)
}

func (s *server) getIssue(w http.ResponseWriter, r *http.Request) {
	issue, err := s.service.Details(r.Context(), r.PathValue("identifier"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, issue)
}

func (s *server) addComment(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	comment, err := s.service.AddComment(r.Context(), r.PathValue("identifier"), input.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, comment)
}

func (s *server) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.service.ListProjects(r.Context(), false)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, app.ErrLabelNotFound), errors.Is(err, app.ErrNotFound), errors.Is(err, app.ErrProjectNotFound):
		status = http.StatusNotFound
	case errors.Is(err, app.ErrInvalidLabel), errors.Is(err, app.ErrInvalidTitle), errors.Is(err, app.ErrInvalidStatus), errors.Is(err, app.ErrInvalidType):
		status = http.StatusBadRequest
	default:
		if _, ok := err.(*json.SyntaxError); ok {
			status = http.StatusBadRequest
		}
	}
	message := err.Error()
	if status == http.StatusInternalServerError {
		message = "internal server error"
	}
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
