package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGetUserErrorDistributionGroupsRowsAndPreservesUnknownUser(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}
	start := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	rows := sqlmock.NewRows([]string{
		"user_id", "username", "email", "deleted", "total", "error_type", "count", "total_users", "overall_total",
	}).
		AddRow(int64(7), "alice", "alice@example.com", false, int64(8), "upstream_api", int64(6), 2, int64(11)).
		AddRow(int64(7), "alice", "alice@example.com", false, int64(8), "other", int64(2), 2, int64(11)).
		AddRow(nil, "", "", false, int64(3), "unknown", int64(3), 2, int64(11))

	mock.ExpectQuery(`WITH filtered AS`).
		WithArgs(start, end).
		WillReturnRows(rows)

	response, err := repo.GetUserErrorDistribution(context.Background(), &service.OpsDashboardFilter{
		StartTime: start,
		EndTime:   end,
	})
	require.NoError(t, err)
	require.Equal(t, int64(11), response.Total)
	require.Equal(t, 2, response.TotalUsers)
	require.Equal(t, opsUserErrorLimit, response.UserLimit)
	require.Len(t, response.Items, 2)
	require.NotNil(t, response.Items[0].UserID)
	require.EqualValues(t, 7, *response.Items[0].UserID)
	require.Equal(t, "alice", response.Items[0].Username)
	require.Len(t, response.Items[0].Errors, 2)
	require.Nil(t, response.Items[1].UserID)
	require.Equal(t, "unknown", response.Items[1].Errors[0].ErrorType)
	require.NoError(t, mock.ExpectationsWereMet())
}
