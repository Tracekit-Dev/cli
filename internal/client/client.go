package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// DefaultBaseURL is the production API endpoint
	DefaultBaseURL = "https://api.tracekit.dev"
	// DevBaseURL is the development API endpoint
	DevBaseURL = "http://localhost:8081"
)

// Client handles API communication with TraceKit backend
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	APIKey     string // Optional, for authenticated requests
}

// NewClient creates a new TraceKit API client
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RegisterRequest is the request body for account registration
type RegisterRequest struct {
	Email            string                 `json:"email"`
	Name             string                 `json:"name,omitempty"`
	OrganizationName string                 `json:"organization_name"`
	ServiceName      string                 `json:"service_name"`
	Source           string                 `json:"source"`
	SourceMetadata   map[string]interface{} `json:"source_metadata,omitempty"`
}

// RegisterResponse is the response from registration
type RegisterResponse struct {
	VerificationRequired bool      `json:"verification_required"`
	SessionID            string    `json:"session_id"`
	Message              string    `json:"message"`
	ExpiresAt            time.Time `json:"expires_at"`
}

// VerifyRequest is the request body for code verification
type VerifyRequest struct {
	SessionID string `json:"session_id"`
	Code      string `json:"code"`
}

// VerifyResponse is the response from verification
type VerifyResponse struct {
	APIKey         string `json:"api_key"`
	OrganizationID string `json:"organization_id"`
	ServiceName    string `json:"service_name"`
	DashboardURL   string `json:"dashboard_url"`
}

// ErrorResponse represents API error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// Register creates a new account and sends verification code
func (c *Client) Register(req *RegisterRequest) (*RegisterResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/v1/integrate/register", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if req.Source != "" {
		httpReq.Header.Set("X-TraceKit-Source", req.Source)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	var registerResp RegisterResponse
	if err := json.Unmarshal(respBody, &registerResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &registerResp, nil
}

// Verify verifies the email code and completes account setup
func (c *Client) Verify(req *VerifyRequest) (*VerifyResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/v1/integrate/verify", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	var verifyResp VerifyResponse
	if err := json.Unmarshal(respBody, &verifyResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &verifyResp, nil
}

// GetStatus checks integration status (requires API key)
func (c *Client) GetStatus() (map[string]interface{}, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("API key required")
	}

	httpReq, err := http.NewRequest("GET", c.BaseURL+"/v1/integrate/status", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("X-API-Key", c.APIKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	var status map[string]interface{}
	if err := json.Unmarshal(respBody, &status); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return status, nil
}
