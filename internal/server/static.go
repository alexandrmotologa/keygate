package server

import (
	"net/http"
	"strings"

	"github.com/alexandrmotologa/keygate/web"
)

// RegisterStaticFiles mounts the embedded React 19 SPA onto the given ServeMux.
func RegisterStaticFiles(mux *http.ServeMux) error {
	subFS, err := web.GetSubFS()
	if err != nil {
		return err
	}

	fileServer := http.FileServer(http.FS(subFS))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Do not handle API routes or healthcheck here
		if strings.HasPrefix(r.URL.Path, "/v1/") || r.URL.Path == "/healthz" {
			http.NotFound(w, r)
			return
		}

		filePath := strings.TrimPrefix(r.URL.Path, "/")
		if filePath == "" {
			filePath = "index.html"
		}

		// Check if file exists in embedded filesystem
		file, err := subFS.Open(filePath)
		if err != nil {
			// SPA routing fallback: rewrite to index.html
			r.URL.Path = "/"
		} else {
			_ = file.Close()
			if strings.HasPrefix(filePath, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
		}

		fileServer.ServeHTTP(w, r)
	})

	return nil
}
