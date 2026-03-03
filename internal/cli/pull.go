package cli

import (
	"context"
	"fmt"

	"github.com/Krunal96369/voicaa/internal/config"
	"github.com/Krunal96369/voicaa/internal/service"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

func newPullCmd() *cobra.Command {
	var token string
	var skipDocker bool
	var force bool

	cmd := &cobra.Command{
		Use:   "pull <model>",
		Short: "Download model weights and Docker image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPull(args[0], token, skipDocker, force)
		},
	}

	cmd.Flags().StringVar(&token, "token", "", "HuggingFace token (overrides config/env)")
	cmd.Flags().BoolVar(&skipDocker, "skip-docker", false, "skip Docker image pull")
	cmd.Flags().BoolVar(&force, "force", false, "re-download even if files exist")

	return cmd
}

func runPull(modelName, token string, skipDocker, force bool) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	ms := service.NewModelService(cfg)

	manifest, err := ms.FindModel(modelName)
	if err != nil {
		return err
	}

	fmt.Printf("Resolving model: %s -> %s\n", modelName, manifest.HuggingFace.Repo)

	if !force && ms.AlreadyDownloaded(modelName) {
		fmt.Printf("Model %q already downloaded. Use --force to re-download.\n", manifest.Name)
	}

	fmt.Printf("\nDownloading model files to %s\n", ms.Store.ModelDir(manifest.Name))

	// Track per-file progress bars
	bars := make(map[string]*progressbar.ProgressBar)

	err = ms.Pull(context.Background(), modelName, token, skipDocker, force, func(p service.PullProgress) {
		if p.Done {
			return
		}
		bar, exists := bars[p.File]
		if !exists {
			bar = progressbar.NewOptions64(
				p.Total,
				progressbar.OptionSetDescription(fmt.Sprintf("  %-45s", p.File)),
				progressbar.OptionSetWidth(30),
				progressbar.OptionShowBytes(true),
				progressbar.OptionShowCount(),
				progressbar.OptionOnCompletion(func() { fmt.Println() }),
			)
			bars[p.File] = bar
		}
		bar.Set64(p.Downloaded)
	})
	if err != nil {
		return err
	}

	fmt.Printf("\nModel %q downloaded successfully.\n", manifest.Name)
	return nil
}
