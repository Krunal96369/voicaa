package cli

import (
	"fmt"
	"os"

	"github.com/Krunal96369/voicaa/internal/config"
	"github.com/Krunal96369/voicaa/internal/service"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func newVoicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "voices <model>",
		Short: "List available voices for a model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVoices(args[0])
		},
	}

	return cmd
}

func runVoices(modelName string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	ms := service.NewModelService(cfg)
	manifest, err := ms.Voices(modelName)
	if err != nil {
		return err
	}

	if len(manifest.Voices.Voices) == 0 {
		fmt.Printf("No voices defined for model %q\n", manifest.Name)
		return nil
	}

	fmt.Printf("Voices for %s (default: %s)\n\n", manifest.Name, manifest.Voices.DefaultVoice)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"VOICE", "GENDER", "CATEGORY", "FILE"})
	table.SetBorder(false)
	table.SetColumnSeparator("")
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)

	for _, v := range manifest.Voices.Voices {
		name := v.Name
		if v.Name == manifest.Voices.DefaultVoice {
			name = v.Name + " (default)"
		}
		table.Append([]string{name, v.Gender, v.Category, v.File})
	}

	table.Render()
	return nil
}
