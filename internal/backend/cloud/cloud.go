package cloud

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Krunal96369/voicaa/internal/backend"
	"github.com/Krunal96369/voicaa/internal/model"
)

// CloudBackend implements backend.Backend using a remote CloudProvider.
type CloudBackend struct {
	provider CloudProvider
	state    *StateStore
	hfToken  string
	gpuType  string
	diskGB   int
}

// CloudBackendConfig holds configuration for the cloud backend.
type CloudBackendConfig struct {
	Provider  CloudProvider
	StatePath string
	HFToken   string
	GPUType   string
	DiskGB    int
}

// New creates a new CloudBackend.
func New(cfg CloudBackendConfig) *CloudBackend {
	diskGB := cfg.DiskGB
	if diskGB == 0 {
		diskGB = 50
	}
	return &CloudBackend{
		provider: cfg.Provider,
		state:    NewStateStore(cfg.StatePath),
		hfToken:  cfg.HFToken,
		gpuType:  cfg.GPUType,
		diskGB:   diskGB,
	}
}

func (b *CloudBackend) Name() string {
	return "cloud:" + b.provider.Name()
}

// RequiresLocalModel returns false — cloud downloads weights inside the container.
func (b *CloudBackend) RequiresLocalModel() bool {
	return false
}

// BuildServerCmd builds the server command for cloud execution.
// Unlike Docker, model paths assume HuggingFace download inside the container.
func (b *CloudBackend) BuildServerCmd(manifest *model.ModelManifest, port int, voice, prompt string, cpuOffload bool, device string) []string {
	var cmd []string
	for _, part := range manifest.Entrypoint.ServerCmd {
		resolved := part
		resolved = strings.ReplaceAll(resolved, "{{.Port}}", fmt.Sprintf("%d", port))
		resolved = strings.ReplaceAll(resolved, "{{.HFRepo}}", manifest.HuggingFace.Repo)
		// Cloud uses same /models/ paths; container downloads weights there
		resolved = strings.ReplaceAll(resolved, "{{.MoshiWeight}}", "/models/model.safetensors")
		resolved = strings.ReplaceAll(resolved, "{{.MimiWeight}}", "/models/tokenizer-e351c8d8-checkpoint125.safetensors")
		resolved = strings.ReplaceAll(resolved, "{{.Tokenizer}}", "/models/tokenizer_spm_32k_3.model")
		resolved = strings.ReplaceAll(resolved, "{{.VoicePromptDir}}", "/models/voices")
		cmd = append(cmd, resolved)
	}
	if device != "" {
		cmd = append(cmd, "--device", device)
	}
	if cpuOffload {
		cmd = append(cmd, "--cpu-offload")
	}
	return cmd
}

func (b *CloudBackend) Start(ctx context.Context, req backend.ServeRequest) (backend.InstanceID, error) {
	env := req.Env
	if env == nil {
		env = make(map[string]string)
	}
	if b.hfToken != "" {
		env["HF_TOKEN"] = b.hfToken
	}

	gpuName := b.gpuType
	if gpuName == "" {
		gpuName = "NVIDIA RTX A6000"
	}

	createReq := CreateInstanceRequest{
		Name:          req.ContainerName,
		Image:         req.Image,
		Cmd:           req.Cmd,
		Env:           env,
		GPU:           GPUType{Name: gpuName, VRAMGB: 24, Count: 1},
		ContainerPort: req.ContainerPort,
		DiskGB:        b.diskGB,
	}

	inst, err := b.provider.CreateInstance(ctx, createReq)
	if err != nil {
		return "", fmt.Errorf("cloud create failed: %w", err)
	}

	// Track the instance
	b.state.Add(&TrackedInstance{
		ProviderID:   inst.ID,
		ProviderName: b.provider.Name(),
		ModelName:    req.ModelName,
		DisplayName:  req.ContainerName,
		Voice:        req.Voice,
		Prompt:       req.Prompt,
		Host:         inst.Host,
		Port:         inst.Port,
		ProxyURL:     inst.ProxyURL,
		CreatedAt:    time.Now(),
	})

	return backend.InstanceID(inst.ID), nil
}

func (b *CloudBackend) Stop(ctx context.Context, id backend.InstanceID, timeoutSec int) error {
	return b.provider.StopInstance(ctx, string(id))
}

func (b *CloudBackend) ForceStop(ctx context.Context, id backend.InstanceID) error {
	return b.provider.DestroyInstance(ctx, string(id))
}

func (b *CloudBackend) Remove(ctx context.Context, id backend.InstanceID) error {
	err := b.provider.DestroyInstance(ctx, string(id))
	b.state.Remove(string(id))
	return err
}

func (b *CloudBackend) List(ctx context.Context) ([]backend.InstanceInfo, error) {
	tracked := b.state.All()
	var result []backend.InstanceInfo
	for _, t := range tracked {
		// Refresh status from provider
		ci, err := b.provider.GetInstance(ctx, t.ProviderID)
		if err != nil {
			// Instance may have been terminated externally
			b.state.Remove(t.ProviderID)
			continue
		}
		if ci.Status == "stopped" || ci.Status == "error" {
			b.state.Remove(t.ProviderID)
			continue
		}
		// Update cached network info
		t.Host = ci.Host
		t.Port = ci.Port
		t.ProxyURL = ci.ProxyURL

		result = append(result, b.toInstanceInfo(t, ci))
	}
	return result, nil
}

func (b *CloudBackend) FindByModel(ctx context.Context, modelName string) (*backend.InstanceInfo, error) {
	t := b.state.FindByModel(modelName)
	if t == nil {
		return nil, fmt.Errorf("no cloud instance for model %q", modelName)
	}

	ci, err := b.provider.GetInstance(ctx, t.ProviderID)
	if err != nil {
		b.state.Remove(t.ProviderID)
		return nil, fmt.Errorf("cloud instance for %q no longer accessible: %w", modelName, err)
	}

	t.Host = ci.Host
	t.Port = ci.Port
	t.ProxyURL = ci.ProxyURL

	info := b.toInstanceInfo(t, ci)
	return &info, nil
}

func (b *CloudBackend) Logs(ctx context.Context, id backend.InstanceID, w io.Writer) error {
	return b.provider.StreamLogs(ctx, string(id), w)
}

func (b *CloudBackend) WaitReady(ctx context.Context, id backend.InstanceID, timeoutSec, intervalSec int) error {
	t := b.state.Get(string(id))
	if t == nil {
		return fmt.Errorf("unknown cloud instance %s", id)
	}

	// Poll provider until running, then TCP dial
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	for time.Now().Before(deadline) {
		ci, err := b.provider.GetInstance(ctx, string(id))
		if err != nil {
			time.Sleep(time.Duration(intervalSec) * time.Second)
			continue
		}
		if ci.Status == "error" {
			return fmt.Errorf("cloud instance entered error state")
		}
		if ci.Status == "running" {
			// Update cached info
			t.Host = ci.Host
			t.Port = ci.Port
			t.ProxyURL = ci.ProxyURL

			host := ci.Host
			port := ci.Port
			if host == "" || port == 0 {
				time.Sleep(time.Duration(intervalSec) * time.Second)
				continue
			}
			remaining := int(time.Until(deadline).Seconds())
			if remaining < 1 {
				remaining = 1
			}
			return backend.WaitForTCP(host, port, remaining, intervalSec)
		}
		time.Sleep(time.Duration(intervalSec) * time.Second)
	}
	return fmt.Errorf("cloud instance not ready after %d seconds", timeoutSec)
}

func (b *CloudBackend) Close() error {
	return b.state.Save()
}

func (b *CloudBackend) toInstanceInfo(t *TrackedInstance, ci *CloudInstance) backend.InstanceInfo {
	wsURL := b.buildWebSocketURL(t, ci)
	status := "running"
	if ci.Status != "running" {
		status = ci.Status
	}
	return backend.InstanceInfo{
		ID:           backend.InstanceID(t.ProviderID),
		Name:         t.DisplayName,
		ModelName:    t.ModelName,
		Port:         ci.Port,
		Voice:        t.Voice,
		Prompt:       t.Prompt,
		Status:       status,
		StartedAt:    t.CreatedAt,
		WebSocketURL: wsURL,
	}
}

func (b *CloudBackend) buildWebSocketURL(t *TrackedInstance, ci *CloudInstance) string {
	if ci.ProxyURL != "" {
		url := ci.ProxyURL
		// RunPod proxy URLs are https; convert to wss for WebSocket
		url = strings.Replace(url, "https://", "wss://", 1)
		url = strings.Replace(url, "http://", "ws://", 1)
		if !strings.HasSuffix(url, "/api/chat") {
			url = strings.TrimRight(url, "/") + "/api/chat"
		}
		return url
	}
	if ci.Host != "" && ci.Port != 0 {
		return fmt.Sprintf("ws://%s:%d/api/chat", ci.Host, ci.Port)
	}
	return ""
}
