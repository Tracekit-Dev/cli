package cmd

import (
	"github.com/spf13/cobra"
)

var deploysCmd = &cobra.Command{
	Use:   "deploys",
	Short: "Manage deployment records for releases",
	Long: `Manage deployment records for releases.

Register deploys to track which releases are running in which environments.

Available subcommands:
  new - Register a new deploy for a release

Examples:
  tracekit deploys new v1.2.3 --env production
  tracekit deploys new --env staging --deployer deploy-bot`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(deploysCmd)

	// Shared persistent flags for all deploy subcommands
	deploysCmd.PersistentFlags().Bool("json", false, "Output in JSON format")
	deploysCmd.PersistentFlags().Bool("dev", false, "Use development API endpoint")
	deploysCmd.PersistentFlags().MarkHidden("dev")
	deploysCmd.PersistentFlags().String("service", "", "Service name to scope the deploy to")

	deploysCmd.AddCommand(deploysNewCmd)
}
