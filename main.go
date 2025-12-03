package main

import (
	"fmt"
	"os"

	"github.com/yourusername/context.io/cli/cmd"
)

// Version is set via ldflags during build
var Version = "dev"

func main() {
	cmd.Version = Version
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
