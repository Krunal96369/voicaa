package docker

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Krunal96369/voicaa/internal/backend"
	dockerclient "github.com/Krunal96369/voicaa/internal/docker"
	"github.com/Krunal96369/voicaa/internal/model"
)

// DockerBackend implements backend.Backend using Docker containers.
type DockerBackend struct {
	client *dockerclient.Client
}

// New creates a DockerBackend with a fresh Docker client.
func New() (*DockerBackend, error) {
	c, err := dockerclient.NewClient()
	if err != nil {
		return nil, err
	}
	return &DockerBackend{client: c}, nil
}

func (b *DockerBackend) Name() string { return "docker" }

func (b *DockerBackend) Start(ctx context.Context, req backend.ServeRequest) (backend.InstanceID, error) {
	containerID, err := b.client.RunContainer(ctx, dockerclient.ServeOptions{
		Image:         req.Image,
		ContainerName: req.ContainerName,
		ModelName:     req.ModelName,
		HostPort:      req.HostPort,
		ContainerPort: req.ContainerPort,
		ModelDir:      req.ModelDir,
		Voice:         req.Voice,
		TextPrompt:    req.Prompt,
		Cmd:           req.Cmd,
		Env:           req.Env,
		GPURuntime:    req.GPURuntime,
		GPUIDs:        req.GPUIDs,
		CpuOffload:    req.CpuOffload,
	})
	if err != nil {
		return "", err
	}
	return backend.InstanceID(containerID), nil
}

func (b *DockerBackend) Stop(ctx context.Context, id backend.InstanceID, timeoutSec int) error {
	return b.client.StopContainer(ctx, string(id), timeoutSec)
}

func (b *DockerBackend) ForceStop(ctx context.Context, id backend.InstanceID) error {
	return b.client.StopContainer(ctx, string(id), 0)
}

func (b *DockerBackend) Remove(ctx context.Context, id backend.InstanceID) error {
	return b.client.RemoveContainer(ctx, string(id))
}

func (b *DockerBackend) List(ctx context.Context) ([]backend.InstanceInfo, error) {
	containers, err := b.client.ListVoicaaContainers(ctx)
	if err != nil {
		return nil, err
	}
	var result []backend.InstanceInfo
	for _, c := range containers {
		result = append(result, toInstanceInfo(c))
	}
	return result, nil
}

func (b *DockerBackend) FindByModel(ctx context.Context, modelName string) (*backend.InstanceInfo, error) {
	inst, err := b.client.FindContainerByModel(ctx, modelName)
	if err != nil {
		return nil, err
	}
	info := toInstanceInfo(*inst)
	return &info, nil
}

func (b *DockerBackend) Logs(ctx context.Context, id backend.InstanceID, w io.Writer) error {
	return b.client.StreamLogs(ctx, string(id), w)
}

func (b *DockerBackend) WaitReady(ctx context.Context, id backend.InstanceID, timeoutSec, intervalSec int) error {
	// For Docker, we don't need the instance ID — we use the host port.
	// The caller should use WaitForReady directly with the known port.
	// This is a convenience that requires knowing the port from the instance.
	instances, err := b.client.ListVoicaaContainers(ctx)
	if err != nil {
		return err
	}
	for _, inst := range instances {
		if inst.ContainerID == string(id) || strings.HasPrefix(inst.ContainerID, string(id)[:12]) {
			return WaitForReady("localhost", inst.Port, timeoutSec, intervalSec)
		}
	}
	return fmt.Errorf("instance %s not found", id)
}

// Close releases the underlying Docker client.
func (b *DockerBackend) Close() error {
	return b.client.Close()
}

func toInstanceInfo(c dockerclient.RunningInstance) backend.InstanceInfo {
	return backend.InstanceInfo{
		ID:           backend.InstanceID(c.ContainerID),
		Name:         c.ContainerName,
		ModelName:    c.ModelName,
		Port:         c.Port,
		Voice:        c.Voice,
		Prompt:       c.TextPrompt,
		Status:       c.Status,
		StartedAt:    c.StartedAt,
		WebSocketURL: fmt.Sprintf("ws://localhost:%d/api/chat", c.Port),
	}
}

// BuildServerCmd resolves the server command template from a model manifest.
// This is Docker-specific because it uses container-internal paths (/models/).
func BuildServerCmd(manifest *model.ModelManifest, port int, voice, prompt string, cpuOffload bool, device string) []string {
	hasVoices := manifest.Voices.DefaultVoice != ""

	var cmd []string
	for _, part := range manifest.Entrypoint.ServerCmd {
		resolved := strings.ReplaceAll(part, "{{.Port}}", fmt.Sprintf("%d", port))
		resolved = strings.ReplaceAll(resolved, "{{.HFRepo}}", manifest.HuggingFace.Repo)
		resolved = strings.ReplaceAll(resolved, "{{.MoshiWeight}}", "/models/model.safetensors")
		resolved = strings.ReplaceAll(resolved, "{{.MimiWeight}}", "/models/tokenizer-e351c8d8-checkpoint125.safetensors")
		resolved = strings.ReplaceAll(resolved, "{{.Tokenizer}}", "/models/tokenizer_spm_32k_3.model")
		resolved = strings.ReplaceAll(resolved, "{{.VoicePromptDir}}", "/models/voices")
		cmd = append(cmd, resolved)
	}

	cmd = append(cmd, "--device", device)

	if cpuOffload {
		cmd = append(cmd, "--cpu-offload")
	}

	if hasVoices && voice != "" {
		// PersonaPlex: voice and prompt are passed via WebSocket query params, not CLI
		_ = voice
		_ = prompt
	}

	return cmd
}
