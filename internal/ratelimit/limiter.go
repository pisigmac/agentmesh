package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	rdb    *redis.Client
	prefix string
}

func New(rdb *redis.Client, prefix string) *Limiter {
	return &Limiter{rdb: rdb, prefix: prefix}
}

func (l *Limiter) Allow(ctx context.Context, orgID, agentID string, limit int, window time.Duration) (bool, error) {
	key := fmt.Sprintf("%s:rl:%s:%s", l.prefix, orgID, agentID)
	n, err := l.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		_ = l.rdb.Expire(ctx, key, window).Err()
	}
	return n <= int64(limit), nil
}
