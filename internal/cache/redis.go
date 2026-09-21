package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis는 여러 인스턴스가 레이트리밋과 멱등성을 공유할 때 쓴다.
type Redis struct {
	client *redis.Client
}

func NewRedis(url string) (*Redis, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("REDIS_URL 파싱: %w", err)
	}
	return &Redis{client: redis.NewClient(opt)}, nil
}

func (r *Redis) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	// INCR 후 NX 옵션으로 EXPIRE를 건다. NX가 있어야 TTL이 첫 증가에만
	// 설정되고, 이후 증가가 창을 밀지 않는다.
	pipe := r.client.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.ExpireNX(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("캐시 증가: %w", err)
	}
	return incr.Val(), nil
}

func (r *Redis) SetNX(ctx context.Context, key, val string, ttl time.Duration) (bool, error) {
	ok, err := r.client.SetNX(ctx, key, val, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("캐시 설정: %w", err)
	}
	return ok, nil
}

func (r *Redis) Get(ctx context.Context, key string) (string, bool, error) {
	v, err := r.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("캐시 조회: %w", err)
	}
	return v, true, nil
}

func (r *Redis) Close() error { return r.client.Close() }
