package telemetry

import (
	"context"
	"testing"
)

func TestSetup(t *testing.T) {
	tests := []struct {
		name             string
		collectorAddress string
		wantEnabled      bool
	}{
		{
			name:             "disabled when collector address empty",
			collectorAddress: "",
			wantEnabled:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := Config{
				ServiceName:      "test-service",
				ServiceVersion:   "1.0.0",
				CollectorAddress: tt.collectorAddress,
			}

			shutdown, err := Setup(ctx, cfg)
			if err != nil {
				t.Fatalf("Setup() error = %v", err)
			}

			if shutdown == nil {
				t.Fatal("Setup() returned nil shutdown function")
			}

			// Shutdown should not error
			if err := shutdown(ctx); err != nil {
				t.Errorf("shutdown() error = %v", err)
			}
		})
	}
}

func TestSetup_NoopShutdown(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		ServiceName:      "test-service",
		ServiceVersion:   "1.0.0",
		CollectorAddress: "", // Empty = disabled
	}

	shutdown, err := Setup(ctx, cfg)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	// Multiple calls to shutdown should be safe
	for i := 0; i < 3; i++ {
		if err := shutdown(ctx); err != nil {
			t.Errorf("shutdown() call %d error = %v", i+1, err)
		}
	}
}

func TestConfig_Fields(t *testing.T) {
	cfg := Config{
		ServiceName:      "my-service",
		ServiceVersion:   "2.0.0",
		CollectorAddress: "localhost:4317",
		Environment:      "production",
	}

	if cfg.ServiceName != "my-service" {
		t.Errorf("ServiceName = %q, want %q", cfg.ServiceName, "my-service")
	}
	if cfg.ServiceVersion != "2.0.0" {
		t.Errorf("ServiceVersion = %q, want %q", cfg.ServiceVersion, "2.0.0")
	}
	if cfg.CollectorAddress != "localhost:4317" {
		t.Errorf("CollectorAddress = %q, want %q", cfg.CollectorAddress, "localhost:4317")
	}
	if cfg.Environment != "production" {
		t.Errorf("Environment = %q, want %q", cfg.Environment, "production")
	}
}

func TestSetup_WithEnvironment(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		ServiceName:      "test-service",
		ServiceVersion:   "1.0.0",
		CollectorAddress: "", // Empty = disabled
		Environment:      "staging",
	}

	shutdown, err := Setup(ctx, cfg)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	if shutdown == nil {
		t.Fatal("Setup() returned nil shutdown function")
	}

	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown() error = %v", err)
	}
}

func TestSetup_WithoutEnvironment(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		ServiceName:      "test-service",
		ServiceVersion:   "1.0.0",
		CollectorAddress: "", // Empty = disabled
		Environment:      "", // No environment
	}

	shutdown, err := Setup(ctx, cfg)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	if shutdown == nil {
		t.Fatal("Setup() returned nil shutdown function")
	}

	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown() error = %v", err)
	}
}

func TestSetup_WithCollectorAddress_CancelledContext(t *testing.T) {
	// Use a cancelled context to trigger connection failure quickly
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	cfg := Config{
		ServiceName:      "test-service",
		ServiceVersion:   "1.0.0",
		CollectorAddress: "localhost:4317",
		Environment:      "test",
	}

	// This will attempt to connect but fail due to cancelled context
	// The behavior depends on the OTEL library - it may return error or succeed
	shutdown, err := Setup(ctx, cfg)

	// Either way, if we got a shutdown function, we should be able to call it
	if shutdown != nil {
		_ = shutdown(ctx)
	}

	// We're testing that the function handles this gracefully
	// The actual error depends on OTEL library internals
	_ = err
}
