package api

import "net/http"

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Web UI
	mux.HandleFunc("GET /{$}", s.handleUI)

	// Health and version
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("GET /api/v1/version", s.handleVersion)

	// Model management
	mux.HandleFunc("POST /api/v1/pull", s.handlePull)
	mux.HandleFunc("GET /api/v1/models", s.handleListModels)
	mux.HandleFunc("GET /api/v1/models/registry", s.handleListRegistry)
	mux.HandleFunc("GET /api/v1/models/{name}/voices", s.handleVoices)

	// Instance management
	mux.HandleFunc("POST /api/v1/serve", s.handleServe)
	mux.HandleFunc("POST /api/v1/run", s.handleRun)
	mux.HandleFunc("POST /api/v1/stop", s.handleStop)
	mux.HandleFunc("GET /api/v1/ps", s.handlePs)

	// WebSocket proxy
	mux.HandleFunc("GET /api/v1/ws/{model}", s.handleWebSocket)

	// Transcript
	mux.HandleFunc("GET /api/v1/transcript/{model}", s.handleTranscript)
}
