package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestQueryPerformanceTrendFillsEmptyBuckets(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}
	start := time.Date(2026, 7, 26, 8, 0, 15, 0, time.UTC)
	end := start.Add(2 * time.Minute)
	where, args := performanceWhere(&service.OpsDashboardFilter{StartTime: start, EndTime: end})

	mock.ExpectQuery(`(?s)SELECT date_bin\(make_interval.*FROM ops_request_performance p.*slow_cause <> 'healthy'.*GROUP BY 1, 2`).
		WithArgs(start, end, 60).
		WillReturnRows(sqlmock.NewRows([]string{"bucket", "slow_cause", "count"}).
			AddRow(time.Date(2026, 7, 26, 8, 1, 0, 0, time.UTC), "upstream_ttft", int64(2)))

	points, err := repo.queryPerformanceTrend(context.Background(), where, args, start, end, 60)
	require.NoError(t, err)
	require.Len(t, points, 3)
	require.Empty(t, points[0].Causes)
	require.Equal(t, int64(2), points[1].Causes["upstream_ttft"])
	require.Empty(t, points[2].Causes)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestInsertRequestPerformanceUpsertsNumericTelemetry(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}
	createdAt := time.Date(2026, 7, 26, 8, 0, 0, 0, time.UTC)
	input := &service.OpsRequestPerformanceInput{
		CreatedAt:          createdAt,
		RequestID:          "request-1",
		UserID:             1,
		APIKeyID:           2,
		AccountID:          3,
		Platform:           service.PlatformOpenAI,
		Model:              "gpt-5",
		Stream:             true,
		RequestBodyLane:    service.RequestBodyLaneHeavy,
		RequestBodyBytes:   1024,
		LogicalStatusCode:  200,
		EndToEndMs:         12_000,
		TimeToFirstTokenMs: 11_000,
		AttemptCount:       1,
		SlowCause:          "upstream_ttft",
	}
	mock.ExpectExec(`(?s)INSERT INTO ops_request_performance.*ON CONFLICT \(request_id, api_key_id\) DO UPDATE`).
		WithArgs(
			createdAt, "request-1", int64(1), int64(2), int64(3), nil,
			service.PlatformOpenAI, "gpt-5", true, "heavy", int64(1024), 200,
			int64(12_000), int64(0), int64(0), int64(0), int64(0), int64(0), int64(0), int64(11_000),
			int64(0), int64(0), int64(0), 1, 0, "", "upstream_ttft",
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, repo.InsertRequestPerformance(context.Background(), input))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryPerformanceImpactsReturnsRowsIterationError(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}
	start := time.Date(2026, 7, 26, 8, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	where, args := performanceWhere(&service.OpsDashboardFilter{StartTime: start, EndTime: end})
	wantErr := errors.New("rows interrupted")

	mock.ExpectQuery(`(?s)SELECT p\.user_id::text.*FROM ops_request_performance p LEFT JOIN users.*ORDER BY COUNT\(\*\) FILTER \(WHERE p\.slow_cause <> 'healthy'\) DESC, COUNT\(\*\) DESC$`).
		WithArgs(start, end).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "request_count", "slow_rate", "e2e", "ttft", "queue", "cause"}).
			AddRow("1", "user", int64(1), float64(0), float64(10), float64(5), float64(0), "healthy").
			RowError(0, wantErr))

	_, err := repo.queryPerformanceImpacts(context.Background(), where, args)
	require.ErrorIs(t, err, wantErr)
	require.NoError(t, mock.ExpectationsWereMet())
}
