package store

import (
	"context"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type RedisLimiter struct {
	client *redis.Client
	limit  int // Max tokens allowed
}

func NewRedisLimiter(client *redis.Client, limit int) *RedisLimiter {
	return &RedisLimiter{
		client: client,
		limit:  limit,
	}
}

func (r *RedisLimiter) CheckLimit(ctx context.Context, userID string) (bool, error) {
	val, err := r.client.Get(ctx, "usage:"+userID).Result()
	if err == redis.Nil {
		return true, nil // No usage yet
	}
	usage, _ := strconv.Atoi(val)
	return usage < r.limit, nil
}

func (r *RedisLimiter) Increment(ctx context.Context, userID string, tokens int) error {
	return r.client.IncrBy(ctx, "usage:"+userID, int64(tokens)).Err()
}
