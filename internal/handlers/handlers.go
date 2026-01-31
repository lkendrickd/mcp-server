package handlers

import (
	"net/http"
)

// HealthHandler is the health check handler.
func HealthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"healthy":true}` + "\n"))
}
