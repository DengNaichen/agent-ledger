package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/DengNaichen/inftask/internal/app"
)

type server struct{ service *app.Service }

// New serves the REST API, the MCP endpoint of each workspace, and the web UI.
// Browsers may only reach them from the same origin; MCP clients send no
// Origin and pass through.
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
	mux.HandleFunc("GET /api/workspaces", s.listWorkspaces)
	mux.HandleFunc("POST /api/workspaces", s.createWorkspace)
	mux.HandleFunc("PATCH /api/workspaces/{workspace}", s.renameWorkspace)
	mux.HandleFunc("GET /api/w/{workspace}/issues", s.in((*server).listIssues))
	mux.HandleFunc("POST /api/w/{workspace}/issues", s.in((*server).createIssue))
	mux.HandleFunc("PATCH /api/w/{workspace}/issues/{identifier}/status", s.in((*server).moveIssue))
	mux.HandleFunc("PATCH /api/w/{workspace}/issues/{identifier}", s.in((*server).updateIssue))
	mux.HandleFunc("GET /api/w/{workspace}/issues/{identifier}", s.in((*server).getIssue))
	mux.HandleFunc("POST /api/w/{workspace}/issues/{identifier}/comments", s.in((*server).addComment))
	mux.HandleFunc("GET /api/w/{workspace}/projects", s.in((*server).listProjects))
	mux.HandleFunc("PATCH /api/w/{workspace}/projects/{name}", s.in((*server).updateProject))
	mux.HandleFunc("PATCH /api/w/{workspace}/teams/{key}", s.in((*server).updateTeam))
	mux.HandleFunc("GET /api/w/{workspace}/teams", s.in((*server).listTeams))
	mux.HandleFunc("POST /api/w/{workspace}/teams", s.in((*server).createTeam))
	for _, method := range []string{"GET", "POST", "DELETE"} {
		mux.Handle(method+" /w/{workspace}/mcp", mcp)
	}
	mux.Handle("GET /", web)
	return http.NewCrossOriginProtection().Handler(mux)
}

// in serves a route of the workspace named in the path; handle reads and
// writes only that workspace. An unknown slug is not found.
func (s *server) in(handle func(*server, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspace, err := s.service.InWorkspace(r.Context(), r.PathValue("workspace"))
		if err != nil {
			writeError(w, err)
			return
		}
		handle(&server{service: workspace}, w, r)
	}
}

func (s *server) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces, err := s.service.ListWorkspaces(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": workspaces})
}

func (s *server) createWorkspace(w http.ResponseWriter, r *http.Request) {
	var input app.Workspace
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	workspace, err := s.service.CreateWorkspace(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, workspace)
}

// renameWorkspace changes only the name; the slug cannot change.
func (s *server) renameWorkspace(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	workspace, err := s.service.RenameWorkspace(r.Context(), r.PathValue("workspace"), input.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, workspace)
}

func (s *server) listIssues(w http.ResponseWriter, r *http.Request) {
	status := app.Status(r.URL.Query().Get("status"))
	issueType := r.URL.Query().Get("type")
	limit := -1
	input := app.ListIssues{Sort: r.URL.Query().Get("sort"), Team: r.URL.Query().Get("team"), Project: r.URL.Query().Get("project"), Query: r.URL.Query().Get("query"), Limit: &limit}
	if r.URL.Query().Has("priority") {
		priority, err := strconv.Atoi(r.URL.Query().Get("priority"))
		if err != nil {
			writeError(w, app.ErrInvalidPriority)
			return
		}
		input.Priority = &priority
	}
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
	writeJSON(w, http.StatusOK, map[string]any{"issues": app.Summaries(issues)})
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

func (s *server) listTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := s.service.ListTeams(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"teams": teams})
}

func (s *server) createTeam(w http.ResponseWriter, r *http.Request) {
	var input app.CreateTeam
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	team, err := s.service.CreateTeam(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, team)
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
	case errors.Is(err, app.ErrLabelNotFound), errors.Is(err, app.ErrNotFound), errors.Is(err, app.ErrProjectNotFound), errors.Is(err, app.ErrTeamNotFound), errors.Is(err, app.ErrWorkspaceNotFound):
		status = http.StatusNotFound
	case errors.Is(err, app.ErrInvalidEmoji), errors.Is(err, app.ErrInvalidPriority), errors.Is(err, app.ErrInvalidSort), errors.Is(err, app.ErrInvalidLabel), errors.Is(err, app.ErrInvalidTitle), errors.Is(err, app.ErrInvalidStatus), errors.Is(err, app.ErrInvalidType), errors.Is(err, app.ErrTeamRequired), errors.Is(err, app.ErrInvalidTeam), errors.Is(err, app.ErrInvalidWorkspace):
		status = http.StatusBadRequest
	case errors.Is(err, app.ErrTeamExists), errors.Is(err, app.ErrWorkspaceExists):
		status = http.StatusConflict
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
