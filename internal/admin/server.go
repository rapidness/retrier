package admin

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
)

//go:embed all:dist
var webFS embed.FS

// Server is the admin web UI + API server.
type Server struct {
	addr    string
	handler http.Handler
}

// NewServer creates an admin server.
func NewServer(handlers *Handlers, addr string) *Server {
	mux := http.NewServeMux()

	// Register REST API routes
	handlers.RegisterRoutes(mux)

	// Serve embedded frontend
	distFS, err := fs.Sub(webFS, "dist")
	if err != nil {
		log.Printf("[admin] warning: frontend dist/ not found, UI disabled: %v", err)
	} else {
		fileServer := http.FileServer(http.FS(distFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Try serving static file first, then fallback to index.html (SPA)
			path := r.URL.Path
			if path != "/" && fileExists(distFS, path) {
				fileServer.ServeHTTP(w, r)
				return
			}
			// SPA fallback: serve index.html for all routes
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
		})
	}

	return &Server{
		addr:    addr,
		handler: mux,
	}
}

// Start starts the admin server (blocking).
func (s *Server) Start() error {
	log.Printf("[admin] listening on %s", s.addr)
	return http.ListenAndServe(s.addr, s.handler)
}

// fileExists checks if a file exists in the embedded FS.
func fileExists(fsys fs.FS, path string) bool {
	// Remove leading slash
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	_, err := fs.Stat(fsys, path)
	return err == nil
}
