package service

import (
	"testing"
	"time"

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
