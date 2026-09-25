package middleware

import (
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
