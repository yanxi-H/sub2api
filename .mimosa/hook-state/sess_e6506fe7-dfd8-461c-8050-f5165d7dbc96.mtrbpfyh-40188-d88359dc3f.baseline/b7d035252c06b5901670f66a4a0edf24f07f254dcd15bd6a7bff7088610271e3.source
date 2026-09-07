package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGetLatencyTrendReturnsPercentilesAndFillsMissingBuckets(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}
	start := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Minute)
	groupID := int64(7)

	rows := sqlmock.NewRows([]string{
		"bucket", "p50", "p90", "p95", "avg", "max", "sample_count",
		"ttft_p50", "ttft_p90", "ttft_p95", "ttft_avg", "ttft_max",
	}).AddRow(
		start, 100.4, 250.2, 400.8, 175.5, int64(900), int64(12),
		50.2, 125.8, 210.4, 90.6, int64(600),
	)
	mock.ExpectQuery(`(?s)SELECT.*percentile_cont\(0\.50\).*FROM ops_request_performance p.*logical_status_code >= 200.*logical_status_code < 400.*GROUP BY 1`).
		WithArgs(start, end, groupID).
		WillReturnRows(rows)

	result, err := repo.GetLatencyTrend(context.Background(), &service.OpsDashboardFilter{
		StartTime: start,
		EndTime:   end,
		GroupID:   &groupID,
	}, 60)
	require.NoError(t, err)
	require.Equal(t, "1m", result.Bucket)
	require.Len(t, result.Points, 2)
	require.Equal(t, 12, int(result.Points[0].SampleCount))
	require.Equal(t, 100, *result.Points[0].P50)
	require.Equal(t, 250, *result.Points[0].P90)
	require.Equal(t, 401, *result.Points[0].P95)
	require.Equal(t, 176, *result.Points[0].Avg)
	require.Equal(t, 900, *result.Points[0].Max)
	require.Equal(t, 50, *result.Points[0].TTFT.P50)
	require.Equal(t, 126, *result.Points[0].TTFT.P90)
	require.Equal(t, 210, *result.Points[0].TTFT.P95)
	require.Equal(t, 91, *result.Points[0].TTFT.Avg)
	require.Equal(t, 600, *result.Points[0].TTFT.Max)
	require.Zero(t, result.Points[1].SampleCount)
	require.Nil(t, result.Points[1].P95)
	require.NoError(t, mock.ExpectationsWereMet())
}
