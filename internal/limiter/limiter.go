package limiter

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type RateLimiter interface {
	Allow(string) bool
}

type Limiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	now     func() time.Time
	entries map[string][]time.Time
}

func New(limit int, window time.Duration) *Limiter {
	return newWithClock(limit, window, time.Now)
}

func newWithClock(limit int, window time.Duration, now func() time.Time) *Limiter {
	return &Limiter{
		limit:   limit,
		window:  window,
		now:     now,
		entries: make(map[string][]time.Time),
	}
}

func (limiter *Limiter) Allow(key string) bool {
	now := limiter.now()
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	cutoff := now.Add(-limiter.window)
	requests := limiter.entries[key]
	firstRecentRequest := 0
	for firstRecentRequest < len(requests) && !requests[firstRecentRequest].After(cutoff) {
		firstRecentRequest++
	}
	requests = requests[firstRecentRequest:]
	if len(requests) >= limiter.limit {
		limiter.entries[key] = requests
		return false
	}
	limiter.entries[key] = append(requests, now)
	return true
}

func ClientIP(request *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(request.RemoteAddr))
	if err == nil {
		return host
	}
	if request.RemoteAddr != "" {
		return request.RemoteAddr
	}
	return "unknown"
}
