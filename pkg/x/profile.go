package x

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/masa-finance/masa-sdk-go/pkg/logger"
)

var (
	DefaultProfilePath = DefaultAPIBase + "/twitter/profile"
)

// ProfileResponse represents the API response structure for profile requests
type ProfileResponse struct {
	Data        map[string]interface{} `json:"data"`
	RecordCount int                    `json:"recordCount"`
}

// GetXProfile sends a GET request to the Masa API endpoint to fetch a Twitter profile
func GetXProfile(baseURL, apiPath, username string, additionalParams map[string]string) (*ProfileResponse, error) {
	logger.Debugf("Starting profile fetch for username: %s", username)

	// Use defaults if empty
	if baseURL == "" {
		baseURL = DefaultBaseURL
		logger.Debugf("Using default base URL: %s", baseURL)
	}
	if apiPath == "" {
		apiPath = DefaultProfilePath
	}

	// Construct full URL
	apiURL := fmt.Sprintf("%s/%s/%s",
		strings.TrimRight(baseURL, "/"),
		strings.TrimLeft(apiPath, "/"),
		username)

	// Create request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	// Add query parameters if any
	if additionalParams != nil {
		q := req.URL.Query()
		for key, value := range additionalParams {
			q.Add(key, value)
		}
		req.URL.RawQuery = q.Encode()
	}

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to API: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		logger.Warnf("Request failed with status %d", resp.StatusCode)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var response ProfileResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("invalid JSON in API response: %w", err)
	}

	// Ensure consistent response structure
	if response.Data == nil {
		response.Data = make(map[string]interface{})
		response.RecordCount = 0
	} else {
		response.RecordCount = 1
	}

	return &response, nil
}
