package docker

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const (
	LabelPrefix = "io.voicaa."
	LabelModel  = LabelPrefix + "model"
	LabelPort   = LabelPrefix + "port"
	LabelVoice  = LabelPrefix + "voice"
	LabelPrompt = LabelPrefix + "prompt"
)

type Client struct {
	cli *client.Client
}

func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Close() error {
	return c.cli.Close()
}

type ServeOptions struct {
	Image         string
	ContainerName string
	ModelName     string
	HostPort      int
	ContainerPort int
	ModelDir      string
	Voice         string
	TextPrompt    string
	Cmd           []string
	Env           map[string]string
	GPURuntime    string
	GPUIDs        string
	CpuOffload    bool
	Detach        bool
}

func (c *Client) RunContainer(ctx context.Context, opts ServeOptions) (string, error) {
	containerPortStr := fmt.Sprintf("%d/tcp", opts.ContainerPort)
	hostPortStr := fmt.Sprintf("%d", opts.HostPort)

	portBindings := nat.PortMap{
		nat.Port(containerPortStr): []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: hostPortStr},
		},
	}

	envList := []string{}
	for k, v := range opts.Env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}

	labels := map[string]string{
		LabelModel:  opts.ModelName,
		LabelPort:   hostPortStr,
		LabelVoice:  opts.Voice,
		LabelPrompt: opts.TextPrompt,
	}

	mounts := []mount.Mount{
		{
			Type:     mount.TypeBind,
			Source:   opts.ModelDir,
			Target:   "/models",
			ReadOnly: true,
		},
	}

	deviceRequests := []container.DeviceRequest{
		{
			Count:        -1,
			Capabilities: [][]string{{"gpu"}},
		},
	}
	if opts.GPUIDs != "" {
		deviceRequests[0].Count = 0
		deviceRequests[0].DeviceIDs = []string{opts.GPUIDs}
	}

	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		Mounts:       mounts,
		Resources: container.Resources{
			DeviceRequests: deviceRequests,
		},
	}
	if opts.GPURuntime != "" {
		hostConfig.Runtime = opts.GPURuntime
	}

	resp, err := c.cli.ContainerCreate(
		ctx,
		&container.Config{
			Image:        opts.Image,
			Cmd:          opts.Cmd,
			Env:          envList,
			Labels:       labels,
			ExposedPorts: nat.PortSet{nat.Port(containerPortStr): struct{}{}},
			WorkingDir:   "/app/moshi",
		},
		hostConfig,
		nil,
		nil,
		opts.ContainerName,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	if err := c.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	return resp.ID, nil
}

type RunningInstance struct {
	ContainerID   string
	ContainerName string
	ModelName     string
	Port          int
	Voice         string
	TextPrompt    string
	StartedAt     time.Time
	Status        string
}

func (c *Client) ListVoicaaContainers(ctx context.Context) ([]RunningInstance, error) {
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(
			filters.Arg("label", LabelModel),
		),
	})
	if err != nil {
		return nil, err
	}
	var instances []RunningInstance
	for _, ctr := range containers {
		port, _ := strconv.Atoi(ctr.Labels[LabelPort])
		name := ""
		if len(ctr.Names) > 0 {
			name = ctr.Names[0]
			if len(name) > 0 && name[0] == '/' {
				name = name[1:]
			}
		}
		instances = append(instances, RunningInstance{
			ContainerID:   ctr.ID[:12],
			ContainerName: name,
			ModelName:     ctr.Labels[LabelModel],
			Port:          port,
			Voice:         ctr.Labels[LabelVoice],
			TextPrompt:    ctr.Labels[LabelPrompt],
			StartedAt:     time.Unix(ctr.Created, 0),
			Status:        ctr.Status,
		})
	}
	return instances, nil
}

func (c *Client) FindContainerByModel(ctx context.Context, modelName string) (*RunningInstance, error) {
	instances, err := c.ListVoicaaContainers(ctx)
	if err != nil {
		return nil, err
	}
	for _, inst := range instances {
		if inst.ModelName == modelName {
			return &inst, nil
		}
	}
	return nil, fmt.Errorf("no running instance found for model %q", modelName)
}

func (c *Client) StopContainer(ctx context.Context, containerID string, timeoutSec int) error {
	timeout := timeoutSec
	return c.cli.ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: &timeout,
	})
}

func (c *Client) RemoveContainer(ctx context.Context, containerID string) error {
	return c.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
}

func (c *Client) StreamLogs(ctx context.Context, containerID string, w io.Writer) error {
	reader, err := c.cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return err
	}
	defer reader.Close()
	_, err = io.Copy(w, reader)
	return err
}
