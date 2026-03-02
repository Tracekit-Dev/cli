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

var releasesNewCmd = &cobra.Command{
	Use:   "new [version]",
	Short: "Create a new release",
	Long: `Create a new release for deployment tracking.

If no version is provided, auto-detects from package.json version field,
then falls back to the latest git tag.

This command is idempotent: re-running with the same version silently succeeds.

Examples:
  tracekit releases new v1.2.3
  tracekit releases new                        # auto-detect version
  tracekit releases new v1.2.3 --commit abc123
  tracekit releases new --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runReleasesNew,
}

func init() {
	releasesNewCmd.Flags().String("commit", "", "Commit SHA (default: auto-detect from git)")
	releasesNewCmd.Flags().String("url", "", "Changelog or release URL")
	releasesNewCmd.Flags().String("author", "", "Release author name")
}

func runReleasesNew(cmd *cobra.Command, args []string) error {
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
		cfg.Endpoint = "http://localhost:8081"
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

	// Determine commit SHA
	commitSHA, _ := cmd.Flags().GetString("commit")
	if commitSHA == "" {
		commitSHA = version.GetGitCommitSHA()
	}

	releaseURL, _ := cmd.Flags().GetString("url")
	author, _ := cmd.Flags().GetString("author")
	service, _ := cmd.Flags().GetString("service")

	// Create client
	c := client.NewClient(cfg.GetAPIBase())
	c.APIKey = cfg.APIKey

	// Create release
	req := &client.CreateReleaseRequest{
		Version:     ver,
		ServiceName: service,
		CommitSHA:   commitSHA,
		URL:         releaseURL,
		Author:      author,
	}

	if commitSHA != "" {
		req.CommitRange = version.GetGitCommitRange(ver)
	}

	release, isNew, err := c.CreateRelease(req)
	if err != nil {
		return fmt.Errorf("failed to create release: %w", err)
	}

	// Output
	useJSON, _ := cmd.Flags().GetBool("json")
	if useJSON {
		out, err := json.MarshalIndent(release, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal response: %w", err)
		}
		fmt.Println(string(out))
		return nil
	}

	if isNew {
		commitInfo := ""
		if release.CommitSHA != "" {
			commitInfo = fmt.Sprintf(" (commit: %.7s)", release.CommitSHA)
		}
		ui.PrintSuccess(fmt.Sprintf("Created release %s%s", release.Version, commitInfo))
	} else {
		ui.PrintInfo(fmt.Sprintf("Release %s already exists", release.Version))
	}

	return nil
}
