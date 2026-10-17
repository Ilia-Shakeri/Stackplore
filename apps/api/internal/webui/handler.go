package webui

import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

func Handler(assets fs.FS) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			http.NotFound(writer, request)
			return
		}
		if assets == nil || request.URL == nil || strings.HasPrefix(request.URL.Path, "/api/") ||
			request.URL.Path == "/api" || request.URL.Path == "/healthz" || request.URL.Path == "/readyz" ||
			request.URL.Path == "/metrics" {
			http.NotFound(writer, request)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+request.URL.Path), "/")
		if name == "." || name == "" {
			name = "index.html"
		}
		if name == "embed.marker" {
			http.NotFound(writer, request)
			return
		}
		if file, err := fs.Stat(assets, name); err == nil && file.Mode().IsRegular() {
			serveAsset(writer, request, assets, name)
			return
		}
		if path.Ext(name) != "" || !strings.Contains(request.Header.Get("Accept"), "text/html") {
			http.NotFound(writer, request)
			return
		}
		serveAsset(writer, request, assets, "index.html")
	}
}

func serveAsset(writer http.ResponseWriter, request *http.Request, assets fs.FS, name string) {
	if name == "index.html" {
		writer.Header().Set("Cache-Control", "no-store")
	} else if strings.HasPrefix(name, "assets/") {
		writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		writer.Header().Set("Content-Type", contentType)
	}
	http.ServeFileFS(writer, request, assets, name)
}
