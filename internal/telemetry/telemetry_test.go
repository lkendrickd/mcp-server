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
