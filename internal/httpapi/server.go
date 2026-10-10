package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/DengNaichen/agent-ledger/internal/app"
)

type server struct{ service *app.Service }

// New serves the REST API, the MCP endpoint, and the web UI. Browsers may only
// reach them from the same origin; MCP clients send no Origin and pass through.
func New(service *app.Service, web, mcp http.Handler) http.Handler {
	s := &server{service: service}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/issues", s.listIssues)
	mux.HandleFunc("POST /api/issues", s.createIssue)
	mux.HandleFunc("PATCH /api/issues/{identifier}/status", s.moveIssue)
	for _, method := range []string{"GET", "POST", "DELETE"} {
		mux.Handle(method+" /mcp", mcp)
	}
	mux.Handle("GET /", web)
	return http.NewCrossOriginProtection().Handler(mux)
}

func (s *server) listIssues(w http.ResponseWriter, r *http.Request) {
	issues, err := s.service.List(r.Context(), app.Status(r.URL.Query().Get("status")))
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
	issue, err := s.service.Move(r.Context(), r.PathValue("identifier"), input.Status)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, issue)
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
	case errors.Is(err, app.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, app.ErrInvalidTitle), errors.Is(err, app.ErrInvalidStatus):
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
