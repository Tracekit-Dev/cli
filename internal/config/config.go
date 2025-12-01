package config

import (
	"fmt"
	"os"
	"strings"
)

// Config represents TraceKit configuration
type Config struct {
	APIKey                string
	Endpoint              string
	ServiceName           string
	Enabled               string
	CodeMonitoringEnabled string
}

// Read reads TraceKit configuration from .env file
func Read() (*Config, error) {
	envPath := ".env"

	// Check if file exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return nil, fmt.Errorf(".env file not found")
	}

	// Read file
	content, err := os.ReadFile(envPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read .env file: %w", err)
	}

	// Parse config
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
		return nil, fmt.Errorf("TRACEKIT_API_KEY not found in .env")
	}

	return config, nil
}

// Save writes TraceKit configuration to .env file
func Save(config *Config) error {
	envPath := ".env"

	// TraceKit config block
	tracekitConfig := fmt.Sprintf(`
# TraceKit Configuration
TRACEKIT_API_KEY=%s
TRACEKIT_ENDPOINT=%s
TRACEKIT_SERVICE_NAME=%s
TRACEKIT_ENABLED=%s
TRACEKIT_CODE_MONITORING_ENABLED=%s
`, config.APIKey, config.Endpoint, config.ServiceName, config.Enabled, config.CodeMonitoringEnabled)

	// Check if .env exists
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

	// Write to file
	return os.WriteFile(envPath, []byte(existingContent), 0644)
}
