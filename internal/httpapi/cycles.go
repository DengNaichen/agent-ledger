package httpapi

import (
	"github.com/DengNaichen/inftask/internal/app"
	"net/http"
	"strconv"
)

func (s *server) cycleRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/w/{workspace}/teams/{key}/cycles", s.in((*server).listCycles))
	mux.HandleFunc("PUT /api/w/{workspace}/teams/{key}/cycles", s.in((*server).configureCycles))
	mux.HandleFunc("GET /api/w/{workspace}/cycles/{id}", s.in((*server).getCycle))
	mux.HandleFunc("PATCH /api/w/{workspace}/cycles/{id}", s.in((*server).updateCycle))
}
func (s *server) listCycles(w http.ResponseWriter, r *http.Request) {
	v, err := s.service.ListCycles(r.Context(), r.PathValue("key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, v)
}
func (s *server) configureCycles(w http.ResponseWriter, r *http.Request) {
	var in app.CycleSettings
	if err := decodeJSON(w, r, &in); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	v, err := s.service.ConfigureCycles(r.Context(), app.ConfigureCycles{Team: r.PathValue("key"), CycleSettings: in})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, v)
}
func (s *server) getCycle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, app.ErrCycleReference)
		return
	}
	v, err := s.service.GetCycle(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, v)
}
func (s *server) updateCycle(w http.ResponseWriter, r *http.Request) {
	var in app.UpdateCycle
	if err := decodeJSON(w, r, &in); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, app.ErrCycleReference)
		return
	}
	in.ID = id
	v, err := s.service.UpdateCycle(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, v)
}
