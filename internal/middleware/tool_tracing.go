package middleware

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("mcp-server/tools")

// TracedTool wraps an MCP tool handler with OpenTelemetry tracing.
// It creates a span for each tool call and records the tool name and error status.
func TracedTool[In any, Out any](toolName string, handler mcp.ToolHandlerFor[In, Out]) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input In) (*mcp.CallToolResult, Out, error) {
		ctx, span := tracer.Start(ctx, "tool/"+toolName,
			trace.WithSpanKind(trace.SpanKindInternal),
			trace.WithAttributes(
				attribute.String("mcp.tool.name", toolName),
			),
		)
		defer span.End()

		// Call the actual handler
		result, output, err := handler(ctx, req, input)

		// Record error if any
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}

		return result, output, err
	}
}
