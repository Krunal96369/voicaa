package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Krunal96369/voicaa/internal/service"
)

func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	var req PullRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Model == "" {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}

	// Stream progress as SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	err := s.modelService.Pull(r.Context(), req.Model, req.Token, req.SkipDocker, req.Force, func(p service.PullProgress) {
		data, _ := json.Marshal(p)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	})

	if err != nil {
		data, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	fmt.Fprintf(w, "data: {\"done\":true}\n\n")
	flusher.Flush()
}

func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	models, err := s.modelService.ListLocal()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var result []ModelInfo
	for _, m := range models {
		info := ModelInfo{
			Name:     m.Name,
			Size:     m.TotalSizeBytes,
			Complete: m.Complete,
			PulledAt: m.DownloadedAt,
		}
		if m.Manifest != nil {
			info.Description = m.Manifest.Description
		}
		result = append(result, info)
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleListRegistry(w http.ResponseWriter, r *http.Request) {
	models, err := s.modelService.ListRegistry()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var result []RegistryModelInfo
	for _, m := range models {
		result = append(result, RegistryModelInfo{
			Name:        m.Name,
			Description: m.Description,
			License:     m.License,
			Version:     m.Version,
			Gated:       m.HuggingFace.Gated,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleVoices(w http.ResponseWriter, r *http.Request) {
	modelName := r.PathValue("name")
	if modelName == "" {
		writeError(w, http.StatusBadRequest, "model name is required")
		return
	}

	manifest, err := s.modelService.Voices(modelName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var result []VoiceInfo
	for _, v := range manifest.Voices.Voices {
		result = append(result, VoiceInfo{
			Name:     v.Name,
			Gender:   v.Gender,
			Category: v.Category,
			File:     v.File,
			Default:  v.Name == manifest.Voices.DefaultVoice,
		})
	}

	writeJSON(w, http.StatusOK, result)
}
