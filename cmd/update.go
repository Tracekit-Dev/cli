package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/ui"
)

const (
	githubReleasesURL = "https://api.github.com/repos/Tracekit-Dev/cli/releases/latest"
	githubDownloadURL = "https://github.com/Tracekit-Dev/cli/releases/download"
)

var checkOnly bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update TraceKit CLI to the latest version",
	Long: `Update the TraceKit CLI binary to the latest release from GitHub.

This command downloads the latest release binary and replaces the current
executable in place. This is separate from 'tracekit upgrade' which manages
your subscription plan.

Use --check to see if an update is available without downloading.

Examples:
  tracekit update          Download and install latest version
  tracekit update --check  Check for updates without installing`,
	RunE: runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVar(&checkOnly, "check", false, "Check for updates without installing")
}

// githubRelease represents the relevant fields from the GitHub Releases API response
type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// fetchLatestRelease queries the GitHub API for the latest release tag
func fetchLatestRelease() (*githubRelease, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	req, err := http.NewRequest("GET", githubReleasesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", fmt.Sprintf("tracekit-cli/%s", Version))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release response: %w", err)
	}

	return &release, nil
}

// normalizeVersion strips a leading "v" prefix for comparison
func normalizeVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}

// downloadBinary downloads the platform-specific binary to a temp file and returns its path
func downloadBinary(tag string) (string, error) {
	osName := runtime.GOOS
	archName := runtime.GOARCH

	binaryName := fmt.Sprintf("tracekit-%s-%s", osName, archName)
	if osName == "windows" {
		binaryName += ".exe"
	}

	downloadURL := fmt.Sprintf("%s/%s/%s", githubDownloadURL, tag, binaryName)

	ui.PrintInfo(fmt.Sprintf("Downloading %s...", binaryName))
	fmt.Println()

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d (URL: %s)", resp.StatusCode, downloadURL)
	}

	// Write to a temp file in the same directory as the target (for atomic rename)
	tmpFile, err := os.CreateTemp("", "tracekit-update-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write downloaded binary: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	return tmpFile.Name(), nil
}

// replaceBinary replaces the current binary with the downloaded one
func replaceBinary(downloadedPath string) error {
	// Resolve the path to the currently running executable
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine current executable path: %w", err)
	}

	currentExe, err = filepath.EvalSymlinks(currentExe)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	backupPath := currentExe + ".old"

	// Remove any leftover backup from a previous update
	os.Remove(backupPath)

	// Rename current binary to backup
	if err := os.Rename(currentExe, backupPath); err != nil {
		return fmt.Errorf("failed to back up current binary: %w", err)
	}

	// Move downloaded binary into place
	if err := os.Rename(downloadedPath, currentExe); err != nil {
		// Restore from backup
		if restoreErr := os.Rename(backupPath, currentExe); restoreErr != nil {
			return fmt.Errorf("failed to install new binary AND failed to restore backup: install=%w, restore=%v", err, restoreErr)
		}
		return fmt.Errorf("failed to install new binary (restored backup): %w", err)
	}

	// Make executable (no-op on Windows, but harmless)
	if err := os.Chmod(currentExe, 0755); err != nil {
		// Non-fatal on Windows
		if runtime.GOOS != "windows" {
			return fmt.Errorf("failed to set executable permissions: %w", err)
		}
	}

	// Clean up backup
	if runtime.GOOS == "windows" {
		// On Windows the old binary may still be locked; leave .old in place
		// and it will be cleaned up on next update
	} else {
		os.Remove(backupPath)
	}

	return nil
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ui.PrintSection("TraceKit CLI Update")

	// Fetch latest release info
	ui.PrintInfo("Checking for updates...")
	fmt.Println()

	release, err := fetchLatestRelease()
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to check for updates: %v", err))
		return err
	}

	latestVersion := normalizeVersion(release.TagName)
	currentVersion := normalizeVersion(Version)

	if currentVersion == "dev" {
		ui.PrintWarning("Running a development build -- cannot determine current version")
		fmt.Println()
		ui.PrintMuted(fmt.Sprintf("Latest available: %s", release.TagName))

		if checkOnly {
			return nil
		}

		ui.PrintInfo("Proceeding with update...")
		fmt.Println()
	} else if latestVersion == currentVersion {
		ui.PrintSuccess(fmt.Sprintf("Already up to date (v%s)", currentVersion))
		return nil
	} else {
		ui.PrintInfo(fmt.Sprintf("Update available: v%s -> %s", currentVersion, release.TagName))
		fmt.Println()

		if checkOnly {
			ui.PrintMuted(fmt.Sprintf("Run 'tracekit update' to install %s", release.TagName))
			return nil
		}
	}

	// Download the binary
	tmpPath, err := downloadBinary(release.TagName)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Download failed: %v", err))
		ui.PrintMuted("Your current installation is unchanged.")
		return err
	}

	// Replace current binary
	ui.PrintInfo("Installing update...")
	fmt.Println()

	if err := replaceBinary(tmpPath); err != nil {
		// Clean up downloaded file on failure
		os.Remove(tmpPath)
		ui.PrintError(fmt.Sprintf("Installation failed: %v", err))
		ui.PrintMuted("Your current installation is unchanged.")
		return err
	}

	ui.PrintSuccess(fmt.Sprintf("Successfully updated to %s", release.TagName))
	fmt.Println()
	ui.PrintMuted("Restart your terminal or run 'tracekit --version' to verify.")

	return nil
}
