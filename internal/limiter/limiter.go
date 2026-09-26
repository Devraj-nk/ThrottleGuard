package limiter

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Limiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]entry
}

type entry struct {
	count       int
	windowStart time.Time
}

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{
		limit:   limit,
		window:  window,
		entries: make(map[string]entry),
	}
}

func (limiter *Limiter) Allow(key string) bool {
	now := time.Now()
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	current, exists := limiter.entries[key]
	if !exists || now.Sub(current.windowStart) >= limiter.window {
		limiter.entries[key] = entry{count: 1, windowStart: now}
		return true
	}
	if current.count >= limiter.limit {
		return false
	}
	current.count++
	limiter.entries[key] = current
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
