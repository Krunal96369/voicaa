package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	debug   bool
	Version = "dev"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "voicaa",
		Short: "Ollama for voice — run speech-to-speech AI models locally",
		Long: `voicaa makes it easy to pull, serve, and manage speech-to-speech
AI models. Run real-time voice agents on your own hardware.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.voicaa/config.yaml)")
	root.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logging")

	root.AddCommand(newPullCmd())
	root.AddCommand(newServeCmd())
	root.AddCommand(newRunCmd())
	root.AddCommand(newModelsCmd())
	root.AddCommand(newVoicesCmd())
	root.AddCommand(newPsCmd())
	root.AddCommand(newStopCmd())
	root.AddCommand(newVersionCmd())

	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print voicaa version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("voicaa version %s\n", Version)
		},
	}
}

func Execute() error {
	return newRootCmd().Execute()
}
