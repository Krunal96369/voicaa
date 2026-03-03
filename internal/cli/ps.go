package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/Krunal96369/voicaa/internal/config"
	"github.com/Krunal96369/voicaa/internal/service"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func newPsCmd() *cobra.Command {
	var quiet bool

	cmd := &cobra.Command{
		Use:   "ps",
		Short: "Show running model instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPs(quiet)
		},
	}

	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "only print container IDs")

	return cmd
}

func runPs(quiet bool) error {
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

	instances, err := is.Ps(context.Background())
	if err != nil {
		return err
	}

	if len(instances) == 0 {
		fmt.Println("No running models.")
		return nil
	}

	if quiet {
		for _, inst := range instances {
			fmt.Println(string(inst.ID))
		}
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"MODEL", "CONTAINER", "PORT", "VOICE", "STATUS"})
	table.SetBorder(false)
	table.SetColumnSeparator("")
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)

	for _, inst := range instances {
		table.Append([]string{
			inst.ModelName,
			inst.Name,
			fmt.Sprintf("%d", inst.Port),
			inst.Voice,
			inst.Status,
		})
	}

	table.Render()
	return nil
}
