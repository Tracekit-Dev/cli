package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/client"
	"github.com/yourusername/context.io/cli/internal/config"
	"github.com/yourusername/context.io/cli/internal/ui"
	"github.com/yourusername/context.io/cli/internal/version"
)

var releasesDeployCmd = &cobra.Command{
	Use:   "deploy [version]",
	Short: "Create, deploy, and finalize a release in one command",
	Long: `Combined shortcut that creates a release, registers a deploy, and finalizes
the release in one command.

If no version is provided, auto-detects from package.json version field,
then falls back to the latest git tag.

Each step is idempotent: if the release already exists, it continues to
the deploy step.

Examples:
  tracekit releases deploy --env production
  tracekit releases deploy v1.2.3 --env staging
  tracekit releases deploy --env production --deployer deploy-bot
  tracekit releases deploy --env production --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runReleasesDeploy,
}

func init() {
	releasesDeployCmd.Flags().String("env", "", "Deployment environment (required)")
	releasesDeployCmd.MarkFlagRequired("env")
	releasesDeployCmd.Flags().String("commit", "", "Commit SHA (default: auto-detect from git)")
	releasesDeployCmd.Flags().String("url", "", "Changelog or release URL")
	releasesDeployCmd.Flags().String("author", "", "Release author name")
	releasesDeployCmd.Flags().String("deployer", "", "Name of the deployer or deploy system")
}

func runReleasesDeploy(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Read()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.APIKey == "" {
		return fmt.Errorf("not authenticated. Run 'tracekit login' first")
	}

	// Check --dev flag
	useDev, _ := cmd.Flags().GetBool("dev")
	if useDev {
		cfg.Endpoint = "https://localhost:8081"
	}

	// Determine version
	var ver string
	if len(args) > 0 {
		ver = args[0]
	} else {
		detected, err := version.DetectVersion()
		if err != nil {
			return fmt.Errorf("version detection failed: %w", err)
		}
		ver = detected
	}

	env, _ := cmd.Flags().GetString("env")
	service, _ := cmd.Flags().GetString("service")
	commitSHA, _ := cmd.Flags().GetString("commit")
	if commitSHA == "" {
		commitSHA = version.GetGitCommitSHA()
	}
	releaseURL, _ := cmd.Flags().GetString("url")
	author, _ := cmd.Flags().GetString("author")
	deployer, _ := cmd.Flags().GetString("deployer")

	// Create client
	c := client.NewClient(cfg.GetAPIBase())
	c.APIKey = cfg.APIKey

	// Step 1: Create release (idempotent)
	releaseReq := &client.CreateReleaseRequest{
		Version:     ver,
		ServiceName: service,
		CommitSHA:   commitSHA,
		URL:         releaseURL,
		Author:      author,
	}

	if commitSHA != "" {
		releaseReq.CommitRange = version.GetGitCommitRange(ver)
	}

	release, _, err := c.CreateRelease(releaseReq)
	if err != nil {
		return fmt.Errorf("failed to create release: %w", err)
	}

	// Step 2: Register deploy
	deployReq := &client.CreateDeployRequest{
		Environment: env,
		Deployer:    deployer,
	}

	deploy, err := c.CreateDeploy(ver, service, deployReq)
	if err != nil {
		return fmt.Errorf("failed to register deploy: %w", err)
	}

	// Step 3: Finalize release
	finalizedRelease, err := c.FinalizeRelease(ver, service)
	if err != nil {
		return fmt.Errorf("failed to finalize release: %w", err)
	}

	// Output
	useJSON, _ := cmd.Flags().GetBool("json")
	if useJSON {
		combined := map[string]interface{}{
			"release": finalizedRelease,
			"deploy":  deploy,
		}
		out, err := json.MarshalIndent(combined, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal response: %w", err)
		}
		fmt.Println(string(out))
		return nil
	}

	_ = release // used for create step
	ui.PrintSuccess(fmt.Sprintf("Released %s to %s (created, deployed, finalized)", ver, env))

	return nil
}
