package main

import (
	"os"

	"github.com/Krunal96369/voicaa/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
