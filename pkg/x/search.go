package x

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/masa-finance/masa-sdk-go/pkg/logger"
)

var (
	DefaultAPIPath = DefaultAPIBase + "/twitter/tweets/recent"
)

// SearchParams represents the parameters for the search request
type SearchParams struct {
	Query           string                 `json:"query"`
	Count           int                    `json:"count"`
	AdditionalProps map[string]interface{} `json:"-"`
}

// SearchResponse represents the API response structure
type SearchResponse struct {
	Data        []map[string]interface{} `json:"data"`
	RecordCount int                      `json:"recordCount"`
}

// SearchX sends a POST request to the Masa API endpoint to search recent tweets
func SearchX(baseURL string, apiPath string, params SearchParams) (*SearchResponse, error) {
	logger.Debugf("Starting search with params: %+v", params)

	// Use defaults if empty
	if baseURL == "" {
		baseURL = DefaultBaseURL
		logger.Debugf("Using default base URL: %s", baseURL)
	}
	if apiPath == "" {
		apiPath = DefaultAPIPath
	}

	// Construct full URL
	apiURL := fmt.Sprintf("%s/%s", strings.TrimRight(baseURL, "/"), strings.TrimLeft(apiPath, "/"))

	// Prepare request body
	body := map[string]interface{}{
		"query": params.Query,
		"count": params.Count,
	}

	// Add additional parameters if any
	for k, v := range params.AdditionalProps {
		body[k] = v
	}

	// Marshal body to JSON
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request body: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

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
	var response SearchResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("invalid JSON in API response: %w", err)
	}

	// Ensure consistent response structure
	if response.Data == nil {
		response.Data = make([]map[string]interface{}, 0)
		response.RecordCount = 0
	} else {
		response.RecordCount = len(response.Data)
	}

	return &response, nil
}
