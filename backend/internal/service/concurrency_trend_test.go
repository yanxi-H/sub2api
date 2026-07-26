package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRecordConcurrencyTrendSampleKeepsPerUserAndSystemPeaks(t *testing.T) {
	pending := make(map[time.Time]map[int64]ConcurrencyPeak)
	system := make(map[time.Time]ConcurrencyPeak)
	userLanes := make(map[time.Time]map[int64]ConcurrencyLanePeaks)
	systemLanes := make(map[time.Time]ConcurrencyLanePeaks)
	bucket := time.Date(2026, 7, 19, 12, 34, 0, 0, time.UTC)
	live := map[int64]userConcurrencyLiveState{
		1: {active: 3, waiting: 2, requestBodyLoad: RequestBodyLaneUserLoad{HeavyActive: 1, PendingActive: 1}},
		2: {active: 4, waiting: 0, requestBodyLoad: RequestBodyLaneUserLoad{RecoveryWaiting: 1, PendingActive: 1}},
	}
	totals := ConcurrencyLaneSnapshots{}
	for _, state := range live {
		addConcurrencyLaneSnapshots(&totals, concurrencyLaneSnapshotsForState(state), 1)
	}

	recordConcurrencyTrendSample(pending, system, userLanes, systemLanes, bucket.Add(10*time.Second), live, 0, 7, 2, totals)
	addConcurrencyLaneSnapshots(&totals, concurrencyLaneSnapshotsForState(live[1]), -1)
	live[1] = userConcurrencyLiveState{active: 2, waiting: 4, requestBodyLoad: RequestBodyLaneUserLoad{HeavyActive: 1, PendingActive: 1}}
	addConcurrencyLaneSnapshots(&totals, concurrencyLaneSnapshotsForState(live[1]), 1)
	recordConcurrencyTrendSample(pending, system, userLanes, systemLanes, bucket.Add(20*time.Second), live, 1, 6, 4, totals)

	require.Equal(t, ConcurrencyPeak{PeakInUse: 3, PeakWaiting: 4, PeakDemand: 6}, pending[bucket][1])
	require.Equal(t, ConcurrencyPeak{PeakInUse: 4, PeakWaiting: 0, PeakDemand: 4}, pending[bucket][2])
	require.Equal(t, ConcurrencyPeak{PeakInUse: 7, PeakWaiting: 4, PeakDemand: 10}, system[bucket])
	require.Equal(t, ConcurrencyPeak{PeakInUse: 2, PeakWaiting: 4, PeakDemand: 5}, userLanes[bucket][1].Normal)
	require.Equal(t, ConcurrencyPeak{PeakInUse: 1, PeakWaiting: 0, PeakDemand: 1}, userLanes[bucket][1].Heavy)
	require.Equal(t, ConcurrencyPeak{PeakInUse: 3, PeakWaiting: 0, PeakDemand: 3}, userLanes[bucket][2].Normal)
	require.Equal(t, ConcurrencyPeak{PeakInUse: 0, PeakWaiting: 1, PeakDemand: 1}, userLanes[bucket][2].Recovery)
	require.Equal(t, ConcurrencyPeak{PeakInUse: 5, PeakWaiting: 4, PeakDemand: 8}, systemLanes[bucket].Normal)
	require.Equal(t, ConcurrencyPeak{PeakInUse: 1, PeakWaiting: 0, PeakDemand: 1}, systemLanes[bucket].Heavy)
	require.Equal(t, ConcurrencyPeak{PeakInUse: 0, PeakWaiting: 1, PeakDemand: 1}, systemLanes[bucket].Recovery)
}

func TestConcurrencyServiceReturnsSixtyEmptyBucketsWithoutTrendCache(t *testing.T) {
	svc := NewConcurrencyService(nil)
	now := time.Now().UTC().Truncate(time.Second)

	trend, err := svc.GetUserConcurrencyTrend(t.Context(), now.Add(-59*time.Minute), now)
	require.NoError(t, err)
	require.Equal(t, "minute", trend.Bucket)
	require.Len(t, trend.Points, 60)
	require.Equal(t, now.Truncate(time.Minute).Add(-59*time.Minute), trend.Points[0].BucketStart)
	require.Equal(t, now.Truncate(time.Minute), trend.Points[59].BucketStart)
}

func TestConcurrencyServiceReturnsUnavailableForExpiredRange(t *testing.T) {
	svc := NewConcurrencyService(nil)
	now := time.Now().UTC().Truncate(time.Minute)

	trend, err := svc.GetUserConcurrencyTrend(t.Context(), now.Add(-72*time.Hour), now.Add(-48*time.Hour))

	require.NoError(t, err)
	require.Equal(t, "5m", trend.Bucket)
	require.Equal(t, now.Add(-72*time.Hour), trend.StartTime)
	require.Equal(t, now.Add(-48*time.Hour), trend.EndTime)
	require.Empty(t, trend.Points)
	require.False(t, trend.CoverageComplete)
	require.Nil(t, trend.CoverageStart)
	require.Nil(t, trend.CoverageEnd)
}

func TestConcurrencyServiceClampsPartiallyExpiredRangeToRetentionWindow(t *testing.T) {
	svc := NewConcurrencyService(nil)
	now := time.Now().UTC().Truncate(time.Minute)
	requestedStart := now.Add(-30 * time.Hour)
	requestedEnd := now.Add(-23 * time.Hour)

	trend, err := svc.GetUserConcurrencyTrend(t.Context(), requestedStart, requestedEnd)

	require.NoError(t, err)
	require.Equal(t, requestedStart, trend.StartTime)
	require.Equal(t, requestedEnd, trend.EndTime)
	require.Equal(t, "5m", trend.Bucket)
	require.False(t, trend.CoverageComplete)
	require.NotNil(t, trend.CoverageStart)
	require.NotNil(t, trend.CoverageEnd)
	require.Equal(t, requestedEnd, *trend.CoverageEnd)
	require.WithinDuration(t, now.Add(-24*time.Hour), *trend.CoverageStart, time.Minute)
	require.NotEmpty(t, trend.Points)
	for _, point := range trend.Points {
		require.False(t, point.BucketStart.Before(trend.CoverageStart.Truncate(5*time.Minute)))
		require.False(t, point.BucketStart.After(requestedEnd))
	}
}

func TestConcurrencyTrendAggregatesFiveMinutePeaksForLongRanges(t *testing.T) {
	start := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
	coverageStart := start.Add(time.Minute)
	coverageEnd := start.Add(5 * time.Minute)
	trend := &UserConcurrencyTrend{
		StartTime:        start,
		EndTime:          start.Add(5 * time.Minute),
		CoverageStart:    &coverageStart,
		CoverageEnd:      &coverageEnd,
		CoverageComplete: false,
		Bucket:           "minute",
		Points: []UserConcurrencyTrendPoint{
			{BucketStart: start, System: ConcurrencyPeak{PeakDemand: 2}, SystemLanes: ConcurrencyLanePeaks{Heavy: ConcurrencyPeak{PeakWaiting: 1}}},
			{BucketStart: start.Add(4 * time.Minute), System: ConcurrencyPeak{PeakDemand: 7}, SystemLanes: ConcurrencyLanePeaks{Heavy: ConcurrencyPeak{PeakWaiting: 3}}},
			{BucketStart: start.Add(5 * time.Minute), System: ConcurrencyPeak{PeakDemand: 4}},
		},
	}

	aggregated := aggregateUserConcurrencyTrend(trend, 5*time.Minute)
	require.Equal(t, "5m", aggregated.Bucket)
	require.Len(t, aggregated.Points, 2)
	require.Equal(t, 7, aggregated.Points[0].System.PeakDemand)
	require.Equal(t, 3, aggregated.Points[0].SystemLanes.Heavy.PeakWaiting)
	require.Equal(t, 4, aggregated.Points[1].System.PeakDemand)
	require.Equal(t, &coverageStart, aggregated.CoverageStart)
	require.Equal(t, &coverageEnd, aggregated.CoverageEnd)
	require.False(t, aggregated.CoverageComplete)
}

func TestRequestBodyClassificationTransitionsDoNotInflateNormalLanePeak(t *testing.T) {
	pending := make(map[time.Time]map[int64]ConcurrencyPeak)
	system := make(map[time.Time]ConcurrencyPeak)
	userLanes := make(map[time.Time]map[int64]ConcurrencyLanePeaks)
	systemLanes := make(map[time.Time]ConcurrencyLanePeaks)
	bucket := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	live := map[int64]userConcurrencyLiveState{
		7: {active: 1, requestBodyLoad: RequestBodyLaneUserLoad{PendingActive: 1}},
	}

	record := func(at time.Time) {
		totals := concurrencyLaneSnapshotsForState(live[7])
		recordConcurrencyTrendSample(pending, system, userLanes, systemLanes, at, live, 7, live[7].active, live[7].waiting, totals)
	}
	record(bucket.Add(time.Second))
	live[7] = userConcurrencyLiveState{
		active:          1,
		requestBodyLoad: RequestBodyLaneUserLoad{HeavyActive: 1, PendingActive: 1},
	}
	record(bucket.Add(2 * time.Second))
	live[7] = userConcurrencyLiveState{
		active:          1,
		requestBodyLoad: RequestBodyLaneUserLoad{PendingActive: 1},
	}
	record(bucket.Add(3 * time.Second))

	require.Zero(t, userLanes[bucket][7].Normal.PeakDemand)
	require.Equal(t, 1, userLanes[bucket][7].Heavy.PeakDemand)
	require.Zero(t, systemLanes[bucket].Normal.PeakDemand)
	require.Equal(t, 1, systemLanes[bucket].Heavy.PeakDemand)
}

func TestPendingUserWaitIsExcludedFromNormalLane(t *testing.T) {
	state := userConcurrencyLiveState{
		active:  3,
		waiting: 1,
		requestBodyLoad: RequestBodyLaneUserLoad{
			PendingWaiting: 1,
		},
	}
	lanes := concurrencyLaneSnapshotsForState(state)
	require.Equal(t, 3, lanes.Normal.InUse)
	require.Zero(t, lanes.Normal.Waiting)
}

func TestActiveLargeLaneRemainsExcludedFromNormalWhenPendingMarkerExpires(t *testing.T) {
	state := userConcurrencyLiveState{
		active: 2,
		requestBodyLoad: RequestBodyLaneUserLoad{
			HeavyActive: 1,
		},
	}

	lanes := concurrencyLaneSnapshotsForState(state)

	require.Equal(t, 1, lanes.Normal.InUse)
	require.Equal(t, 1, lanes.Heavy.InUse)
	require.Equal(t, 2, lanes.Normal.Demand+lanes.Heavy.Demand+lanes.Recovery.Demand)
}
