package x

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/masa-finance/masa-sdk-go/pkg/logger"
)

func init() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		// It's okay if .env doesn't exist, we'll use defaults
		logger.Warnf(".env file not found, using default values")
	}

	// Initialize logger with environment configuration
	logLevel := getEnvWithFallback("LOG_LEVEL", "INFO")
	logger.Init(logLevel)
	logger.Debugf("Logger initialized with level: %s", logLevel)
	logger.Debugf("API Base URL: %s", DefaultBaseURL)
	logger.Debugf("API Base Path: %s", DefaultAPIBase)
}

// Default configuration values
var (
	DefaultBaseURL = getEnvWithFallback("MASA_PROTOCOL_NODE_API", "http://localhost:8080")
	DefaultAPIBase = getEnvWithFallback("MASA_PROTOCOL_NODE_API_PATH", "/api/v1/data")
)

// Helper function to get environment variable with fallback
func getEnvWithFallback(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		logger.Debugf("Loading %s from environment: %s", key, value)
		return value
	}
	logger.Debugf("Using fallback value for %s: %s", key, fallback)
	return fallback
}
