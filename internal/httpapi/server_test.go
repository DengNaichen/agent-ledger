package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRejectsCrossSiteBrowserRequests(t *testing.T) {
	reached := false
	mcp := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { reached = true })
	handler := New(nil, http.NotFoundHandler(), mcp)

	for _, tc := range []struct {
		site string
		want int
	}{
		{site: "cross-site", want: http.StatusForbidden},
		{site: "", want: http.StatusOK},
	} {
		reached = false
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader("{}"))
		if tc.site != "" {
			req.Header.Set("Sec-Fetch-Site", tc.site)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != tc.want || reached != (tc.want == http.StatusOK) {
			t.Fatalf("Sec-Fetch-Site %q: status %d, reached %v", tc.site, rec.Code, reached)
		}
	}
}
