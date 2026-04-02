package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const DefaultEndpoint = "https://app.tracekit.dev"

// Config represents TraceKit configuration for a single profile
type Config struct {
	APIKey                string `json:"api_key"`
	UserID                string `json:"user_id"`
	Endpoint              string `json:"endpoint"`
	ServiceName           string `json:"service_name"`
	Enabled               string `json:"enabled"`
	CodeMonitoringEnabled string `json:"code_monitoring_enabled"`
}

// configFile is the on-disk JSON structure for ~/.tracekitconfig
type configFile struct {
	Active   string             `json:"active"`
	Profiles map[string]*Config `json:"profiles"`
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
		return DefaultEndpoint + "/v1/traces"
	}
	return c.Endpoint + "/v1/traces"
}

// GetAPIBase returns the base API URL for v1 endpoints
func (c *Config) GetAPIBase() string {
	if c.Endpoint == "" {
		return DefaultEndpoint
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

// RequireAuthForURL loads config for a specific server URL.
// Falls back to active profile if url is empty.
func RequireAuthForURL(envFlag string, url string) (*Config, error) {
	if url == "" {
		return RequireAuth(envFlag)
	}

	cf, err := readConfigFile(GlobalConfigPath())
	if err != nil {
		return nil, fmt.Errorf("not authenticated. Run 'tracekit login' to connect your account")
	}

	profile, ok := cf.Profiles[normalizeURL(url)]
	if !ok {
		return nil, fmt.Errorf("no credentials for %s. Run 'tracekit login --api-url %s'", url, url)
	}

	if profile.UserID == "" {
		return nil, fmt.Errorf("user identity missing for %s. Run 'tracekit login --api-url %s'", url, url)
	}

	return profile, nil
}

// ReadWithFallback reads config using the fallback chain:
// explicit envFlag path > global ~/.tracekitconfig (active profile) > local .env
func ReadWithFallback(envFlag string) (*Config, error) {
	// 1. Explicit --env flag: URL (profile lookup) or file path (legacy)
	if envFlag != "" {
		if strings.HasPrefix(envFlag, "http://") || strings.HasPrefix(envFlag, "https://") {
			return RequireAuthForURL("", envFlag)
		}
		return readLegacyFromPath(envFlag)
	}

	// 2. Global config (primary)
	globalPath := GlobalConfigPath()
	if globalPath != "" {
		if _, err := os.Stat(globalPath); err == nil {
			return readActiveProfile(globalPath)
		}
	}

	// 3. Local .env fallback (legacy)
	if _, err := os.Stat(".env"); err == nil {
		return readLegacyFromPath(".env")
	}

	return nil, fmt.Errorf("not authenticated. Run 'tracekit login' to connect your account")
}

// Read reads TraceKit configuration (legacy, calls ReadWithFallback)
func Read() (*Config, error) {
	return ReadWithFallback("")
}

// readActiveProfile reads the active profile from a JSON config file,
// with automatic migration from legacy env format.
func readActiveProfile(path string) (*Config, error) {
	cf, err := readConfigFile(path)
	if err != nil {
		return nil, err
	}

	if cf.Active == "" || len(cf.Profiles) == 0 {
		return nil, fmt.Errorf("no active profile in %s", path)
	}

	profile, ok := cf.Profiles[cf.Active]
	if !ok {
		return nil, fmt.Errorf("active profile %q not found in %s", cf.Active, path)
	}

	// Ensure endpoint is set from the profile key
	if profile.Endpoint == "" {
		profile.Endpoint = cf.Active
	}

	return profile, nil
}

// readConfigFile reads and parses the config file, migrating legacy format if needed.
func readConfigFile(path string) (*configFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	trimmed := strings.TrimSpace(string(content))

	// Try JSON first
	if strings.HasPrefix(trimmed, "{") {
		var cf configFile
		if err := json.Unmarshal([]byte(trimmed), &cf); err != nil {
			return nil, fmt.Errorf("failed to parse config %s: %w", path, err)
		}
		return &cf, nil
	}

	// Legacy env format -- migrate
	legacy, err := parseLegacyContent(trimmed)
	if err != nil {
		return nil, err
	}

	endpoint := legacy.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	endpoint = normalizeURL(endpoint)

	cf := &configFile{
		Active: endpoint,
		Profiles: map[string]*Config{
			endpoint: legacy,
		},
	}

	// Auto-migrate: write JSON back
	_ = writeConfigFile(path, cf)

	return cf, nil
}

// SaveGlobal saves a config profile to ~/.tracekitconfig keyed by endpoint.
// Sets it as the active profile.
func SaveGlobal(cfg *Config) error {
	globalPath := GlobalConfigPath()
	if globalPath == "" {
		return fmt.Errorf("could not determine home directory")
	}
	return SaveProfile(cfg, globalPath)
}

// SaveProfile saves a config profile to the given path, keyed by its endpoint.
func SaveProfile(cfg *Config, path string) error {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	endpoint = normalizeURL(endpoint)
	cfg.Endpoint = endpoint

	// Read existing config file (or start fresh)
	cf := &configFile{Profiles: map[string]*Config{}}
	if _, err := os.Stat(path); err == nil {
		existing, err := readConfigFile(path)
		if err == nil {
			cf = existing
		}
	}

	cf.Active = endpoint
	cf.Profiles[endpoint] = cfg

	return writeConfigFile(path, cf)
}

// Save writes TraceKit configuration to the specified path.
// For .env files, writes legacy format. For global config, writes JSON.
func Save(cfg *Config, paths ...string) error {
	envPath := ".env"
	if len(paths) > 0 && paths[0] != "" {
		envPath = paths[0]
	}

	// Global config path uses JSON format
	if envPath == GlobalConfigPath() {
		return SaveProfile(cfg, envPath)
	}

	// .env files use legacy flat format for SDK compatibility
	return saveLegacy(cfg, envPath)
}

// ListProfiles returns all saved profiles and the active endpoint.
func ListProfiles() (profiles map[string]*Config, active string, err error) {
	globalPath := GlobalConfigPath()
	if globalPath == "" {
		return nil, "", fmt.Errorf("could not determine home directory")
	}

	cf, err := readConfigFile(globalPath)
	if err != nil {
		return nil, "", err
	}

	return cf.Profiles, cf.Active, nil
}

// SetActiveProfile switches the active profile to the given endpoint URL.
func SetActiveProfile(url string) error {
	globalPath := GlobalConfigPath()
	if globalPath == "" {
		return fmt.Errorf("could not determine home directory")
	}

	cf, err := readConfigFile(globalPath)
	if err != nil {
		return err
	}

	key := normalizeURL(url)
	if _, ok := cf.Profiles[key]; !ok {
		return fmt.Errorf("no profile for %s. Run 'tracekit login --api-url %s'", url, url)
	}

	cf.Active = key
	return writeConfigFile(globalPath, cf)
}

// RemoveProfile deletes a saved profile by URL.
func RemoveProfile(url string) error {
	globalPath := GlobalConfigPath()
	if globalPath == "" {
		return fmt.Errorf("could not determine home directory")
	}

	cf, err := readConfigFile(globalPath)
	if err != nil {
		return err
	}

	key := normalizeURL(url)
	if _, ok := cf.Profiles[key]; !ok {
		return fmt.Errorf("no profile for %s", url)
	}

	delete(cf.Profiles, key)

	// If we removed the active profile, switch to another one
	if cf.Active == key {
		cf.Active = ""
		for k := range cf.Profiles {
			cf.Active = k
			break
		}
	}

	return writeConfigFile(globalPath, cf)
}

// writeConfigFile writes the config file as indented JSON.
func writeConfigFile(path string, cf *configFile) error {
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return os.WriteFile(path, append(data, '\n'), 0600)
}

// normalizeURL strips trailing slashes for consistent map keys.
func normalizeURL(url string) string {
	return strings.TrimRight(url, "/")
}

// --- Legacy env format support ---

func parseLegacyContent(content string) (*Config, error) {
	config := &Config{}
	lines := strings.Split(content, "\n")

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
		return nil, fmt.Errorf("TRACEKIT_API_KEY not found")
	}

	return config, nil
}

func readLegacyFromPath(path string) (*Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}
	cfg, err := parseLegacyContent(string(content))
	if err != nil {
		return nil, fmt.Errorf("%w in %s", err, path)
	}
	return cfg, nil
}

func saveLegacy(cfg *Config, envPath string) error {
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
