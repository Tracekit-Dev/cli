package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set by main.go via ldflags
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "tracekit",
	Short: "TraceKit CLI - Zero-friction APM setup",
	Long: `TraceKit CLI enables single-command account creation, framework detection,
and SDK installation for application monitoring.

Examples:
  tracekit init              Initialize TraceKit in your project
  tracekit login             Login to existing account
  tracekit status            Show configuration and usage
  tracekit upgrade           Upgrade your subscription plan`,
	Version: Version,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Custom version template
	rootCmd.SetVersionTemplate(fmt.Sprintf("TraceKit CLI %s\n", Version))
}
