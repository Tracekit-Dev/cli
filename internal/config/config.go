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
	UserID                string // UUID of the authenticated user
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

// RequireAuth loads config and validates that the user has logged in (has user_id).
// Returns config or an error telling the user to login.
func RequireAuth(envFlag string) (*Config, error) {
	cfg, err := ReadWithFallback(envFlag)
	if err != nil {
		return nil, fmt.Errorf("not authenticated. Run 'tracekit login' to connect your account")
	}

	if cfg.UserID == "" {
		return nil, fmt.Errorf("user identity missing. Run 'tracekit login' to re-authenticate.\n  Your API key is valid but no user_id was stored (pre-v20 config)")
	}

	return cfg, nil
}

// ReadWithFallback reads config using the fallback chain:
// explicit envFlag path > global ~/.tracekitconfig > local .env
func ReadWithFallback(envFlag string) (*Config, error) {
	// 1. Explicit --env flag path
	if envFlag != "" {
		return readFromPath(envFlag)
	}

	// 2. Global config (primary -- always has auth details)
	globalPath := GlobalConfigPath()
	if globalPath != "" {
		if _, err := os.Stat(globalPath); err == nil {
			return readFromPath(globalPath)
		}
	}

	// 3. Local .env fallback (legacy)
	if _, err := os.Stat(".env"); err == nil {
		return readFromPath(".env")
	}

	return nil, fmt.Errorf("not authenticated. Run 'tracekit login' to connect your account")
}

// Read reads TraceKit configuration (legacy, calls ReadWithFallback)
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

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "TRACEKIT_API_KEY":
			config.APIKey = value
		case "TRACEKIT_USER_ID":
			config.UserID = value
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

	if config.APIKey == "" {
		return nil, fmt.Errorf("TRACEKIT_API_KEY not found in %s", path)
	}

	return config, nil
}

// SaveGlobal always saves auth config to ~/.tracekitconfig
func SaveGlobal(cfg *Config) error {
	globalPath := GlobalConfigPath()
	if globalPath == "" {
		return fmt.Errorf("could not determine home directory")
	}
	return Save(cfg, globalPath)
}

// Save writes TraceKit configuration to the specified path (defaults to .env)
func Save(cfg *Config, paths ...string) error {
	envPath := ".env"
	if len(paths) > 0 && paths[0] != "" {
		envPath = paths[0]
	}

	tracekitConfig := fmt.Sprintf(`
# TraceKit Configuration
TRACEKIT_API_KEY=%s
TRACEKIT_USER_ID=%s
TRACEKIT_ENDPOINT=%s
TRACEKIT_SERVICE_NAME=%s
TRACEKIT_ENABLED=%s
TRACEKIT_CODE_MONITORING_ENABLED=%s
`, cfg.APIKey, cfg.UserID, cfg.Endpoint, cfg.ServiceName, cfg.Enabled, cfg.CodeMonitoringEnabled)

	var existingContent string
	if _, err := os.Stat(envPath); err == nil {
		content, err := os.ReadFile(envPath)
		if err != nil {
			return err
		}
		existingContent = string(content)

		if strings.Contains(existingContent, "# TraceKit Configuration") {
			lines := strings.Split(existingContent, "\n")
			var newLines []string
			skipUntilNextSection := false

			for _, line := range lines {
				if strings.Contains(line, "# TraceKit Configuration") {
					skipUntilNextSection = true
					continue
				}
				if skipUntilNextSection {
					if strings.HasPrefix(strings.TrimSpace(line), "TRACEKIT_") {
						continue
					}
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
			existingContent += tracekitConfig
		}
	} else {
		existingContent = tracekitConfig
	}

	dir := filepath.Dir(envPath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return os.WriteFile(envPath, []byte(existingContent), 0644)
}
