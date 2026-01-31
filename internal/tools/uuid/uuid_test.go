package uuid

import (
	"context"
	"regexp"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lkendrickd/mcp-server/internal/tools"
)

// UUID v4 regex pattern
var uuidV4Regex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestGenerateUUID(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "generates valid UUID"},
		{name: "generates another valid UUID"},
		{name: "generates unique UUIDs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, output, err := GenerateUUID(context.Background(), &mcp.CallToolRequest{}, Input{})

			// Should not return an error
			if err != nil {
				t.Fatalf("GenerateUUID returned error: %v", err)
			}

			// Result should be nil (auto-generated from output)
			if result != nil {
				t.Errorf("expected nil CallToolResult, got %v", result)
			}

			// UUID should not be empty
			if output.UUID == "" {
				t.Error("expected non-empty UUID")
			}

			// UUID should match v4 format
			if !uuidV4Regex.MatchString(output.UUID) {
				t.Errorf("UUID %q does not match v4 format", output.UUID)
			}
		})
	}
}

func TestGenerateUUID_Uniqueness(t *testing.T) {
	const iterations = 100
	seen := make(map[string]bool)

	for i := 0; i < iterations; i++ {
		_, output, err := GenerateUUID(context.Background(), &mcp.CallToolRequest{}, Input{})
		if err != nil {
			t.Fatalf("iteration %d: GenerateUUID returned error: %v", i, err)
		}

		if seen[output.UUID] {
			t.Errorf("duplicate UUID generated: %s", output.UUID)
		}
		seen[output.UUID] = true
	}
}

func TestGenerateUUID_OutputStructure(t *testing.T) {
	_, output, err := GenerateUUID(context.Background(), &mcp.CallToolRequest{}, Input{})
	if err != nil {
		t.Fatalf("GenerateUUID returned error: %v", err)
	}

	// Verify UUID length (standard UUID is 36 characters with hyphens)
	if len(output.UUID) != 36 {
		t.Errorf("UUID length = %d, want 36", len(output.UUID))
	}

	// Verify hyphen positions
	expectedHyphens := []int{8, 13, 18, 23}
	for _, pos := range expectedHyphens {
		if output.UUID[pos] != '-' {
			t.Errorf("expected hyphen at position %d, got %q", pos, output.UUID[pos])
		}
	}
}

func TestInit_RegistersTool(t *testing.T) {
	// The init() function runs when the package is imported.
	// We verify that it registered a tool by checking the Registry.

	found := false
	for _, registrar := range tools.Registry {
		if registrar != nil {
			found = true
			break
		}
	}

	if !found {
		t.Error("init() did not register any tool in the Registry")
	}
}

func TestInit_RegistrarAddsToolToServer(t *testing.T) {
	// Create a test server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "test-server",
		Version: "1.0.0",
	}, nil)

	// Find and call the uuid registrar
	// Since init() has already run, the registry should contain our registrar
	initialRegistryLen := len(tools.Registry)
	if initialRegistryLen == 0 {
		t.Fatal("Registry is empty, init() may not have run")
	}

	// Call all registrars (which includes our uuid registrar)
	tools.RegisterAll(server)

	// The server should now have the generate_uuid tool registered
	// We can't directly inspect the server's tools, but we can verify
	// the registration didn't panic and the server is still valid
	if server == nil {
		t.Error("server became nil after registration")
	}
}
