package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisScript = `
local bucket = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local seqKey = bucket .. ":seq"

redis.call('ZREMRANGEBYSCORE', bucket, '-inf', now - window)
local currentCount = redis.call('ZCARD', bucket)
if currentCount >= limit then
    return 0
end

local seq = redis.call('INCR', seqKey)
redis.call('ZADD', bucket, now, tostring(seqKey) .. ':' .. tostring(seq))
return 1
`

type RedisLimiter struct {
	client *redis.Client
	limit  int
	window time.Duration
	now    func() time.Time
}

func NewRedis(address string, limit int, window time.Duration) (*RedisLimiter, error) {
	client := redis.NewClient(&redis.Options{
		Addr: address,
		DB:   0,
	})
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("connect redis: %w", err)
	}
	return &RedisLimiter{client: client, limit: limit, window: window, now: time.Now}, nil
}

func (r *RedisLimiter) Allow(key string) bool {
	if r.now == nil {
		r.now = time.Now
	}
	bucket := fmt.Sprintf("limit:%s", key)
	result, err := r.client.Eval(
		context.Background(),
		redisScript,
		[]string{bucket},
		r.now().UnixMilli(),
		int64(r.window/time.Millisecond),
		int64(r.limit),
	).Result()
	if err != nil {
		return true
	}
	value, ok := result.(int64)
	if !ok {
		return true
	}
	return value == 1
}
