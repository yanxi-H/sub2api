package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestGetAccountWindowStatsByStartsUsesOneQuery(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	secondStart := time.Date(2026, 7, 9, 1, 0, 0, 0, time.UTC)
	ninthStart := time.Date(2026, 7, 10, 2, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)WITH windows\(account_id, start_time\) AS \(VALUES.*LEFT JOIN usage_logs`).
		WithArgs(int64(2), secondStart, int64(9), ninthStart).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "requests", "tokens", "cost", "standard_cost", "user_cost"}).
			AddRow(int64(2), int64(3), int64(400), 8.0, 7.0, 9.0).
			AddRow(int64(9), int64(1), int64(100), 2.0, 1.5, 2.5))

	repo := &usageLogRepository{sql: db}
	stats, err := repo.GetAccountWindowStatsByStarts(context.Background(), map[int64]time.Time{
		9: ninthStart,
		2: secondStart,
	})
	require.NoError(t, err)
	require.InDelta(t, 9, stats[2].UserCost, 1e-9)
	require.InDelta(t, 2.5, stats[9].UserCost, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}
