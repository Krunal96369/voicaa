package api

import "time"

// --- Requests ---

// PullRequest is the JSON body for POST /api/v1/pull.
type PullRequest struct {
	Model      string `json:"model"`
	Token      string `json:"token,omitempty"`
	SkipDocker bool   `json:"skip_docker,omitempty"`
	Force      bool   `json:"force,omitempty"`
}

// ServeRequest is the JSON body for POST /api/v1/serve and POST /api/v1/run.
type ServeRequest struct {
	Model      string `json:"model"`
	Port       int    `json:"port,omitempty"`
	Voice      string `json:"voice,omitempty"`
	Prompt     string `json:"prompt,omitempty"`
	CpuOffload bool   `json:"cpu_offload,omitempty"`
	Device     string `json:"device,omitempty"`
	Name       string `json:"name,omitempty"`
	GPUIDs     string `json:"gpu_ids,omitempty"`
	Token      string `json:"token,omitempty"` // only used by /run
}

// StopRequest is the JSON body for POST /api/v1/stop.
type StopRequest struct {
	Model   string `json:"model"`
	Force   bool   `json:"force,omitempty"`
	Timeout int    `json:"timeout,omitempty"`
}

// --- Responses ---

// ModelInfo describes a locally downloaded model.
type ModelInfo struct {
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Size        int64     `json:"size"`
	Complete    bool      `json:"complete"`
	PulledAt    time.Time `json:"pulled_at,omitempty"`
}

// RegistryModelInfo describes a model available in the registry.
type RegistryModelInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	License     string `json:"license"`
	Version     string `json:"version"`
	Gated       bool   `json:"gated"`
}

// InstanceInfo describes a running model instance.
type InstanceInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Model     string    `json:"model"`
	Port      int       `json:"port"`
	Voice     string    `json:"voice,omitempty"`
	Prompt    string    `json:"prompt,omitempty"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at,omitempty"`
	WSURL     string    `json:"ws_url"`
}

// VoiceInfo describes an available voice for a model.
type VoiceInfo struct {
	Name     string `json:"name"`
	Gender   string `json:"gender"`
	Category string `json:"category"`
	File     string `json:"file"`
	Default  bool   `json:"default,omitempty"`
}

// ErrorResponse is the standard error body.
type ErrorResponse struct {
	Error string `json:"error"`
}

// VersionResponse is returned by GET /api/v1/version.
type VersionResponse struct {
	Version string `json:"version"`
}

// HealthResponse is returned by GET /api/v1/health.
type HealthResponse struct {
	Status string `json:"status"`
}

// TranscriptResponse is returned by GET /api/v1/transcript/{model}.
type TranscriptResponse struct {
	Model  string            `json:"model"`
	Text   string            `json:"text"`
	Tokens []TranscriptToken `json:"tokens"`
}
