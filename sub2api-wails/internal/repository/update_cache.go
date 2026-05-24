package repository

import (
	"context"
	"time"

	"sub2api-wails/internal/service"
	
)

const updateCacheKey = "update:latest"

type updateCache struct {
	rdb *RedisStub
}

func NewUpdateCache(rdb *RedisStub) service.UpdateCache {
	return &updateCache{rdb: rdb}
}

func (c *updateCache) GetUpdateInfo(ctx context.Context) (string, error) {
	return c.rdb.Get(ctx, updateCacheKey).Result()
}

func (c *updateCache) SetUpdateInfo(ctx context.Context, data string, ttl time.Duration) error {
	return c.rdb.Set(ctx, updateCacheKey, data, ttl).Err()
}
