// Package ratelimit owns the Redis lifecycle and the atomic authentication
// fixed-window counter. Redis contains only expiring rate-limit state.
package ratelimit

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrUnavailable = errors.New("rate limiter unavailable")

var fixedWindowScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
local ttl = redis.call('PTTL', KEYS[1])
return {count, ttl}
`)

type Limiter struct {
	client *redis.Client
}

func Open(ctx context.Context, redisURL string) (*Limiter, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, ErrUnavailable
	}
	options.DialTimeout = 2 * time.Second
	options.ReadTimeout = 2 * time.Second
	options.WriteTimeout = 2 * time.Second
	options.MaxRetries = 1
	options.ContextTimeoutEnabled = true
	client := redis.NewClient(options)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, ErrUnavailable
	}
	return &Limiter{client: client}, nil
}

func (l *Limiter) Allow(ctx context.Context, scope, subject string, limit int, window time.Duration) (bool, time.Duration, error) {
	if l == nil || l.client == nil || scope == "" || subject == "" || limit <= 0 || window <= 0 {
		return false, 0, ErrUnavailable
	}
	value, err := fixedWindowScript.Run(ctx, l.client, []string{rateLimitKey(scope, subject)}, window.Milliseconds()).Result()
	if err != nil {
		return false, 0, ErrUnavailable
	}
	values, ok := value.([]interface{})
	if !ok || len(values) != 2 {
		return false, 0, ErrUnavailable
	}
	count, countOK := redisInteger(values[0])
	ttlMilliseconds, ttlOK := redisInteger(values[1])
	if !countOK || !ttlOK || count < 1 || ttlMilliseconds < 1 {
		return false, 0, ErrUnavailable
	}
	return count <= int64(limit), time.Duration(ttlMilliseconds) * time.Millisecond, nil
}

func (l *Limiter) Probe(ctx context.Context) error {
	if l == nil || l.client == nil || l.client.Ping(ctx).Err() != nil {
		return ErrUnavailable
	}
	return nil
}

func (l *Limiter) Close() error {
	if l == nil || l.client == nil {
		return nil
	}
	return l.client.Close()
}

func rateLimitKey(scope, subject string) string {
	digest := sha256.Sum256([]byte(subject))
	return fmt.Sprintf("then:auth-rate:%s:%x", scope, digest)
}

func redisInteger(value interface{}) (int64, bool) {
	switch number := value.(type) {
	case int64:
		return number, true
	case int:
		return int64(number), true
	default:
		return 0, false
	}
}
