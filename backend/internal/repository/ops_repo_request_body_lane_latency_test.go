package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestGetRequestBodyLaneLatencySummaries(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}
	start := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	rows := sqlmock.NewRows([]string{"request_body_lane", "p50", "p90", "p95", "avg", "max"}).
		AddRow("normal", 110.4, 310.2, 410.8, 180.6, int64(700)).
		AddRow("heavy", 1200.0, 2100.0, 2600.0, 1500.0, int64(3200)).
		AddRow("recovery", 3500.0, 4800.0, 5100.0, 3900.0, int64(6200))
	mock.ExpectQuery(`(?s)SELECT\s+request_body_lane,.*FROM ops_request_performance.*logical_status_code >= 200.*logical_status_code < 400.*GROUP BY request_body_lane`).
		WithArgs(start, end).
		WillReturnRows(rows)

	result, err := repo.GetRequestBodyLaneLatencySummaries(context.Background(), start, end)
	require.NoError(t, err)
	require.Equal(t, 411, *result.Normal.P95)
	require.Equal(t, 310, *result.Normal.P90)
	require.Equal(t, 110, *result.Normal.P50)
	require.Equal(t, 181, *result.Normal.Avg)
	require.Equal(t, 700, *result.Normal.Max)
	require.Equal(t, 2600, *result.Heavy.P95)
	require.Equal(t, 6200, *result.Recovery.Max)
	require.NoError(t, mock.ExpectationsWereMet())
}
