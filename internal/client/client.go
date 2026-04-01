package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"
)

const (
	// DefaultBaseURL is the production API endpoint
	DefaultBaseURL = "https://app.tracekit.dev"
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

// Release and Deploy request/response types

// CreateReleaseRequest is the request body for creating a release
type CreateReleaseRequest struct {
	Version     string `json:"version"`
	ServiceName string `json:"service_name,omitempty"`
	CommitSHA   string `json:"commit_sha,omitempty"`
	CommitRange string `json:"commit_range,omitempty"`
	URL         string `json:"url,omitempty"`
	Author      string `json:"author,omitempty"`
}

// CreateReleaseResponse is the response from creating or retrieving a release
type CreateReleaseResponse struct {
	ID          string  `json:"id"`
	Version     string  `json:"version"`
	ServiceName string  `json:"service_name,omitempty"`
	Source      string  `json:"source"`
	CommitSHA   string  `json:"commit_sha,omitempty"`
	CommitRange string  `json:"commit_range,omitempty"`
	URL         string  `json:"url,omitempty"`
	Author      string  `json:"author,omitempty"`
	CreatedAt   string  `json:"created_at"`
	FinalizedAt *string `json:"finalized_at,omitempty"`
}

// CreateDeployRequest is the request body for registering a deploy
type CreateDeployRequest struct {
	Environment string `json:"environment"`
	Deployer    string `json:"deployer,omitempty"`
}

// CreateDeployResponse is the response from registering a deploy
type CreateDeployResponse struct {
	ID          string `json:"id"`
	ReleaseID   string `json:"release_id"`
	Environment string `json:"environment"`
	Deployer    string `json:"deployer,omitempty"`
	DeployedAt  string `json:"deployed_at"`
}

// ReleaseListResponse is the paginated list of releases
type ReleaseListResponse struct {
	Releases     []CreateReleaseResponse `json:"releases"`
	Total        int                     `json:"total"`
	Page         int                     `json:"page"`
	PageSize     int                     `json:"page_size"`
	Environments []string                `json:"environments"`
	Services     []string                `json:"services"`
}

// CreateRelease creates a new release via the API.
// Returns (release, isNew, error). isNew is true if the release was newly created (201),
// false if it already existed (200/409). This supports idempotent behavior.
func (c *Client) CreateRelease(req *CreateReleaseRequest) (*CreateReleaseResponse, bool, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, false, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/api/releases", bytes.NewReader(body))
	if err != nil {
		return nil, false, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", c.APIKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		var release CreateReleaseResponse
		if err := json.Unmarshal(respBody, &release); err != nil {
			return nil, false, fmt.Errorf("failed to parse response: %w", err)
		}
		return &release, true, nil
	case http.StatusOK, http.StatusConflict:
		var release CreateReleaseResponse
		if err := json.Unmarshal(respBody, &release); err != nil {
			return nil, false, fmt.Errorf("failed to parse response: %w", err)
		}
		return &release, false, nil
	default:
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return nil, false, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, false, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}
}

// FinalizeRelease marks a release as finalized via PATCH /api/releases/{version}.
func (c *Client) FinalizeRelease(version, service string) (*CreateReleaseResponse, error) {
	reqURL := c.BaseURL + "/api/releases/" + version
	if service != "" {
		reqURL += "?service=" + service
	}
	httpReq, err := http.NewRequest("PATCH", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
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

	var release CreateReleaseResponse
	if err := json.Unmarshal(respBody, &release); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &release, nil
}

// CreateDeploy registers a deploy for a release via POST /api/releases/{version}/deploys.
func (c *Client) CreateDeploy(version, service string, req *CreateDeployRequest) (*CreateDeployResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	reqURL := c.BaseURL + "/api/releases/" + version + "/deploys"
	if service != "" {
		reqURL += "?service=" + service
	}
	httpReq, err := http.NewRequest("POST", reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
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

	if resp.StatusCode != http.StatusCreated {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	var deploy CreateDeployResponse
	if err := json.Unmarshal(respBody, &deploy); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &deploy, nil
}

// ListReleases retrieves a paginated list of releases via GET /api/releases.
func (c *Client) ListReleases(page, pageSize int, service string) (*ReleaseListResponse, error) {
	url := fmt.Sprintf("%s/api/releases?page=%d&page_size=%d", c.BaseURL, page, pageSize)
	if service != "" {
		url += "&service=" + service
	}

	httpReq, err := http.NewRequest("GET", url, nil)
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

	var listResp ReleaseListResponse
	if err := json.Unmarshal(respBody, &listResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &listResp, nil
}

// DashboardData holds the full dashboard response from /v1/alerts/dashboard
type DashboardData struct {
	Stats struct {
		HealthScore float64 `json:"health_score"`
		Services    int     `json:"services"`
		TotalTraces int     `json:"total_traces"`
		Errors24h   int     `json:"errors_24h"`
		AvgResponse int     `json:"avg_response"`
		Traces24h   int     `json:"traces_24h"`
		ErrorRate   float64 `json:"error_rate"`
		Deltas      struct {
			HasPrevious bool    `json:"has_previous"`
			Errors      float64 `json:"errors"`
			AvgResponse float64 `json:"avg_response"`
			Health      float64 `json:"health"`
		} `json:"deltas"`
	} `json:"stats"`
	Services []struct {
		Name        string  `json:"name"`
		Traces      int     `json:"traces"`
		Errors      int     `json:"errors"`
		ErrorRate   float64 `json:"error_rate"`
		AvgResponse int     `json:"avg_response"`
	} `json:"services"`
	Alerts struct {
		Count int `json:"count"`
		Items []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Severity    string `json:"severity"`
			Message     string `json:"message"`
			TriggeredAt string `json:"triggered_at"`
			Duration    string `json:"duration"`
		} `json:"items"`
	} `json:"alerts"`
	Anomalies struct {
		Unacknowledged int `json:"unacknowledged"`
		Critical       int `json:"critical"`
	} `json:"anomalies"`
	ErrorHotspots []struct {
		Service   string  `json:"service"`
		Operation string  `json:"operation"`
		Total     int     `json:"total"`
		Errors    int     `json:"errors"`
		ErrorRate float64 `json:"error_rate"`
	} `json:"error_hotspots"`
	TimeSeries []struct {
		Time        string `json:"time"`
		Requests    int    `json:"requests"`
		Errors      int    `json:"errors"`
		AvgDuration int    `json:"avg_duration"`
		P50         int    `json:"p50"`
		P95         int    `json:"p95"`
		P99         int    `json:"p99"`
	} `json:"time_series"`
	Timestamp string `json:"timestamp"`
}

// GetDashboard fetches the full dashboard data
func (c *Client) GetDashboard(window string) (*DashboardData, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("API key required")
	}

	url := c.BaseURL + "/v1/alerts/dashboard"
	if window != "" {
		url += "?window=" + window
	}
	httpReq, err := http.NewRequest("GET", url, nil)
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
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var data DashboardData
	if err := json.Unmarshal(respBody, &data); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &data, nil
}

// PostHealthCheck creates a new health check configuration
func (c *Client) PostHealthCheck(apiURL, apiKey string, requestBody map[string]interface{}) error {
	body, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", apiURL+"/api/health-checks", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", apiKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// Source Map request/response types

// SourceMapUploadResponse is the response from uploading a source map
type SourceMapUploadResponse struct {
	ID        string `json:"id"`
	DebugID   string `json:"debug_id"`
	Release   string `json:"release,omitempty"`
	Filename  string `json:"filename"`
	SizeBytes int64  `json:"size_bytes"`
	CreatedAt string `json:"created_at"`
}

// SourceMapDeleteResponse is the response from deleting source maps
type SourceMapDeleteResponse struct {
	DeletedCount int    `json:"deleted_count"`
	Release      string `json:"release"`
}

// UploadSourceMap uploads a source map file via multipart POST to /api/sourcemaps.
// Uses a 60-second timeout for large uploads.
func (c *Client) UploadSourceMap(debugID, release, filename string, mapData []byte) (*SourceMapUploadResponse, error) {
	// Build multipart form body
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add debug_id field
	if err := writer.WriteField("debug_id", debugID); err != nil {
		return nil, fmt.Errorf("failed to write debug_id field: %w", err)
	}

	// Add release field
	if release != "" {
		if err := writer.WriteField("release", release); err != nil {
			return nil, fmt.Errorf("failed to write release field: %w", err)
		}
	}

	// Add sourcemap file
	part, err := writer.CreateFormFile("sourcemap", filepath.Base(filename))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(mapData); err != nil {
		return nil, fmt.Errorf("failed to write file data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/api/sourcemaps", &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("X-API-Key", c.APIKey)

	// Use longer timeout for large uploads
	uploadClient := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := uploadClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	var uploadResp SourceMapUploadResponse
	if err := json.Unmarshal(respBody, &uploadResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &uploadResp, nil
}

// DeleteSourceMaps deletes all source maps for a release via DELETE /api/sourcemaps?release={release}.
func (c *Client) DeleteSourceMaps(release string) (*SourceMapDeleteResponse, error) {
	url := fmt.Sprintf("%s/api/sourcemaps?release=%s", c.BaseURL, release)

	httpReq, err := http.NewRequest("DELETE", url, nil)
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

	var deleteResp SourceMapDeleteResponse
	if err := json.Unmarshal(respBody, &deleteResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &deleteResp, nil
}
