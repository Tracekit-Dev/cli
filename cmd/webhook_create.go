package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/config"
)

var webhookCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new webhook",
	Long: `Create a new webhook to receive event notifications.

You will be prompted to configure:
- Webhook name
- Destination URL
- Event types to subscribe to

Example:
  tracekit webhook create`,
	RunE: runWebhookCreate,
}

func runWebhookCreate(cmd *cobra.Command, args []string) error {
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

	reader := bufio.NewReader(os.Stdin)

	// Get webhook name
	fmt.Print("\n🔗 Webhook name: ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("webhook name is required")
	}

	// Get webhook URL
	if useDev {
		fmt.Print("📡 Webhook URL (http:// or https://): ")
	} else {
		fmt.Print("📡 Webhook URL (HTTPS required, or HTTP for localhost): ")
	}
	url, _ := reader.ReadString('\n')
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("webhook URL is required")
	}

	// Validate URL format
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return fmt.Errorf("webhook URL must start with https:// or http://")
	}

	// Enforce HTTPS for non-localhost URLs (unless in dev mode)
	if !useDev && !strings.HasPrefix(url, "https://") && !strings.Contains(url, "localhost") && !strings.Contains(url, "127.0.0.1") {
		return fmt.Errorf("webhook URL must use HTTPS for non-localhost URLs (got: %s)", url)
	}

	// Get description (optional)
	fmt.Print("📝 Description (optional): ")
	description, _ := reader.ReadString('\n')
	description = strings.TrimSpace(description)

	// Show available events
	availableEvents := []string{
		"health_check.failed",
		"health_check.recovered",
		"alert.triggered",
		"alert.resolved",
		"trace.error",
		"anomaly.detected",
	}

	fmt.Println("\n📋 Available event types:")
	for i, event := range availableEvents {
		fmt.Printf("  %d. %s\n", i+1, event)
	}

	// Get event selection
	fmt.Print("\n🎯 Select events (comma-separated numbers, e.g., 1,3,4): ")
	eventsInput, _ := reader.ReadString('\n')
	eventsInput = strings.TrimSpace(eventsInput)

	// Parse selected events
	selectedEvents := []string{}
	if eventsInput != "" {
		selections := strings.Split(eventsInput, ",")
		for _, sel := range selections {
			sel = strings.TrimSpace(sel)
			var eventNum int
			fmt.Sscanf(sel, "%d", &eventNum)
			if eventNum >= 1 && eventNum <= len(availableEvents) {
				selectedEvents = append(selectedEvents, availableEvents[eventNum-1])
			}
		}
	}

	if len(selectedEvents) == 0 {
		return fmt.Errorf("at least one event must be selected")
	}

	// Create webhook payload
	payload := map[string]interface{}{
		"name":        name,
		"url":         url,
		"description": description,
		"events":      selectedEvents,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to create payload: %w", err)
	}

	// Send request
	req, err := http.NewRequest("POST", cfg.GetAPIBase()+"/v1/webhooks", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", cfg.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create webhook: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create webhook: %s", string(body))
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Display success with secret
	green := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow, color.Bold)

	fmt.Println("\n✅ Webhook created successfully!")
	fmt.Printf("\n📦 Webhook ID: %s\n", result["id"])
	fmt.Printf("🔗 Name: %s\n", result["name"])
	fmt.Printf("📡 URL: %s\n", result["url"])

	// Show secret prominently
	secret := result["secret"]
	if secret != nil {
		yellow.Println("\n⚠️  IMPORTANT: Save this secret securely!")
		fmt.Printf("🔐 Secret: %s\n", secret)
		yellow.Println("\nThis secret will only be shown once. You'll need it to verify webhook signatures.")
		fmt.Println("\nExample verification code:")
		fmt.Println("  const crypto = require('crypto');")
		fmt.Println("  const signature = req.headers['x-webhook-signature'];")
		fmt.Printf("  const secret = '%s';\n", secret)
		fmt.Println("  const hash = crypto.createHmac('sha256', secret)")
		fmt.Println("                    .update(JSON.stringify(req.body))")
		fmt.Println("                    .digest('hex');")
		fmt.Println("  const expected = 'sha256=' + hash;")
		fmt.Println("  if (signature === expected) { /* verified */ }")
	}

	fmt.Printf("\n📋 Subscribed events:\n")
	for _, event := range selectedEvents {
		green.Printf("  ✓ %s\n", event)
	}

	return nil
}
