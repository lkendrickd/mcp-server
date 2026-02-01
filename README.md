## MCP-Server

<img src="images/logo.png" alt="MCP Server Logo" width="400"/>

### Create Tools for your LLMs with MCP-Server! 

A Model Context Protocol (MCP) server built with Go, supporting both stdio and HTTP transports for integration with MCP-compatible clients.

### Features

- MCP protocol support via [go-sdk](https://github.com/modelcontextprotocol/go-sdk)
- Dual transport modes: stdio (CLI) and HTTP (Streamable HTTP)
- API key authentication middleware for HTTP transport
- OpenTelemetry tracing with MCP-specific attributes (tool calls, methods, arguments)
- Prometheus metrics with path, method, and status labels
- Structured JSON logging via `slog`
- Configuration via environment variables
- Distroless Docker image for minimal attack surface
- Comprehensive unit tests with table-driven patterns using only standard library no external test frameworks

### Tools

| Tool | Description |
|------|-------------|
| `generate_uuid` | Generate a UUID v4 |

> **Want to add your own tool?** Check out the [Developer Guide](docs/DEVELOPER_GUIDE.md) for a step-by-step walkthrough.

**MCP Design Flowchart:**
The flowchart can help guide you during a new tool implementation. [Flowchart](docs/mcptool_chart.md)

### Requirements

- **Docker** and **Docker Compose** (recommended)
- **Go 1.25+** (for local development only)
- **Make** (optional, for convenience targets)

### Endpoints (HTTP Transport)

| Endpoint | Method | Auth Required | Description |
|----------|--------|---------------|-------------|
| `/health` | GET | No | Health check |
| `/metrics` | GET | No | Prometheus metrics |
| `/mcp` | POST | Yes* | MCP HTTP endpoint |

*When `AUTH_ENABLED=true`

### Quick Start

```bash
# 1. Configure environment
make config

# 2. Edit .env to set your API key (optional but recommended)
#    API_KEYS=your-secret-key

# 3. Start the server
make docker-up

# 4. Verify it's running
curl http://localhost:8080/health

# 5. View logs
make docker-logs

# 6. Check the health endpoint
curl http://localhost:8080/health
```

For local development without Docker, see the [Development](#development) section.

### Configuration

All configuration is via environment variables.

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `MCP_TRANSPORT` | `stdio` | Transport mode: `stdio` or `http` |
| `AUTH_ENABLED` | `false` | Enable API key authentication (HTTP only) |
| `API_KEYS` | | Comma-separated list of valid API keys |
| `ENVIRONMENT` | `development` | Deployment environment (used in telemetry) |
| `RATE_LIMIT_ENABLED` | `true` | Enable per-IP rate limiting (DDoS protection) |
| `RATE_LIMIT_RPS` | `10` | Requests per second per IP |
| `RATE_LIMIT_BURST` | `20` | Maximum burst size per IP |
| `OTEL_COLLECTOR_HOST` | | OpenTelemetry collector hostname (enables tracing) |
| `OTEL_COLLECTOR_PORT` | `4317` | OpenTelemetry collector gRPC port |

```bash
# Example: Run HTTP with authentication
MCP_TRANSPORT=http AUTH_ENABLED=true API_KEYS="key1,key2" make run

# Example: Run with OpenTelemetry tracing
OTEL_COLLECTOR_HOST=localhost OTEL_COLLECTOR_PORT=4317 make run

# Example: Run with custom rate limiting
RATE_LIMIT_RPS=100 RATE_LIMIT_BURST=50 make run
```

### Transport Modes

**Stdio (default):** For use with MCP clients and CLI tools. The server communicates via stdin/stdout.

```bash
make run
```

**HTTP:** For HTTP-based deployments. Exposes the MCP protocol over Streamable HTTP.

```bash
MCP_TRANSPORT=http make run
```

### Security

The server includes several security features enabled by default:

#### Rate Limiting

Per-IP rate limiting protects against DDoS attacks using a token bucket algorithm:

- **Enabled by default** (`RATE_LIMIT_ENABLED=true`)
- Default: 10 requests/second with burst of 20
- Supports `X-Forwarded-For` and `X-Real-IP` headers for proxy deployments
- Returns `429 Too Many Requests` with `Retry-After` header when exceeded

```bash
# Customize rate limits
RATE_LIMIT_RPS=100 RATE_LIMIT_BURST=200 make run
```

#### Secure Defaults

- **No payload logging**: Request/response payloads are never logged to traces
- **Constant-time API key comparison**: Prevents timing attacks on authentication

### Authentication

For simplicity API key authentication is implemented in the middleware. This can obviously be replaced with a more robust solution as needed.

When `AUTH_ENABLED=true`, the `/mcp` endpoint requires a valid API key in the `X-API-Key` header.

```bash
# Generate a secure API key
openssl rand -hex 32

# Request with API key
curl -X POST http://localhost:8080/mcp \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"initialize","id":1,"params":{...}}'
```

Unauthenticated requests return `401 Unauthorized`:
```json
{"error":"missing API key"}
```

Invalid keys return:
```json
{"error":"invalid API key"}
```

### MCP Client Integration - Example Config

For MCP clients with HTTP transport:

```json
{
  "mcpServers": {
    "mcp-server": {
      "type": "http",
      "url": "http://localhost:8080/mcp",
      "headers": {
        "X-API-Key": "your-api-key"
      }
    }
  }
}
```

### Curl Examples

Health Check:
```bash
curl http://localhost:8080/health
```

Metrics:
```bash
curl http://localhost:8080/metrics
```

#### MCP Session Workflow

MCP uses a stateful session protocol. To call tools, you must:
1. Initialize a session and capture the session ID
2. Send an initialized notification
3. Call tools using the session ID

**Step 1: Initialize session and capture session ID**
```bash
# Initialize and extract session ID from response headers
SESSION_ID=$(curl -s -D - -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"curl-test","version":"1.0"}}}' \
  2>&1 | grep -i "mcp-session-id" | awk '{print $2}' | tr -d '\r')

echo "Session ID: $SESSION_ID"
```

**Step 2: Send initialized notification**
```bash
curl -s -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","method":"notifications/initialized"}'
```

**Step 3: Call a tool**
```bash
curl -s -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"generate_uuid","arguments":{}}}'
```

**Complete script example:**
```bash
#!/bin/bash
# Test MCP server with a complete session workflow

API_KEY="your-api-key"
BASE_URL="http://localhost:8080"

# Initialize session
echo "Initializing session..."
SESSION_ID=$(curl -s -D - -X POST "$BASE_URL/mcp" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"curl-test","version":"1.0"}}}' \
  2>&1 | grep -i "mcp-session-id" | awk '{print $2}' | tr -d '\r')

if [ -z "$SESSION_ID" ]; then
  echo "Failed to get session ID"
  exit 1
fi
echo "Session ID: $SESSION_ID"

# Send initialized notification
echo "Sending initialized notification..."
curl -s -X POST "$BASE_URL/mcp" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","method":"notifications/initialized"}'

# Call generate_uuid tool
echo -e "\nCalling generate_uuid tool..."
curl -s -X POST "$BASE_URL/mcp" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"generate_uuid","arguments":{}}}' | jq .

echo -e "\nDone!"
```

### Docker

The Docker image uses a multi-stage build with a distroless runtime image for security.

```bash
# Build and run standalone container (HTTP mode)
make docker-run

# Run with auth enabled
AUTH_ENABLED=true API_KEYS="secret-key" make docker-run

# Stop and remove container
make docker-clean
```

#### Docker Compose

```bash
# Start services
make docker-up

# View logs
make docker-logs

# Restart services
make docker-restart

# Stop services
make docker-down
```

### Instrumentation

The server includes OpenTelemetry instrumentation for distributed tracing. When running with Docker Compose, traces flow through a complete observability stack:

```
mcp-server → otel-collector → Jaeger
```

#### Architecture

| Service | Port | Description |
|---------|------|-------------|
| mcp-server | 8080 | MCP server with OTLP gRPC exporter |
| otel-collector | 4317 | OpenTelemetry Collector (gRPC receiver) |
| jaeger | 16686 | Jaeger UI for trace visualization |

#### Trace Attributes

The instrumentation captures MCP-specific attributes on each trace:

| Attribute | Description |
|-----------|-------------|
| `mcp.method` | JSON-RPC method (e.g., `tools/call`, `initialize`) |
| `mcp.tool.name` | Tool being called (e.g., `generate_uuid`) |
| `mcp.request.id` | JSON-RPC request ID |
| `service.name` | Service identifier (`mcp-server`) |
| `service.version` | Version from the `version` file |
| `deployment.environment` | Environment (e.g., `development`, `production`) |

#### Viewing Traces

1. Start the stack with `make docker-up`
2. Make some requests to the MCP server
3. Open Jaeger UI at http://localhost:16686
4. Select `mcp-server` from the Service dropdown
5. Click "Find Traces" to view request traces

#### Configuration

Tracing is enabled when `OTEL_COLLECTOR_HOST` is set:

```bash
# In .env or environment
OTEL_COLLECTOR_HOST=otel-collector  # hostname of the collector
OTEL_COLLECTOR_PORT=4317            # gRPC port (default: 4317)
ENVIRONMENT=production              # deployment environment label
```

When `OTEL_COLLECTOR_HOST` is not set, tracing is disabled (no-op).

### Project Structure

```
.
├── cmd/                      # Application entrypoint
│   └── mcp-server.go         # Main server with transport switching
├── internal/
│   ├── config/               # Environment configuration
│   ├── handlers/             # HTTP handlers (health)
│   ├── middleware/           # Auth and metrics middleware
│   └── tools/                # MCP tool implementations
│       └── uuid/             # UUID generation tool
├── example.env               # Example environment file
├── docker-compose.yml        # Docker Compose configuration
├── Dockerfile                # Multi-stage distroless build
├── Makefile                  # Build and run targets
├── DEVELOPER_GUIDE.md        # Guide for adding new tools
└── README.md                 # This file
```

### Development

Requires Go 1.24+ for local development.

```bash
# Build and run locally (stdio mode)
make run

# Run with HTTP transport
MCP_TRANSPORT=http make run

# Run tests
make test

# Run tests with verbose output
make test-verbose

# Run tests with coverage
make coverage

# Generate HTML coverage report
make coverage-html

# Run linter
make lint

# Format code
make fmt
```

### Adding Tools

See [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) for a complete walkthrough on adding new tools.

Quick overview:

1. Create a new package in `internal/tools/<toolname>/`
2. Implement the tool with Input/Output structs
3. Register via `init()` with `tools.Register()`
4. Add blank import in `cmd/mcp-server.go`

Example:
```go
package mytool

import (
    "context"

    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/lkendrickd/mcp-server/internal/tools"
)

type Input struct {
    Name string `json:"name" jsonschema:"description=The name to greet"`
}

type Output struct {
    Message string `json:"message"`
}

func Greet(_ context.Context, _ *mcp.CallToolRequest, input Input) (*mcp.CallToolResult, Output, error) {
    return nil, Output{Message: "Hello, " + input.Name}, nil
}

func init() {
    tools.Register(func(s *mcp.Server) {
        mcp.AddTool(s, &mcp.Tool{
            Name:        "greet",
            Description: "Greet someone by name",
        }, Greet)
    })
}
```

### Make Targets

```bash
make help  # Show all available targets
```

| Target | Description |
|--------|-------------|
| `make` | Show help |
| `make config` | Create .env from example.env |
| `make build` | Build the binary |
| `make run` | Build and run locally |
| `make test` | Run unit tests |
| `make test-verbose` | Run tests with verbose output |
| `make coverage` | Run tests with coverage |
| `make lint` | Run golangci-lint |
| `make fmt` | Format code |
| `make docker-build` | Build Docker image |
| `make docker-run` | Build and run container |
| `make docker-clean` | Stop and remove container |
| `make docker-up` | Start with docker-compose |
| `make docker-down` | Stop docker-compose |
| `make docker-logs` | View docker-compose logs |
| `make docker-restart` | Restart docker-compose |
