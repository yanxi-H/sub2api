package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
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

func TestBuildStoredAccountUsageZhipuCodingPlan(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 2, 22, 0, 0, 0, time.UTC)
	updatedAt := now.Add(-3 * time.Minute)
	account := &Account{
		Platform: PlatformZhipu,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":     "https://open.bigmodel.cn/api/coding/paas/v4",
			"account_mode": AccountModeCoding,
		},
		Extra: map[string]any{
			"zhipu_5h_used_percent":     46.5,
			"zhipu_5h_reset_at":         now.Add(2 * time.Hour).Format(time.RFC3339),
			"zhipu_weekly_used_percent": 18.0,
			"zhipu_weekly_reset_at":     now.Add(4 * 24 * time.Hour).Format(time.RFC3339),
			"zhipu_usage_updated_at":    updatedAt.Format(time.RFC3339),
			"zhipu_plan_level":          "pro",
		},
	}

	usage := BuildStoredAccountUsage(account, now)

	require.Equal(t, "stored", usage.Source)
	require.NotNil(t, usage.FiveHour)
	require.Equal(t, 46.5, usage.FiveHour.Utilization)
	require.NotNil(t, usage.FiveHour.ResetsAt)
	require.Equal(t, int((2 * time.Hour).Seconds()), usage.FiveHour.RemainingSeconds)
	require.NotNil(t, usage.SevenDay)
	require.Equal(t, 18.0, usage.SevenDay.Utilization)
	require.Equal(t, int((4 * 24 * time.Hour).Seconds()), usage.SevenDay.RemainingSeconds)
	require.NotNil(t, usage.UpdatedAt)
	require.True(t, usage.UpdatedAt.Equal(updatedAt))
	require.NotNil(t, usage.FixedQuotaCapacity)
	require.Equal(t, "zhipu_glm_pro", usage.FixedQuotaCapacity.Source)
	require.Equal(t, 120.0, usage.FixedQuotaCapacity.FiveHourUSD)
	require.Equal(t, 600.0, usage.FixedQuotaCapacity.SevenDayUSD)
}

func TestCNFixedQuotaCapacityRecognizesGLMProAliases(t *testing.T) {
	t.Parallel()

	for _, planLevel := range []string{"pro", "GLM-Pro", " PRO "} {
		capacity := cnFixedQuotaCapacity(PlatformZhipu, planLevel)
		require.NotNil(t, capacity, planLevel)
		require.Equal(t, 120.0, capacity.FiveHourUSD)
		require.Equal(t, 600.0, capacity.SevenDayUSD)
	}
	require.Nil(t, cnFixedQuotaCapacity(PlatformZhipu, "max"))
	require.Nil(t, cnFixedQuotaCapacity(PlatformKimi, "pro"))
}

func TestBuildStoredAccountUsageZhipuCodingPlanDoesNotInventMissingWindows(t *testing.T) {
	t.Parallel()
	usage := BuildStoredAccountUsage(&Account{
		Platform: PlatformZhipu,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":     "https://open.bigmodel.cn/api/coding/paas/v4",
			"account_mode": AccountModeCoding,
		},
	}, time.Now())

	require.Equal(t, "stored", usage.Source)
	require.Nil(t, usage.FiveHour)
	require.Nil(t, usage.SevenDay)
}

func TestBuildStoredAccountUsageZhipuZeroUsageWithResetStillShows(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 2, 22, 0, 0, 0, time.UTC)
	usage := BuildStoredAccountUsage(&Account{
		Platform: PlatformZhipu,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":     "https://open.bigmodel.cn/api/coding/paas/v4",
			"account_mode": AccountModeCoding,
		},
		Extra: map[string]any{
			"zhipu_5h_used_percent": 0.0,
			"zhipu_5h_reset_at":     now.Add(time.Hour).Format(time.RFC3339),
		},
	}, now)

	require.NotNil(t, usage.FiveHour)
	require.Equal(t, 0.0, usage.FiveHour.Utilization)
	require.Nil(t, usage.SevenDay)
}

func TestSupportsLiveAccountUsageRefreshCNProviders(t *testing.T) {
	t.Parallel()
	zhipuCoding := &Account{
		Platform: PlatformZhipu,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":     "https://open.bigmodel.cn/api/coding/paas/v4",
			"account_mode": AccountModeCoding,
		},
	}
	require.True(t, SupportsLiveAccountUsageRefresh(zhipuCoding))

	kimiCoding := &Account{
		Platform: PlatformKimi,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":     "https://api.kimi.com/coding/v1",
			"account_mode": AccountModeCoding,
		},
	}
	require.True(t, SupportsLiveAccountUsageRefresh(kimiCoding))

	// payg 模式没有 coding plan 额度端点。
	zhipuPayg := &Account{
		Platform: PlatformZhipu,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":     "https://open.bigmodel.cn/api/paas/v4",
			"account_mode": AccountModePayG,
		},
	}
	require.False(t, SupportsLiveAccountUsageRefresh(zhipuPayg))

	// deepseek 无官方 coding plan 额度端点。
	deepseekCoding := &Account{
		Platform: PlatformDeepseek,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":     "https://api.deepseek.com",
			"account_mode": AccountModeCoding,
		},
	}
	require.False(t, SupportsLiveAccountUsageRefresh(deepseekCoding))
}

func TestAccountSevenDayResetAtZhipuWeekly(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 2, 22, 0, 0, 0, time.UTC)
	resetAt := now.Add(3 * 24 * time.Hour)

	got, ok := accountSevenDayResetAt(&Account{
		Platform: PlatformZhipu,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":     "https://open.bigmodel.cn/api/coding/paas/v4",
			"account_mode": AccountModeCoding,
		},
		Extra: map[string]any{
			"zhipu_weekly_reset_at": resetAt.Format(time.RFC3339),
		},
	}, now)
	require.True(t, ok)
	require.True(t, got.Equal(resetAt))

	// 过期的重置时间不作为窗口来源。
	_, ok = accountSevenDayResetAt(&Account{
		Platform: PlatformZhipu,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":     "https://open.bigmodel.cn/api/coding/paas/v4",
			"account_mode": AccountModeCoding,
		},
		Extra: map[string]any{
			"zhipu_weekly_reset_at": now.Add(-time.Hour).Format(time.RFC3339),
		},
	}, now)
	require.False(t, ok)
}

func TestBuildStoredAccountUsageZhipuExpiredWindowZeroesUtilization(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 2, 22, 0, 0, 0, time.UTC)
	usage := BuildStoredAccountUsage(&Account{
		Platform: PlatformZhipu,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":     "https://open.bigmodel.cn/api/coding/paas/v4",
			"account_mode": AccountModeCoding,
		},
		Extra: map[string]any{
			"zhipu_5h_used_percent": 46.5,
			"zhipu_5h_reset_at":     now.Add(-time.Hour).Format(time.RFC3339),
		},
	}, now)

	// 探测未及时刷新时，过期窗口不得继续显示重置前的高用量（对齐 Codex 语义）。
	require.NotNil(t, usage.FiveHour)
	require.Equal(t, 0.0, usage.FiveHour.Utilization)
	require.NotNil(t, usage.FiveHour.ResetsAt)
	require.Equal(t, 0, usage.FiveHour.RemainingSeconds)
}

// stubCNQuotaUpstream 返回固定额度响应，供 live refresh 链路测试。
type stubCNQuotaUpstream struct {
	status int
	body   string
	calls  int
}

func (u *stubCNQuotaUpstream) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	u.calls++
	return &http.Response{
		StatusCode: u.status,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(u.body)),
	}, nil
}

func (u *stubCNQuotaUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

type stubCNQuotaAccountRepo struct {
	AccountRepository
	extraUpdates map[string]any
}

func (r *stubCNQuotaAccountRepo) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	r.extraUpdates = updates
	return nil
}

func zhipuCodingQuotaAccount() *Account {
	return &Account{
		ID:       6,
		Platform: PlatformZhipu,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"base_url":     "https://open.bigmodel.cn/api/coding/paas/v4",
			"account_mode": AccountModeCoding,
			"api_key":      "sk-test",
		},
	}
}

func zhipuQuotaBody(tiers ...string) string {
	limitItems := make([]string, 0, len(tiers))
	for i, tier := range tiers {
		limitItems = append(limitItems, fmt.Sprintf(
			`{"type":"TOKENS_LIMIT","unit":%s,"percentage":%s,"nextResetTime":%d}`,
			tier, tier, time.Now().Add(time.Duration(i+1)*time.Hour).UnixMilli(),
		))
	}
	return fmt.Sprintf(`{"success":true,"data":{"level":"pro","limits":[%s]}}`, strings.Join(limitItems, ","))
}

func TestGetCNQuotaUsageLiveRefreshResponses(t *testing.T) {
	t.Parallel()

	t.Run("empty tiers fail instead of dressing stale values as fresh", func(t *testing.T) {
		t.Parallel()
		upstream := &stubCNQuotaUpstream{status: http.StatusOK, body: `{"success":true,"data":{"limits":[]}}`}
		svc := &AccountUsageService{cnQuotaService: NewCNProviderQuotaService(
			&stubCNQuotaAccountRepo{}, nil, upstream, nil,
		)}

		usage, err := svc.getCNQuotaUsage(context.Background(), zhipuCodingQuotaAccount())

		require.Error(t, err)
		require.Contains(t, err.Error(), "no usage tiers")
		require.Nil(t, usage)
		require.Equal(t, 1, upstream.calls)
	})

	t.Run("partial tiers only build the windows actually returned", func(t *testing.T) {
		t.Parallel()
		upstream := &stubCNQuotaUpstream{status: http.StatusOK, body: zhipuQuotaBody("3")}
		svc := &AccountUsageService{cnQuotaService: NewCNProviderQuotaService(
			&stubCNQuotaAccountRepo{}, nil, upstream, nil,
		)}

		usage, err := svc.getCNQuotaUsage(context.Background(), zhipuCodingQuotaAccount())

		require.NoError(t, err)
		require.NotNil(t, usage.FiveHour)
		require.Nil(t, usage.SevenDay)
		require.NotNil(t, usage.UpdatedAt)
		// 旧快照残留的 weekly 值不得出现在本轮结果里。
		require.Equal(t, "active", usage.Source)
	})

	t.Run("full tiers build both windows", func(t *testing.T) {
		t.Parallel()
		upstream := &stubCNQuotaUpstream{status: http.StatusOK, body: zhipuQuotaBody("3", "6")}
		repo := &stubCNQuotaAccountRepo{}
		svc := &AccountUsageService{cnQuotaService: NewCNProviderQuotaService(
			repo, nil, upstream, nil,
		)}

		usage, err := svc.getCNQuotaUsage(context.Background(), zhipuCodingQuotaAccount())

		require.NoError(t, err)
		require.NotNil(t, usage.FiveHour)
		require.NotNil(t, usage.SevenDay)
		require.Greater(t, usage.FiveHour.RemainingSeconds, 0)
		require.Greater(t, usage.SevenDay.RemainingSeconds, 0)
		require.NotNil(t, usage.FixedQuotaCapacity)
		require.Equal(t, 600.0, usage.FixedQuotaCapacity.SevenDayUSD)
		require.Equal(t, "pro", repo.extraUpdates["zhipu_plan_level"])
	})
}
