package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config represents TraceKit configuration
type Config struct {
	APIKey                string
	Endpoint              string // Base API URL (e.g., http://localhost:8081 or https://app.tracekit.dev)
	ServiceName           string
	Enabled               string
	CodeMonitoringEnabled string
}

// GlobalConfigPath returns the path to the global TraceKit config file (~/.tracekitconfig)
func GlobalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".tracekitconfig")
}

// GetTraceEndpoint returns the full trace ingestion endpoint
func (c *Config) GetTraceEndpoint() string {
	if c.Endpoint == "" {
		return "https://app.tracekit.dev/v1/traces"
	}
	return c.Endpoint + "/v1/traces"
}

// GetAPIBase returns the base API URL for v1 endpoints
func (c *Config) GetAPIBase() string {
	if c.Endpoint == "" {
		return "https://app.tracekit.dev"
	}
	return c.Endpoint
}

// ReadWithFallback reads config using the fallback chain:
// explicit envFlag path > local .env > global ~/.tracekitconfig
func ReadWithFallback(envFlag string) (*Config, error) {
	// 1. Explicit --env flag path
	if envFlag != "" {
		return readFromPath(envFlag)
	}

	// 2. Local .env in current directory
	if _, err := os.Stat(".env"); err == nil {
		return readFromPath(".env")
	}

	// 3. Global config fallback
	globalPath := GlobalConfigPath()
	if globalPath != "" {
		if _, err := os.Stat(globalPath); err == nil {
			return readFromPath(globalPath)
		}
	}

	return nil, fmt.Errorf("no TraceKit config found (checked .env and %s)", globalPath)
}

// Read reads TraceKit configuration from .env file (legacy, calls ReadWithFallback)
func Read() (*Config, error) {
	return ReadWithFallback("")
}

// readFromPath reads TraceKit configuration from a specific file path
func readFromPath(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	config := &Config{}
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse key=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "TRACEKIT_API_KEY":
			config.APIKey = value
		case "TRACEKIT_ENDPOINT":
			config.Endpoint = value
		case "TRACEKIT_SERVICE_NAME":
			config.ServiceName = value
		case "TRACEKIT_ENABLED":
			config.Enabled = value
		case "TRACEKIT_CODE_MONITORING_ENABLED":
			config.CodeMonitoringEnabled = value
		}
	}

	// Validate required fields
	if config.APIKey == "" {
		return nil, fmt.Errorf("TRACEKIT_API_KEY not found in %s", path)
	}

	return config, nil
}

// Save writes TraceKit configuration to the specified path (defaults to .env)
func Save(cfg *Config, paths ...string) error {
	envPath := ".env"
	if len(paths) > 0 && paths[0] != "" {
		envPath = paths[0]
	}

	// TraceKit config block
	tracekitConfig := fmt.Sprintf(`
# TraceKit Configuration
TRACEKIT_API_KEY=%s
TRACEKIT_ENDPOINT=%s
TRACEKIT_SERVICE_NAME=%s
TRACEKIT_ENABLED=%s
TRACEKIT_CODE_MONITORING_ENABLED=%s
`, cfg.APIKey, cfg.Endpoint, cfg.ServiceName, cfg.Enabled, cfg.CodeMonitoringEnabled)

	// Check if file exists
	var existingContent string
	if _, err := os.Stat(envPath); err == nil {
		// File exists, read it
		content, err := os.ReadFile(envPath)
		if err != nil {
			return err
		}
		existingContent = string(content)

		// Check if TraceKit config already exists
		if strings.Contains(existingContent, "# TraceKit Configuration") {
			// Replace existing TraceKit section
			lines := strings.Split(existingContent, "\n")
			var newLines []string
			skipUntilNextSection := false

			for _, line := range lines {
				if strings.Contains(line, "# TraceKit Configuration") {
					skipUntilNextSection = true
					continue
				}
				if skipUntilNextSection {
					// Skip lines that start with TRACEKIT_
					if strings.HasPrefix(strings.TrimSpace(line), "TRACEKIT_") {
						continue
					}
					// Stop skipping when we hit a non-TraceKit line
					if strings.TrimSpace(line) != "" && !strings.HasPrefix(strings.TrimSpace(line), "TRACEKIT_") {
						skipUntilNextSection = false
					}
				}
				if !skipUntilNextSection {
					newLines = append(newLines, line)
				}
			}
			existingContent = strings.Join(newLines, "\n") + tracekitConfig
		} else {
			// Append TraceKit config
			existingContent += tracekitConfig
		}
	} else {
		// File doesn't exist, create new with TraceKit config
		existingContent = tracekitConfig
	}

	// Ensure parent directory exists (for global config path)
	dir := filepath.Dir(envPath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Write to file
	return os.WriteFile(envPath, []byte(existingContent), 0644)
}
