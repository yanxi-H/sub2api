package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestListHistoryRangeUsesBoundedWorstStatusBuckets(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	start := time.Date(2026, 7, 27, 8, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	mock.ExpectQuery("WITH ranked AS").
		WithArgs(int64(7), start, end, "gpt-test", int64(60)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "model", "status", "latency_ms", "ping_latency_ms", "message", "checked_at"}).
			AddRow(int64(11), "gpt-test", "failed", 500, nil, "timeout", start.Add(30*time.Second)))

	repo := &channelMonitorRepository{db: db}
	entries, err := repo.ListHistoryRange(context.Background(), 7, "gpt-test", start, end, time.Minute)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "failed", entries[0].Status)
	require.Equal(t, "timeout", entries[0].Message)
	require.NoError(t, mock.ExpectationsWereMet())
}
