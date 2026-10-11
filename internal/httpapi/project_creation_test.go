package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProjectCreationValidationHTTP(t *testing.T) {
	root, _ := openService(t)
	h := New(root, http.NotFoundHandler(), http.NotFoundHandler())
	for _, tc := range []struct {
		body   string
		status int
	}{{`{"name":" ","teams":["AL"]}`, 400}, {`{"name":" Release ","teams":["AL"]}`, 201}, {`{"name":"Release","teams":["AL"]}`, 409}} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest("POST", "/api/w/default/projects", strings.NewReader(tc.body)))
		if r.Code != tc.status {
			t.Fatalf("%s: %d %s", tc.body, r.Code, r.Body.String())
		}
	}
}
