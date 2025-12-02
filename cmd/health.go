package cmd

import (
	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Manage health checks for your services",
	Long: `Manage health checks for your services.

Available subcommands:
  setup - Configure a new health check
  list  - List all configured health checks

Example:
  tracekit health setup
  tracekit health list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Show help if no subcommand
		return cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(healthCmd)
	healthCmd.AddCommand(healthSetupCmd)
	healthCmd.AddCommand(healthListCmd)
}
