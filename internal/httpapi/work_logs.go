package httpapi

import (
	"net/http"
	"strconv"

	"github.com/DengNaichen/inftask/internal/app"
)

func (s *server) workLogRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/w/{workspace}/issues/{identifier}/work-logs", s.in((*server).listWorkLogs))
	mux.HandleFunc("GET /api/w/{workspace}/issues/{identifier}/work-logs/{id}", s.in((*server).getWorkLog))
}

func (s *server) listWorkLogs(w http.ResponseWriter, r *http.Request) {
	input := app.ListWorkLogs{Identifier: r.PathValue("identifier"), Kind: r.URL.Query().Get("kind")}
	for key, values := range r.URL.Query() {
		if len(values) != 1 || (key != "kind" && key != "limit" && key != "before") {
			writeError(w, app.ErrInvalidWorkLogQuery)
			return
		}
		if key == "kind" {
			continue
		}
		n, err := strconv.ParseInt(values[0], 10, 64)
		if err != nil {
			writeError(w, app.ErrInvalidWorkLogQuery)
			return
		}
		if key == "limit" {
			if n < 0 || n > 100 {
				writeError(w, app.ErrInvalidWorkLogQuery)
				return
			}
			input.Limit = int(n)
		} else {
			input.Before = n
		}
	}
	page, err := s.service.ListWorkLogs(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *server) getWorkLog(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, app.ErrNotFound)
		return
	}
	log, err := s.service.GetWorkLog(r.Context(), app.GetWorkLog{Identifier: r.PathValue("identifier"), ID: id})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, log)
}
