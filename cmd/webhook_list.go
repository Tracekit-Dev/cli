package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/config"
)

var webhookListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured webhooks",
	Long: `List all configured webhooks for your organization.

Shows webhook details including name, URL, subscribed events, and delivery statistics.

Example:
  tracekit webhook list`,
	RunE: runWebhookList,
}

func runWebhookList(cmd *cobra.Command, args []string) error {
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

	// Send request
	req, err := http.NewRequest("GET", cfg.GetAPIBase()+"/v1/webhooks", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-Key", cfg.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to list webhooks: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to list webhooks: %s", string(body))
	}

	// Parse response
	var result struct {
		Webhooks []struct {
			ID                   string    `json:"id"`
			Name                 string    `json:"name"`
			URL                  string    `json:"url"`
			Description          string    `json:"description"`
			Events               []string  `json:"events"`
			Enabled              bool      `json:"enabled"`
			Status               string    `json:"status"`
			TotalDeliveries      int       `json:"total_deliveries"`
			SuccessfulDeliveries int       `json:"successful_deliveries"`
			FailedDeliveries     int       `json:"failed_deliveries"`
			LastDeliveryAt       *string   `json:"last_delivery_at"`
			CreatedAt            string    `json:"created_at"`
		} `json:"webhooks"`
		Total int `json:"total"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Display results
	if result.Total == 0 {
		fmt.Println("\n📭 No webhooks configured yet.")
		fmt.Println("\nCreate your first webhook:")
		fmt.Println("  tracekit webhook create")
		return nil
	}

	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed)
	cyan := color.New(color.FgCyan, color.Bold)

	fmt.Printf("\n📬 Found %d webhook(s):\n\n", result.Total)

	for i, webhook := range result.Webhooks {
		// Header
		cyan.Printf("─── %s ───\n", webhook.Name)
		fmt.Printf("ID:          %s\n", webhook.ID)
		fmt.Printf("URL:         %s\n", webhook.URL)

		if webhook.Description != "" {
			fmt.Printf("Description: %s\n", webhook.Description)
		}

		// Status
		fmt.Print("Status:      ")
		if webhook.Enabled && webhook.Status == "active" {
			green.Println("✓ Active")
		} else if !webhook.Enabled {
			yellow.Println("⏸ Disabled")
		} else {
			red.Printf("✗ %s\n", webhook.Status)
		}

		// Events
		fmt.Println("Events:")
		for _, event := range webhook.Events {
			fmt.Printf("  • %s\n", event)
		}

		// Statistics
		successRate := 0.0
		if webhook.TotalDeliveries > 0 {
			successRate = float64(webhook.SuccessfulDeliveries) / float64(webhook.TotalDeliveries) * 100
		}

		fmt.Printf("Deliveries:  %d total", webhook.TotalDeliveries)
		if webhook.TotalDeliveries > 0 {
			fmt.Printf(" (%d successful, %d failed, %.1f%% success rate)",
				webhook.SuccessfulDeliveries, webhook.FailedDeliveries, successRate)
		}
		fmt.Println()

		if webhook.LastDeliveryAt != nil {
			lastDelivery, _ := time.Parse(time.RFC3339, *webhook.LastDeliveryAt)
			fmt.Printf("Last Delivery: %s\n", lastDelivery.Format("2006-01-02 15:04:05"))
		}

		// Created
		created, _ := time.Parse(time.RFC3339, webhook.CreatedAt)
		fmt.Printf("Created:     %s\n", created.Format("2006-01-02 15:04:05"))

		if i < len(result.Webhooks)-1 {
			fmt.Println()
		}
	}

	fmt.Println()
	return nil
}
