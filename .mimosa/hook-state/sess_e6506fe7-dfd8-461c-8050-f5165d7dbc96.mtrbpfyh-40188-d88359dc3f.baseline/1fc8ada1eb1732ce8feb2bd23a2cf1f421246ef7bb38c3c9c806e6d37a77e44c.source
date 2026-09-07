package repository

import (
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBuildAdaptiveLatencyBuckets_UsesFriendlyP95ScaleAndTailBucket(t *testing.T) {
	buckets := buildAdaptiveLatencyBuckets(28_000, 150_000)

	require.Len(t, buckets, 7)
	require.Equal(t, "0 ms - 5 s", buckets[0].label)
	require.Equal(t, 25_000, buckets[5].minMs)
	require.NotNil(t, buckets[5].maxMs)
	require.Equal(t, 30_000, *buckets[5].maxMs)
	require.Equal(t, ">= 30 s", buckets[6].label)
	require.Nil(t, buckets[6].maxMs)
}

func TestBuildAdaptiveLatencyBuckets_AvoidsUnnecessaryTailBucket(t *testing.T) {
	buckets := buildAdaptiveLatencyBuckets(800, 799)

	require.Len(t, buckets, 4)
	require.Equal(t, "600 ms - 800 ms", buckets[3].label)
	require.NotNil(t, buckets[3].maxMs)
}

func TestNiceLatencyStep_UsesOneTwoFiveSequence(t *testing.T) {
	require.Equal(t, 1, niceLatencyStep(0.2))
	require.Equal(t, 200, niceLatencyStep(133))
	require.Equal(t, 5_000, niceLatencyStep(4_666))
}

func TestBuildUsageWhereIncludesSelectedLatencyUser(t *testing.T) {
	start := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	userID := int64(42)

	_, where, args, next := buildUsageWhere(&service.OpsDashboardFilter{UserID: &userID}, start, end, 1)

	require.True(t, strings.Contains(where, "ul.user_id = $3"))
	require.Equal(t, []any{start, end, userID}, args)
	require.Equal(t, 4, next)
}
