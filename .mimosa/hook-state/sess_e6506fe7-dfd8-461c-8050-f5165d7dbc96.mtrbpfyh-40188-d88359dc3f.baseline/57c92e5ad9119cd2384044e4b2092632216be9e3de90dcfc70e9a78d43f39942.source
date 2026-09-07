package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	monitorCenterRedisStatusKey   = "monitor-center:openai:status:v1"
	monitorCenterRedisPollLockKey = "monitor-center:openai:poll-lock:v1"
)

type monitorCenterCache struct {
	rdb *redis.Client
}

func NewMonitorCenterCache(rdb *redis.Client) service.MonitorCenterCache {
	if rdb == nil {
		return nil
	}
	return &monitorCenterCache{rdb: rdb}
}

func (c *monitorCenterCache) TryAcquireMonitorCenterPollLock(ctx context.Context, owner string, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, monitorCenterRedisPollLockKey, owner, ttl).Result()
}

func (c *monitorCenterCache) StoreMonitorCenterOpenAIStatus(ctx context.Context, payload []byte, ttl time.Duration) error {
	return c.rdb.Set(ctx, monitorCenterRedisStatusKey, payload, ttl).Err()
}

func (c *monitorCenterCache) LoadMonitorCenterOpenAIStatus(ctx context.Context) ([]byte, error) {
	return c.rdb.Get(ctx, monitorCenterRedisStatusKey).Bytes()
}
