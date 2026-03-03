package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Krunal96369/voicaa/internal/backend"
	dockerbackend "github.com/Krunal96369/voicaa/internal/backend/docker"
	"github.com/Krunal96369/voicaa/internal/config"
	"github.com/Krunal96369/voicaa/internal/model"
)

// ServeOptions contains the parameters for serving a model.
type ServeOptions struct {
	ModelName     string
	Port          int
	Voice         string
	Prompt        string
	CpuOffload    bool
	Device        string
	Detach        bool
	ContainerName string
	GPUIDs        string
}

// InstanceService handles model instance lifecycle via a Backend.
type InstanceService struct {
	Config       *config.Config
	Store        *model.Store
	Backend      backend.Backend
	ModelService *ModelService
}

// NewInstanceService creates an InstanceService.
func NewInstanceService(cfg *config.Config, b backend.Backend, ms *ModelService) *InstanceService {
	return &InstanceService{
		Config:       cfg,
		Store:        model.NewStore(cfg.ModelsDir),
		Backend:      b,
		ModelService: ms,
	}
}

// requiresLocalModel checks whether the backend needs local model weights.
func (s *InstanceService) requiresLocalModel() bool {
	if checker, ok := s.Backend.(backend.LocalModelChecker); ok {
		return checker.RequiresLocalModel()
	}
	return true // default: require local model
}

// buildServerCmd builds the server command using the backend's builder if available.
func (s *InstanceService) buildServerCmd(manifest *model.ModelManifest, port int, voice, prompt string, cpuOffload bool, device string) []string {
	if builder, ok := s.Backend.(backend.CommandBuilder); ok {
		return builder.BuildServerCmd(manifest, port, voice, prompt, cpuOffload, device)
	}
	return dockerbackend.BuildServerCmd(manifest, port, voice, prompt, cpuOffload, device)
}

// resolveModelForBackend adjusts model lookup based on the active backend.
// When the MLX backend is active and the user requests a Docker-only model,
// tries to find the MLX equivalent (e.g., "moshi" -> "moshi:q4").
func (s *InstanceService) resolveModelForBackend(name string) (*model.ModelManifest, error) {
	manifest, err := s.ModelService.FindModel(name)
	if err != nil {
		return nil, err
	}

	manifestBackend := manifest.Backend
	if manifestBackend == "" {
		manifestBackend = "docker"
	}
	activeBackend := s.Backend.Name()
	if strings.HasPrefix(activeBackend, "cloud:") {
		activeBackend = "cloud"
	}

	if manifestBackend == activeBackend {
		return manifest, nil
	}

	// Mismatch: try to find an equivalent for the active backend
	if activeBackend == "mlx" && manifestBackend == "docker" {
		mlxName := name + ":q4"
		if mlxManifest, err := s.ModelService.FindModel(mlxName); err == nil {
			if mlxManifest.Backend == "mlx" {
				return mlxManifest, nil
			}
		}
	}

	return manifest, nil
}

// Serve starts a model instance. Returns the instance info.
func (s *InstanceService) Serve(ctx context.Context, opts ServeOptions) (*backend.InstanceInfo, error) {
	manifest, err := s.resolveModelForBackend(opts.ModelName)
	if err != nil {
		return nil, err
	}

	if s.requiresLocalModel() && !s.Store.IsComplete(manifest.Name) {
		return nil, fmt.Errorf(
			"model %q is not downloaded\n\n  Run: voicaa pull %s",
			manifest.Name, manifest.Name,
		)
	}

	port := opts.Port
	if port == 0 {
		port = manifest.Entrypoint.DefaultPort
		if port == 0 {
			port = s.Config.DefaultPort
		}
	}

	voice := opts.Voice
	if voice == "" {
		voice = manifest.Voices.DefaultVoice
	}

	containerName := opts.ContainerName
	if containerName == "" {
		containerName = fmt.Sprintf("voicaa-%s", manifest.Name)
	}

	modelDir := s.Store.ModelDir(manifest.Name)

	// Check for existing instance
	existing, err := s.Backend.FindByModel(ctx, manifest.Name)
	if err == nil && existing != nil {
		return nil, fmt.Errorf(
			"model %q is already running (container: %s, port: %d)\n\n  Stop it first: voicaa stop %s",
			manifest.Name, existing.Name, existing.Port, manifest.Name,
		)
	}

	serverCmd := s.buildServerCmd(manifest, port, voice, opts.Prompt, opts.CpuOffload, opts.Device)

	env := make(map[string]string)
	for k, v := range manifest.Entrypoint.Env {
		env[k] = v
	}
	if hfToken := os.Getenv("HF_TOKEN"); hfToken != "" {
		env["HF_TOKEN"] = hfToken
	}

	instanceID, err := s.Backend.Start(ctx, backend.ServeRequest{
		ModelName:     manifest.Name,
		ContainerName: containerName,
		Image:         manifest.Docker.Image,
		Cmd:           serverCmd,
		Env:           env,
		WorkDir:       manifest.Entrypoint.WorkDir,
		HostPort:      port,
		ContainerPort: manifest.Entrypoint.DefaultPort,
		ModelDir:      modelDir,
		Voice:         voice,
		Prompt:        opts.Prompt,
		GPURuntime:    s.Config.GPURuntime,
		GPUIDs:        opts.GPUIDs,
		CpuOffload:    opts.CpuOffload,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Wait for health check
	hc := manifest.Entrypoint.HealthCheck
	healthErr := s.Backend.WaitReady(ctx, instanceID, hc.TimeoutSec, hc.IntervalSec)

	// Get authoritative instance info from backend (has correct WebSocketURL)
	info, lookupErr := s.Backend.FindByModel(ctx, manifest.Name)
	if lookupErr != nil || info == nil {
		// Fallback: construct manually
		info = &backend.InstanceInfo{
			ID:           instanceID,
			Name:         containerName,
			ModelName:    manifest.Name,
			Port:         port,
			Voice:        voice,
			Prompt:       opts.Prompt,
			WebSocketURL: fmt.Sprintf("ws://localhost:%d/api/chat", port),
		}
	}

	if healthErr != nil {
		info.Status = "loading"
	} else {
		info.Status = "ready"
	}

	return info, nil
}

// Run pulls (if needed) then serves.
func (s *InstanceService) Run(ctx context.Context, opts ServeOptions, token string) (*backend.InstanceInfo, error) {
	manifest, err := model.FindModel(opts.ModelName)
	if err != nil {
		return nil, err
	}

	if !s.Store.IsComplete(manifest.Name) {
		if err := s.ModelService.Pull(ctx, opts.ModelName, token, false, false, nil); err != nil {
			return nil, err
		}
	}

	return s.Serve(ctx, opts)
}

// Stop stops a running model instance.
func (s *InstanceService) Stop(ctx context.Context, modelName string, force bool, timeout int) error {
	inst, err := s.Backend.FindByModel(ctx, modelName)
	if err != nil {
		return fmt.Errorf("no running instance of %q found", modelName)
	}

	if force {
		timeout = 0
	}

	if err := s.Backend.Stop(ctx, inst.ID, timeout); err != nil {
		return fmt.Errorf("failed to stop instance: %w", err)
	}

	if err := s.Backend.Remove(ctx, inst.ID); err != nil {
		// Removal failure is non-fatal
		_ = err
	}

	return nil
}

// Ps lists running instances.
func (s *InstanceService) Ps(ctx context.Context) ([]backend.InstanceInfo, error) {
	return s.Backend.List(ctx)
}

// FindByModel returns the running instance for a model.
func (s *InstanceService) FindByModel(ctx context.Context, modelName string) (*backend.InstanceInfo, error) {
	return s.Backend.FindByModel(ctx, modelName)
}

// Logs streams logs for a model instance.
func (s *InstanceService) Logs(ctx context.Context, modelName string, w io.Writer) error {
	inst, err := s.Backend.FindByModel(ctx, modelName)
	if err != nil {
		return fmt.Errorf("no running instance of %q found", modelName)
	}
	return s.Backend.Logs(ctx, inst.ID, w)
}
