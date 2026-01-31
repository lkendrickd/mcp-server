package config

import (
	"os"
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
	OTELCollectorHost    string
	OTELCollectorPort    string
	OTELCollectorAddress string // Combined host:port for backward compatibility
	apiKeys              map[string]struct{}
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
		OTELCollectorHost:    otelHost,
		OTELCollectorPort:    otelPort,
		OTELCollectorAddress: otelAddress,
		apiKeys:              make(map[string]struct{}),
	}

	// Parse API keys from comma-separated list
	keysStr := getEnv("API_KEYS", "")
	if keysStr != "" {
		keys := strings.Split(keysStr, ",")
		for _, key := range keys {
			trimmed := strings.TrimSpace(key)
			if trimmed != "" {
				cfg.apiKeys[trimmed] = struct{}{}
			}
		}
	}

	return cfg
}

// ValidateAPIKey checks if the provided key is valid using constant-time comparison
func (c *Config) ValidateAPIKey(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, exists := c.apiKeys[key]
	return exists
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
