package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheRepo defines the interface for cache operations.
type CacheRepo interface {
	WarmCache(ctx context.Context, shortCode, longURL string) error
	UpdateBloom(ctx context.Context, shortCode string) error
	GetURL(ctx context.Context, shortCode string) (string, error)
	CheckBloom(ctx context.Context, shortCode string) (bool, error)
	DeleteURL(ctx context.Context, shortCode string) error
}

type cacheRepo struct {
	rdb *redis.Client
}

func NewCacheRepo(addr string) CacheRepo {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &cacheRepo{rdb: rdb}
}

// WarmCache sets the shortCode to longURL mapping in Redis with a 1-hour TTL.
func (r *cacheRepo) WarmCache(ctx context.Context, shortCode, longURL string) error {
	return r.rdb.Set(ctx, shortCode, longURL, time.Hour).Err()
}

func (r *cacheRepo) UpdateBloom(ctx context.Context, shortCode string) error {
	// For now, we'll use a simple Redis Set as a mock Bloom filter
	// In production, we would use RedisBloom module: BF.ADD
	return r.rdb.SAdd(ctx, "bloom_filter", shortCode).Err()
}

// GetURL retrieves the longURL for a given shortCode from Redis.
func (r *cacheRepo) GetURL(ctx context.Context, shortCode string) (string, error) {
	return r.rdb.Get(ctx, shortCode).Result()
}

func (r *cacheRepo) CheckBloom(ctx context.Context, shortCode string) (bool, error) {
	// For now, we'll use a simple Redis Set as a mock Bloom filter
	// In production, we would use RedisBloom module: BF.EXISTS
	return r.rdb.SIsMember(ctx, "bloom_filter", shortCode).Result()
}

func (r *cacheRepo) DeleteURL(ctx context.Context, shortCode string) error {
	return r.rdb.Del(ctx, shortCode).Err()
}
