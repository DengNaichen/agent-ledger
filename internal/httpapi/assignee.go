package httpapi

import (
	"github.com/DengNaichen/inftask/internal/app"
	"net/http"
)

func (s *server) listAssignees(w http.ResponseWriter, r *http.Request) {
	members, err := s.service.ListAssignees(r.Context(), app.ListAssignees{Team: r.PathValue("key")})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"assignees": members})
}
