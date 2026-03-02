package version

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// packageJSON represents a minimal package.json structure
type packageJSON struct {
	Version string `json:"version"`
}

// DetectVersion auto-detects the project version.
// It first tries reading the version field from package.json in the current directory,
// then falls back to the latest git tag. Returns an error if neither is found.
func DetectVersion() (string, error) {
	// Try package.json first
	if v, err := readPackageJSONVersion(); err == nil && v != "" {
		return v, nil
	}

	// Fall back to git tag
	if v, err := getLatestGitTag(); err == nil && v != "" {
		return v, nil
	}

	return "", fmt.Errorf("could not detect version: no package.json version field or git tag found. Provide version as argument")
}

// GetGitCommitSHA returns the current HEAD commit SHA, or empty string on error.
func GetGitCommitSHA() string {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// GetGitCommitRange returns the commit range between the previous tag and the current tag
// in "prev..current" format. Returns empty string if no previous tag exists.
func GetGitCommitRange(currentTag string) string {
	out, err := exec.Command("git", "describe", "--tags", "--abbrev=0", "HEAD^").Output()
	if err != nil {
		return ""
	}
	prevTag := strings.TrimSpace(string(out))
	if prevTag == "" {
		return ""
	}
	return prevTag + ".." + currentTag
}

// readPackageJSONVersion reads the version field from package.json in the current directory.
func readPackageJSONVersion() (string, error) {
	data, err := os.ReadFile("package.json")
	if err != nil {
		return "", err
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", err
	}

	return pkg.Version, nil
}

// getLatestGitTag returns the latest git tag.
func getLatestGitTag() (string, error) {
	out, err := exec.Command("git", "describe", "--tags", "--abbrev=0").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
