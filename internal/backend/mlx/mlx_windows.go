//go:build windows

package mlx

import (
	"context"
	"fmt"
	"io"
	"runtime"

	"github.com/Krunal96369/voicaa/internal/backend"
	"github.com/Krunal96369/voicaa/internal/model"
)

// Config holds configuration for the MLX backend.
type Config struct {
	StatePath string
	UVPath    string
}

// MLXBackend is not supported on Windows — MLX requires macOS with Apple Silicon.
type MLXBackend struct{}

// New returns an error on Windows since MLX is not supported.
func New(cfg Config) (*MLXBackend, error) {
	return nil, fmt.Errorf("MLX backend is not supported on %s; it requires macOS with Apple Silicon", runtime.GOOS)
}

func (b *MLXBackend) Name() string             { return "mlx" }
func (b *MLXBackend) RequiresLocalModel() bool { return false }
func (b *MLXBackend) Close() error             { return nil }

func (b *MLXBackend) BuildServerCmd(manifest *model.ModelManifest, port int, voice, prompt string, cpuOffload bool, device string) []string {
	return nil
}

func (b *MLXBackend) Start(ctx context.Context, req backend.ServeRequest) (backend.InstanceID, error) {
	return "", fmt.Errorf("MLX backend is not supported on %s", runtime.GOOS)
}

func (b *MLXBackend) Stop(ctx context.Context, id backend.InstanceID, timeoutSec int) error {
	return fmt.Errorf("MLX backend is not supported on %s", runtime.GOOS)
}

func (b *MLXBackend) ForceStop(ctx context.Context, id backend.InstanceID) error {
	return fmt.Errorf("MLX backend is not supported on %s", runtime.GOOS)
}

func (b *MLXBackend) Remove(ctx context.Context, id backend.InstanceID) error {
	return fmt.Errorf("MLX backend is not supported on %s", runtime.GOOS)
}

func (b *MLXBackend) List(ctx context.Context) ([]backend.InstanceInfo, error) {
	return nil, fmt.Errorf("MLX backend is not supported on %s", runtime.GOOS)
}

func (b *MLXBackend) FindByModel(ctx context.Context, modelName string) (*backend.InstanceInfo, error) {
	return nil, fmt.Errorf("MLX backend is not supported on %s", runtime.GOOS)
}

func (b *MLXBackend) Logs(ctx context.Context, id backend.InstanceID, w io.Writer) error {
	return fmt.Errorf("MLX backend is not supported on %s", runtime.GOOS)
}

func (b *MLXBackend) WaitReady(ctx context.Context, id backend.InstanceID, timeoutSec, intervalSec int) error {
	return fmt.Errorf("MLX backend is not supported on %s", runtime.GOOS)
}
