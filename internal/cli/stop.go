package cli

import (
	"context"
	"fmt"

	"github.com/Krunal96369/voicaa/internal/config"
	"github.com/Krunal96369/voicaa/internal/service"
	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	var force bool
	var timeout int

	cmd := &cobra.Command{
		Use:   "stop <model>",
		Short: "Stop a running model instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStop(args[0], force, timeout)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "force stop")
	cmd.Flags().IntVar(&timeout, "timeout", 10, "seconds to wait before force-killing")

	return cmd
}

func runStop(modelName string, force bool, timeout int) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	b, err := createBackend(cfg, "")
	if err != nil {
		return err
	}
	if closer, ok := b.(interface{ Close() error }); ok {
		defer closer.Close()
	}

	ms := service.NewModelService(cfg)
	is := service.NewInstanceService(cfg, b, ms)

	fmt.Printf("Stopping %s...", modelName)

	if err := is.Stop(context.Background(), modelName, force, timeout); err != nil {
		return err
	}

	fmt.Println("done.")
	return nil
}
