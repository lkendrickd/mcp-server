package middleware

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("mcp-server/tools")

// logPayloadsEnabled controls whether sensitive data (inputs/outputs) are logged to traces.
// Defaults to false for security. Set via SetLogPayloads.
var logPayloadsEnabled = false

// SetLogPayloads configures whether tool inputs and outputs are logged to traces.
// When false (default), only tool names and error status are recorded.
// When true, full input/output data is recorded (security risk in production).
func SetLogPayloads(enabled bool) {
	logPayloadsEnabled = enabled
}

// TracedTool wraps an MCP tool handler with OpenTelemetry tracing.
// It creates a span for each tool call and records the tool name.
// Input parameters and output are only recorded if payload logging is enabled via SetLogPayloads.
func TracedTool[In any, Out any](toolName string, handler mcp.ToolHandlerFor[In, Out]) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input In) (*mcp.CallToolResult, Out, error) {
		ctx, span := tracer.Start(ctx, "tool/"+toolName,
			trace.WithSpanKind(trace.SpanKindInternal),
			trace.WithAttributes(
				attribute.String("mcp.tool.name", toolName),
			),
		)
		defer span.End()

		// Only record sensitive data if explicitly enabled
		if logPayloadsEnabled {
			// Record input parameters as JSON
			if inputJSON, err := json.Marshal(input); err == nil {
				span.SetAttributes(attribute.String("mcp.tool.input", string(inputJSON)))
			}

			// Record raw arguments if available
			if req != nil && req.Params.Arguments != nil {
				if argsJSON, err := json.Marshal(req.Params.Arguments); err == nil {
					span.SetAttributes(attribute.String("mcp.tool.arguments", string(argsJSON)))
				}
			}
		}

		// Call the actual handler
		result, output, err := handler(ctx, req, input)

		// Record error if any
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
			// Only record output if payload logging is enabled
			if logPayloadsEnabled {
				if outputJSON, err := json.Marshal(output); err == nil {
					span.SetAttributes(attribute.String("mcp.tool.output", string(outputJSON)))
				}
			}
		}

		return result, output, err
	}
}
