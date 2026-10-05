package limiter

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisLimitIsSharedAcrossGatewayInstances(t *testing.T) {
	server := miniredis.RunT(t)
	currentTime := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	firstInstance := newTestRedisLimiter(server.Addr(), 3, currentTime)
	secondInstance := newTestRedisLimiter(server.Addr(), 3, currentTime)

	for request := 0; request < 2; request++ {
		if !firstInstance.Allow("distributed-client") {
			t.Fatalf("gateway one rejected request %d before the shared limit", request+1)
		}
	}
	if !secondInstance.Allow("distributed-client") {
		t.Fatal("gateway two rejected the final request within the shared limit")
	}
	if firstInstance.Allow("distributed-client") {
		t.Fatal("gateway one allowed a request after both instances reached the shared limit")
	}
}

func newTestRedisLimiter(address string, limit int, currentTime time.Time) *RedisLimiter {
	return &RedisLimiter{
		client: redis.NewClient(&redis.Options{Addr: address}),
		limit:  limit,
		window: time.Second,
		now:    func() time.Time { return currentTime },
	}
}
