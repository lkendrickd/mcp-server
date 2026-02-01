package config

import (
	"crypto/subtle"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Config holds the application configuration loaded from environment variables
type Config struct {
	Port                 string
	LogLevel             string
	MCPTransport         string // Transport mode: "stdio" or "http"
	Environment          string // Deployment environment (e.g., "production", "staging", "development")
	AuthEnabled          bool
	RateLimitEnabled     bool    // Whether to enable rate limiting
	RateLimitRPS         float64 // Requests per second per IP
	RateLimitBurst       int     // Maximum burst size per IP
	OTELCollectorHost    string
	OTELCollectorPort    string
	OTELCollectorAddress string   // Combined host:port for backward compatibility
	apiKeys              []string // Stored as slice for constant-time iteration
	mu                   sync.RWMutex
}

// New creates a new Config from environment variables
func New() *Config {
	otelHost := getEnv("OTEL_COLLECTOR_HOST", "")
	otelPort := getEnv("OTEL_COLLECTOR_PORT", "4317")

	// Build collector address from host and port
	// For backward compatibility, also check OTEL_COLLECTOR_ADDRESS
	otelAddress := getEnv("OTEL_COLLECTOR_ADDRESS", "")
	if otelAddress == "" && otelHost != "" {
		otelAddress = otelHost + ":" + otelPort
	}

	cfg := &Config{
		Port:                 getEnv("PORT", "8080"),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		MCPTransport:         getEnv("MCP_TRANSPORT", "stdio"),
		Environment:          getEnv("ENVIRONMENT", "development"),
		AuthEnabled:          getEnvBool("AUTH_ENABLED", false),
		RateLimitEnabled:     getEnvBool("RATE_LIMIT_ENABLED", true),
		RateLimitRPS:         getEnvFloat("RATE_LIMIT_RPS", 10.0),
		RateLimitBurst:       getEnvInt("RATE_LIMIT_BURST", 20),
		OTELCollectorHost:    otelHost,
		OTELCollectorPort:    otelPort,
		OTELCollectorAddress: otelAddress,
		apiKeys:              []string{},
	}

	// Parse API keys from comma-separated list
	keysStr := getEnv("API_KEYS", "")
	if keysStr != "" {
		keys := strings.Split(keysStr, ",")
		for _, key := range keys {
			trimmed := strings.TrimSpace(key)
			if trimmed != "" {
				cfg.apiKeys = append(cfg.apiKeys, trimmed)
			}
		}
	}

	return cfg
}

// ValidateAPIKey checks if the provided key is valid using constant-time comparison.
// It iterates through all keys to prevent timing attacks.
func (c *Config) ValidateAPIKey(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keyBytes := []byte(key)
	valid := false
	for _, storedKey := range c.apiKeys {
		if subtle.ConstantTimeCompare(keyBytes, []byte(storedKey)) == 1 {
			valid = true
		}
	}
	return valid
}

// APIKeyCount returns the number of configured API keys
func (c *Config) APIKeyCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.apiKeys)
}

// HasAPIKeys returns true if any API keys are configured
func (c *Config) HasAPIKeys() bool {
	return c.APIKeyCount() > 0
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvBool retrieves an environment variable as a boolean
func getEnvBool(key string, defaultValue bool) bool {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}

	switch strings.ToLower(value) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return defaultValue
	}
}

// getEnvFloat retrieves an environment variable as a float64
func getEnvFloat(key string, defaultValue float64) float64 {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}

	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return defaultValue
	}
	return f
}

// getEnvInt retrieves an environment variable as an int
func getEnvInt(key string, defaultValue int) int {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}

	i, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return i
}
