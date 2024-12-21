// Package nineteen provides a client for the Nineteen AI API
package nineteen

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/masa-finance/masa-sdk-go/pkg/logger"
	"golang.org/x/time/rate"
)

// API constants
const (
	defaultBaseURL      = "https://api.nineteen.ai/v1"
	chatCompletionsPath = "/chat/completions"
	defaultTimeout      = 30 * time.Second
)

// Default model settings
const (
	DefaultModel       = "unsloth/Meta-Llama-3.1-8B-Instruct"
	DefaultTemperature = 0.5
	DefaultMaxTokens   = 500
	DefaultTopP        = 0.5
	DefaultStream      = false
	DefaultRateLimit   = 6.0
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
	Delta        *Delta  `json:"delta,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatCompletionRequest struct {
	Messages    []Message `json:"messages"`
	Model       string    `json:"model,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type ChatCompletionChunk struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

type Client struct {
	baseURL     string
	apiKey      string
	httpClient  *http.Client
	rateLimiter *rate.Limiter
}

type ClientOption func(*Client)

type apiError struct {
	statusCode int
	message    string
}

func (e *apiError) Error() string {
	switch e.statusCode {
	case http.StatusTooManyRequests:
		return fmt.Sprintf("rate limit exceeded: %s", e.message)
	case http.StatusUnprocessableEntity:
		return fmt.Sprintf("validation error (%d): %s", e.statusCode, e.message)
	case http.StatusInternalServerError:
		return fmt.Sprintf("server error (%d): %s", e.statusCode, e.message)
	default:
		return fmt.Sprintf("API error (%d): %s", e.statusCode, e.message)
	}
}

func NewChatCompletionRequest(messages []Message) ChatCompletionRequest {
	return ChatCompletionRequest{
		Messages:    messages,
		Model:       DefaultModel,
		Temperature: DefaultTemperature,
		MaxTokens:   DefaultMaxTokens,
		TopP:        DefaultTopP,
		Stream:      DefaultStream,
	}
}

func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

func WithRateLimit(rps float64) ClientOption {
	return func(c *Client) {
		c.rateLimiter = rate.NewLimiter(rate.Limit(rps), 1)
	}
}

func NewClient(apiKey string, opts ...ClientOption) *Client {
	logger.Init("INFO")
	c := &Client{
		baseURL: defaultBaseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		rateLimiter: rate.NewLimiter(rate.Limit(DefaultRateLimit), 1),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Client) createRequest(ctx context.Context, req ChatCompletionRequest) (*http.Request, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit: %w", err)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+chatCompletionsPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	return httpReq, nil
}

func (c *Client) handleResponse(resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return &apiError{
		statusCode: resp.StatusCode,
		message:    string(body),
	}
}

func (c *Client) CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	httpReq, err := c.createRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if err := c.handleResponse(resp); err != nil {
		return nil, err
	}

	var result ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

func (c *Client) CreateChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (<-chan ChatCompletionChunk, <-chan error) {
	chunkChan := make(chan ChatCompletionChunk)
	errChan := make(chan error, 1)

	req.Stream = true
	httpReq, err := c.createRequest(ctx, req)
	if err != nil {
		errChan <- err
		close(chunkChan)
		close(errChan)
		return chunkChan, errChan
	}

	go c.handleStream(ctx, httpReq, chunkChan, errChan)

	return chunkChan, errChan
}

func (c *Client) processStreamChunk(data []byte) (ChatCompletionChunk, error) {
	var chunk ChatCompletionChunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		return chunk, err
	}

	for i := range chunk.Choices {
		if chunk.Choices[i].Delta != nil {
			chunk.Choices[i].Message.Role = chunk.Choices[i].Delta.Role
			chunk.Choices[i].Message.Content = chunk.Choices[i].Delta.Content
		}
	}

	return chunk, nil
}

func (c *Client) hasContent(chunk ChatCompletionChunk) bool {
	for _, choice := range chunk.Choices {
		if choice.Message.Content != "" || (choice.Delta != nil && choice.Delta.Content != "") {
			return true
		}
	}
	return chunk.ID != ""
}

func (c *Client) handleStream(ctx context.Context, req *http.Request, chunkChan chan ChatCompletionChunk, errChan chan error) {
	defer close(chunkChan)
	defer close(errChan)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		errChan <- fmt.Errorf("do request: %w", err)
		return
	}
	defer resp.Body.Close()

	if err := c.handleResponse(resp); err != nil {
		errChan <- err
		return
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				errChan <- fmt.Errorf("read line: %w", err)
			}
			return
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 || bytes.Equal(line, []byte("[DONE]")) || bytes.Equal(line, []byte("data: [DONE]")) {
			continue
		}

		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}

		data := bytes.TrimPrefix(line, []byte("data: "))
		if len(data) == 0 {
			continue
		}

		chunk, err := c.processStreamChunk(data)
		if err != nil {
			continue
		}

		if !c.hasContent(chunk) {
			continue
		}

		select {
		case <-ctx.Done():
			errChan <- ctx.Err()
			return
		case chunkChan <- chunk:
		}
	}
}
