package cmd

import (
	"github.com/spf13/cobra"
)

var releasesCmd = &cobra.Command{
	Use:   "releases",
	Short: "Manage releases for tracking deployments",
	Long: `Manage releases for tracking deployments.

Track software releases, register deploys, and finalize releases
for deployment tracking and error attribution.

Available subcommands:
  new      - Create a new release
  finalize - Mark a release as finalized
  deploy   - Create, deploy, and finalize in one command
  list     - List all releases

Examples:
  tracekit releases new v1.2.3
  tracekit releases new                  # auto-detect version
  tracekit releases deploy --env production
  tracekit releases finalize v1.2.3
  tracekit releases list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(releasesCmd)

	// Shared persistent flags for all release subcommands
	releasesCmd.PersistentFlags().Bool("json", false, "Output in JSON format")
	releasesCmd.PersistentFlags().Bool("dev", false, "Use development API endpoint")
	releasesCmd.PersistentFlags().MarkHidden("dev")
	releasesCmd.PersistentFlags().String("service", "", "Service name to scope the release to")

	releasesCmd.AddCommand(releasesNewCmd)
	releasesCmd.AddCommand(releasesFinalizeCmd)
	releasesCmd.AddCommand(releasesDeployCmd)
	releasesCmd.AddCommand(releasesListCmd)
}
