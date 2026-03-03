package api

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/Krunal96369/voicaa/internal/service"
)

// Server is the voicaa daemon HTTP server.
type Server struct {
	modelService    *service.ModelService
	instanceService *service.InstanceService
	transcripts     *TranscriptStore
	httpServer      *http.Server
	version         string
}

// NewServer creates a new daemon server.
func NewServer(ms *service.ModelService, is *service.InstanceService, addr, version string) *Server {
	s := &Server{
		modelService:    ms,
		instanceService: is,
		transcripts:     NewTranscriptStore(),
		version:         version,
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // SSE and WebSocket need no write timeout
		IdleTimeout:  120 * time.Second,
	}

	return s
}

// Start begins listening. Blocks until the server is shut down.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.httpServer.Addr, err)
	}
	log.Printf("voicaa daemon listening on %s", s.httpServer.Addr)
	return s.httpServer.Serve(ln)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
