package x

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/masa-finance/masa-sdk-go/pkg/logger"
)

// Default configuration values
var (
	DefaultBaseURL string
	DefaultAPIBase string
)

func init() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		logger.Init("INFO")
		logger.Warnf(".env file not found, using default values")
	} else {
		logLevel := getEnvWithFallbackSilent("LOG_LEVEL", "INFO")
		logger.Init(logLevel)
	}

	// Initialize URLs after .env is loaded and log their source
	DefaultBaseURL = getEnvWithFallbackSilent("MASA_PROTOCOL_NODE_API", "http://localhost:8080")
	if _, exists := os.LookupEnv("MASA_PROTOCOL_NODE_API"); exists {
		logger.Infof("Using MASA_PROTOCOL_NODE_API from environment: %s", DefaultBaseURL)
	} else {
		logger.Infof("Using default MASA_PROTOCOL_NODE_API: %s", DefaultBaseURL)
	}

	DefaultAPIBase = getEnvWithFallbackSilent("MASA_PROTOCOL_NODE_API_PATH", "/api/v1/data")
	if _, exists := os.LookupEnv("MASA_PROTOCOL_NODE_API_PATH"); exists {
		logger.Infof("Using MASA_PROTOCOL_NODE_API_PATH from environment: %s", DefaultAPIBase)
	} else {
		logger.Infof("Using default MASA_PROTOCOL_NODE_API_PATH: %s", DefaultAPIBase)
	}

	// Debug logging for logger level
	logger.Debugf("Logger initialized with level: %s", getEnvWithFallbackSilent("LOG_LEVEL", "INFO"))
}

// Helper function that doesn't log (used during initialization)
func getEnvWithFallbackSilent(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
