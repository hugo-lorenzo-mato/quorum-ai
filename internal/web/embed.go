package web

import (
	"embed"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
)

//go:embed all:dist
var distFS embed.FS

// DistFS returns the embedded frontend filesystem rooted at dist/.
// Returns an error if the dist directory doesn't exist (frontend not built).
func DistFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}

// StaticHandler returns an http.Handler that serves the embedded frontend files.
// It implements SPA (Single Page Application) routing by serving index.html
// for any path that doesn't match a static file.
func StaticHandler() (http.Handler, error) {
	distSubFS, err := DistFS()
	if err != nil {
		return nil, err
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" || path == "" {
			path = "index.html"
		} else {
			// Remove leading slash
			path = path[1:]
		}

		// Try to serve the requested file
		if serveFile(w, distSubFS, path) {
			return
		}

		// For SPA routing: serve index.html for non-existent paths
		serveFile(w, distSubFS, "index.html")
	}), nil
}

// serveFile serves a file from the filesystem and returns true if successful.
func serveFile(w http.ResponseWriter, fsys fs.FS, path string) bool {
	f, err := fsys.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil || stat.IsDir() {
		return false
	}

	// Set content type based on extension
	ext := filepath.Ext(path)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)

	// Copy content to response
	_, _ = io.Copy(w, f)
	return true
}
