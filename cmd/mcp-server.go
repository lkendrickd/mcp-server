package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/lkendrickd/mcp-server/internal/config"
	"github.com/lkendrickd/mcp-server/internal/handlers"
	"github.com/lkendrickd/mcp-server/internal/middleware"
	"github.com/lkendrickd/mcp-server/internal/telemetry"
	"github.com/lkendrickd/mcp-server/internal/tools"
	_ "github.com/lkendrickd/mcp-server/internal/tools/uuid"
)

// version is set via ldflags at build time
var version = "dev"

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	// Load configuration from environment
	cfg := config.New()

	// Initialize OpenTelemetry (no-op if OTEL_COLLECTOR_ADDRESS not set)
	shutdownTelemetry, err := telemetry.Setup(ctx, telemetry.Config{
		ServiceName:      "mcp-server",
		ServiceVersion:   version,
		CollectorAddress: cfg.OTELCollectorAddress,
		Environment:      cfg.Environment,
	})
	if err != nil {
		logger.Error("failed to setup telemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := shutdownTelemetry(ctx); err != nil {
			logger.Error("failed to shutdown telemetry", "error", err)
		}
	}()

	if cfg.OTELCollectorAddress != "" {
		logger.Info("telemetry enabled", "collector", cfg.OTELCollectorAddress)
	}

	// Register prometheus metrics
	prometheus.MustRegister(middleware.RequestDuration, middleware.EndpointCount)

	// Create MCP server with capabilities
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "mcp-server",
		Version: version,
	}, nil)
	tools.RegisterAll(server)

	// Determine transport mode from config
	transport := cfg.MCPTransport

	switch transport {
	case "sse", "http":
		// HTTP transport - Streamable HTTP handler for MCP
		mux := http.NewServeMux()
		mux.HandleFunc("GET /health", handlers.HealthHandler)
		mux.Handle("GET /metrics", promhttp.Handler())

		// Streamable HTTP handler for MCP
		httpHandler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
			return server
		}, nil)
		mux.Handle("/mcp", httpHandler)
		mux.Handle("/mcp/", httpHandler)

		// Build handler chain: otelhttp -> mcp tracing -> metrics -> auth (if enabled) -> mux
		var handler http.Handler = mux
		if cfg.AuthEnabled {
			// Protect /mcp endpoints with API key authentication
			protectedPrefixes := []string{"/mcp"}
			handler = middleware.AuthMiddleware(cfg, protectedPrefixes)(handler)
			logger.Info("API key authentication enabled", "key_count", cfg.APIKeyCount())
		}
		handler = middleware.MetricsMiddleware(handler)
		handler = middleware.MCPTracingMiddleware(handler)
		handler = otelhttp.NewHandler(handler, "mcp-server")

		logger.Info("mcp server starting with HTTP transport", "port", cfg.Port)
		srv := &http.Server{
			Addr:         ":" + cfg.Port,
			Handler:      handler,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		}
		if err := srv.ListenAndServe(); err != nil {
			logger.Error("http server error", "error", err)
			os.Exit(1)
		}

	default:
		// Stdio transport (default) - for CLI usage
		// Start HTTP server for health/metrics in background
		go func() {
			mux := http.NewServeMux()
			mux.HandleFunc("GET /health", handlers.HealthHandler)
			mux.Handle("GET /metrics", promhttp.Handler())
			logger.Info("http server starting", "port", cfg.Port)
			srv := &http.Server{
				Addr:         ":" + cfg.Port,
				Handler:      middleware.MetricsMiddleware(mux),
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
				IdleTimeout:  120 * time.Second,
			}
			if err := srv.ListenAndServe(); err != nil {
				logger.Error("http server error", "error", err)
			}
		}()

		logger.Info("mcp server running with stdio transport")
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			logger.Error("mcp server error", "error", err)
			os.Exit(1)
		}
	}
}

