package tools

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRegister(t *testing.T) {
	// Save original registry state and restore after test
	originalRegistry := Registry
	t.Cleanup(func() {
		Registry = originalRegistry
	})

	// Reset registry for this test
	Registry = nil

	tests := []struct {
		name             string
		registrarsToAdd  int
		expectedLenAfter int
	}{
		{
			name:             "register single registrar",
			registrarsToAdd:  1,
			expectedLenAfter: 1,
		},
		{
			name:             "register multiple registrars",
			registrarsToAdd:  3,
			expectedLenAfter: 4, // 1 from previous + 3 new
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < tt.registrarsToAdd; i++ {
				Register(func(_ *mcp.Server) {
					// no-op registrar for testing
				})
			}

			if len(Registry) != tt.expectedLenAfter {
				t.Errorf("Registry length = %d, want %d", len(Registry), tt.expectedLenAfter)
			}
		})
	}
}

func TestRegisterAll(t *testing.T) {
	// Save original registry state and restore after test
	originalRegistry := Registry
	t.Cleanup(func() {
		Registry = originalRegistry
	})

	tests := []struct {
		name           string
		registrarCount int
	}{
		{
			name:           "empty registry",
			registrarCount: 0,
		},
		{
			name:           "single registrar",
			registrarCount: 1,
		},
		{
			name:           "multiple registrars",
			registrarCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset registry for each test case
			Registry = nil

			// Track which registrars were called
			called := make([]bool, tt.registrarCount)

			// Register test registrars
			for i := 0; i < tt.registrarCount; i++ {
				idx := i // capture loop variable
				Register(func(_ *mcp.Server) {
					called[idx] = true
				})
			}

			// Create a test server
			server := mcp.NewServer(&mcp.Implementation{
				Name:    "test-server",
				Version: "1.0.0",
			}, nil)

			// Call RegisterAll
			RegisterAll(server)

			// Verify all registrars were called
			for i, wasCalled := range called {
				if !wasCalled {
					t.Errorf("registrar %d was not called", i)
				}
			}
		})
	}
}

func TestRegisterAllPassesServer(t *testing.T) {
	// Save original registry state and restore after test
	originalRegistry := Registry
	t.Cleanup(func() {
		Registry = originalRegistry
	})

	// Reset registry
	Registry = nil

	// Create a test server
	expectedServer := mcp.NewServer(&mcp.Implementation{
		Name:    "test-server",
		Version: "1.0.0",
	}, nil)

	var receivedServer *mcp.Server

	Register(func(s *mcp.Server) {
		receivedServer = s
	})

	RegisterAll(expectedServer)

	if receivedServer != expectedServer {
		t.Error("registrar did not receive the expected server instance")
	}
}
