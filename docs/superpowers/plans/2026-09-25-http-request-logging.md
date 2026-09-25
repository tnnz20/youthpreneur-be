# HTTP Request Logging & Observability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add HTTP request logging middleware, database connection log, and log rotation to provide immediate, real-time observability in container environments (`podman logs`) matching the pattern used in `bagor-be`.

**Architecture:** Implement an HTTP middleware in `internal/delivery/http/middleware/request_logger.go` that wraps `http.ResponseWriter` to intercept status codes, measures execution latency, and logs completion using Go standard library `log/slog`. Wire this middleware in `internal/config/bootstrap.go` at the outermost layer so every incoming HTTP request (including health checks and errors) is logged. Add database connection success logging on startup and configure `k8s-file` with size-based log rotation in `compose.yaml`.

**Tech Stack:** Go standard library (`net/http`, `log/slog`, `time`), Docker/Podman Compose.

**Spec:** Requirements discussed and agreed in conversation.

## Global Constraints

- Use Go standard library `log/slog` exclusively (no new external dependencies).
- Log format must match existing JSON logger configuration (`slog.NewJSONHandler`).
- Middleware logs level: `INFO` for status `< 400`, `WARN` for `400-499`, and `ERROR` for `>= 500`.
- Preserve existing middleware signature conventions in `internal/delivery/http/middleware`.
- Must pass `go test ./...`, `go vet ./...`, and `git diff --check`.

## Review Focus

1. **Default Status Code:** A handler that writes response body without calling `WriteHeader` must record status `200 OK`.
2. **Panic Safety / Error Handling:** If downstream handler errors or returns 4xx/5xx, status must be captured accurately.
3. **Log Sanitization:** Sensitive paths (like query params containing tokens/passwords if any) should use `r.URL.Path` or standard `RequestURI`.
4. **Log Rotation:** Compose logging options must be syntactically valid for both Podman Compose and Docker Compose.

---

### Task 1: HTTP Request Logger Middleware

**Files:**
- Create: `internal/delivery/http/middleware/request_logger.go`
- Test: `internal/delivery/http/middleware/request_logger_test.go`

**Interfaces:**
- Produces: `RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler`

- [ ] **Step 1: Write failing unit test for `RequestLogger`**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/delivery/http/middleware -run TestRequestLogger`
Expected: Compile failure / function not defined.

- [ ] **Step 3: Implement `RequestLogger` middleware**

Create `internal/delivery/http/middleware/request_logger.go`:
```go
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// RequestLogger returns an HTTP middleware that records incoming HTTP requests,
// latency, status codes, and paths with dynamic slog log levels.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			lrw := &loggingResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(lrw, r)

			duration := time.Since(start)
			ctx := r.Context()

			level := slog.LevelInfo
			if lrw.statusCode >= http.StatusInternalServerError {
				level = slog.LevelError
			} else if lrw.statusCode >= http.StatusBadRequest {
				level = slog.LevelWarn
			}

			logger.Log(ctx, level, "Request completed",
				"method", r.Method,
				"path", r.URL.RequestURI(),
				"status", lrw.statusCode,
				"duration_ms", duration.Milliseconds(),
				"duration_ns", duration.Nanoseconds(),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/delivery/http/middleware -run TestRequestLogger`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/delivery/http/middleware/request_logger.go internal/delivery/http/middleware/request_logger_test.go
git commit -m "feat(http): add request logging middleware with dynamic slog levels"
```

---

### Task 2: Wire RequestLogger & Database Connection Log in Bootstrap

**Files:**
- Modify: `internal/config/bootstrap.go`
- Test: `internal/config/bootstrap_test.go` (or run full test suite)

- [ ] **Step 1: Add DB connection log and wrap HTTP handler in `Bootstrap`**

In `internal/config/bootstrap.go`:
1. After `OpenPostgres(ctx, cfg.Postgres)` returns successfully, add:
```go
	logger.Info("database connected successfully",
		"host", cfg.Postgres.Host,
		"port", cfg.Postgres.Port,
		"database", cfg.Postgres.Database,
	)
```
2. Wrap the final router with `middleware.RequestLogger(logger)`:
```go
	generalLimit := middleware.NewRateLimiter(cfg.RateLimit.GeneralPerMinute, rateLimitWindow).Middleware

	// Rate limiting wraps CORS so disallowed-origin requests are also counted
	// and limited. CORS policy itself is unchanged.
	corsAndLimited := generalLimit(middleware.CORS(cfg.CORS.AllowedOrigins)(mux))
	return middleware.RequestLogger(logger)(corsAndLimited), db, nil
```

- [ ] **Step 2: Run all tests to ensure bootstrap wiring passes**

Run: `go test ./...` and `go vet ./...`
Expected: PASS across all packages.

- [ ] **Step 3: Commit**

```bash
git add internal/config/bootstrap.go
git commit -m "feat(config): wire request logger middleware and log database connection on startup"
```

---

### Task 3: Add Log Rotation to `compose.yaml`

**Files:**
- Modify: `compose.yaml`

- [ ] **Step 1: Configure `logging` options in `compose.yaml`**

Under `backend`:
```yaml
    logging:
      driver: "k8s-file"
      options:
        max-size: "10m"
        max-file: "3"
```

- [ ] **Step 2: Commit**

```bash
git add compose.yaml
git commit -m "build(compose): configure k8s-file log rotation for backend service"
```

---

### Task 4: Full Repository Verification

- [ ] **Step 1: Run comprehensive tests and lint checks**

Run:
```bash
go test ./...
go vet ./...
git diff --check
```
Expected: All clean, zero errors.
