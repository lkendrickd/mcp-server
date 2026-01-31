package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPTracingMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		wantBodyPassed bool
	}{
		{
			name:           "POST to /mcp passes body through",
			method:         http.MethodPost,
			path:           "/mcp",
			body:           `{"jsonrpc":"2.0","method":"initialize","id":1}`,
			wantBodyPassed: true,
		},
		{
			name:           "POST to /mcp/ passes body through",
			method:         http.MethodPost,
			path:           "/mcp/",
			body:           `{"jsonrpc":"2.0","method":"tools/call","id":2,"params":{"name":"test","arguments":{}}}`,
			wantBodyPassed: true,
		},
		{
			name:           "GET to /mcp skips processing",
			method:         http.MethodGet,
			path:           "/mcp",
			body:           "",
			wantBodyPassed: true,
		},
		{
			name:           "POST to /health skips processing",
			method:         http.MethodPost,
			path:           "/health",
			body:           `{"test":"data"}`,
			wantBodyPassed: true,
		},
		{
			name:           "POST to /metrics skips processing",
			method:         http.MethodPost,
			path:           "/metrics",
			body:           `{"test":"data"}`,
			wantBodyPassed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedBody string
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				receivedBody = string(body)
				w.WriteHeader(http.StatusOK)
			})

			wrapped := MCPTracingMiddleware(handler)

			var reqBody io.Reader
			if tt.body != "" {
				reqBody = bytes.NewBufferString(tt.body)
			}
			req := httptest.NewRequest(tt.method, tt.path, reqBody)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}

			if tt.wantBodyPassed && receivedBody != tt.body {
				t.Errorf("body = %q, want %q", receivedBody, tt.body)
			}
		})
	}
}

func TestMCPTracingMiddleware_JSONParsing(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "valid initialize request",
			body: `{"jsonrpc":"2.0","method":"initialize","id":1,"params":{}}`,
		},
		{
			name: "valid tools/call request",
			body: `{"jsonrpc":"2.0","method":"tools/call","id":2,"params":{"name":"generate_uuid","arguments":{"key":"value"}}}`,
		},
		{
			name: "notification without id",
			body: `{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		},
		{
			name: "string id",
			body: `{"jsonrpc":"2.0","method":"test","id":"abc-123"}`,
		},
		{
			name: "invalid JSON still passes through",
			body: `{invalid json}`,
		},
		{
			name: "empty body still passes through",
			body: ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			wrapped := MCPTracingMiddleware(handler)

			req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}
		})
	}
}

func TestMCPTracingMiddleware_BodyRestoration(t *testing.T) {
	originalBody := `{"jsonrpc":"2.0","method":"tools/call","id":1,"params":{"name":"test"}}`

	readCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read body: %v", err)
		}
		if string(body) != originalBody {
			t.Errorf("body = %q, want %q", string(body), originalBody)
		}
		readCount++
		w.WriteHeader(http.StatusOK)
	})

	wrapped := MCPTracingMiddleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(originalBody))
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if readCount != 1 {
		t.Errorf("handler called %d times, want 1", readCount)
	}
}

func TestMcpRequest_Unmarshal(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		wantMethod  string
		wantVersion string
	}{
		{
			name:        "initialize method",
			json:        `{"jsonrpc":"2.0","method":"initialize","id":1}`,
			wantMethod:  "initialize",
			wantVersion: "2.0",
		},
		{
			name:        "tools/call method",
			json:        `{"jsonrpc":"2.0","method":"tools/call","id":2}`,
			wantMethod:  "tools/call",
			wantVersion: "2.0",
		},
		{
			name:        "tools/list method",
			json:        `{"jsonrpc":"2.0","method":"tools/list","id":3}`,
			wantMethod:  "tools/list",
			wantVersion: "2.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req mcpRequest
			if err := json.Unmarshal([]byte(tt.json), &req); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if req.Method != tt.wantMethod {
				t.Errorf("method = %q, want %q", req.Method, tt.wantMethod)
			}
			if req.JSONRPC != tt.wantVersion {
				t.Errorf("jsonrpc = %q, want %q", req.JSONRPC, tt.wantVersion)
			}
		})
	}
}

func TestToolCallParams_Unmarshal(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantName string
		wantArgs string
	}{
		{
			name:     "simple tool call",
			json:     `{"name":"generate_uuid","arguments":{}}`,
			wantName: "generate_uuid",
			wantArgs: "{}",
		},
		{
			name:     "tool call with arguments",
			json:     `{"name":"greet","arguments":{"name":"world"}}`,
			wantName: "greet",
			wantArgs: `{"name":"world"}`,
		},
		{
			name:     "tool call without arguments",
			json:     `{"name":"test_tool"}`,
			wantName: "test_tool",
			wantArgs: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var params toolCallParams
			if err := json.Unmarshal([]byte(tt.json), &params); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if params.Name != tt.wantName {
				t.Errorf("name = %q, want %q", params.Name, tt.wantName)
			}
			if tt.wantArgs != "" && string(params.Arguments) != tt.wantArgs {
				t.Errorf("arguments = %q, want %q", string(params.Arguments), tt.wantArgs)
			}
		})
	}
}
