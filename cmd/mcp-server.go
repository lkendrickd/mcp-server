package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/lkendrickd/mcp-server/internal/config"
	"github.com/lkendrickd/mcp-server/internal/handlers"
	"github.com/lkendrickd/mcp-server/internal/middleware"
	"github.com/lkendrickd/mcp-server/internal/tools"
	_ "github.com/lkendrickd/mcp-server/internal/tools/uuid"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	// Load configuration from environment
	cfg := config.New()

	// Register prometheus metrics
	prometheus.MustRegister(middleware.RequestDuration, middleware.EndpointCount)

	// Create MCP server with capabilities
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "mcp-server",
		Version: "0.0.1",
	}, nil)
	tools.RegisterAll(server)

	// Determine transport mode from environment
	transport := getEnv("MCP_TRANSPORT", "stdio")

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

		// Build handler chain: metrics -> auth (if enabled) -> mux
		var handler http.Handler = mux
		if cfg.AuthEnabled {
			// Protect /mcp endpoints with API key authentication
			protectedPrefixes := []string{"/mcp"}
			handler = middleware.AuthMiddleware(cfg, protectedPrefixes)(handler)
			logger.Info("API key authentication enabled", "key_count", cfg.APIKeyCount())
		}
		handler = middleware.MetricsMiddleware(handler)

		logger.Info("mcp server starting with HTTP transport", "port", cfg.Port)
		if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
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
			if err := http.ListenAndServe(":"+cfg.Port, middleware.MetricsMiddleware(mux)); err != nil {
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

func getEnv(key, defaultValue string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultValue
}
