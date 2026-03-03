package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/Krunal96369/voicaa/internal/api"
	"github.com/Krunal96369/voicaa/internal/backend"
	"github.com/Krunal96369/voicaa/internal/backend/cloud"
	"github.com/Krunal96369/voicaa/internal/backend/cloud/runpod"
	"github.com/Krunal96369/voicaa/internal/backend/cloud/vastai"
	dockerbackend "github.com/Krunal96369/voicaa/internal/backend/docker"
	mlxbackend "github.com/Krunal96369/voicaa/internal/backend/mlx"
	"github.com/Krunal96369/voicaa/internal/config"
	"github.com/Krunal96369/voicaa/internal/daemon"
	"github.com/Krunal96369/voicaa/internal/service"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	var port int
	var voice string
	var prompt string
	var cpuOffload bool
	var device string
	var detach bool
	var name string
	var gpuIDs string
	var daemonMode bool
	var daemonAddr string
	var backendType string

	cmd := &cobra.Command{
		Use:   "serve [model]",
		Short: "Start the daemon, or serve a specific model",
		Long: `Without arguments, starts the voicaa daemon (HTTP API server).
With a model name, starts the inference server for that model.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || daemonMode {
				return runDaemon(daemonAddr, backendType)
			}
			return runServe(args[0], port, voice, prompt, cpuOffload, device, detach, name, gpuIDs, backendType)
		},
	}

	cmd.Flags().IntVar(&port, "port", 0, "host port (default: model default)")
	cmd.Flags().StringVar(&voice, "voice", "", "voice embedding name (default: model default)")
	cmd.Flags().StringVar(&prompt, "prompt", "", "system text prompt")
	cmd.Flags().BoolVar(&cpuOffload, "cpu-offload", false, "enable CPU offload for lower VRAM")
	cmd.Flags().StringVar(&device, "device", "cuda", "device: cuda, cpu")
	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "run in background")
	cmd.Flags().StringVar(&name, "name", "", "container name (default: voicaa-<model>)")
	cmd.Flags().StringVar(&gpuIDs, "gpu-ids", "", "GPU device IDs (e.g., \"0\" or \"0,1\")")
	cmd.Flags().BoolVar(&daemonMode, "daemon", false, "start in daemon mode (used internally)")
	cmd.Flags().StringVar(&daemonAddr, "daemon-addr", "", "daemon listen address (default: localhost:8899)")
	cmd.Flags().StringVar(&backendType, "backend", "", "backend: docker, cloud, mlx (default: auto)")
	cmd.Flags().MarkHidden("daemon")

	return cmd
}

// createBackend creates the appropriate backend based on config and flag.
func createBackend(cfg *config.Config, backendType string) (backend.Backend, error) {
	// Auto-detect
	if backendType == "" {
		if cfg.Cloud.Provider != "" && cfg.Cloud.APIKey != "" {
			backendType = "cloud"
		} else if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
			// On Apple Silicon Mac, prefer MLX if uv is available
			uvPath := cfg.MLX.UVPath
			if uvPath == "" {
				uvPath = "uv"
			}
			if _, err := exec.LookPath(uvPath); err == nil {
				backendType = "mlx"
			} else {
				backendType = "docker"
			}
		} else {
			backendType = "docker"
		}
	}

	switch backendType {
	case "docker":
		b, err := dockerbackend.New()
		if err != nil {
			return nil, fmt.Errorf("Docker is required: %w", err)
		}
		return b, nil

	case "cloud":
		provider, err := createCloudProvider(cfg)
		if err != nil {
			return nil, err
		}
		home, _ := os.UserHomeDir()
		statePath := filepath.Join(home, config.DefaultConfigDir, "cloud-instances.yaml")
		return cloud.New(cloud.CloudBackendConfig{
			Provider:  provider,
			StatePath: statePath,
			HFToken:   cfg.HFToken,
			GPUType:   cfg.Cloud.GPUType,
			DiskGB:    cfg.Cloud.DiskGB,
		}), nil

	case "mlx":
		home, _ := os.UserHomeDir()
		statePath := filepath.Join(home, config.DefaultConfigDir, "mlx-instances.yaml")
		b, err := mlxbackend.New(mlxbackend.Config{
			StatePath: statePath,
			UVPath:    cfg.MLX.UVPath,
		})
		if err != nil {
			return nil, err
		}
		return b, nil

	default:
		return nil, fmt.Errorf("unknown backend: %s (valid: docker, cloud, mlx)", backendType)
	}
}

func createCloudProvider(cfg *config.Config) (cloud.CloudProvider, error) {
	if cfg.Cloud.APIKey == "" {
		return nil, fmt.Errorf("cloud API key required: set cloud.api_key in config or VOICAA_CLOUD_API_KEY env var")
	}

	switch cfg.Cloud.Provider {
	case "runpod":
		return runpod.New(runpod.Config{
			APIKey:    cfg.Cloud.APIKey,
			GPUType:   cfg.Cloud.GPUType,
			CloudType: cfg.Cloud.RunPod.CloudType,
			Region:    cfg.Cloud.RunPod.Region,
		}), nil
	case "vastai":
		return vastai.New(vastai.Config{APIKey: cfg.Cloud.APIKey}), nil
	default:
		return nil, fmt.Errorf("unknown cloud provider: %s (valid: runpod, vastai)", cfg.Cloud.Provider)
	}
}

func runDaemon(addr, backendType string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if addr == "" {
		addr = daemon.Addr(cfg.DaemonPort)
	}

	b, err := createBackend(cfg, backendType)
	if err != nil {
		return err
	}
	if closer, ok := b.(interface{ Close() error }); ok {
		defer closer.Close()
	}

	ms := service.NewModelService(cfg)
	is := service.NewInstanceService(cfg, b, ms)

	srv := api.NewServer(ms, is, addr, Version)

	// Write PID file
	pidFile := daemon.PidPath()
	os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
	defer os.Remove(pidFile)

	// Graceful shutdown on signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("shutting down daemon...")
		srv.Shutdown(ctx)
		cancel()
	}()

	fmt.Printf("voicaa daemon starting on %s\n", addr)
	fmt.Printf("  API:     http://%s/api/v1/\n", addr)
	fmt.Printf("  UI:      http://%s/\n", addr)
	fmt.Printf("  Backend: %s\n", b.Name())

	if err := srv.Start(); err != nil {
		// http.ErrServerClosed is expected on graceful shutdown
		if err.Error() != "http: Server closed" {
			return err
		}
	}

	return nil
}

func runServe(modelName string, port int, voice, prompt string, cpuOffload bool, device string, detach bool, containerName, gpuIDs, backendType string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	b, err := createBackend(cfg, backendType)
	if err != nil {
		return err
	}
	if closer, ok := b.(interface{ Close() error }); ok {
		defer closer.Close()
	}

	ms := service.NewModelService(cfg)
	is := service.NewInstanceService(cfg, b, ms)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := service.ServeOptions{
		ModelName:     modelName,
		Port:          port,
		Voice:         voice,
		Prompt:        prompt,
		CpuOffload:    cpuOffload,
		Device:        device,
		Detach:        detach,
		ContainerName: containerName,
		GPUIDs:        gpuIDs,
	}

	fmt.Printf("Starting %s...\n", modelName)
	fmt.Printf("  Backend: %s\n", b.Name())
	if voice != "" {
		fmt.Printf("  Voice:   %s\n", voice)
	}
	if prompt != "" {
		fmt.Printf("  Prompt:  %s\n", prompt)
	}
	if cpuOffload {
		fmt.Println("  Mode:    CPU offload enabled")
	}

	info, err := is.Serve(ctx, opts)
	if err != nil {
		return err
	}

	fmt.Printf("  Instance: %s (%s)\n", info.Name, string(info.ID)[:min(12, len(info.ID))])

	if info.Status == "ready" {
		fmt.Printf("\nServer ready!\n")
		fmt.Printf("  WebSocket: %s\n", info.WebSocketURL)
	} else {
		fmt.Printf("\nWarning: health check timed out\n")
		fmt.Println("The model may still be loading.")
	}

	if detach {
		fmt.Printf("\nRunning in background. Stop with: voicaa stop %s\n", info.ModelName)
		return nil
	}

	fmt.Println("\nStreaming logs (Ctrl+C to stop)...")
	fmt.Println("---")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\n---\nStopping...")
		b.Stop(context.Background(), info.ID, 10)
		b.Remove(context.Background(), info.ID)
		cancel()
	}()

	b.Logs(ctx, info.ID, os.Stdout)

	return nil
}
