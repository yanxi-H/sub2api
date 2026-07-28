package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSumActive7dRateLimitsByGroupIDs(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newAPIKeyRepositoryWithSQL(nil, db)
	mock.ExpectQuery(`(?s)SELECT COALESCE\(group_id, 0\).*FROM api_keys`).
		WithArgs("active", sqlmock.AnyArg(), true).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "allocated", "unlimited"}).
			AddRow(int64(0), 20.0, true).
			AddRow(int64(3), 75.5, false))

	allocated, err := repo.SumActive7dRateLimitsByGroupIDs(context.Background(), []int64{3}, true)
	require.NoError(t, err)
	require.Equal(t, map[int64]service.APIKey7dAllocation{
		0: {AllocatedUSD: 20, Unlimited: true},
		3: {AllocatedUSD: 75.5},
	}, allocated)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSumActive7dRateLimitsByGroupIDsSkipsEmptyQuery(t *testing.T) {
	repo := newAPIKeyRepositoryWithSQL(nil, nil)
	allocated, err := repo.SumActive7dRateLimitsByGroupIDs(context.Background(), nil, false)
	require.NoError(t, err)
	require.Empty(t, allocated)
}
