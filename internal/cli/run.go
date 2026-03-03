package cli

import (
	"context"
	"fmt"

	"github.com/Krunal96369/voicaa/internal/config"
	"github.com/Krunal96369/voicaa/internal/service"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var token string
	var port int
	var voice string
	var prompt string
	var cpuOffload bool
	var device string
	var detach bool
	var name string
	var gpuIDs string

	cmd := &cobra.Command{
		Use:   "run <model>",
		Short: "Pull (if needed) and start serving a model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			ms := service.NewModelService(cfg)

			if !ms.AlreadyDownloaded(modelName) {
				fmt.Printf("Model %q not found locally. Pulling...\n\n", modelName)
				if err := runPull(modelName, token, false, false); err != nil {
					return err
				}
				fmt.Println()
			}

			// Now check if the daemon is running, otherwise start directly
			b, err := createBackend(cfg, "")
			if err != nil {
				return err
			}
			if closer, ok := b.(interface{ Close() error }); ok {
				defer closer.Close()
			}

			is := service.NewInstanceService(cfg, b, ms)

			ctx := context.Background()
			opts := service.ServeOptions{
				ModelName:     modelName,
				Port:          port,
				Voice:         voice,
				Prompt:        prompt,
				CpuOffload:    cpuOffload,
				Device:        device,
				Detach:        detach,
				ContainerName: name,
				GPUIDs:        gpuIDs,
			}

			info, err := is.Serve(ctx, opts)
			if err != nil {
				return err
			}

			fmt.Printf("  Container: %s (%s)\n", info.Name, string(info.ID)[:12])

			if info.Status == "ready" {
				fmt.Printf("\nServer ready!\n")
				fmt.Printf("  WebSocket: %s\n", info.WebSocketURL)
			} else {
				fmt.Printf("\nWarning: health check timed out\n")
				fmt.Println("The model may still be loading. Check logs with: docker logs -f", info.Name)
			}

			if detach {
				fmt.Printf("\nRunning in background. Stop with: voicaa stop %s\n", info.ModelName)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&token, "token", "", "HuggingFace token")
	cmd.Flags().IntVar(&port, "port", 0, "host port")
	cmd.Flags().StringVar(&voice, "voice", "", "voice embedding name")
	cmd.Flags().StringVar(&prompt, "prompt", "", "system text prompt")
	cmd.Flags().BoolVar(&cpuOffload, "cpu-offload", false, "enable CPU offload")
	cmd.Flags().StringVar(&device, "device", "cuda", "device: cuda, cpu")
	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "run in background")
	cmd.Flags().StringVar(&name, "name", "", "container name")
	cmd.Flags().StringVar(&gpuIDs, "gpu-ids", "", "GPU device IDs")

	return cmd
}
