# Developer Guide: Adding Tools to MCP-Server

This guide walks you through adding a new tool to the MCP server.

## Overview

Tools in MCP are functions that LLM applications can call to perform actions. Each tool has:
- A **name** - unique identifier (e.g., `generate_uuid`)
- A **description** - what the tool does (shown to the LLM)
- An **input schema** - parameters the tool accepts (auto-generated from struct)
- A **handler function** - the implementation

## Project Structure

```
internal/tools/
├── registry.go          # Tool registration system
└── uuid/                # Example tool package
    └── uuid.go
```

Each tool lives in its own package under `internal/tools/`.

## Step-by-Step: Adding a New Tool

We'll create a `timestamp` tool that returns the current time in various formats.

### Step 1: Create the Tool Package

```bash
mkdir -p internal/tools/timestamp
```

### Step 2: Implement the Tool

**`internal/tools/timestamp/timestamp.go`**
```go
package timestamp

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lkendrickd/mcp-server/internal/tools"
)

// Input defines the tool's input parameters.
// Use jsonschema tags to provide descriptions for the LLM.
type Input struct {
	Format string `json:"format,omitempty" jsonschema:"enum=unix,enum=iso8601,enum=rfc3339,description=Output format (default: rfc3339)"`
}

// Output defines the tool's response structure.
type Output struct {
	Timestamp string `json:"timestamp" jsonschema:"description=The formatted timestamp"`
	Unix      int64  `json:"unix" jsonschema:"description=Unix timestamp in seconds"`
}

// GetTimestamp returns the current time in the requested format.
func GetTimestamp(_ context.Context, _ *mcp.CallToolRequest, input Input) (*mcp.CallToolResult, Output, error) {
	now := time.Now()

	var formatted string
	switch input.Format {
	case "unix":
		formatted = fmt.Sprintf("%d", now.Unix())
	case "iso8601":
		formatted = now.Format("2006-01-02T15:04:05Z07:00")
	case "rfc3339", "":
		formatted = now.Format(time.RFC3339)
	default:
		formatted = now.Format(time.RFC3339)
	}

	return nil, Output{
		Timestamp: formatted,
		Unix:      now.Unix(),
	}, nil
}

// init registers the tool with the MCP server.
func init() {
	tools.Register(func(server *mcp.Server) {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "get_timestamp",
			Description: "Get the current timestamp in various formats",
		}, GetTimestamp)
	})
}
```

### Step 3: Import the Tool Package

Add a blank import to the main server file.

**`cmd/mcp-server.go`** (add to imports)
```go
import (
	// ... existing imports ...

	_ "github.com/lkendrickd/mcp-server/internal/tools/uuid"
	_ "github.com/lkendrickd/mcp-server/internal/tools/timestamp"  // Add this
)
```

The blank import (`_`) triggers the package's `init()` function, which registers the tool.

### Step 4: Build and Test

```bash
# Build
make build

# Run tests
make test

# Test locally
make run
```

## Tool Handler Function Signature

The MCP SDK uses generics. Your handler must match this signature:

```go
func HandlerName(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input InputType,
) (*mcp.CallToolResult, OutputType, error)
```

Where:
- `ctx` - Context for cancellation/timeouts
- `req` - Raw MCP request (rarely needed)
- `input` - Your input struct (auto-parsed from JSON)
- Returns: `(*mcp.CallToolResult, OutputType, error)`
  - Return `nil` for CallToolResult to auto-generate from OutputType
  - Return error for failures

## Input/Output Struct Tags

Use struct tags to define the JSON schema the LLM sees:

```go
type Input struct {
	// Required field
	Name string `json:"name" jsonschema:"description=The user's name"`

	// Optional field (use omitempty)
	Age int `json:"age,omitempty" jsonschema:"description=The user's age"`

	// Enum values
	Format string `json:"format" jsonschema:"enum=json,enum=xml,enum=csv"`

	// With default description
	Verbose bool `json:"verbose,omitempty" jsonschema:"description=Enable verbose output"`
}
```

## Error Handling

Return errors for failures:

**`internal/tools/mytool/mytool.go`** (example pattern)
```go
func MyTool(ctx context.Context, req *mcp.CallToolRequest, input Input) (*mcp.CallToolResult, Output, error) {
	if input.Name == "" {
		return nil, Output{}, fmt.Errorf("name is required")
	}

	result, err := doSomething(input.Name)
	if err != nil {
		return nil, Output{}, fmt.Errorf("failed to process: %w", err)
	}

	return nil, Output{Result: result}, nil
}
```

## Logging

Use structured logging with `slog`:

**`internal/tools/mytool/mytool.go`** (example pattern)
```go
package mytool

import (
	"log/slog"
	"os"
)

var logger = slog.New(slog.NewJSONHandler(os.Stderr, nil))

func MyTool(ctx context.Context, req *mcp.CallToolRequest, input Input) (*mcp.CallToolResult, Output, error) {
	logger.Info("tool called", "tool", "my_tool", "input", input.Name)

	// ... implementation ...

	logger.Debug("tool completed", "tool", "my_tool", "result", result)
	return nil, Output{Result: result}, nil
}
```

## Testing Tools

**`internal/tools/timestamp/timestamp_test.go`**
```go
package timestamp

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGetTimestamp(t *testing.T) {
	tests := []struct {
		name   string
		input  Input
		check  func(t *testing.T, out Output)
	}{
		{
			name:  "default format",
			input: Input{},
			check: func(t *testing.T, out Output) {
				if out.Timestamp == "" {
					t.Error("expected non-empty timestamp")
				}
				if out.Unix == 0 {
					t.Error("expected non-zero unix timestamp")
				}
			},
		},
		{
			name:  "unix format",
			input: Input{Format: "unix"},
			check: func(t *testing.T, out Output) {
				if out.Timestamp == "" {
					t.Error("expected non-empty timestamp")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, out, err := GetTimestamp(context.Background(), &mcp.CallToolRequest{}, tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, out)
		})
	}
}
```

Run tests:

```bash
make test
```

## Complete Example: HTTP Fetch Tool

Here's a more complex example that fetches data from a URL.

**`internal/tools/httpfetch/httpfetch.go`**
```go
package httpfetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lkendrickd/mcp-server/internal/tools"
)

type Input struct {
	URL     string `json:"url" jsonschema:"description=The URL to fetch"`
	Method  string `json:"method,omitempty" jsonschema:"enum=GET,enum=POST,enum=HEAD,description=HTTP method (default: GET)"`
	Timeout int    `json:"timeout,omitempty" jsonschema:"description=Timeout in seconds (default: 30)"`
}

type Output struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

func Fetch(ctx context.Context, _ *mcp.CallToolRequest, input Input) (*mcp.CallToolResult, Output, error) {
	if input.URL == "" {
		return nil, Output{}, fmt.Errorf("url is required")
	}

	method := input.Method
	if method == "" {
		method = "GET"
	}

	timeout := input.Timeout
	if timeout == 0 {
		timeout = 30
	}

	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}

	req, err := http.NewRequestWithContext(ctx, method, input.URL, nil)
	if err != nil {
		return nil, Output{}, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, Output{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, Output{}, fmt.Errorf("failed to read body: %w", err)
	}

	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return nil, Output{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func init() {
	tools.Register(func(server *mcp.Server) {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "http_fetch",
			Description: "Fetch content from a URL via HTTP",
		}, Fetch)
	})
}
```

## Checklist for New Tools

- [ ] Create package directory: `internal/tools/<toolname>/`
- [ ] Implement tool with proper Input/Output structs
- [ ] Add jsonschema tags for LLM visibility
- [ ] Register via `init()` using `tools.Register()`
- [ ] Add blank import in `cmd/mcp-server.go`
- [ ] Write unit tests
- [ ] Run `make test` and `make lint`
- [ ] Test with an MCP client

## Common Patterns

### Tool with No Input

**`internal/tools/ping/ping.go`** (example)
```go
type Input struct{}

func Ping(_ context.Context, _ *mcp.CallToolRequest, _ Input) (*mcp.CallToolResult, Output, error) {
	return nil, Output{Result: "pong"}, nil
}
```

### Tool with Complex Output

```go
type Output struct {
	Items []Item `json:"items"`
	Total int    `json:"total"`
	Page  int    `json:"page"`
}

type Item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
```

### Tool with Context Timeout

```go
func MyTool(ctx context.Context, _ *mcp.CallToolRequest, input Input) (*mcp.CallToolResult, Output, error) {
	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Use ctx in operations that support it
	result, err := someOperation(ctx, input)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, Output{}, fmt.Errorf("operation timed out")
		}
		return nil, Output{}, err
	}

	return nil, Output{Result: result}, nil
}
```

## Resources

- [MCP Go SDK Documentation](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp)
- [MCP Protocol Specification](https://modelcontextprotocol.io/)
- [JSON Schema Reference](https://json-schema.org/)
