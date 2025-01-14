package x

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	twitterscraper "github.com/imperatrona/twitter-scraper"
	"github.com/masa-finance/masa-sdk-go/pkg/logger"
)

// AuthOptions represents different authentication methods
type AuthOptions struct {
	Username      string
	Password      string
	AuthCode      string // Optional: for 2FA
	AuthToken     string // Optional: for token-based auth
	CSRFToken     string // Optional: for token-based auth
	ProxyURL      string // Optional: for proxy support
	ProxyEnable   bool   // Optional: enable/disable proxy
	ProxyUsername string // Optional: proxy authentication
	ProxyPassword string // Optional: proxy authentication
}

// NewAuthOptions creates AuthOptions from environment variables
func NewAuthOptions() AuthOptions {
	proxyEnable := os.Getenv("X_PROXY_ENABLE") == "true"
	return AuthOptions{
		Username:      os.Getenv("X_USERNAME"),
		Password:      os.Getenv("X_PASSWORD"),
		AuthCode:      os.Getenv("X_AUTH_CODE"),
		AuthToken:     os.Getenv("X_AUTH_TOKEN"),
		CSRFToken:     os.Getenv("X_CSRF_TOKEN"),
		ProxyURL:      os.Getenv("X_PROXY_URL"),
		ProxyEnable:   proxyEnable,
		ProxyUsername: os.Getenv("X_PROXY_USERNAME"),
		ProxyPassword: os.Getenv("X_PROXY_PASSWORD"),
	}
}

// Login authenticates with X using environment variables
func Login() (*twitterscraper.Scraper, error) {
	opts := NewAuthOptions()
	scraper := twitterscraper.New()

	// Configure proxy if enabled and provided
	if opts.ProxyEnable && opts.ProxyURL != "" {
		proxyURL := opts.ProxyURL

		// Add authentication to proxy URL if credentials are provided
		if opts.ProxyUsername != "" {
			// Parse the existing proxy URL
			u, err := url.Parse(opts.ProxyURL)
			if err != nil {
				logger.Errorf("Failed to parse proxy URL: %v", err)
				return nil, fmt.Errorf("invalid proxy URL: %w", err)
			}

			// Add authentication
			if opts.ProxyPassword != "" {
				u.User = url.UserPassword(opts.ProxyUsername, opts.ProxyPassword)
			} else {
				u.User = url.User(opts.ProxyUsername)
			}
			proxyURL = u.String()
		}

		logger.Debugf("Setting up proxy with authentication")
		if err := scraper.SetProxy(proxyURL); err != nil {
			logger.Errorf("Failed to set proxy: %v", err)
			return nil, fmt.Errorf("failed to set proxy: %w", err)
		}
	}

	// Try token-based auth first if available
	if opts.AuthToken != "" && opts.CSRFToken != "" {
		logger.Debugf("Attempting token-based authentication")
		scraper.SetAuthToken(twitterscraper.AuthToken{
			Token:     opts.AuthToken,
			CSRFToken: opts.CSRFToken,
		})

		if scraper.IsLoggedIn() {
			logger.Infof("Successfully authenticated using auth token")
			return scraper, nil
		}
		logger.Warnf("Token-based authentication failed, falling back to credentials")
	}

	// Validate required credentials
	if opts.Username == "" || opts.Password == "" {
		logger.Errorf("Missing required credentials in environment variables")
		return nil, fmt.Errorf("missing required credentials: X_USERNAME and X_PASSWORD must be set")
	}

	// Attempt to load existing cookies
	if cookies, err := loadCookies(opts.Username); err == nil {
		logger.Debugf("Found existing cookies for %s, attempting to reuse", opts.Username)
		scraper.SetCookies(cookies)
		if scraper.IsLoggedIn() {
			logger.Infof("Successfully authenticated using existing cookies")
			return scraper, nil
		}
		logger.Debugf("Existing cookies invalid, attempting fresh login")
	}

	// Prepare login credentials
	credentials := []string{opts.Username, opts.Password}
	if opts.AuthCode != "" {
		logger.Debugf("2FA/Email verification code provided, adding to credentials")
		credentials = append(credentials, opts.AuthCode)
	}

	// Attempt login
	logger.Infof("Attempting login for user: %s", opts.Username)
	if err := scraper.Login(credentials...); err != nil {
		logger.Errorf("Login failed: %v", err)
		return nil, fmt.Errorf("login failed: %w", err)
	}

	// Verify login success
	if !scraper.IsLoggedIn() {
		logger.Errorf("Login verification failed")
		return nil, fmt.Errorf("login verification failed")
	}

	// Save cookies for future use
	logger.Debugf("Saving cookies for future sessions")
	if err := saveCookies(opts.Username, scraper.GetCookies()); err != nil {
		logger.Warnf("Failed to save cookies: %v", err)
	}

	logger.Infof("Successfully authenticated to X")
	return scraper, nil
}

func getCookiePath(username string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Errorf("Failed to get home directory: %v", err)
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	masaDir := filepath.Join(homeDir, ".masa")
	if err := os.MkdirAll(masaDir, 0700); err != nil {
		logger.Errorf("Failed to create .masa directory: %v", err)
		return "", fmt.Errorf("failed to create .masa directory: %w", err)
	}

	path := filepath.Join(masaDir, fmt.Sprintf("%s_twitter_cookies.json", username))
	logger.Debugf("Cookie path for %s: %s", username, path)
	return path, nil
}

func saveCookies(username string, cookies []*http.Cookie) error {
	path, err := getCookiePath(username)
	if err != nil {
		return err
	}

	data, err := json.Marshal(cookies)
	if err != nil {
		logger.Errorf("Failed to marshal cookies for %s: %v", username, err)
		return fmt.Errorf("failed to marshal cookies: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		logger.Errorf("Failed to write cookie file for %s: %v", username, err)
		return fmt.Errorf("failed to write cookie file: %w", err)
	}

	logger.Debugf("Successfully saved %d cookies for %s", len(cookies), username)
	return nil
}

func loadCookies(username string) ([]*http.Cookie, error) {
	path, err := getCookiePath(username)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		logger.Debugf("No existing cookie file found for %s: %v", username, err)
		return nil, fmt.Errorf("failed to read cookie file: %w", err)
	}

	var cookies []*http.Cookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		logger.Errorf("Failed to unmarshal cookies for %s: %v", username, err)
		return nil, fmt.Errorf("failed to unmarshal cookies: %w", err)
	}

	logger.Debugf("Successfully loaded %d cookies for %s", len(cookies), username)
	return cookies, nil
}
