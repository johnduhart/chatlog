package health

import (
	"context"
	"log"
	"net/http"
)

// Server provides HTTP health check endpoint
type Server struct {
	server *http.Server
}

// New creates a new health check server
func New(addr string) *Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	return &Server{
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}
}

// Start begins serving HTTP requests
func (s *Server) Start() error {
	log.Printf("Health check server listening on %s", s.server.Addr)
	if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down health check server...")
	return s.server.Shutdown(ctx)
}
