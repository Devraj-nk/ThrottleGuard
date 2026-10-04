package limiter

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisLimiterAllowsWithinLimitAndRejectsWhenFull(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	currentTime := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	limiter := &RedisLimiter{client: client, limit: 2, window: time.Second, now: func() time.Time { return currentTime }}

	if !limiter.Allow("client-1") {
		t.Fatal("first request should be allowed")
	}
	if !limiter.Allow("client-1") {
		t.Fatal("second request should be allowed")
	}
	if limiter.Allow("client-1") {
		t.Fatal("third request should be rejected")
	}
}

func TestRedisLimiterExpiresOldEntries(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	currentTime := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	limiter := &RedisLimiter{client: client, limit: 1, window: time.Second, now: func() time.Time { return currentTime }}

	if !limiter.Allow("client-2") {
		t.Fatal("first request should be allowed")
	}
	currentTime = currentTime.Add(2 * time.Second)
	if !limiter.Allow("client-2") {
		t.Fatal("request after the window should be allowed")
	}
}
