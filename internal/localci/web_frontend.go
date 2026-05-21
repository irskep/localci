package localci

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

func serveFrontendApp(assetFS fs.FS) http.HandlerFunc {
	fileServer := http.FileServerFS(assetFS)

	return func(w http.ResponseWriter, r *http.Request) {
		cleanPath := path.Clean(r.URL.Path)
		trimmed := strings.TrimPrefix(cleanPath, "/")
		if trimmed != "" && trimmed != "." {
			if info, err := fs.Stat(assetFS, trimmed); err == nil && !info.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		indexData, err := fs.ReadFile(assetFS, "index.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexData)
	}
}
