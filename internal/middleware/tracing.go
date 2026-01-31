package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// mcpRequest represents the JSON-RPC request structure for MCP
type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// toolCallParams represents the params for a tools/call request
type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// MCPTracingMiddleware adds MCP-specific attributes to the trace span.
// It captures the JSON-RPC method, tool name (for tool calls), and optionally arguments.
// When logPayloads is false (default), sensitive data like payloads and tool arguments
// are not recorded to prevent exposure of credentials or personal information.
func MCPTracingMiddleware(logPayloads bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only process POST requests to /mcp
			if r.Method != http.MethodPost || (r.URL.Path != "/mcp" && r.URL.Path != "/mcp/") {
				next.ServeHTTP(w, r)
				return
			}

			// Get the current span from context
			span := trace.SpanFromContext(r.Context())

			// Refactored reading the body with a limited size to prevent excessive memory usage
			const maxBodySize = 1 << 20 // 1 MB
			r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

			// Read the request body
			body, err := io.ReadAll(r.Body)
			if err != nil {
				// Record error in span for tracing
				if err == http.ErrBodyNotAllowed {
					span.SetAttributes(attribute.Bool("mcp.request.body_too_large", true))
				} else {
					span.SetAttributes(attribute.String("mcp.request.read_error", err.Error()))
				}
				next.ServeHTTP(w, r)
				return
			}
			// Restore the body for downstream handlers
			r.Body = io.NopCloser(bytes.NewReader(body))

			// Parse the JSON-RPC request
			var req mcpRequest
			if err := json.Unmarshal(body, &req); err == nil {
				// Add MCP-specific attributes (safe metadata only)
				span.SetAttributes(
					attribute.String("mcp.jsonrpc.version", req.JSONRPC),
					attribute.String("mcp.method", req.Method),
				)

				// Add request ID if present
				if req.ID != nil {
					switch id := req.ID.(type) {
					case float64:
						span.SetAttributes(attribute.Int("mcp.request.id", int(id)))
					case string:
						span.SetAttributes(attribute.String("mcp.request.id", id))
					}
				}

				// For tool calls, extract tool name (always safe to log)
				if req.Method == "tools/call" && req.Params != nil {
					var toolParams toolCallParams
					if err := json.Unmarshal(req.Params, &toolParams); err == nil {
						span.SetAttributes(attribute.String("mcp.tool.name", toolParams.Name))
						// Only log arguments if payload logging is explicitly enabled
						if logPayloads && toolParams.Arguments != nil {
							span.SetAttributes(attribute.String("mcp.tool.arguments", string(toolParams.Arguments)))
						}
					}
				}

				// Only record full payload if explicitly enabled (security risk)
				if logPayloads {
					payload := string(body)
					if len(payload) > 4096 {
						payload = payload[:4096] + "...(truncated)"
					}
					span.SetAttributes(attribute.String("mcp.request.payload", payload))
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
