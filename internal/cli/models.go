package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/Krunal96369/voicaa/internal/config"
	"github.com/Krunal96369/voicaa/internal/service"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func newModelsCmd() *cobra.Command {
	var jsonOutput bool
	var quiet bool

	cmd := &cobra.Command{
		Use:     "models",
		Aliases: []string{"ls", "list"},
		Short:   "List downloaded models",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModels(jsonOutput, quiet)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "only print model names")

	return cmd
}

func runModels(jsonOutput, quiet bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ms := service.NewModelService(cfg)
	models, err := ms.ListLocal()
	if err != nil {
		return err
	}

	if len(models) == 0 {
		fmt.Println("No models downloaded yet.")
		fmt.Println()
		fmt.Println("Pull a model:")
		fmt.Println("  voicaa pull moshi")
		fmt.Println()
		fmt.Println("Available models:")
		registryModels, _ := ms.ListRegistry()
		for _, m := range registryModels {
			fmt.Printf("  %-20s %s\n", m.Name, m.Description)
		}
		return nil
	}

	if quiet {
		for _, m := range models {
			fmt.Println(m.Name)
		}
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"MODEL", "SIZE", "STATUS", "PULLED"})
	table.SetBorder(false)
	table.SetColumnSeparator("")
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)

	for _, m := range models {
		status := color.RedString("incomplete")
		if m.Complete {
			status = color.GreenString("complete")
		}
		table.Append([]string{
			m.Name,
			formatBytes(m.TotalSizeBytes),
			status,
			m.DownloadedAt.Format("2006-01-02 15:04"),
		})
	}

	table.Render()
	return nil
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
