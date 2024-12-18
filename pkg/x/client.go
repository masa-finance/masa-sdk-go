package x

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/masa-finance/masa-sdk-go/pkg/logger"
)

func init() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		// It's okay if .env doesn't exist, we'll use defaults
		log.Printf("Warning: .env file not found, using default values")
	}

	// Initialize logger with environment configuration
	logger.Init(getEnvWithFallback("LOG_LEVEL", "INFO"))
}

// Default configuration values
var (
	DefaultBaseURL = getEnvWithFallback("MASA_BASE_URL", "http://localhost:8080")
	DefaultAPIBase = getEnvWithFallback("MASA_API_PATH", "/api/v1/data")
)

// Helper function to get environment variable with fallback
func getEnvWithFallback(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
