package cloud

import (
	"context"
	"io"
	"time"
)

// GPUType describes the GPU hardware requested.
type GPUType struct {
	Name   string // e.g. "NVIDIA RTX 4090", "NVIDIA A100 80GB"
	VRAMGB int
	Count  int
}

// CreateInstanceRequest contains everything needed to launch a cloud GPU instance.
type CreateInstanceRequest struct {
	Name          string
	Image         string
	Cmd           []string
	Env           map[string]string
	GPU           GPUType
	ContainerPort int
	DiskGB        int
	VolumeID      string // optional persistent volume for model weights
}

// CloudInstance represents a running instance on a cloud provider.
type CloudInstance struct {
	ID       string
	Status   string // "pending", "building", "running", "stopping", "stopped", "error"
	Host     string // public IP or hostname
	Port     int    // mapped container port
	ProxyURL string // full proxy URL; takes precedence over Host:Port (RunPod uses this)

	CreatedAt    time.Time
	CostPerHrUSD float64
	ProviderMeta map[string]string
}

// CloudProvider is the interface that each cloud GPU provider implements.
// Implementations should be stateless — all state management lives in CloudBackend.
type CloudProvider interface {
	// Name returns the provider identifier (e.g. "runpod", "vastai").
	Name() string

	// CreateInstance launches a new GPU instance.
	CreateInstance(ctx context.Context, req CreateInstanceRequest) (*CloudInstance, error)

	// GetInstance fetches the current state of an instance.
	GetInstance(ctx context.Context, instanceID string) (*CloudInstance, error)

	// ListInstances returns all instances that match the voicaa label/tag.
	ListInstances(ctx context.Context) ([]CloudInstance, error)

	// StopInstance gracefully stops an instance.
	StopInstance(ctx context.Context, instanceID string) error

	// DestroyInstance terminates and removes an instance permanently.
	DestroyInstance(ctx context.Context, instanceID string) error

	// StreamLogs retrieves logs from the instance.
	StreamLogs(ctx context.Context, instanceID string, w io.Writer) error
}
