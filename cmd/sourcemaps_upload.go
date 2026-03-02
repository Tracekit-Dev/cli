package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/client"
	"github.com/yourusername/context.io/cli/internal/config"
	"github.com/yourusername/context.io/cli/internal/ui"
	"github.com/yourusername/context.io/cli/internal/version"
)

var sourcemapsUploadCmd = &cobra.Command{
	Use:   "upload [path]",
	Short: "Upload source maps with automatic debug ID injection",
	Long: `Upload source maps from your build output with automatic debug ID injection.

Recursively discovers .js.map files, injects debug IDs into both the .js and
.map files (idempotent), and uploads to the TraceKit server. Existing debug IDs
in .js files are preserved on re-run.

The release version is auto-detected from package.json or git tag, but can be
overridden with --release.

Examples:
  tracekit sourcemaps upload ./dist
  tracekit sourcemaps upload ./dist --release v1.2.3
  tracekit sourcemaps upload                         # uses current directory
  tracekit sourcemaps upload ./dist --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSourcemapsUpload,
}

func init() {
	sourcemapsUploadCmd.Flags().String("release", "", "Release version (default: auto-detect from package.json or git tag)")
}

// uploadResult tracks per-file upload results for JSON output
type uploadResult struct {
	DebugID   string `json:"debug_id"`
	Filename  string `json:"filename"`
	SizeBytes int    `json:"size_bytes"`
	Release   string `json:"release,omitempty"`
}

func runSourcemapsUpload(cmd *cobra.Command, args []string) error {
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

	// Create client
	c := client.NewClient(cfg.GetAPIBase())
	c.APIKey = cfg.APIKey

	// Determine release version
	release, _ := cmd.Flags().GetString("release")
	if release == "" {
		detected, err := version.DetectVersion()
		if err != nil {
			ui.PrintWarning("Could not auto-detect release version. Uploading without release tag.")
		} else {
			release = detected
		}
	}

	// Determine path to scan
	scanPath := "."
	if len(args) > 0 {
		scanPath = args[0]
	}

	// Discover source map files
	mapFiles, err := discoverSourceMaps(scanPath)
	if err != nil {
		return fmt.Errorf("failed to discover source maps: %w", err)
	}

	if len(mapFiles) == 0 {
		ui.PrintWarning(fmt.Sprintf("No .js.map files found in %s", scanPath))
		return nil
	}

	ui.PrintInfo(fmt.Sprintf("Found %d source map(s) in %s", len(mapFiles), scanPath))

	// Process each source map file
	var results []uploadResult
	var uploadCount int
	var errorCount int

	for _, mapFile := range mapFiles {
		// Find the corresponding .js file (strip .map suffix)
		jsFile := strings.TrimSuffix(mapFile, ".map")

		// Inject debug ID into .js file
		debugID, err := injectDebugIDIntoJS(jsFile)
		if err != nil {
			ui.PrintError(fmt.Sprintf("Failed to inject debug ID into %s: %v", jsFile, err))
			errorCount++
			continue
		}

		// Inject debug ID into .map file
		if err := injectDebugIDIntoMap(mapFile, debugID); err != nil {
			ui.PrintError(fmt.Sprintf("Failed to inject debug ID into %s: %v", mapFile, err))
			errorCount++
			continue
		}

		// Read the modified .map file
		mapData, err := os.ReadFile(mapFile)
		if err != nil {
			ui.PrintError(fmt.Sprintf("Failed to read %s: %v", mapFile, err))
			errorCount++
			continue
		}

		// Upload to server
		_, err = c.UploadSourceMap(debugID, release, mapFile, mapData)
		if err != nil {
			ui.PrintError(fmt.Sprintf("Failed to upload %s: %v", filepath.Base(mapFile), err))
			errorCount++
			continue
		}

		uploadCount++
		results = append(results, uploadResult{
			DebugID:   debugID,
			Filename:  filepath.Base(mapFile),
			SizeBytes: len(mapData),
			Release:   release,
		})

		ui.PrintSuccess(fmt.Sprintf("%s (%s)", filepath.Base(mapFile), humanFileSize(len(mapData))))
	}

	// Print summary
	useJSON, _ := cmd.Flags().GetBool("json")
	if useJSON {
		out, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal results: %w", err)
		}
		fmt.Println(string(out))
		return nil
	}

	fmt.Println()
	if release != "" {
		ui.PrintInfo(fmt.Sprintf("Uploaded %d source map(s) for release %s", uploadCount, release))
	} else {
		ui.PrintInfo(fmt.Sprintf("Uploaded %d source map(s)", uploadCount))
	}

	if errorCount > 0 {
		ui.PrintWarning(fmt.Sprintf("%d file(s) failed to upload", errorCount))
	}

	return nil
}

// discoverSourceMaps recursively walks the given path to find all .js.map files,
// skipping node_modules directories.
func discoverSourceMaps(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("path not found: %s", root)
	}

	// If path is a file, check if it's a .js.map file
	if !info.IsDir() {
		if strings.HasSuffix(root, ".js.map") {
			return []string{root}, nil
		}
		return nil, fmt.Errorf("%s is not a .js.map file", root)
	}

	// Recursively walk directory
	var mapFiles []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip node_modules directories
		if d.IsDir() && d.Name() == "node_modules" {
			return filepath.SkipDir
		}

		// Collect .js.map files
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".js.map") {
			mapFiles = append(mapFiles, path)
		}

		return nil
	})

	return mapFiles, err
}

// debugIDPattern matches existing //# debugId=<uuid> comments (case-insensitive)
var debugIDPattern = regexp.MustCompile(`(?i)//# debugId=([0-9a-f-]+)`)

// injectDebugIDIntoJS injects a debug ID comment into a .js file.
// If a debug ID already exists, it returns the existing ID (idempotent).
// The format follows TC39 ECMA-426: //# debugId=<uuid>
func injectDebugIDIntoJS(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read JS file: %w", err)
	}

	content := string(data)

	// Check if debugId already exists (idempotent)
	matches := debugIDPattern.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1], nil
	}

	// Generate new UUID
	debugID := uuid.New().String()
	debugComment := fmt.Sprintf("//# debugId=%s", debugID)

	// Insert before //# sourceMappingURL= if it exists, otherwise append
	sourceMappingIdx := strings.LastIndex(content, "//# sourceMappingURL=")
	if sourceMappingIdx >= 0 {
		// Insert before the sourceMappingURL comment
		content = content[:sourceMappingIdx] + debugComment + "\n" + content[sourceMappingIdx:]
	} else {
		// Append to end of file
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += debugComment + "\n"
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write JS file: %w", err)
	}

	return debugID, nil
}

// injectDebugIDIntoMap adds or updates the "debugId" field in a .map JSON file.
func injectDebugIDIntoMap(filePath, debugID string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read map file: %w", err)
	}

	// Parse JSON into generic map
	var mapContent map[string]interface{}
	if err := json.Unmarshal(data, &mapContent); err != nil {
		return fmt.Errorf("failed to parse map file JSON: %w", err)
	}

	// Set debugId field
	mapContent["debugId"] = debugID

	// Marshal back to JSON with indentation
	output, err := json.MarshalIndent(mapContent, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal map file JSON: %w", err)
	}

	// Append newline for clean file ending
	output = append(output, '\n')

	if err := os.WriteFile(filePath, output, 0644); err != nil {
		return fmt.Errorf("failed to write map file: %w", err)
	}

	return nil
}

// humanFileSize formats a byte count into a human-readable string.
func humanFileSize(bytes int) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)

	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1fGB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1fMB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1fkb", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
