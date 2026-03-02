package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version, CommitHash, and BuildDate are set via ldflags at build time
var Version = "dev"
var CommitHash = "unknown"
var BuildDate = "unknown"

var rootCmd = &cobra.Command{
	Use:   "tracekit",
	Short: "TraceKit CLI - Zero-friction APM setup",
	Long: `TraceKit CLI enables single-command account creation, framework detection,
and SDK installation for application monitoring.

Examples:
  tracekit init              Initialize TraceKit in your project
  tracekit login             Login to existing account
  tracekit status            Show configuration and usage
  tracekit upgrade           Upgrade your subscription plan
  tracekit update            Update the CLI to the latest version`,
	Version: Version,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Custom version template with commit hash and build date
	rootCmd.SetVersionTemplate(fmt.Sprintf("tracekit v%s (%s, %s)\n", Version, CommitHash, BuildDate))
}
