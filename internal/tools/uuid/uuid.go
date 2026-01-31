package uuid

import (
	"context"
	"log/slog"
	"os"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lkendrickd/mcp-server/internal/middleware"
	"github.com/lkendrickd/mcp-server/internal/tools"
)

var logger = slog.New(slog.NewJSONHandler(os.Stderr, nil))

// Input is the input for the UUID generator (empty, no parameters needed).
type Input struct{}

// Output is the output of the UUID generator.
type Output struct {
	UUID string `json:"uuid" jsonschema:"the generated UUID v4"`
}

// GenerateUUID generates a new UUID v4.
func GenerateUUID(_ context.Context, _ *mcp.CallToolRequest, _ Input) (*mcp.CallToolResult, Output, error) {
	result := uuid.New().String()
	logger.Info("tool called", "tool", "generate_uuid", "uuid", result)
	return nil, Output{UUID: result}, nil
}

func init() {
	tools.Register(func(server *mcp.Server) {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "generate_uuid",
			Description: "Generate a new UUID v4",
		}, middleware.TracedTool("generate_uuid", GenerateUUID))
	})
}
