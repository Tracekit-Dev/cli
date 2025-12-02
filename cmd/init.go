package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/client"
	"github.com/yourusername/context.io/cli/internal/config"
	"github.com/yourusername/context.io/cli/internal/detector"
	"github.com/yourusername/context.io/cli/internal/trace"
	"github.com/yourusername/context.io/cli/internal/ui"
	"github.com/yourusername/context.io/cli/internal/utils"
)

const CLIVersion = "1.0.0"

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize TraceKit in your project",
	Long: `Initialize TraceKit by creating an account, detecting your framework,
and automatically configuring your project for monitoring.

This command will:
  1. Detect your framework (gemvc, Laravel, Express, etc.)
  2. Create a TraceKit account (or use existing)
  3. Generate an API key
  4. Create .env file with configuration
  5. Provide setup instructions

Example:
  tracekit init`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().String("email", "", "Your email address")
	initCmd.Flags().String("api-url", "", "API base URL (default: https://api.tracekit.dev)")
	initCmd.Flags().Bool("dev", false, "")
	initCmd.Flags().MarkHidden("dev")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Print beautiful banner
	ui.PrintBanner()
	fmt.Println()

	// Step 1: Detect framework
	ui.PrintSection("🔍 Framework Detection")
	fmt.Println()
	framework, err := detector.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect framework: %w", err)
	}

	if framework.Name == "generic" {
		ui.PrintWarning("No framework detected")
		ui.PrintMuted("   Continuing with generic setup...")
	} else {
		ui.PrintSuccess(fmt.Sprintf("Detected: %s (%s)", framework.Name, framework.Type))
	}
	fmt.Println()

	// Step 2: Get email
	email, _ := cmd.Flags().GetString("email")
	if email == "" {
		email, err = promptEmail()
		if err != nil {
			return err
		}
	}

	// Get service name from directory (auto-detect, no prompt)
	cwd, _ := os.Getwd()
	serviceName := filepath.Base(cwd)
	// Sanitize service name: replace spaces with dashes, lowercase
	serviceName = strings.ToLower(strings.ReplaceAll(serviceName, " ", "-"))

	// Determine API URL
	apiURL, _ := cmd.Flags().GetString("api-url")
	useDev, _ := cmd.Flags().GetBool("dev")
	if useDev {
		apiURL = client.DevBaseURL
		ui.PrintInfo("Using development API: " + apiURL)
		fmt.Println()
	}

	// Step 3: Register account
	ui.PrintSection("📧 Account Creation")
	fmt.Println()
	apiClient := client.NewClient(apiURL)

	registerReq := &client.RegisterRequest{
		Email:            email,
		OrganizationName: serviceName,
		ServiceName:      serviceName,
		Source:           framework.Name,
		SourceMetadata: map[string]interface{}{
			"cli_version":       CLIVersion,
			"framework_version": framework.Version,
			"platform":          runtime.GOOS + "_" + runtime.GOARCH,
		},
	}

	registerResp, err := apiClient.Register(registerReq)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	ui.PrintSuccess(fmt.Sprintf("Verification code sent to %s", email))
	fmt.Println()

	// Step 4: Get verification code
	ui.PrintSection("🔑 Email Verification")
	fmt.Println()
	ui.PrintPrompt("Enter 6-digit code:")
	var code string
	fmt.Scanln(&code)
	fmt.Println()

	// Step 5: Verify and get API key
	ui.PrintInfo("Verifying...")
	verifyReq := &client.VerifyRequest{
		SessionID: registerResp.SessionID,
		Code:      code,
	}

	verifyResp, err := apiClient.Verify(verifyReq)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	ui.PrintSuccess("Account created!")
	fmt.Println()

	// Step 6: Save TraceKit config to .env
	cfg := &config.Config{
		APIKey:                 verifyResp.APIKey,
		Endpoint:               apiClient.BaseURL + "/v1/traces",
		ServiceName:            serviceName,
		Enabled:                "true",
		CodeMonitoringEnabled:  "true",
	}
	if err := config.Save(cfg); err != nil {
		ui.PrintWarning(fmt.Sprintf("Failed to save .env file: %v", err))
		fmt.Println()
		ui.PrintMuted("📝 Manual setup required:")
		ui.PrintMuted(fmt.Sprintf("   Add to your .env file: TRACEKIT_API_KEY=%s", verifyResp.APIKey))
		fmt.Println()

		// Show summary and exit if .env save failed
		ui.PrintDivider()
		fmt.Println()
		summary := fmt.Sprintf("Dashboard:  %s\nAPI Key:    %s\nService:    %s\nPlan:       Hacker (Free - 200k traces/month)",
			verifyResp.DashboardURL,
			utils.MaskAPIKey(verifyResp.APIKey),
			verifyResp.ServiceName)
		ui.PrintSummaryBox("⚠️  Setup Incomplete", summary)
		return nil
	}

	ui.PrintSuccess("API key saved to .env")
	fmt.Println()

	// Step 7: Send test trace automatically
	ui.PrintSection("🧪 Sending Test Trace")
	fmt.Println()
	ui.PrintInfo("Verifying your setup...")

	if err := sendTestTraceInternal(cfg); err != nil {
		ui.PrintWarning(fmt.Sprintf("Test trace failed: %v", err))
		ui.PrintMuted("   Don't worry, you can run 'tracekit test' later")
	} else {
		ui.PrintSuccess("Test trace sent successfully!")
	}
	fmt.Println()

	// Step 8: Show status automatically
	ui.PrintSection("📊 Integration Status")
	fmt.Println()

	if err := showStatusInternal(cfg, apiClient, useDev); err != nil {
		ui.PrintWarning(fmt.Sprintf("Could not fetch status: %v", err))
	}
	fmt.Println()

	// Step 9: Show final summary and next steps
	ui.PrintDivider()
	fmt.Println()

	summary := fmt.Sprintf("Dashboard:  %s\nAPI Key:    %s\nService:    %s\nPlan:       Hacker (Free - 200k traces/month)",
		verifyResp.DashboardURL,
		utils.MaskAPIKey(verifyResp.APIKey),
		verifyResp.ServiceName)

	ui.PrintSummaryBox("🎉 Setup Complete!", summary)
	fmt.Println()

	// Next steps based on framework
	var steps []string
	switch framework.Type {
	case "go":
		steps = []string{
			"Install SDK: go get github.com/yourusername/context.io/sdk",
			"Import in your code: import \"github.com/yourusername/context.io/sdk\"",
			"Initialize: tracekit.Init()",
			"Visit " + verifyResp.DashboardURL + " to view your test trace",
		}
	case "php":
		steps = []string{
			"Install SDK: composer require tracekit/sdk",
			"Require in your code: require 'vendor/autoload.php';",
			"Initialize: TraceKit\\SDK::init();",
			"Visit " + verifyResp.DashboardURL + " to view your test trace",
		}
	case "node":
		steps = []string{
			"Install SDK: npm install tracekit-sdk",
			"Import in your code: const tracekit = require('tracekit-sdk');",
			"Initialize: tracekit.init();",
			"Visit " + verifyResp.DashboardURL + " to view your test trace",
		}
	case "python":
		steps = []string{
			"Install SDK: pip install tracekit-sdk",
			"Import in your code: import tracekit",
			"Initialize: tracekit.init()",
			"Visit " + verifyResp.DashboardURL + " to view your test trace",
		}
	default:
		steps = []string{
			"Install the appropriate TraceKit SDK for your language",
			"Initialize with your API key",
			"Visit " + verifyResp.DashboardURL + " to view your test trace",
		}
	}

	ui.PrintNextSteps(steps)

	return nil
}

func promptEmail() (string, error) {
	ui.PrintPrompt("Enter your email:")
	reader := bufio.NewReader(os.Stdin)
	email, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read email: %w", err)
	}

	email = strings.TrimSpace(email)
	if email == "" {
		return "", fmt.Errorf("email is required")
	}

	return email, nil
}

// sendTestTraceInternal sends a test trace (reused from test.go logic)
func sendTestTraceInternal(cfg *config.Config) error {
	testTrace := trace.GenerateTestTrace(cfg.ServiceName)
	return trace.SendTrace(cfg, testTrace)
}

// showStatusInternal shows integration status (reused from status.go logic)
func showStatusInternal(cfg *config.Config, apiClient *client.Client, useDev bool) error {
	// Detect framework
	framework, _ := detector.Detect()
	if framework != nil && framework.Name != "generic" {
		ui.PrintMuted(fmt.Sprintf("   Framework: %s (%s)", framework.Name, framework.Type))
	}

	// Get integration status from API
	apiClient.APIKey = cfg.APIKey
	status, err := apiClient.GetStatus()
	if err != nil {
		return err
	}

	// Display integration status
	if statusStr, ok := status["status"].(string); ok && statusStr == "active" {
		if integration, ok := status["integration"].(map[string]interface{}); ok {
			ui.PrintMuted(fmt.Sprintf("   Service:     %v", integration["service_name"]))
			ui.PrintMuted(fmt.Sprintf("   Source:      %v", integration["source"]))

			if firstData := integration["first_data_at"]; firstData != nil {
				ui.PrintMuted(fmt.Sprintf("   First trace: %v", firstData))
			} else {
				ui.PrintMuted("   First trace: Just now!")
			}
		}
	}

	return nil
}
