package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIResetCreditSnapshotFromExtra(t *testing.T) {
	checkedAt := time.Now().UTC().Truncate(time.Second)
	snapshot := OpenAIResetCreditSnapshotFromExtra(map[string]any{
		OpenAIResetCreditSnapshotExtraKey: map[string]any{
			"available_count": 2,
			"credits":         []map[string]any{{"expires_at": "2026-08-01T00:00:00Z"}},
			"checked_at":      checkedAt.Format(time.RFC3339),
		},
	})
	require.NotNil(t, snapshot)
	require.Equal(t, 2, snapshot.AvailableCount)
	require.Equal(t, "2026-08-01T00:00:00Z", snapshot.Credits[0].ExpiresAt)
	require.Equal(t, checkedAt, snapshot.CheckedAt)
}

func TestOpenAIResetCreditSnapshotFromExtraRejectsInvalidValues(t *testing.T) {
	require.Nil(t, OpenAIResetCreditSnapshotFromExtra(map[string]any{
		OpenAIResetCreditSnapshotExtraKey: map[string]any{"available_count": -1, "checked_at": time.Now().UTC().Format(time.RFC3339)},
	}))
	require.Nil(t, OpenAIResetCreditSnapshotFromExtra(map[string]any{
		OpenAIResetCreditSnapshotExtraKey: map[string]any{"available_count": 1},
	}))
}

func TestResetCreditRefreshesStoredSnapshot(t *testing.T) {
	account := &Account{
		ID:       100,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"chatgpt_account_id": "org-test",
		},
	}
	repo := &stubQuotaAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	tokenProvider := NewOpenAITokenProvider(repo, &stubQuotaTokenCache{
		tokens: map[string]string{OpenAITokenCacheKey(account): "test-token"},
	}, nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		switch r.URL.Path {
		case "/backend-api/wham/rate-limit-reset-credits/consume":
			_, _ = w.Write([]byte(`{"code":"ok","windows_reset":2}`))
		case "/backend-api/wham/rate-limit-reset-credits":
			_, _ = w.Write([]byte(`{"available_count":1,"credits":[{"expires_at":"2026-08-01T00:00:00Z"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	svc := NewOpenAIQuotaService(repo, nil, tokenProvider, newQuotaRedirectingFactory(srv))
	_, err := svc.ResetCredit(context.Background(), account.ID)
	require.NoError(t, err)

	snapshot := OpenAIResetCreditSnapshotFromExtra(account.Extra)
	require.NotNil(t, snapshot)
	require.Equal(t, 1, snapshot.AvailableCount)
	require.Equal(t, "2026-08-01T00:00:00Z", snapshot.Credits[0].ExpiresAt)
}
