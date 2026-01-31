package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 10,
		BurstSize:         5,
		CleanupInterval:   time.Minute,
	})
	defer rl.Stop()

	ip := "192.168.1.1"

	// First 5 requests should be allowed (burst)
	for i := 0; i < 5; i++ {
		if !rl.Allow(ip) {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 6th request should be denied (bucket empty)
	if rl.Allow(ip) {
		t.Error("6th request should be denied")
	}
}

func TestRateLimiter_TokenReplenishment(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 100, // 100 tokens/sec = 1 token per 10ms
		BurstSize:         1,
		CleanupInterval:   time.Minute,
	})
	defer rl.Stop()

	ip := "192.168.1.1"

	// First request allowed
	if !rl.Allow(ip) {
		t.Error("first request should be allowed")
	}

	// Immediate second request denied
	if rl.Allow(ip) {
		t.Error("immediate second request should be denied")
	}

	// Wait for token replenishment
	time.Sleep(15 * time.Millisecond)

	// Should be allowed now
	if !rl.Allow(ip) {
		t.Error("request after wait should be allowed")
	}
}

func TestRateLimiter_DifferentIPs(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 10,
		BurstSize:         2,
		CleanupInterval:   time.Minute,
	})
	defer rl.Stop()

	ip1 := "192.168.1.1"
	ip2 := "192.168.1.2"

	// Exhaust ip1's bucket
	rl.Allow(ip1)
	rl.Allow(ip1)
	if rl.Allow(ip1) {
		t.Error("ip1 should be rate limited")
	}

	// ip2 should still be allowed
	if !rl.Allow(ip2) {
		t.Error("ip2 should be allowed")
	}
}

func TestRateLimiter_Middleware(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 10,
		BurstSize:         2,
		CleanupInterval:   time.Minute,
	})
	defer rl.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := rl.Middleware(handler)

	// First 2 requests should pass
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: got status %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("3rd request: got status %d, want %d", rec.Code, http.StatusTooManyRequests)
	}

	if rec.Header().Get("Retry-After") != "1" {
		t.Error("expected Retry-After header")
	}
}

func TestRateLimiter_XForwardedFor(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 10,
		BurstSize:         1,
		CleanupInterval:   time.Minute,
	})
	defer rl.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := rl.Middleware(handler)

	// First request with X-Forwarded-For
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 192.168.1.1")
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("first request: got status %d, want %d", rec.Code, http.StatusOK)
	}

	// Second request from same X-Forwarded-For IP should be limited
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 192.168.1.1")
	req.RemoteAddr = "127.0.0.1:12345"
	rec = httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("second request: got status %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimiter_XRealIP(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 10,
		BurstSize:         1,
		CleanupInterval:   time.Minute,
	})
	defer rl.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := rl.Middleware(handler)

	// First request with X-Real-IP
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "10.0.0.1")
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("first request: got status %d, want %d", rec.Code, http.StatusOK)
	}

	// Second request from same X-Real-IP should be limited
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "10.0.0.1")
	req.RemoteAddr = "127.0.0.1:12345"
	rec = httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("second request: got status %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 1000,
		BurstSize:         100,
		CleanupInterval:   time.Minute,
	})
	defer rl.Stop()

	var wg sync.WaitGroup
	allowed := make(chan bool, 200)

	// Launch 200 concurrent requests from the same IP
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed <- rl.Allow("192.168.1.1")
		}()
	}

	wg.Wait()
	close(allowed)

	// Count allowed requests
	count := 0
	for a := range allowed {
		if a {
			count++
		}
	}

	// Should allow approximately burst size (100), with some tolerance
	if count < 90 || count > 110 {
		t.Errorf("expected ~100 allowed requests, got %d", count)
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		xri        string
		want       string
	}{
		{
			name:       "remote addr only",
			remoteAddr: "192.168.1.1:12345",
			want:       "192.168.1.1",
		},
		{
			name:       "x-forwarded-for single",
			remoteAddr: "127.0.0.1:12345",
			xff:        "10.0.0.1",
			want:       "10.0.0.1",
		},
		{
			name:       "x-forwarded-for multiple",
			remoteAddr: "127.0.0.1:12345",
			xff:        "10.0.0.1, 192.168.1.1, 172.16.0.1",
			want:       "10.0.0.1",
		},
		{
			name:       "x-real-ip",
			remoteAddr: "127.0.0.1:12345",
			xri:        "10.0.0.1",
			want:       "10.0.0.1",
		},
		{
			name:       "x-forwarded-for takes precedence",
			remoteAddr: "127.0.0.1:12345",
			xff:        "10.0.0.1",
			xri:        "10.0.0.2",
			want:       "10.0.0.1",
		},
		{
			name:       "x-forwarded-for with spaces",
			remoteAddr: "127.0.0.1:12345",
			xff:        "  10.0.0.1  ",
			want:       "10.0.0.1",
		},
		{
			name:       "remote addr without port",
			remoteAddr: "192.168.1.1",
			want:       "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}

			got := extractIP(req)
			if got != tt.want {
				t.Errorf("extractIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewRateLimiter_Defaults(t *testing.T) {
	// Test with zero/negative values to trigger defaults
	rl := NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 0,
		BurstSize:         -1,
		CleanupInterval:   0,
	})
	defer rl.Stop()

	// Should use defaults
	if rl.rate != 10 {
		t.Errorf("rate = %v, want 10 (default)", rl.rate)
	}
	if rl.burst != 20 {
		t.Errorf("burst = %v, want 20 (default)", rl.burst)
	}
	if rl.cleanupInt != 5*time.Minute {
		t.Errorf("cleanupInt = %v, want 5m (default)", rl.cleanupInt)
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	// Use very short cleanup interval for testing
	rl := NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 10,
		BurstSize:         5,
		CleanupInterval:   10 * time.Millisecond,
	})
	defer rl.Stop()

	// Add an entry
	rl.Allow("192.168.1.1")

	// Verify entry exists
	rl.mu.Lock()
	if len(rl.clients) != 1 {
		t.Errorf("clients count = %d, want 1", len(rl.clients))
	}
	rl.mu.Unlock()

	// Wait for cleanup to run (2x cleanup interval for stale entries + buffer)
	time.Sleep(50 * time.Millisecond)

	// Entry should be cleaned up
	rl.mu.Lock()
	count := len(rl.clients)
	rl.mu.Unlock()

	if count != 0 {
		t.Errorf("clients count after cleanup = %d, want 0", count)
	}
}

func TestRateLimiter_CleanupKeepsActiveEntries(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 10,
		BurstSize:         5,
		CleanupInterval:   20 * time.Millisecond,
	})
	defer rl.Stop()

	// Keep refreshing the entry
	for i := 0; i < 5; i++ {
		rl.Allow("192.168.1.1")
		time.Sleep(10 * time.Millisecond)
	}

	// Entry should still exist because it's being actively used
	rl.mu.Lock()
	count := len(rl.clients)
	rl.mu.Unlock()

	if count != 1 {
		t.Errorf("clients count = %d, want 1 (active entry should not be cleaned)", count)
	}
}
