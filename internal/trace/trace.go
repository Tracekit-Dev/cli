package trace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/context.io/cli/internal/config"
)

const CLIVersion = "1.0.0"

// GenerateTestTrace creates a test trace payload
func GenerateTestTrace(serviceName string) map[string]interface{} {
	now := time.Now()
	traceID := uuid.New().String()
	spanID := uuid.New().String()

	return map[string]interface{}{
		"trace_id":  traceID,
		"span_id":   spanID,
		"parent_id": nil,
		"name":      "CLI Test Trace",
		"kind":      "internal",
		"timestamp": now.UnixMilli(),
		"duration":  150, // 150ms simulated duration
		"service": map[string]interface{}{
			"name":    serviceName,
			"version": "1.0.0",
		},
		"resource": map[string]interface{}{
			"type": "cli_test",
			"name": "tracekit test",
		},
		"attributes": map[string]interface{}{
			"test":         true,
			"source":       "tracekit-cli",
			"cli_version":  CLIVersion,
			"generated_at": now.Format(time.RFC3339),
		},
		"events": []map[string]interface{}{
			{
				"timestamp": now.UnixMilli(),
				"name":      "test.start",
				"attributes": map[string]interface{}{
					"message": "TraceKit CLI test trace initiated",
				},
			},
			{
				"timestamp": now.Add(50 * time.Millisecond).UnixMilli(),
				"name":      "test.processing",
				"attributes": map[string]interface{}{
					"message": "Processing test trace",
				},
			},
			{
				"timestamp": now.Add(150 * time.Millisecond).UnixMilli(),
				"name":      "test.complete",
				"attributes": map[string]interface{}{
					"message": "Test trace completed successfully",
				},
			},
		},
		"status": map[string]interface{}{
			"code":    "ok",
			"message": "Test trace completed",
		},
	}
}

// SendTrace sends the trace to TraceKit endpoint
func SendTrace(cfg *config.Config, trace map[string]interface{}) error {
	// Determine endpoint
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.tracekit.dev/v1/traces"
	}

	// Prepare request body
	body, err := json.Marshal(trace)
	if err != nil {
		return fmt.Errorf("failed to marshal trace: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", cfg.APIKey)
	req.Header.Set("User-Agent", "TraceKit-CLI/"+CLIVersion)

	// Send request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read error response
		var errBody bytes.Buffer
		errBody.ReadFrom(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, errBody.String())
	}

	return nil
}
