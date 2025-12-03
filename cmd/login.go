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
	"github.com/yourusername/context.io/cli/internal/ui"
	"github.com/yourusername/context.io/cli/internal/utils"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to existing TraceKit account",
	Long: `Login to your existing TraceKit account and generate a new API key
for this project.

This command will:
  1. Verify your email with a verification code
  2. Generate a new API key for your organization
  3. Update your .env file with the new configuration

Example:
  tracekit login`,
	RunE: runLogin,
}

func init() {
	rootCmd.AddCommand(loginCmd)
	loginCmd.Flags().String("email", "", "Your email address")
	loginCmd.Flags().String("api-url", "", "API base URL (default: https://app.tracekit.dev)")
	loginCmd.Flags().Bool("dev", false, "")
	loginCmd.Flags().MarkHidden("dev")
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Print beautiful banner
	ui.PrintBanner()
	fmt.Println()

	// Step 1: Get email
	ui.PrintSection("📧 Account Login")
	fmt.Println()

	email, _ := cmd.Flags().GetString("email")
	if email == "" {
		var err error
		email, err = promptEmail()
		if err != nil {
			return err
		}
	}

	// Get service name from directory
	cwd, _ := os.Getwd()
	serviceName := strings.ToLower(strings.ReplaceAll(filepath.Base(cwd), " ", "-"))

	// Determine API URL
	apiURL, _ := cmd.Flags().GetString("api-url")
	useDev, _ := cmd.Flags().GetBool("dev")
	if useDev {
		apiURL = client.DevBaseURL
		ui.PrintInfo("Using development API: " + apiURL)
		fmt.Println()
	}

	// Step 2: Register session (same as init, but will get existing account)
	apiClient := client.NewClient(apiURL)

	registerReq := &client.RegisterRequest{
		Email:            email,
		OrganizationName: serviceName,
		ServiceName:      serviceName,
		Source:           "cli_login",
		SourceMetadata: map[string]interface{}{
			"cli_version": CLIVersion,
			"platform":    runtime.GOOS + "_" + runtime.GOARCH,
		},
	}

	registerResp, err := apiClient.Register(registerReq)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	ui.PrintSuccess(fmt.Sprintf("Verification code sent to %s", email))
	fmt.Println()

	// Step 3: Get verification code
	ui.PrintSection("🔑 Email Verification")
	fmt.Println()
	ui.PrintPrompt("Enter 6-digit code:")
	var code string
	fmt.Scanln(&code)
	fmt.Println()

	// Step 4: Verify and get API key
	ui.PrintInfo("Verifying...")
	verifyReq := &client.VerifyRequest{
		SessionID: registerResp.SessionID,
		Code:      code,
	}

	verifyResp, err := apiClient.Verify(verifyReq)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	ui.PrintSuccess("Login successful!")
	fmt.Println()

	// Step 5: Save TraceKit config to .env
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
	} else {
		ui.PrintSuccess("API key saved to .env")
	}
	fmt.Println()

	// Step 6: Show summary
	ui.PrintDivider()
	fmt.Println()

	summary := fmt.Sprintf("Dashboard:  %s\nAPI Key:    %s\nService:    %s",
		verifyResp.DashboardURL,
		utils.MaskAPIKey(verifyResp.APIKey),
		verifyResp.ServiceName)

	ui.PrintSummaryBox("✅ Login Complete!", summary)
	fmt.Println()

	steps := []string{
		"Your new API key has been saved to .env",
		"Run 'tracekit status' to verify your setup",
		"Run 'tracekit test' to send a test trace",
		"Visit " + verifyResp.DashboardURL + " to view traces",
	}
	ui.PrintNextSteps(steps)

	return nil
}

func promptEmailForLogin() (string, error) {
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
