package vastai

import (
	"context"
	"fmt"
	"io"

	"github.com/Krunal96369/voicaa/internal/backend/cloud"
)

// Config holds Vast.ai-specific configuration.
type Config struct {
	APIKey string
}

// VastAIProvider implements cloud.CloudProvider for Vast.ai.
type VastAIProvider struct {
	config Config
}

// New creates a VastAIProvider.
func New(cfg Config) *VastAIProvider {
	return &VastAIProvider{config: cfg}
}

func (p *VastAIProvider) Name() string { return "vastai" }

func (p *VastAIProvider) CreateInstance(ctx context.Context, req cloud.CreateInstanceRequest) (*cloud.CloudInstance, error) {
	return nil, fmt.Errorf("Vast.ai provider not yet implemented")
}

func (p *VastAIProvider) GetInstance(ctx context.Context, instanceID string) (*cloud.CloudInstance, error) {
	return nil, fmt.Errorf("Vast.ai provider not yet implemented")
}

func (p *VastAIProvider) ListInstances(ctx context.Context) ([]cloud.CloudInstance, error) {
	return nil, fmt.Errorf("Vast.ai provider not yet implemented")
}

func (p *VastAIProvider) StopInstance(ctx context.Context, instanceID string) error {
	return fmt.Errorf("Vast.ai provider not yet implemented")
}

func (p *VastAIProvider) DestroyInstance(ctx context.Context, instanceID string) error {
	return fmt.Errorf("Vast.ai provider not yet implemented")
}

func (p *VastAIProvider) StreamLogs(ctx context.Context, instanceID string, w io.Writer) error {
	return fmt.Errorf("Vast.ai provider not yet implemented")
}
