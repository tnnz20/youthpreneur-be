package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRateLimiterAllowsUpToLimitThenResets(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute)
	now := time.Now()
	limiter.now = func() time.Time { return now }

	if allowed, _ := limiter.allow("client"); !allowed {
		t.Fatal("first request should be allowed")
	}
	if allowed, _ := limiter.allow("client"); !allowed {
		t.Fatal("second request should be allowed")
	}
	if allowed, _ := limiter.allow("client"); allowed {
		t.Fatal("third request should be rejected within the window")
	}

	now = now.Add(time.Minute)

	if allowed, _ := limiter.allow("client"); !allowed {
		t.Fatal("request after window reset should be allowed")
	}
}

func TestRateLimiterIsolatesClients(t *testing.T) {
	limiter := NewRateLimiter(1, time.Minute)

	if allowed, _ := limiter.allow("a"); !allowed {
		t.Fatal("first client should be allowed")
	}
	if allowed, _ := limiter.allow("a"); allowed {
		t.Fatal("first client should be limited on the second request")
	}
	if allowed, _ := limiter.allow("b"); !allowed {
		t.Fatal("second client should have an independent window")
	}
}

func TestRateLimitMiddlewareReturns429WithRetryAfter(t *testing.T) {
	limiter := NewRateLimiter(1, time.Minute)
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/users", nil))
	if first.Code != http.StatusNoContent {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusNoContent)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/users", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d", second.Code, http.StatusTooManyRequests)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Error("429 response missing Retry-After header")
	}
}

func TestRateLimiterConcurrentRequestsRespectLimit(t *testing.T) {
	const (
		limit    = 50
		requests = 200
	)

	limiter := NewRateLimiter(limit, time.Minute)
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	var allowed, rejected atomic.Int64
	var wg sync.WaitGroup
	for range requests {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := httptest.NewRequest(http.MethodGet, "/users", nil)
			req.RemoteAddr = "10.0.0.1:1234"
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			switch rec.Code {
			case http.StatusNoContent:
				allowed.Add(1)
			case http.StatusTooManyRequests:
				rejected.Add(1)
			default:
				t.Errorf("unexpected status %d", rec.Code)
			}
		}()
	}
	wg.Wait()

	if allowed.Load() != limit {
		t.Errorf("allowed = %d, want %d", allowed.Load(), limit)
	}
	if rejected.Load() != requests-limit {
		t.Errorf("rejected = %d, want %d", rejected.Load(), requests-limit)
	}
}

func TestRateLimitMiddlewareKeysByClientIP(t *testing.T) {
	limiter := NewRateLimiter(1, time.Minute)
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	first := httptest.NewRequest(http.MethodGet, "/users", nil)
	first.RemoteAddr = "10.0.0.1:1234"
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, first)
	if firstRec.Code != http.StatusNoContent {
		t.Fatalf("first status = %d, want %d", firstRec.Code, http.StatusNoContent)
	}

	second := httptest.NewRequest(http.MethodGet, "/users", nil)
	second.RemoteAddr = "10.0.0.1:5678"
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, second)
	if secondRec.Code != http.StatusTooManyRequests {
		t.Fatalf("same client different port status = %d, want %d", secondRec.Code, http.StatusTooManyRequests)
	}

	other := httptest.NewRequest(http.MethodGet, "/users", nil)
	other.RemoteAddr = "10.0.0.2:1234"
	otherRec := httptest.NewRecorder()
	handler.ServeHTTP(otherRec, other)
	if otherRec.Code != http.StatusNoContent {
		t.Fatalf("other client status = %d, want %d", otherRec.Code, http.StatusNoContent)
	}
}
