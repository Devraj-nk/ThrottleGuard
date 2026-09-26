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
	requestLimiter := New(1, time.Millisecond)

	if !requestLimiter.Allow("198.51.100.10") {
		t.Fatal("first request should be allowed")
	}
	time.Sleep(2 * time.Millisecond)
	if !requestLimiter.Allow("198.51.100.10") {
		t.Fatal("request after the window should be allowed")
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
