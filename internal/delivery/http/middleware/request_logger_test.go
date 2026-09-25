package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestLoggerLogsCompletedRequests(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/test-path", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log JSON: %v, raw: %s", err, buf.String())
	}

	if logEntry["msg"] != "Request completed" {
		t.Errorf("expected msg 'Request completed', got %v", logEntry["msg"])
	}
	if logEntry["method"] != "POST" {
		t.Errorf("expected method 'POST', got %v", logEntry["method"])
	}
	if logEntry["path"] != "/test-path" {
		t.Errorf("expected path '/test-path', got %v", logEntry["path"])
	}
	if int(logEntry["status"].(float64)) != http.StatusCreated {
		t.Errorf("expected status %d, got %v", http.StatusCreated, logEntry["status"])
	}
	if logEntry["level"] != "INFO" {
		t.Errorf("expected level 'INFO', got %v", logEntry["level"])
	}
}

func TestRequestLoggerLogsWarnFor4xxAndErrorFor5xx(t *testing.T) {
	tests := []struct {
		statusCode int
		wantLevel  string
	}{
		{http.StatusOK, "INFO"},
		{http.StatusBadRequest, "WARN"},
		{http.StatusNotFound, "WARN"},
		{http.StatusInternalServerError, "ERROR"},
	}

	for _, tt := range tests {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

		handler := RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tt.statusCode)
		}))

		req := httptest.NewRequest(http.MethodGet, "/status-check", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		var logEntry map[string]any
		if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
			t.Fatalf("status %d: failed to parse log JSON: %v", tt.statusCode, err)
		}

		if logEntry["level"] != tt.wantLevel {
			t.Errorf("status %d: expected level %s, got %s", tt.statusCode, tt.wantLevel, logEntry["level"])
		}
	}
}
