package middleware

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// maxTrackedWindows bounds the number of client windows kept in memory. When
// exceeded, expired windows are swept.
const maxTrackedWindows = 10000

// window is a fixed-window request counter for one client key.
type window struct {
	start time.Time
	count int
}

// RateLimiter enforces a fixed-window per-key request limit. Keys are caller
// chosen, typically client IP plus an endpoint bucket.
type RateLimiter struct {
	mu       sync.Mutex
	windows  map[string]window
	limit    int
	interval time.Duration
	now      func() time.Time
}

// NewRateLimiter creates a limiter allowing limit requests per interval per
// key.
func NewRateLimiter(limit int, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		windows:  make(map[string]window),
		limit:    limit,
		interval: interval,
		now:      time.Now,
	}
}

// Allow reports whether key may make another request in the current window. It
// is exported for tests and programmatic checks.
func (l *RateLimiter) Allow(key string) bool {
	allowed, _ := l.allow(key)

	return allowed
}

// Middleware limits requests by client IP. Rejected requests receive 429 with a
// Retry-After header.
func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed, retryAfter := l.allow(clientIP(r))
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
			writeJSONError(w, http.StatusTooManyRequests, "too many requests")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (l *RateLimiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	current, ok := l.windows[key]
	if !ok || now.Sub(current.start) >= l.interval {
		l.windows[key] = window{start: now, count: 1}
		if len(l.windows) > maxTrackedWindows {
			l.sweepLocked(now)
		}

		return true, 0
	}
	if current.count >= l.limit {
		return false, l.interval - now.Sub(current.start)
	}

	current.count++
	l.windows[key] = current

	return true, 0
}

// sweepLocked drops expired windows. The caller must hold l.mu.
func (l *RateLimiter) sweepLocked(now time.Time) {
	for key, w := range l.windows {
		if now.Sub(w.start) >= l.interval {
			delete(l.windows, key)
		}
	}
}

// clientIP returns the request remote IP without its port.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
