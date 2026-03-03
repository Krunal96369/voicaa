package runpod

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Krunal96369/voicaa/internal/backend/cloud"
)

const apiURL = "https://api.runpod.io/graphql"

// Config holds RunPod-specific configuration.
type Config struct {
	APIKey    string
	GPUType   string // e.g. "NVIDIA RTX A6000"
	CloudType string // "COMMUNITY" or "SECURE"
	Region    string
}

// RunPodProvider implements cloud.CloudProvider for RunPod.
type RunPodProvider struct {
	config     Config
	httpClient *http.Client
}

// New creates a RunPodProvider.
func New(cfg Config) *RunPodProvider {
	if cfg.CloudType == "" {
		cfg.CloudType = "COMMUNITY"
	}
	return &RunPodProvider{
		config:     cfg,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *RunPodProvider) Name() string { return "runpod" }

func (p *RunPodProvider) CreateInstance(ctx context.Context, req cloud.CreateInstanceRequest) (*cloud.CloudInstance, error) {
	gpuType := req.GPU.Name
	if gpuType == "" {
		gpuType = p.config.GPUType
	}
	if gpuType == "" {
		gpuType = "NVIDIA RTX A6000"
	}

	// Build environment variable array for GraphQL
	var envEntries []map[string]string
	for k, v := range req.Env {
		envEntries = append(envEntries, map[string]string{"key": k, "value": v})
	}

	// Build port string
	ports := fmt.Sprintf("%d/http", req.ContainerPort)

	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"name":              req.Name,
			"imageName":         req.Image,
			"gpuTypeId":         gpuType,
			"cloudType":         p.config.CloudType,
			"containerDiskInGb": req.DiskGB,
			"ports":             ports,
			"env":               envEntries,
			"dockerArgs":        strings.Join(req.Cmd, " "),
			"gpuCount":          max(req.GPU.Count, 1),
			"volumeInGb":        0,
			"startJupyter":      false,
			"startSsh":          false,
			"supportPublicIp":   true,
		},
	}

	query := `mutation($input: PodFindAndDeployOnDemandInput!) {
		podFindAndDeployOnDemand(input: $input) {
			id
			name
			desiredStatus
			imageName
			costPerHr
		}
	}`

	resp, err := p.doGraphQL(ctx, query, variables)
	if err != nil {
		return nil, fmt.Errorf("RunPod create failed: %w", err)
	}

	var result struct {
		PodFindAndDeployOnDemand struct {
			ID            string  `json:"id"`
			Name          string  `json:"name"`
			DesiredStatus string  `json:"desiredStatus"`
			CostPerHr     float64 `json:"costPerHr"`
		} `json:"podFindAndDeployOnDemand"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("RunPod response parse failed: %w", err)
	}

	pod := result.PodFindAndDeployOnDemand
	proxyURL := fmt.Sprintf("https://%s-%d.proxy.runpod.net", pod.ID, req.ContainerPort)

	return &cloud.CloudInstance{
		ID:           pod.ID,
		Status:       normalizeStatus(pod.DesiredStatus),
		ProxyURL:     proxyURL,
		Port:         req.ContainerPort,
		CreatedAt:    time.Now(),
		CostPerHrUSD: pod.CostPerHr,
		ProviderMeta: map[string]string{
			"name":           pod.Name,
			"desired_status": pod.DesiredStatus,
		},
	}, nil
}

func (p *RunPodProvider) GetInstance(ctx context.Context, instanceID string) (*cloud.CloudInstance, error) {
	query := `query($podId: String!) {
		pod(input: {podId: $podId}) {
			id
			name
			desiredStatus
			runtime {
				uptimeInSeconds
				ports {
					ip
					isIpPublic
					privatePort
					publicPort
				}
			}
			costPerHr
		}
	}`

	resp, err := p.doGraphQL(ctx, query, map[string]interface{}{
		"podId": instanceID,
	})
	if err != nil {
		return nil, err
	}

	var result struct {
		Pod struct {
			ID            string  `json:"id"`
			Name          string  `json:"name"`
			DesiredStatus string  `json:"desiredStatus"`
			CostPerHr     float64 `json:"costPerHr"`
			Runtime       *struct {
				UptimeInSeconds int `json:"uptimeInSeconds"`
				Ports           []struct {
					IP          string `json:"ip"`
					IsIPPublic  bool   `json:"isIpPublic"`
					PrivatePort int    `json:"privatePort"`
					PublicPort  int    `json:"publicPort"`
				} `json:"ports"`
			} `json:"runtime"`
		} `json:"pod"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("RunPod response parse failed: %w", err)
	}

	pod := result.Pod
	if pod.ID == "" {
		return nil, fmt.Errorf("RunPod pod %s not found", instanceID)
	}

	ci := &cloud.CloudInstance{
		ID:           pod.ID,
		Status:       normalizeStatus(pod.DesiredStatus),
		CostPerHrUSD: pod.CostPerHr,
		ProviderMeta: map[string]string{
			"name": pod.Name,
		},
	}

	// Extract public endpoint
	if pod.Runtime != nil {
		for _, p := range pod.Runtime.Ports {
			if p.IsIPPublic && p.PublicPort != 0 {
				ci.Host = p.IP
				ci.Port = p.PublicPort
				break
			}
		}
	}

	// Always provide proxy URL as fallback
	if ci.ProxyURL == "" {
		ci.ProxyURL = fmt.Sprintf("https://%s-%d.proxy.runpod.net", pod.ID, 8998)
	}

	return ci, nil
}

func (p *RunPodProvider) ListInstances(ctx context.Context) ([]cloud.CloudInstance, error) {
	query := `query {
		myself {
			pods {
				id
				name
				desiredStatus
				costPerHr
			}
		}
	}`

	resp, err := p.doGraphQL(ctx, query, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Myself struct {
			Pods []struct {
				ID            string  `json:"id"`
				Name          string  `json:"name"`
				DesiredStatus string  `json:"desiredStatus"`
				CostPerHr     float64 `json:"costPerHr"`
			} `json:"pods"`
		} `json:"myself"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, err
	}

	var instances []cloud.CloudInstance
	for _, pod := range result.Myself.Pods {
		// Only include voicaa-managed pods
		if !strings.HasPrefix(pod.Name, "voicaa-") {
			continue
		}
		instances = append(instances, cloud.CloudInstance{
			ID:           pod.ID,
			Status:       normalizeStatus(pod.DesiredStatus),
			CostPerHrUSD: pod.CostPerHr,
			ProviderMeta: map[string]string{"name": pod.Name},
		})
	}

	return instances, nil
}

func (p *RunPodProvider) StopInstance(ctx context.Context, instanceID string) error {
	query := `mutation($podId: String!) {
		podStop(input: {podId: $podId}) {
			id
			desiredStatus
		}
	}`
	_, err := p.doGraphQL(ctx, query, map[string]interface{}{"podId": instanceID})
	return err
}

func (p *RunPodProvider) DestroyInstance(ctx context.Context, instanceID string) error {
	query := `mutation($podId: String!) {
		podTerminate(input: {podId: $podId})
	}`
	_, err := p.doGraphQL(ctx, query, map[string]interface{}{"podId": instanceID})
	return err
}

func (p *RunPodProvider) StreamLogs(ctx context.Context, instanceID string, w io.Writer) error {
	_, _ = fmt.Fprintf(w, "RunPod does not support real-time log streaming.\nView logs at: https://www.runpod.io/console/pods/%s\n", instanceID)
	return nil
}

// --- GraphQL helpers ---

type graphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (p *RunPodProvider) doGraphQL(ctx context.Context, query string, variables map[string]interface{}) (*graphQLResponse, error) {
	body, err := json.Marshal(graphQLRequest{Query: query, Variables: variables})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("RunPod API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("RunPod API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var gqlResp graphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return nil, fmt.Errorf("RunPod response decode failed: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("RunPod API error: %s", gqlResp.Errors[0].Message)
	}

	return &gqlResp, nil
}

func normalizeStatus(runpodStatus string) string {
	switch runpodStatus {
	case "CREATED", "QUEUED":
		return "pending"
	case "BUILDING":
		return "building"
	case "RUNNING":
		return "running"
	case "STOPPING":
		return "stopping"
	case "STOPPED", "EXITED", "TERMINATED":
		return "stopped"
	default:
		return "error"
	}
}
