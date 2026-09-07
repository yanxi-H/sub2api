package repository

import (
	"context"
	"testing"
	"time"

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

func TestResolveMissingMonitorCenterIncidentsKeepsActiveIDs(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}
	observedAt := time.Date(2026, 7, 27, 1, 2, 0, 0, time.UTC)
	mock.ExpectExec(`(?s)UPDATE monitor_center_openai_incidents.*jsonb_array_elements_text`).
		WithArgs(observedAt, `["incident-active"]`).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.ResolveMissingMonitorCenterIncidents(context.Background(), observedAt, []string{"incident-active"})
	require.NoError(t, err)
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
