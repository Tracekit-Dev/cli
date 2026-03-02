package cmd

import (
	"github.com/spf13/cobra"
)

var sourcemapsCmd = &cobra.Command{
	Use:   "sourcemaps",
	Short: "Manage source maps for stack trace symbolication",
	Long: `Manage source maps for stack trace symbolication.

Upload source maps from your build output to enable human-readable stack traces
in production error reports. Source maps are linked to releases via debug IDs
injected into your JavaScript files during upload.

Available subcommands:
  upload   - Upload source maps with automatic debug ID injection
  delete   - Delete source maps for a release

Examples:
  tracekit sourcemaps upload ./dist
  tracekit sourcemaps upload ./dist --release v1.2.3
  tracekit sourcemaps delete --release v1.2.3
  tracekit sourcemaps upload --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(sourcemapsCmd)

	// Shared persistent flags for all sourcemap subcommands
	sourcemapsCmd.PersistentFlags().Bool("json", false, "Output in JSON format")
	sourcemapsCmd.PersistentFlags().Bool("dev", false, "Use development API endpoint")
	sourcemapsCmd.PersistentFlags().MarkHidden("dev")

	sourcemapsCmd.AddCommand(sourcemapsUploadCmd)
	sourcemapsCmd.AddCommand(sourcemapsDeleteCmd)
}
