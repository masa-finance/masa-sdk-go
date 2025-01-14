// Package x provides functionality for interacting with the Masa Protocol X (formerly Twitter) API.
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
	// DefaultAPIPath is the default API endpoint path for searching recent tweets
	DefaultAPIPath = "/twitter/tweets/recent"
)

// SearchParams represents the parameters for the search request.
// It encapsulates the query string, result count, and any additional properties
// that may be needed for the search operation.
type SearchParams struct {
	// Query is the search query string to find matching tweets
	Query string `json:"query"`

	// Count specifies the maximum number of tweets to return
	Count int `json:"count"`

	// AdditionalProps allows passing extra parameters to the API
	// that aren't covered by the standard fields
	AdditionalProps map[string]interface{} `json:"-"`
}

// SearchResponse represents the API response structure for a search request.
// It contains the matched tweets and metadata about the results.
type SearchResponse struct {
	// Data contains the array of tweet objects returned by the search
	Data []map[string]interface{} `json:"data"`

	// RecordCount indicates the total number of tweets found
	RecordCount int `json:"recordCount"`
}

// ToMap converts SearchParams to a map for the queue.
// This is used internally to prepare the parameters for request processing.
//
// Returns:
//   - map[string]interface{}: A map containing the search parameters
func (p SearchParams) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"query": p.Query,
		"count": p.Count,
	}
}

// SearchX sends a POST request to the Masa API endpoint to search recent tweets.
// It handles the full lifecycle of the request including parameter validation,
// request preparation, execution, and response parsing.
//
// Parameters:
//   - baseURL: The base URL for the API (uses DefaultBaseURL if empty)
//   - apiPath: The API endpoint path (uses DefaultAPIPath if empty)
//   - params: SearchParams containing the search configuration
//
// Returns:
//   - *SearchResponse: Contains the search results and metadata
//   - error: Any error encountered during the request
//
// Example:
//
//	params := SearchParams{
//	    Query: "blockchain",
//	    Count: 10,
//	}
//	response, err := SearchX("", "", params)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Found %d tweets\n", response.RecordCount)
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

	// Construct full URL with proper path components
	baseURL = strings.TrimRight(baseURL, "/")
	apiBase := strings.TrimRight(DefaultAPIBase, "/")
	apiPath = strings.TrimLeft(apiPath, "/")

	apiURL := fmt.Sprintf("%s%s/%s", baseURL, apiBase, apiPath)
	logger.Debugf("Constructed API URL: %s", apiURL)

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
