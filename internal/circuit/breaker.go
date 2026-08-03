package circuit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	StateClosed   = "closed"
	StateOpen     = "open"
	StateHalfOpen = "half-open"
)

type Config struct {
	FailureThreshold int
	WindowSeconds    int
	HalfOpenSeconds  int
}

type Breaker struct {
	rdb    *redis.Client
	prefix string
	cfg    Config
}

func New(rdb *redis.Client, prefix string, cfg Config) *Breaker {
	return &Breaker{rdb: rdb, prefix: prefix, cfg: cfg}
}

func key(prefix, org, dest string) string {
	return fmt.Sprintf("%s:cb:%s:%s", prefix, org, dest)
}

var recordFailureScript = redis.NewScript(`
local k = KEYS[1]
local threshold = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
redis.call('ZREMRANGEBYSCORE', k, '-inf', now - window)
redis.call('ZADD', k, now, now .. ':' .. math.random(1000000))
local count = redis.call('ZCARD', k)
redis.call('EXPIRE', k, window + 60)
if count >= threshold then
  redis.call('SET', k .. ':state', 'open', 'EX', window + 60)
  redis.call('SET', k .. ':opened_at', now, 'EX', window + 60)
  return count
end
return count
`)

func (b *Breaker) RecordFailure(ctx context.Context, orgID, destAgentID string) (int, error) {
	return b.RecordFailureWithConfig(ctx, orgID, destAgentID, b.cfg)
}

func (b *Breaker) RecordFailureWithConfig(ctx context.Context, orgID, destAgentID string, cfg Config) (int, error) {
	now := time.Now().Unix()
	n, err := recordFailureScript.Run(ctx, b.rdb, []string{key(b.prefix, orgID, destAgentID)},
		cfg.FailureThreshold, cfg.WindowSeconds, now).Int()
	return n, err
}

func (b *Breaker) RecordSuccess(ctx context.Context, orgID, destAgentID string) error {
	k := key(b.prefix, orgID, destAgentID)
	pipe := b.rdb.Pipeline()
	pipe.Del(ctx, k)
	pipe.Del(ctx, k+":state")
	pipe.Del(ctx, k+":opened_at")
	_, err := pipe.Exec(ctx)
	return err
}

type State struct {
	State            string
	FailureCount     int32
	OpenedAtUnix     int64
	HalfOpenAtUnix   int64
	Threshold        int32
	WindowSeconds    int32
}

func (b *Breaker) GetState(ctx context.Context, orgID, destAgentID string) (State, error) {
	return b.GetStateWithConfig(ctx, orgID, destAgentID, b.cfg)
}

func (b *Breaker) GetStateWithConfig(ctx context.Context, orgID, destAgentID string, cfg Config) (State, error) {
	k := key(b.prefix, orgID, destAgentID)
	st := State{
		State:         StateClosed,
		Threshold:     int32(cfg.FailureThreshold),
		WindowSeconds: int32(cfg.WindowSeconds),
	}
	stateVal, err := b.rdb.Get(ctx, k+":state").Result()
	if err == redis.Nil {
		count, _ := b.rdb.ZCard(ctx, k).Result()
		st.FailureCount = int32(count)
		return st, nil
	}
	if err != nil {
		return st, err
	}
	st.State = stateVal
	opened, _ := b.rdb.Get(ctx, k+":opened_at").Int64()
	st.OpenedAtUnix = opened
	count, _ := b.rdb.ZCard(ctx, k).Result()
	st.FailureCount = int32(count)
	if st.State == StateOpen {
		half := opened + int64(cfg.HalfOpenSeconds)
		if time.Now().Unix() >= half {
			st.State = StateHalfOpen
			st.HalfOpenAtUnix = half
		}
	}
	return st, nil
}

func (b *Breaker) AllowRequestWithConfig(ctx context.Context, orgID, destAgentID string, cfg Config) (bool, State, error) {
	st, err := b.GetStateWithConfig(ctx, orgID, destAgentID, cfg)
	if err != nil {
		return true, st, err
	}
	switch st.State {
	case StateOpen:
		return false, st, nil
	default:
		return true, st, nil
	}
}

func (b *Breaker) AllowRequest(ctx context.Context, orgID, destAgentID string) (bool, State, error) {
	return b.AllowRequestWithConfig(ctx, orgID, destAgentID, b.cfg)
}
