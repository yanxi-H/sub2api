package service

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func TestBuildStoredAccountUsageOpenAI(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	updatedAt := now.Add(-5 * time.Minute)
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_5h_used_percent":  34.5,
			"codex_5h_reset_at":      now.Add(2 * time.Hour).Format(time.RFC3339),
			"codex_7d_used_percent":  72.0,
			"codex_7d_reset_at":      now.Add(4 * 24 * time.Hour).Format(time.RFC3339),
			"codex_usage_updated_at": updatedAt.Format(time.RFC3339),
		},
	}

	usage := BuildStoredAccountUsage(account, now)

	require.Equal(t, "stored", usage.Source)
	require.Equal(t, 34.5, usage.FiveHour.Utilization)
	require.Equal(t, 72.0, usage.SevenDay.Utilization)
	require.NotNil(t, usage.UpdatedAt)
	require.True(t, usage.UpdatedAt.Equal(updatedAt))
}

func TestBuildStoredAccountUsageAnthropic(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	fiveHourReset := now.Add(90 * time.Minute)
	sevenDayReset := now.Add(3 * 24 * time.Hour)
	sampledAt := now.Add(-2 * time.Minute)
	account := &Account{
		Platform:         PlatformAnthropic,
		Type:             AccountTypeOAuth,
		SessionWindowEnd: &fiveHourReset,
		Extra: map[string]any{
			"session_window_utilization":   0.41,
			"passive_usage_7d_utilization": 0.67,
			"passive_usage_7d_reset":       sevenDayReset.Unix(),
			"passive_usage_sampled_at":     sampledAt.Format(time.RFC3339),
		},
	}

	usage := BuildStoredAccountUsage(account, now)

	require.Equal(t, 41.0, usage.FiveHour.Utilization)
	require.Equal(t, 67.0, usage.SevenDay.Utilization)
	require.Equal(t, int((90 * time.Minute).Seconds()), usage.FiveHour.RemainingSeconds)
	require.NotNil(t, usage.UpdatedAt)
	require.True(t, usage.UpdatedAt.Equal(sampledAt))
}

func TestSupportsLiveAccountUsageRefresh(t *testing.T) {
	t.Parallel()
	require.True(t, SupportsLiveAccountUsageRefresh(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}))
	require.True(t, SupportsLiveAccountUsageRefresh(&Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}))
	require.False(t, SupportsLiveAccountUsageRefresh(&Account{Platform: PlatformAnthropic, Type: AccountTypeSetupToken}))
	require.False(t, SupportsLiveAccountUsageRefresh(&Account{Platform: PlatformGemini, Type: AccountTypeOAuth}))
	require.True(t, SupportsLiveAccountUsageRefresh(&Account{Platform: PlatformGrok, Type: AccountTypeOAuth}))
	require.False(t, SupportsLiveAccountUsageRefresh(&Account{Platform: PlatformGrok, Type: AccountTypeAPIKey}))
}

func TestBuildStoredAccountUsageDoesNotInventMissingWindows(t *testing.T) {
	t.Parallel()
	usage := BuildStoredAccountUsage(&Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeSetupToken,
	}, time.Now())

	require.Nil(t, usage.FiveHour)
	require.Nil(t, usage.SevenDay)
}

func TestBuildStoredAccountUsageGrok(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	sevenDayReset := now.Add(2 * 24 * time.Hour)
	requestReset := now.Add(90 * time.Minute)
	usagePercent := 42.5
	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			grokBillingExtraKey: &xai.BillingSummary{
				PeriodType:   "weekly",
				UsagePercent: &usagePercent,
				PeriodEnd:    sevenDayReset.Format(time.RFC3339),
				UpdatedAt:    now.Add(-3 * time.Minute).Format(time.RFC3339),
			},
			grokQuotaSnapshotExtraKey: &xai.QuotaSnapshot{
				Requests: &xai.QuotaWindow{
					Limit:     func(v int64) *int64 { return &v }(100),
					Remaining: func(v int64) *int64 { return &v }(25),
					ResetAt:   requestReset.Format(time.RFC3339),
				},
				StatusCode: 200,
				UpdatedAt:  now.Add(-3 * time.Minute).Format(time.RFC3339),
			},
		},
	}

	usage := BuildStoredAccountUsage(account, now)

	require.Equal(t, "stored", usage.Source)
	require.NotNil(t, usage.FiveHour)
	require.InDelta(t, 75.0, usage.FiveHour.Utilization, 1e-9)
	require.Equal(t, int((90 * time.Minute).Seconds()), usage.FiveHour.RemainingSeconds)
	require.NotNil(t, usage.SevenDay)
	require.InDelta(t, 42.5, usage.SevenDay.Utilization, 1e-9)
	require.NotNil(t, usage.SevenDay.ResetsAt)
	require.True(t, usage.SevenDay.ResetsAt.Equal(sevenDayReset))
}

func TestBuildStoredAccountUsageGrokDoesNotInventMissingWindows(t *testing.T) {
	t.Parallel()
	usage := BuildStoredAccountUsage(&Account{
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
	}, time.Now())

	require.Equal(t, "stored", usage.Source)
	require.Nil(t, usage.FiveHour)
	require.Nil(t, usage.SevenDay)
}

func TestAccountSevenDayResetAtGrokWeekly(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	resetAt := now.Add(48 * time.Hour)
	usagePercent := 12.0

	got, ok := accountSevenDayResetAt(&Account{
		Platform: PlatformGrok,
		Extra: map[string]any{
			grokBillingExtraKey: &xai.BillingSummary{
				PeriodType:   "weekly",
				UsagePercent: &usagePercent,
				PeriodEnd:    resetAt.Format(time.RFC3339),
			},
		},
	}, now)
	require.True(t, ok)
	require.True(t, got.Equal(resetAt))

	_, ok = accountSevenDayResetAt(&Account{
		Platform: PlatformGrok,
		Extra: map[string]any{
			grokBillingExtraKey: &xai.BillingSummary{
				PeriodType: "monthly",
				PeriodEnd:  resetAt.Format(time.RFC3339),
			},
		},
	}, now)
	require.False(t, ok)
}
