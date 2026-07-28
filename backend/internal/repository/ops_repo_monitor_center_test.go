package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestLoadLatestMonitorCenterOpenAIEventHash(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}
	mock.ExpectQuery(`(?s)SELECT content_hash.*ORDER BY observed_at DESC, id DESC.*LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"content_hash"}).AddRow("stable-hash"))

	hash, err := repo.LoadLatestMonitorCenterOpenAIEventHash(context.Background())
	require.NoError(t, err)
	require.Equal(t, "stable-hash", hash)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLoadLatestMonitorCenterOpenAIEventHashAllowsEmptyTable(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}
	mock.ExpectQuery(`(?s)SELECT content_hash.*LIMIT 1`).WillReturnRows(sqlmock.NewRows([]string{"content_hash"}))

	hash, err := repo.LoadLatestMonitorCenterOpenAIEventHash(context.Background())
	require.NoError(t, err)
	require.Empty(t, hash)
	require.NoError(t, mock.ExpectationsWereMet())
}
