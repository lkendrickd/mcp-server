package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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

const (
	shutdownTimeout = 30 * time.Second
)

// version is set via ldflags at build time
var version = "dev"

func main() {
	// Handle --version flag
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	// Create context that listens for SIGINT and SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

		// Build handler chain: otelhttp -> rate limit -> mcp tracing -> metrics -> auth (if enabled) -> mux
		var handler http.Handler = mux
		if cfg.AuthEnabled {
			// Protect /mcp endpoints with API key authentication
			protectedPrefixes := []string{"/mcp"}
			handler = middleware.AuthMiddleware(cfg, protectedPrefixes)(handler)
			logger.Info("API key authentication enabled", "key_count", cfg.APIKeyCount())
		}
		handler = middleware.MetricsMiddleware(handler)
		handler = middleware.MCPTracingMiddleware()(handler)

		// Add rate limiting if enabled (applied early to reject before expensive ops)
		var rateLimiter *middleware.RateLimiter
		if cfg.RateLimitEnabled {
			rateLimiter = middleware.NewRateLimiter(middleware.RateLimiterConfig{
				RequestsPerSecond: cfg.RateLimitRPS,
				BurstSize:         cfg.RateLimitBurst,
			})
			handler = rateLimiter.Middleware(handler)
			logger.Info("rate limiting enabled", "rps", cfg.RateLimitRPS, "burst", cfg.RateLimitBurst)
		}

		handler = otelhttp.NewHandler(handler, "mcp-server")

		srv := &http.Server{
			Addr:         ":" + cfg.Port,
			Handler:      handler,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		}

		// Start server in goroutine
		go func() {
			logger.Info("mcp server starting with HTTP transport", "port", cfg.Port)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Error("http server error", "error", err)
				os.Exit(1)
			}
		}()

		// Wait for shutdown signal
		<-ctx.Done()
		stop() // Stop receiving further signals
		logger.Info("shutting down gracefully, press Ctrl+C again to force")

		// Create shutdown context with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		// Shutdown HTTP server
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown error", "error", err)
		}

		// Stop rate limiter cleanup goroutine
		if rateLimiter != nil {
			rateLimiter.Stop()
		}

		logger.Info("server shutdown complete")

	default:
		// Stdio transport (default) - for CLI usage
		// Start HTTP server for health/metrics in background
		mux := http.NewServeMux()
		mux.HandleFunc("GET /health", handlers.HealthHandler)
		mux.Handle("GET /metrics", promhttp.Handler())

		srv := &http.Server{
			Addr:         ":" + cfg.Port,
			Handler:      middleware.MetricsMiddleware(mux),
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		}

		go func() {
			logger.Info("http server starting", "port", cfg.Port)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Error("http server error", "error", err)
			}
		}()

		// Run MCP server with stdio transport (blocks until done or context cancelled)
		logger.Info("mcp server running with stdio transport")
		mcpCtx, mcpCancel := context.WithCancel(ctx)
		defer mcpCancel()

		go func() {
			if err := server.Run(mcpCtx, &mcp.StdioTransport{}); err != nil {
				logger.Error("mcp server error", "error", err)
			}
		}()

		// Wait for shutdown signal
		<-ctx.Done()
		stop()
		logger.Info("shutting down gracefully")

		// Create shutdown context with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		// Cancel MCP context
		mcpCancel()

		// Shutdown HTTP server
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown error", "error", err)
		}

		logger.Info("server shutdown complete")
	}
}
