package backend

import (
	"context"
	"io"
	"time"

	"github.com/Krunal96369/voicaa/internal/model"
)

// InstanceID is an opaque identifier for a running model instance.
type InstanceID string

// ServeRequest contains everything needed to start a model instance.
type ServeRequest struct {
	// Model identity
	ModelName     string
	ContainerName string

	// Image and command
	Image   string
	Cmd     []string
	Env     map[string]string
	WorkDir string

	// Network
	HostPort      int
	ContainerPort int

	// Storage
	ModelDir string // absolute path to local model weights

	// Model options
	Voice  string
	Prompt string

	// GPU
	GPURuntime string
	GPUIDs     string
	CpuOffload bool
}

// InstanceInfo describes a running model instance.
type InstanceInfo struct {
	ID           InstanceID
	Name         string // human-readable name (e.g. container name)
	ModelName    string
	Port         int
	Voice        string
	Prompt       string
	Status       string
	StartedAt    time.Time
	WebSocketURL string
}

// Backend is the interface that all inference backends must implement.
type Backend interface {
	// Start launches a model instance and returns its ID.
	Start(ctx context.Context, req ServeRequest) (InstanceID, error)

	// Stop gracefully stops a running instance.
	Stop(ctx context.Context, id InstanceID, timeoutSec int) error

	// ForceStop immediately kills an instance.
	ForceStop(ctx context.Context, id InstanceID) error

	// Remove removes a stopped instance.
	Remove(ctx context.Context, id InstanceID) error

	// List returns all running instances managed by this backend.
	List(ctx context.Context) ([]InstanceInfo, error)

	// FindByModel returns info for the running instance of a model.
	// Returns nil, nil if not found.
	FindByModel(ctx context.Context, modelName string) (*InstanceInfo, error)

	// Logs streams instance logs to the writer until context is cancelled.
	Logs(ctx context.Context, id InstanceID, w io.Writer) error

	// WaitReady blocks until the instance is accepting connections or timeout.
	WaitReady(ctx context.Context, id InstanceID, timeoutSec, intervalSec int) error

	// Name returns the backend identifier (e.g. "docker", "subprocess").
	Name() string
}

// CommandBuilder is an optional interface that backends can implement
// to provide custom server command resolution.
type CommandBuilder interface {
	BuildServerCmd(manifest *model.ModelManifest, port int, voice, prompt string, cpuOffload bool, device string) []string
}

// LocalModelChecker is an optional interface. Backends that do not
// require locally downloaded model weights (e.g. cloud) return false.
type LocalModelChecker interface {
	RequiresLocalModel() bool
}
