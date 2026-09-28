package limiter

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestAllowStopsRequestsAfterLimit(t *testing.T) {
	requestLimiter := New(2, time.Minute)

	if !requestLimiter.Allow("198.51.100.10") {
		t.Fatal("first request should be allowed")
	}
	if !requestLimiter.Allow("198.51.100.10") {
		t.Fatal("second request should be allowed")
	}
	if requestLimiter.Allow("198.51.100.10") {
		t.Fatal("third request should be rejected")
	}
}

func TestAllowResetsAfterWindow(t *testing.T) {
	currentTime := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	requestLimiter := newWithClock(2, time.Second, func() time.Time {
		return currentTime
	})

	if !requestLimiter.Allow("198.51.100.10") {
		t.Fatal("first request should be allowed")
	}
	currentTime = currentTime.Add(500 * time.Millisecond)
	if !requestLimiter.Allow("198.51.100.10") {
		t.Fatal("second request should be allowed")
	}
	currentTime = currentTime.Add(499 * time.Millisecond)
	if requestLimiter.Allow("198.51.100.10") {
		t.Fatal("request should be rejected while both recent requests are in the window")
	}
	currentTime = currentTime.Add(500 * time.Millisecond)
	if !requestLimiter.Allow("198.51.100.10") {
		t.Fatal("request should be allowed after the oldest request leaves the window")
	}
}

func TestClientIPUsesConnectionAddress(t *testing.T) {
	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = "192.0.2.10:4321"
	request.Header.Set("X-Forwarded-For", "203.0.113.20")

	if got := ClientIP(request); got != "192.0.2.10" {
		t.Fatalf("ClientIP() = %q, want connection address", got)
	}
}
