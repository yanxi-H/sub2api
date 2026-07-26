package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newConcurrencyTrendTestCache(t *testing.T) (*concurrencyCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return &concurrencyCache{rdb: rdb, slotTTLSeconds: 900, waitQueueTTLSeconds: 900}, mr
}

func TestConcurrencyTrendMergeKeepsMinuteMaxAndExpires(t *testing.T) {
	cache, _ := newConcurrencyTrendTestCache(t)
	ctx := context.Background()
	bucket := time.Date(2026, 7, 19, 12, 34, 0, 0, time.UTC)

	require.NoError(t, cache.MergeUserConcurrencyTrend(ctx, bucket,
		map[int64]service.ConcurrencyPeak{7: {PeakInUse: 3, PeakWaiting: 2, PeakDemand: 5}},
		service.ConcurrencyPeak{PeakInUse: 8, PeakWaiting: 3, PeakDemand: 11},
		map[int64]service.ConcurrencyLanePeaks{
			7: {
				Normal: service.ConcurrencyPeak{PeakInUse: 2, PeakWaiting: 2, PeakDemand: 4},
				Heavy:  service.ConcurrencyPeak{PeakInUse: 1, PeakDemand: 1},
			},
		},
		service.ConcurrencyLanePeaks{
			Normal: service.ConcurrencyPeak{PeakInUse: 6, PeakWaiting: 3, PeakDemand: 9},
			Heavy:  service.ConcurrencyPeak{PeakInUse: 2, PeakDemand: 2},
		},
	))
	require.NoError(t, cache.MergeUserConcurrencyTrend(ctx, bucket,
		map[int64]service.ConcurrencyPeak{
			7: {PeakInUse: 2, PeakWaiting: 1, PeakDemand: 3},
			9: {PeakInUse: 4, PeakWaiting: 0, PeakDemand: 4},
		},
		service.ConcurrencyPeak{PeakInUse: 6, PeakWaiting: 1, PeakDemand: 7},
		map[int64]service.ConcurrencyLanePeaks{
			7: {Normal: service.ConcurrencyPeak{PeakInUse: 1, PeakWaiting: 1, PeakDemand: 2}},
			9: {Recovery: service.ConcurrencyPeak{PeakWaiting: 1, PeakDemand: 1}},
		},
		service.ConcurrencyLanePeaks{
			Normal:   service.ConcurrencyPeak{PeakInUse: 5, PeakWaiting: 1, PeakDemand: 6},
			Recovery: service.ConcurrencyPeak{PeakWaiting: 1, PeakDemand: 1},
		},
	))

	trend, err := cache.GetUserConcurrencyTrend(ctx, bucket.Add(-time.Minute), bucket.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, trend.Points, 3)
	require.Empty(t, trend.Points[0].Users)
	require.Equal(t, service.ConcurrencyPeak{PeakInUse: 8, PeakWaiting: 3, PeakDemand: 11}, trend.Points[1].System)
	require.Equal(t, service.ConcurrencyPeak{PeakInUse: 3, PeakWaiting: 2, PeakDemand: 5}, trend.Points[1].Users[7])
	require.Equal(t, service.ConcurrencyPeak{PeakInUse: 4, PeakWaiting: 0, PeakDemand: 4}, trend.Points[1].Users[9])
	require.Equal(t, service.ConcurrencyPeak{PeakInUse: 6, PeakWaiting: 3, PeakDemand: 9}, trend.Points[1].SystemLanes.Normal)
	require.Equal(t, service.ConcurrencyPeak{PeakInUse: 2, PeakDemand: 2}, trend.Points[1].SystemLanes.Heavy)
	require.Equal(t, service.ConcurrencyPeak{PeakWaiting: 1, PeakDemand: 1}, trend.Points[1].SystemLanes.Recovery)
	require.Equal(t, service.ConcurrencyPeak{PeakInUse: 2, PeakWaiting: 2, PeakDemand: 4}, trend.Points[1].UserLanes[7].Normal)
	require.Equal(t, service.ConcurrencyPeak{PeakInUse: 1, PeakDemand: 1}, trend.Points[1].UserLanes[7].Heavy)
	require.Equal(t, service.ConcurrencyPeak{PeakWaiting: 1, PeakDemand: 1}, trend.Points[1].UserLanes[9].Recovery)
	require.Empty(t, trend.Points[2].Users)

	ttl, err := cache.rdb.TTL(ctx, userConcurrencyTrendKey(bucket)).Result()
	require.NoError(t, err)
	require.Positive(t, ttl)
	require.LessOrEqual(t, ttl, userConcurrencyTrendTTL)
}

func TestConcurrencyTrendReadsLegacyBucketsAsNormalLane(t *testing.T) {
	cache, _ := newConcurrencyTrendTestCache(t)
	ctx := context.Background()
	bucket := time.Date(2026, 7, 19, 12, 34, 0, 0, time.UTC)
	require.NoError(t, cache.rdb.HSet(ctx, userConcurrencyTrendKey(bucket),
		"s:a", 5, "s:w", 2, "s:d", 7,
		"u:7:a", 3, "u:7:w", 1, "u:7:d", 4,
	).Err())

	trend, err := cache.GetUserConcurrencyTrend(ctx, bucket, bucket)
	require.NoError(t, err)
	require.Equal(t, trend.Points[0].System, trend.Points[0].SystemLanes.Normal)
	require.Equal(t, trend.Points[0].Users[7], trend.Points[0].UserLanes[7].Normal)
}

func TestRequestBodyLaneLoadsAreIndexedAndClassifiedByLane(t *testing.T) {
	cache, _ := newConcurrencyTrendTestCache(t)
	ctx := context.Background()

	acquired, state, observedAt, err := cache.AcquireRequestBodyLaneWithState(ctx, service.RequestBodyLaneHeavy, 42, 1001, 2, 1, "heavy-1")
	require.NoError(t, err)
	require.True(t, acquired)
	require.False(t, observedAt.IsZero())
	require.Equal(t, service.RequestBodyLaneUserLoad{HeavyActive: 1}, state)

	allowed, state, _, err := cache.IncrementRequestBodyLaneScopedWaitCountWithState(ctx, service.RequestBodyLaneRecovery, 0, 1001, 1, "recovery-waiter")
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, service.RequestBodyLaneUserLoad{HeavyActive: 1, RecoveryWaiting: 1}, state)

	loads, err := cache.GetActiveRequestBodyLaneLoads(ctx)
	require.NoError(t, err)
	require.Equal(t, state, loads[1001])

	state, _, err = cache.DecrementRequestBodyLaneScopedWaitCountWithState(ctx, service.RequestBodyLaneRecovery, 0, 1001, "recovery-waiter")
	require.NoError(t, err)
	require.Equal(t, service.RequestBodyLaneUserLoad{HeavyActive: 1}, state)
	state, _, err = cache.ReleaseRequestBodyLaneWithState(ctx, service.RequestBodyLaneHeavy, 42, 1001, 1, "heavy-1")
	require.NoError(t, err)
	require.Equal(t, service.RequestBodyLaneUserLoad{}, state)

	loads, err = cache.GetActiveRequestBodyLaneLoads(ctx)
	require.NoError(t, err)
	require.Empty(t, loads)
	require.Zero(t, cache.rdb.ZCard(ctx, requestBodyActiveIndexKey).Val())

	allowed, _, _, err = cache.IncrementRequestBodyLaneScopedWaitCountWithState(ctx, service.RequestBodyLaneHeavy, 43, 1002, 1, "heavy-from-queue")
	require.NoError(t, err)
	require.True(t, allowed)
	acquired, state, _, err = cache.AcquireRequestBodyLaneWithState(ctx, service.RequestBodyLaneHeavy, 43, 1002, 1, 1, "heavy-from-queue")
	require.NoError(t, err)
	require.True(t, acquired)
	require.Equal(t, service.RequestBodyLaneUserLoad{HeavyActive: 1}, state, "acquiring a queued request must atomically clear its wait marker")
	require.Zero(t, cache.rdb.ZCard(ctx, requestBodyLaneScopeWaitKey(service.RequestBodyLaneHeavy, 43)).Val())
}

func TestRequestBodyLaneWaitCountIsBoundedByScope(t *testing.T) {
	cache, _ := newConcurrencyTrendTestCache(t)
	ctx := context.Background()

	allowed, _, _, err := cache.IncrementRequestBodyLaneScopedWaitCountWithState(ctx, service.RequestBodyLaneHeavy, 42, 1001, 1, "heavy-1")
	require.NoError(t, err)
	require.True(t, allowed)

	allowed, _, _, err = cache.IncrementRequestBodyLaneScopedWaitCountWithState(ctx, service.RequestBodyLaneHeavy, 42, 1002, 1, "heavy-2")
	require.NoError(t, err)
	require.False(t, allowed, "one account scope must not retain more queued bodies than its wait limit")

	allowed, _, _, err = cache.IncrementRequestBodyLaneScopedWaitCountWithState(ctx, service.RequestBodyLaneHeavy, 43, 1002, 1, "heavy-2")
	require.NoError(t, err)
	require.True(t, allowed, "heavy queues are isolated by account")

	_, _, err = cache.DecrementRequestBodyLaneScopedWaitCountWithState(ctx, service.RequestBodyLaneHeavy, 42, 1001, "heavy-1")
	require.NoError(t, err)
	allowed, _, _, err = cache.IncrementRequestBodyLaneScopedWaitCountWithState(ctx, service.RequestBodyLaneHeavy, 42, 1003, 1, "heavy-3")
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestConcurrencyStateMethodsReturnObservedCounts(t *testing.T) {
	cache, _ := newConcurrencyTrendTestCache(t)
	ctx := context.Background()

	acquired, active, observedAt, err := cache.AcquireUserSlotWithState(ctx, 42, 2, "request-1")
	require.NoError(t, err)
	require.True(t, acquired)
	require.Equal(t, 1, active)
	require.False(t, observedAt.IsZero())

	acquired, active, _, err = cache.AcquireUserSlotWithState(ctx, 42, 2, "request-2")
	require.NoError(t, err)
	require.True(t, acquired)
	require.Equal(t, 2, active)

	acquired, active, _, err = cache.AcquireUserSlotWithState(ctx, 42, 2, "request-3")
	require.NoError(t, err)
	require.False(t, acquired)
	require.Equal(t, 2, active)

	incremented, waiting, _, err := cache.IncrementWaitCountWithState(ctx, 42, 22)
	require.NoError(t, err)
	require.True(t, incremented)
	require.Equal(t, 1, waiting)

	active, _, err = cache.ReleaseUserSlotWithState(ctx, 42, "request-1")
	require.NoError(t, err)
	require.Equal(t, 1, active)
	waiting, _, err = cache.DecrementWaitCountWithState(ctx, 42)
	require.NoError(t, err)
	require.Zero(t, waiting)

	unlimited, _, err := cache.TrackUserSlotWithState(ctx, 99, "unlimited-1")
	require.NoError(t, err)
	require.Equal(t, 1, unlimited)
	unlimited, _, err = cache.TrackUserSlotWithState(ctx, 99, "unlimited-2")
	require.NoError(t, err)
	require.Equal(t, 2, unlimited)
}

func TestRequestBodyClassificationReservationSurvivesLaneTransitions(t *testing.T) {
	cache, _ := newConcurrencyTrendTestCache(t)
	ctx := context.Background()
	const userID int64 = 1003

	state, _, err := cache.SetRequestBodyClassificationStateWithState(ctx, userID, "responses-1", true, false)
	require.NoError(t, err)
	require.Equal(t, service.RequestBodyLaneUserLoad{PendingActive: 1}, state)

	acquired, state, _, err := cache.AcquireRequestBodyLaneWithState(
		ctx, service.RequestBodyLaneHeavy, 45, userID, 1, 1, "heavy-1",
	)
	require.NoError(t, err)
	require.True(t, acquired, "classification reservations must not consume the per-user large-request slot")
	require.Equal(t, service.RequestBodyLaneUserLoad{HeavyActive: 1, PendingActive: 1}, state)

	state, _, err = cache.ReleaseRequestBodyLaneWithState(ctx, service.RequestBodyLaneHeavy, 45, userID, 1, "heavy-1")
	require.NoError(t, err)
	require.Equal(t, service.RequestBodyLaneUserLoad{PendingActive: 1}, state, "the reservation spans failover gaps")

	state, _, err = cache.SetRequestBodyClassificationStateWithState(ctx, userID, "responses-1", false, false)
	require.NoError(t, err)
	require.Empty(t, state)
}

func TestRequestBodyLaneAcquireIsIdempotentForSameRequest(t *testing.T) {
	cache, _ := newConcurrencyTrendTestCache(t)
	ctx := context.Background()

	acquired, state, _, err := cache.AcquireRequestBodyLaneWithState(
		ctx, service.RequestBodyLaneHeavy, 46, 1006, 1, 1, "same-request",
	)
	require.NoError(t, err)
	require.True(t, acquired)
	require.Equal(t, service.RequestBodyLaneUserLoad{HeavyActive: 1}, state)

	acquired, state, _, err = cache.AcquireRequestBodyLaneWithState(
		ctx, service.RequestBodyLaneHeavy, 46, 1006, 1, 1, "same-request",
	)
	require.NoError(t, err)
	require.True(t, acquired)
	require.Equal(t, service.RequestBodyLaneUserLoad{HeavyActive: 1}, state)
	require.EqualValues(t, 1, cache.rdb.ZCard(ctx, requestBodyLaneScopeKey(service.RequestBodyLaneHeavy, 46)).Val())
}

func TestRecoveryLaneEnforcesGlobalAccountAndUserLimits(t *testing.T) {
	cache, _ := newConcurrencyTrendTestCache(t)
	ctx := context.Background()

	acquired, _, _, err := cache.AcquireRequestBodyLaneWithState(
		ctx, service.RequestBodyLaneRecovery, 41, 1001, service.RequestBodyRecoveryGlobalConcurrency, 1, "recovery-1",
	)
	require.NoError(t, err)
	require.True(t, acquired)

	acquired, _, _, err = cache.AcquireRequestBodyLaneWithState(
		ctx, service.RequestBodyLaneRecovery, 41, 1002, service.RequestBodyRecoveryGlobalConcurrency, 1, "recovery-same-account",
	)
	require.NoError(t, err)
	require.False(t, acquired, "one account may only run one recovery request")

	acquired, _, _, err = cache.AcquireRequestBodyLaneWithState(
		ctx, service.RequestBodyLaneRecovery, 42, 1002, service.RequestBodyRecoveryGlobalConcurrency, 1, "recovery-2",
	)
	require.NoError(t, err)
	require.True(t, acquired, "a second account may use the other global recovery slot")

	acquired, _, _, err = cache.AcquireRequestBodyLaneWithState(
		ctx, service.RequestBodyLaneRecovery, 43, 1003, service.RequestBodyRecoveryGlobalConcurrency, 1, "recovery-global-full",
	)
	require.NoError(t, err)
	require.False(t, acquired, "the recovery lane has two global slots")

	acquired, _, _, err = cache.AcquireRequestBodyLaneWithState(
		ctx, service.RequestBodyLaneHeavy, 44, 1001, 1, 1, "heavy-same-user",
	)
	require.NoError(t, err)
	require.False(t, acquired, "one user may only hold one large-request slot across lanes")
}

func TestRequestBodyLeaseRefreshDoesNotRecreateLostLease(t *testing.T) {
	cache, _ := newConcurrencyTrendTestCache(t)
	ctx := context.Background()

	acquired, _, _, err := cache.AcquireRequestBodyLaneWithState(
		ctx, service.RequestBodyLaneRecovery, 41, 1001, service.RequestBodyRecoveryGlobalConcurrency, 1, "recovery-refresh",
	)
	require.NoError(t, err)
	require.True(t, acquired)

	refreshed, err := cache.RefreshRequestBodyLane(ctx, service.RequestBodyLaneRecovery, 41, 1001, 1, "recovery-refresh")
	require.NoError(t, err)
	require.True(t, refreshed)

	require.NoError(t, cache.rdb.Del(ctx, requestBodyLaneUserKey(1001)).Err())
	refreshed, err = cache.RefreshRequestBodyLane(ctx, service.RequestBodyLaneRecovery, 41, 1001, 1, "recovery-refresh")
	require.NoError(t, err)
	require.False(t, refreshed, "refresh must not recreate a partially lost lease")
}

func TestRequestBodyClassificationReservationFollowsUserSlotLifecycle(t *testing.T) {
	cache, _ := newConcurrencyTrendTestCache(t)
	svc := service.NewConcurrencyService(cache)
	ctx := context.Background()
	const userID int64 = 1004

	result, err := svc.AcquireUserSlotForRequestBodyAdmission(ctx, userID, 2, "responses-lifecycle", false)
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.NotNil(t, result.ResolveNormalFunc)
	require.NotNil(t, result.ReserveNonNormalFunc)

	loads, err := cache.GetActiveRequestBodyLaneLoads(ctx)
	require.NoError(t, err)
	require.Equal(t, service.RequestBodyLaneUserLoad{PendingActive: 1}, loads[userID])

	result.ResolveNormalFunc()
	loads, err = cache.GetActiveRequestBodyLaneLoads(ctx)
	require.NoError(t, err)
	require.NotContains(t, loads, userID)

	result.ReserveNonNormalFunc()
	loads, err = cache.GetActiveRequestBodyLaneLoads(ctx)
	require.NoError(t, err)
	require.Equal(t, service.RequestBodyLaneUserLoad{PendingActive: 1}, loads[userID])

	result.ReleaseFunc()
	loads, err = cache.GetActiveRequestBodyLaneLoads(ctx)
	require.NoError(t, err)
	require.NotContains(t, loads, userID)
	active, err := cache.GetUserConcurrency(ctx, userID)
	require.NoError(t, err)
	require.Zero(t, active)
}

func TestRequestBodyClassificationReservationMovesFromUserWaitToActive(t *testing.T) {
	cache, _ := newConcurrencyTrendTestCache(t)
	svc := service.NewConcurrencyService(cache)
	ctx := context.Background()
	const userID int64 = 1005

	blocker, err := svc.AcquireUserSlot(ctx, userID, 1)
	require.NoError(t, err)
	require.True(t, blocker.Acquired)

	allowed, err := svc.IncrementWaitCountForRequestBodyAdmission(ctx, userID, 1, "responses-waiting")
	require.NoError(t, err)
	require.True(t, allowed)
	loads, err := cache.GetActiveRequestBodyLaneLoads(ctx)
	require.NoError(t, err)
	require.Equal(t, service.RequestBodyLaneUserLoad{PendingWaiting: 1}, loads[userID])

	blocked, err := svc.AcquireUserSlotForRequestBodyAdmission(ctx, userID, 1, "responses-waiting", true)
	require.NoError(t, err)
	require.False(t, blocked.Acquired)
	blocker.ReleaseFunc()

	result, err := svc.AcquireUserSlotForRequestBodyAdmission(ctx, userID, 1, "responses-waiting", true)
	require.NoError(t, err)
	require.True(t, result.Acquired)
	loads, err = cache.GetActiveRequestBodyLaneLoads(ctx)
	require.NoError(t, err)
	require.Equal(t, service.RequestBodyLaneUserLoad{PendingActive: 1, PendingWaiting: 1}, loads[userID])

	svc.DecrementWaitCountForRequestBodyAdmission(ctx, userID, "responses-waiting", true)
	loads, err = cache.GetActiveRequestBodyLaneLoads(ctx)
	require.NoError(t, err)
	require.Equal(t, service.RequestBodyLaneUserLoad{PendingActive: 1}, loads[userID])
	result.ResolveNormalFunc()
	result.ReleaseFunc()
}

func TestReleaseUserSlotWithStateExcludesExpiredMembers(t *testing.T) {
	cache, _ := newConcurrencyTrendTestCache(t)
	ctx := context.Background()
	userID := int64(42)
	now, err := cache.rdb.Time(ctx).Result()
	require.NoError(t, err)

	require.NoError(t, cache.rdb.ZAdd(ctx, userSlotKey(userID),
		redis.Z{Score: float64(now.Unix() - int64(cache.slotTTLSeconds) - 1), Member: "expired-request"},
		redis.Z{Score: float64(now.Unix()), Member: "current-request"},
	).Err())

	remaining, _, err := cache.ReleaseUserSlotWithState(ctx, userID, "current-request")
	require.NoError(t, err)
	require.Zero(t, remaining)
	require.Zero(t, cache.rdb.Exists(ctx, userSlotKey(userID)).Val())
}
