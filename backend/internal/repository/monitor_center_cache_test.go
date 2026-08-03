//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newMonitorCenterCacheTest(t *testing.T) (*monitorCenterCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return &monitorCenterCache{rdb: rdb}, mr
}

func TestNewMonitorCenterCache_NilClientDisablesCache(t *testing.T) {
	require.Nil(t, NewMonitorCenterCache(nil))
}

func TestMonitorCenterCache_StatusRoundTrip(t *testing.T) {
	cache, mr := newMonitorCenterCacheTest(t)
	ctx := context.Background()
	payload := []byte(`{"status":"ok"}`)
	ttl := 3 * time.Minute

	require.NoError(t, cache.StoreMonitorCenterOpenAIStatus(ctx, payload, ttl))
	stored, err := cache.LoadMonitorCenterOpenAIStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, payload, stored)
	require.True(t, mr.Exists("monitor-center:openai:status:v1"))
	require.Equal(t, ttl, mr.TTL(monitorCenterRedisStatusKey))
}

func TestMonitorCenterCache_PollLockIsExclusiveUntilExpiry(t *testing.T) {
	cache, mr := newMonitorCenterCacheTest(t)
	ctx := context.Background()
	ttl := 55 * time.Second

	acquired, err := cache.TryAcquireMonitorCenterPollLock(ctx, "instance-a", ttl)
	require.NoError(t, err)
	require.True(t, acquired)
	require.True(t, mr.Exists("monitor-center:openai:poll-lock:v1"))
	require.Equal(t, ttl, mr.TTL(monitorCenterRedisPollLockKey))

	acquired, err = cache.TryAcquireMonitorCenterPollLock(ctx, "instance-b", ttl)
	require.NoError(t, err)
	require.False(t, acquired)

	mr.FastForward(ttl)
	acquired, err = cache.TryAcquireMonitorCenterPollLock(ctx, "instance-b", ttl)
	require.NoError(t, err)
	require.True(t, acquired)
}
