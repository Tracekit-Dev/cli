package cmd

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/config"
)

var webhookDeleteCmd = &cobra.Command{
	Use:   "delete <webhook-id>",
	Short: "Delete a webhook",
	Long: `Delete a webhook by its ID.

This will permanently remove the webhook and stop all future deliveries.

Example:
  tracekit webhook delete 550e8400-e29b-41d4-a716-446655440000`,
	Args: cobra.ExactArgs(1),
	RunE: runWebhookDelete,
}

func runWebhookDelete(cmd *cobra.Command, args []string) error {
	webhookID := args[0]

	// Load config
	cfg, err := config.Read()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.APIKey == "" {
		return fmt.Errorf("not authenticated. Run 'tracekit login' first")
	}

	// Check if --dev flag is set
	useDev, _ := cmd.Flags().GetBool("dev")
	if useDev {
		cfg.Endpoint = "http://localhost:8081"
	}

	// Confirm deletion
	reader := bufio.NewReader(os.Stdin)
	yellow := color.New(color.FgYellow, color.Bold)
	yellow.Printf("\n⚠️  Are you sure you want to delete webhook %s? (y/N): ", webhookID)

	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm != "y" && confirm != "yes" {
		fmt.Println("Deletion cancelled.")
		return nil
	}

	// Send request
	req, err := http.NewRequest("DELETE", cfg.GetAPIBase()+"/v1/webhooks/"+webhookID, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-Key", cfg.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("webhook not found")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete webhook: %s", string(body))
	}

	// Success
	green := color.New(color.FgGreen, color.Bold)
	green.Println("\n✅ Webhook deleted successfully!")

	return nil
}
