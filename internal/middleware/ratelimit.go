package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimiter provides per-IP rate limiting using a token bucket algorithm.
type RateLimiter struct {
	rate       float64 // tokens per second
	burst      int     // maximum tokens (bucket size)
	clients    map[string]*bucket
	mu         sync.Mutex
	cleanupInt time.Duration
	stopClean  chan struct{}
}

type bucket struct {
	tokens    float64
	lastCheck time.Time
}

// RateLimiterConfig holds configuration for the rate limiter.
type RateLimiterConfig struct {
	RequestsPerSecond float64       // Rate of token replenishment
	BurstSize         int           // Maximum burst size
	CleanupInterval   time.Duration // How often to clean stale entries
}

// NewRateLimiter creates a new rate limiter with the given configuration.
func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	if cfg.RequestsPerSecond <= 0 {
		cfg.RequestsPerSecond = 10 // default: 10 req/s
	}
	if cfg.BurstSize <= 0 {
		cfg.BurstSize = 20 // default: burst of 20
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = 5 * time.Minute
	}

	rl := &RateLimiter{
		rate:       cfg.RequestsPerSecond,
		burst:      cfg.BurstSize,
		clients:    make(map[string]*bucket),
		cleanupInt: cfg.CleanupInterval,
		stopClean:  make(chan struct{}),
	}

	// Start background cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given IP should be allowed.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.clients[ip]
	if !exists {
		rl.clients[ip] = &bucket{
			tokens:    float64(rl.burst) - 1, // consume one token
			lastCheck: now,
		}
		return true
	}

	// Calculate tokens to add based on elapsed time
	elapsed := now.Sub(b.lastCheck).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > float64(rl.burst) {
		b.tokens = float64(rl.burst)
	}
	b.lastCheck = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}

	return false
}

// cleanup removes stale client entries periodically.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInt)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for ip, b := range rl.clients {
				// Remove entries idle for more than 2x cleanup interval
				if now.Sub(b.lastCheck) > 2*rl.cleanupInt {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopClean:
			return
		}
	}
}

// Stop stops the background cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopClean)
}

// Middleware returns an HTTP middleware that enforces rate limiting.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)

		if !rl.Allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "rate limit exceeded",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractIP extracts the client IP from the request.
// It checks X-Forwarded-For and X-Real-IP headers before falling back to RemoteAddr.
func extractIP(r *http.Request) string {
	// Check X-Forwarded-For first (may contain multiple IPs)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (original client)
		if ip, _, found := splitFirst(xff, ','); found {
			return trimSpace(ip)
		}
		return trimSpace(xff)
	}

	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return trimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// splitFirst splits a string on the first occurrence of sep.
func splitFirst(s string, sep byte) (before, after string, found bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}

// trimSpace trims leading and trailing whitespace.
func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
