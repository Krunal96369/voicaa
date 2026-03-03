package api

import (
	"encoding/json"
	"net/http"

	"github.com/Krunal96369/voicaa/internal/backend"
	"github.com/Krunal96369/voicaa/internal/service"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, VersionResponse{Version: s.version})
}

func (s *Server) handleServe(w http.ResponseWriter, r *http.Request) {
	var req ServeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Model == "" {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}

	opts := service.ServeOptions{
		ModelName:     req.Model,
		Port:          req.Port,
		Voice:         req.Voice,
		Prompt:        req.Prompt,
		CpuOffload:    req.CpuOffload,
		Device:        req.Device,
		ContainerName: req.Name,
		GPUIDs:        req.GPUIDs,
	}

	info, err := s.instanceService.Serve(r.Context(), opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toAPIInstanceInfo(info))
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	var req ServeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Model == "" {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}

	opts := service.ServeOptions{
		ModelName:     req.Model,
		Port:          req.Port,
		Voice:         req.Voice,
		Prompt:        req.Prompt,
		CpuOffload:    req.CpuOffload,
		Device:        req.Device,
		ContainerName: req.Name,
		GPUIDs:        req.GPUIDs,
	}

	info, err := s.instanceService.Run(r.Context(), opts, req.Token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toAPIInstanceInfo(info))
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	var req StopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Model == "" {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}

	timeout := req.Timeout
	if timeout == 0 {
		timeout = 10
	}

	if err := s.instanceService.Stop(r.Context(), req.Model, req.Force, timeout); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (s *Server) handlePs(w http.ResponseWriter, r *http.Request) {
	instances, err := s.instanceService.Ps(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var result []InstanceInfo
	for _, inst := range instances {
		result = append(result, InstanceInfo{
			ID:        string(inst.ID),
			Name:      inst.Name,
			Model:     inst.ModelName,
			Port:      inst.Port,
			Voice:     inst.Voice,
			Prompt:    inst.Prompt,
			Status:    inst.Status,
			StartedAt: inst.StartedAt,
			WSURL:     inst.WebSocketURL,
		})
	}

	if result == nil {
		result = []InstanceInfo{}
	}

	writeJSON(w, http.StatusOK, result)
}

// helpers

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}

func toAPIInstanceInfo(info *backend.InstanceInfo) InstanceInfo {
	return InstanceInfo{
		ID:     string(info.ID),
		Name:   info.Name,
		Model:  info.ModelName,
		Port:   info.Port,
		Voice:  info.Voice,
		Prompt: info.Prompt,
		Status: info.Status,
		WSURL:  info.WebSocketURL,
	}
}
