package config

import (
	"os"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name            string
		envVars         map[string]string
		wantPort        string
		wantLogLevel    string
		wantAuthEnabled bool
		wantKeyCount    int
	}{
		{
			name:            "default values",
			envVars:         map[string]string{},
			wantPort:        "8080",
			wantLogLevel:    "info",
			wantAuthEnabled: false,
			wantKeyCount:    0,
		},
		{
			name: "custom port and log level",
			envVars: map[string]string{
				"PORT":      "9090",
				"LOG_LEVEL": "debug",
			},
			wantPort:        "9090",
			wantLogLevel:    "debug",
			wantAuthEnabled: false,
			wantKeyCount:    0,
		},
		{
			name: "auth enabled with single key",
			envVars: map[string]string{
				"AUTH_ENABLED": "true",
				"API_KEYS":     "secret-key-123",
			},
			wantPort:        "8080",
			wantLogLevel:    "info",
			wantAuthEnabled: true,
			wantKeyCount:    1,
		},
		{
			name: "auth enabled with multiple keys",
			envVars: map[string]string{
				"AUTH_ENABLED": "true",
				"API_KEYS":     "key1,key2,key3",
			},
			wantPort:        "8080",
			wantLogLevel:    "info",
			wantAuthEnabled: true,
			wantKeyCount:    3,
		},
		{
			name: "keys with whitespace",
			envVars: map[string]string{
				"API_KEYS": " key1 , key2 , key3 ",
			},
			wantPort:        "8080",
			wantLogLevel:    "info",
			wantAuthEnabled: false,
			wantKeyCount:    3,
		},
		{
			name: "empty keys filtered out",
			envVars: map[string]string{
				"API_KEYS": "key1,,key2,  ,key3",
			},
			wantPort:        "8080",
			wantLogLevel:    "info",
			wantAuthEnabled: false,
			wantKeyCount:    3,
		},
		{
			name: "auth enabled variations",
			envVars: map[string]string{
				"AUTH_ENABLED": "1",
			},
			wantPort:        "8080",
			wantLogLevel:    "info",
			wantAuthEnabled: true,
			wantKeyCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			clearEnv(t)

			// Set test environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			cfg := New()

			if cfg.Port != tt.wantPort {
				t.Errorf("Port = %q, want %q", cfg.Port, tt.wantPort)
			}

			if cfg.LogLevel != tt.wantLogLevel {
				t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, tt.wantLogLevel)
			}

			if cfg.AuthEnabled != tt.wantAuthEnabled {
				t.Errorf("AuthEnabled = %v, want %v", cfg.AuthEnabled, tt.wantAuthEnabled)
			}

			if cfg.APIKeyCount() != tt.wantKeyCount {
				t.Errorf("APIKeyCount = %d, want %d", cfg.APIKeyCount(), tt.wantKeyCount)
			}
		})
	}
}

func TestConfig_ValidateAPIKey(t *testing.T) {
	tests := []struct {
		name       string
		configKeys string
		testKey    string
		want       bool
	}{
		{
			name:       "valid key",
			configKeys: "key1,key2,key3",
			testKey:    "key2",
			want:       true,
		},
		{
			name:       "invalid key",
			configKeys: "key1,key2,key3",
			testKey:    "invalid",
			want:       false,
		},
		{
			name:       "empty key",
			configKeys: "key1,key2",
			testKey:    "",
			want:       false,
		},
		{
			name:       "no keys configured",
			configKeys: "",
			testKey:    "anykey",
			want:       false,
		},
		{
			name:       "case sensitive",
			configKeys: "SecretKey",
			testKey:    "secretkey",
			want:       false,
		},
		{
			name:       "exact match required",
			configKeys: "key1",
			testKey:    "key1 ",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("API_KEYS", tt.configKeys)

			cfg := New()

			if got := cfg.ValidateAPIKey(tt.testKey); got != tt.want {
				t.Errorf("ValidateAPIKey(%q) = %v, want %v", tt.testKey, got, tt.want)
			}
		})
	}
}

func TestConfig_HasAPIKeys(t *testing.T) {
	tests := []struct {
		name       string
		configKeys string
		want       bool
	}{
		{
			name:       "has keys",
			configKeys: "key1,key2",
			want:       true,
		},
		{
			name:       "no keys",
			configKeys: "",
			want:       false,
		},
		{
			name:       "only whitespace",
			configKeys: "  ,  ,  ",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("API_KEYS", tt.configKeys)

			cfg := New()

			if got := cfg.HasAPIKeys(); got != tt.want {
				t.Errorf("HasAPIKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		defaultValue bool
		want         bool
	}{
		{name: "true", value: "true", defaultValue: false, want: true},
		{name: "TRUE", value: "TRUE", defaultValue: false, want: true},
		{name: "True", value: "True", defaultValue: false, want: true},
		{name: "1", value: "1", defaultValue: false, want: true},
		{name: "yes", value: "yes", defaultValue: false, want: true},
		{name: "YES", value: "YES", defaultValue: false, want: true},
		{name: "on", value: "on", defaultValue: false, want: true},
		{name: "false", value: "false", defaultValue: true, want: false},
		{name: "FALSE", value: "FALSE", defaultValue: true, want: false},
		{name: "0", value: "0", defaultValue: true, want: false},
		{name: "no", value: "no", defaultValue: true, want: false},
		{name: "off", value: "off", defaultValue: true, want: false},
		{name: "invalid returns default true", value: "invalid", defaultValue: true, want: true},
		{name: "invalid returns default false", value: "invalid", defaultValue: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("TEST_BOOL", tt.value)

			if got := getEnvBool("TEST_BOOL", tt.defaultValue); got != tt.want {
				t.Errorf("getEnvBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetEnvBool_NotSet(t *testing.T) {
	clearEnv(t)

	if got := getEnvBool("NOT_SET", true); got != true {
		t.Errorf("getEnvBool() = %v, want true (default)", got)
	}

	if got := getEnvBool("NOT_SET", false); got != false {
		t.Errorf("getEnvBool() = %v, want false (default)", got)
	}
}

// clearEnv unsets relevant environment variables for clean test state
func clearEnv(t *testing.T) {
	t.Helper()
	vars := []string{"PORT", "LOG_LEVEL", "AUTH_ENABLED", "API_KEYS", "TEST_BOOL",
		"OTEL_COLLECTOR_HOST", "OTEL_COLLECTOR_PORT", "OTEL_COLLECTOR_ADDRESS"}
	for _, v := range vars {
		os.Unsetenv(v)
	}
}

func TestNew_OTELConfig(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		wantHost    string
		wantPort    string
		wantAddress string
	}{
		{
			name:        "defaults when not set",
			envVars:     map[string]string{},
			wantHost:    "",
			wantPort:    "4317",
			wantAddress: "",
		},
		{
			name: "host and default port",
			envVars: map[string]string{
				"OTEL_COLLECTOR_HOST": "localhost",
			},
			wantHost:    "localhost",
			wantPort:    "4317",
			wantAddress: "localhost:4317",
		},
		{
			name: "host and custom port",
			envVars: map[string]string{
				"OTEL_COLLECTOR_HOST": "otel.example.com",
				"OTEL_COLLECTOR_PORT": "4318",
			},
			wantHost:    "otel.example.com",
			wantPort:    "4318",
			wantAddress: "otel.example.com:4318",
		},
		{
			name: "backward compatibility with OTEL_COLLECTOR_ADDRESS",
			envVars: map[string]string{
				"OTEL_COLLECTOR_ADDRESS": "legacy-host:9999",
			},
			wantHost:    "",
			wantPort:    "4317",
			wantAddress: "legacy-host:9999",
		},
		{
			name: "ADDRESS takes precedence when set",
			envVars: map[string]string{
				"OTEL_COLLECTOR_HOST":    "new-host",
				"OTEL_COLLECTOR_PORT":    "5555",
				"OTEL_COLLECTOR_ADDRESS": "old-host:1234",
			},
			wantHost:    "new-host",
			wantPort:    "5555",
			wantAddress: "old-host:1234", // ADDRESS is preserved for backward compatibility
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			cfg := New()

			if cfg.OTELCollectorHost != tt.wantHost {
				t.Errorf("OTELCollectorHost = %q, want %q", cfg.OTELCollectorHost, tt.wantHost)
			}
			if cfg.OTELCollectorPort != tt.wantPort {
				t.Errorf("OTELCollectorPort = %q, want %q", cfg.OTELCollectorPort, tt.wantPort)
			}
			if cfg.OTELCollectorAddress != tt.wantAddress {
				t.Errorf("OTELCollectorAddress = %q, want %q", cfg.OTELCollectorAddress, tt.wantAddress)
			}
		})
	}
}
