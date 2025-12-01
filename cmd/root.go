package cmd

import (
	"github.com/spf13/cobra"
)

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
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags can be added here
	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.tracekit.yaml)")
}
