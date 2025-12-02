package cmd

import (
	"github.com/spf13/cobra"
)

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Manage webhooks for event notifications",
	Long: `Manage webhooks for event notifications.

Webhooks allow you to receive real-time notifications when events occur,
such as health check failures, alerts being triggered, or traces with errors.

Available subcommands:
  create - Create a new webhook
  list   - List all configured webhooks
  delete - Delete a webhook

Example:
  tracekit webhook create
  tracekit webhook list
  tracekit webhook delete <webhook-id>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Show help if no subcommand
		return cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(webhookCmd)

	// Add --dev flag to all webhook commands
	webhookCmd.PersistentFlags().Bool("dev", false, "Use development API endpoint")
	webhookCmd.PersistentFlags().MarkHidden("dev")

	webhookCmd.AddCommand(webhookCreateCmd)
	webhookCmd.AddCommand(webhookListCmd)
	webhookCmd.AddCommand(webhookDeleteCmd)
}
