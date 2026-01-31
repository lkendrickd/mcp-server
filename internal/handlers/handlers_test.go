package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		wantStatus  int
		wantBody    string
		contentType string
	}{
		{
			name:        "GET request",
			method:      http.MethodGet,
			wantStatus:  http.StatusOK,
			wantBody:    `{"healthy":true}` + "\n",
			contentType: "application/json",
		},
		{
			name:        "HEAD request",
			method:      http.MethodHead,
			wantStatus:  http.StatusOK,
			wantBody:    `{"healthy":true}` + "\n",
			contentType: "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/health", nil)
			rec := httptest.NewRecorder()

			HealthHandler(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if rec.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}

			if ct := rec.Header().Get("Content-Type"); ct != tt.contentType {
				t.Errorf("Content-Type = %q, want %q", ct, tt.contentType)
			}
		})
	}
}
