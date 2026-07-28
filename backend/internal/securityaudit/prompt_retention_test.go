package securityaudit

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestPromptRetentionCleanupPurgesOnlyFullPrompt(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	cutoff := now.AddDate(0, 0, -45)
	mock.ExpectExec(`UPDATE prompt_audit_events e\s+SET full_prompt = ''`).
		WithArgs(cutoff, retentionCleanupBatchSize).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectClose()

	service := &PromptService{
		config:  &fakeConfigStore{cfg: ActiveConfig{RetentionDays: 45}, active: true},
		repo:    NewPostgreSQLRepository(db),
		payload: NewRedisPayloadStore(nil),
		clock:   fixedClock{now: now},
	}
	require.False(t, service.cleanupExpiredPrompts(context.Background()))
	require.NoError(t, db.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPromptRetentionCleanupContinuesPastFormerBatchCap(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	cutoff := now.AddDate(0, 0, -DefaultRetentionDays)
	for range 12 {
		mock.ExpectExec(`UPDATE prompt_audit_events e\s+SET full_prompt = ''`).
			WithArgs(cutoff, retentionCleanupBatchSize).
			WillReturnResult(sqlmock.NewResult(0, retentionCleanupBatchSize))
	}
	mock.ExpectExec(`UPDATE prompt_audit_events e\s+SET full_prompt = ''`).
		WithArgs(cutoff, retentionCleanupBatchSize).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectClose()

	service := &PromptService{
		config: &fakeConfigStore{cfg: ActiveConfig{RetentionDays: DefaultRetentionDays}, active: true},
		repo:   NewPostgreSQLRepository(db),
		clock:  fixedClock{now: now},
	}
	require.False(t, service.cleanupExpiredPrompts(context.Background()))
	require.NoError(t, db.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}
