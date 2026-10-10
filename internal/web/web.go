package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// dist/.gitkeep is tracked so the pattern matches before the frontend is
// built. Vite writes into dist/app and empties only that directory.
//
//go:embed all:dist
var files embed.FS

func Handler() http.Handler {
	root, err := fs.Sub(files, "dist/app")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(root, path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		clone := r.Clone(r.Context())
		clone.URL.Path = "/"
		fileServer.ServeHTTP(w, clone)
	})
}
