package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupAccountListRouter() (*gin.Engine, *stubAdminService) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adminSvc := newStubAdminService()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router.GET("/api/v1/admin/accounts", handler.List)
	router.GET("/api/v1/admin/accounts/usage-windows", handler.ListUsageWindows)
	router.POST("/api/v1/admin/accounts/usage-windows/refresh", handler.RefreshUsageWindows)
	router.POST("/api/v1/admin/accounts/usage-windows/openai-reset-credits/refresh", handler.RefreshOpenAIResetCredits)
	return router, adminSvc
}

func TestAccountHandlerListUsageWindowsUsesStoredSnapshots(t *testing.T) {
	router, adminSvc := setupAccountListRouter()
	now := time.Now().UTC()
	adminSvc.accounts = []service.Account{{
		ID:       81,
		Name:     "codex-primary",
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Status:   service.StatusActive,
		Extra: map[string]any{
			"codex_5h_used_percent": 25.0,
			"codex_5h_reset_at":     now.Add(time.Hour).Format(time.RFC3339),
			"codex_7d_used_percent": 60.0,
			"codex_7d_reset_at":     now.Add(24 * time.Hour).Format(time.RFC3339),
			"openai_rate_limit_reset_credits": map[string]any{
				"available_count": 3,
				"credits":         []map[string]any{{"expires_at": now.Add(48 * time.Hour).Format(time.RFC3339)}},
				"checked_at":      now.Format(time.RFC3339),
			},
		},
	}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/usage-windows?page=1&page_size=10&search=codex", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "codex", adminSvc.lastListAccounts.search)
	require.Equal(t, "name", adminSvc.lastListAccounts.sortBy)

	var payload struct {
		Data struct {
			Items []AccountUsageWindowItem `json:"items"`
			Total int64                    `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, int64(1), payload.Data.Total)
	require.Len(t, payload.Data.Items, 1)
	require.Equal(t, "codex-primary", payload.Data.Items[0].Name)
	require.Equal(t, 25.0, payload.Data.Items[0].FiveHour.Utilization)
	require.Equal(t, 60.0, payload.Data.Items[0].SevenDay.Utilization)
	require.True(t, payload.Data.Items[0].SupportsLiveRefresh)
	require.True(t, payload.Data.Items[0].SupportsOpenAIResetCredits)
	require.NotNil(t, payload.Data.Items[0].OpenAIResetCredits)
	require.Equal(t, 3, payload.Data.Items[0].OpenAIResetCredits.AvailableCount)
}

func TestAccountHandlerListUsageWindowsDoesNotExposeResetCreditsForShadow(t *testing.T) {
	router, adminSvc := setupAccountListRouter()
	parentID := int64(81)
	adminSvc.accounts = []service.Account{{
		ID:              82,
		Name:            "codex-shadow",
		Platform:        service.PlatformOpenAI,
		Type:            service.AccountTypeOAuth,
		Status:          service.StatusActive,
		ParentAccountID: &parentID,
		Extra: map[string]any{
			"openai_rate_limit_reset_credits": map[string]any{"available_count": 9, "checked_at": time.Now().UTC().Format(time.RFC3339)},
		},
	}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/usage-windows?page=1&page_size=10", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var payload struct {
		Data struct {
			Items []AccountUsageWindowItem `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Items, 1)
	require.False(t, payload.Data.Items[0].SupportsOpenAIResetCredits)
	require.Nil(t, payload.Data.Items[0].OpenAIResetCredits)
}

func TestAccountHandlerRefreshUsageWindowsRejectsOversizedBatch(t *testing.T) {
	router, _ := setupAccountListRouter()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/usage-windows/refresh", strings.NewReader(`{"account_ids":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAccountHandlerRefreshOpenAIResetCreditsRejectsOversizedBatch(t *testing.T) {
	router, _ := setupAccountListRouter()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/usage-windows/openai-reset-credits/refresh", strings.NewReader(`{"account_ids":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAccountHandlerListUsageWindowsKeepsSnapshotsWhenAllocationFails(t *testing.T) {
	router, adminSvc := setupAccountListRouter()
	now := time.Now().UTC()
	adminSvc.apiKey7dAllocationErr = errors.New("allocation unavailable")
	adminSvc.accounts = []service.Account{{
		ID:       82,
		Name:     "codex-fallback",
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Status:   service.StatusActive,
		Extra: map[string]any{
			"codex_7d_used_percent": 55.0,
			"codex_7d_reset_at":     now.Add(24 * time.Hour).Format(time.RFC3339),
		},
	}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/usage-windows?page=1&page_size=10", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var payload struct {
		Data struct {
			Items []AccountUsageWindowItem `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Items, 1)
	require.Equal(t, 55.0, payload.Data.Items[0].SevenDay.Utilization)
}

func TestBuildSevenDayQuotaCapacity(t *testing.T) {
	capacity := buildSevenDayQuotaCapacity(&service.UsageProgress{
		Utilization: 40,
		WindowStats: &service.WindowStats{Cost: 999, UserCost: 32},
	}, &service.APIKey7dAllocation{AllocatedUSD: 60})

	require.NotNil(t, capacity)
	require.InDelta(t, 80, capacity.EstimatedTotalUSD, 1e-9)
	require.InDelta(t, 32, capacity.ActualUsedUSD, 1e-9)
	require.InDelta(t, 48, capacity.ActualRemainingUSD, 1e-9)
	require.InDelta(t, 60, *capacity.AllocatedUSD, 1e-9)
	require.InDelta(t, 20, *capacity.UnallocatedRemainingUSD, 1e-9)
	require.InDelta(t, 60, capacity.ActualRemainingPercent, 1e-9)
	require.InDelta(t, 25, *capacity.UnallocatedRemainingPercent, 1e-9)
}

func TestBuildSevenDayQuotaCapacityClampsOverageAndRejectsUnknownEstimate(t *testing.T) {
	overallocated := buildSevenDayQuotaCapacity(&service.UsageProgress{
		Utilization: 120,
		WindowStats: &service.WindowStats{UserCost: 120},
	}, &service.APIKey7dAllocation{AllocatedUSD: 150})
	require.NotNil(t, overallocated)
	require.Zero(t, overallocated.ActualRemainingUSD)
	require.Zero(t, *overallocated.UnallocatedRemainingUSD)
	require.Zero(t, overallocated.ActualRemainingPercent)
	require.Zero(t, *overallocated.UnallocatedRemainingPercent)

	unlimited := buildSevenDayQuotaCapacity(&service.UsageProgress{
		Utilization: 50,
		WindowStats: &service.WindowStats{UserCost: 50},
	}, &service.APIKey7dAllocation{Unlimited: true})
	require.NotNil(t, unlimited)
	require.True(t, unlimited.AllocationUnlimited)
	require.Zero(t, *unlimited.UnallocatedRemainingUSD)
	require.Zero(t, *unlimited.UnallocatedRemainingPercent)

	allocationUnavailable := buildSevenDayQuotaCapacity(&service.UsageProgress{
		Utilization: 50,
		WindowStats: &service.WindowStats{UserCost: 50},
	}, nil)
	require.NotNil(t, allocationUnavailable)
	require.Nil(t, allocationUnavailable.AllocatedUSD)
	require.Nil(t, allocationUnavailable.UnallocatedRemainingUSD)

	require.Nil(t, buildSevenDayQuotaCapacity(&service.UsageProgress{
		Utilization: 0,
		WindowStats: &service.WindowStats{UserCost: 10},
	}, &service.APIKey7dAllocation{AllocatedUSD: 5}))
	require.Nil(t, buildSevenDayQuotaCapacity(&service.UsageProgress{
		Utilization: 25,
		WindowStats: &service.WindowStats{UserCost: 0},
	}, &service.APIKey7dAllocation{AllocatedUSD: 5}))
}

func TestSevenDayWindowStartUsesCurrentUpstreamWindow(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	resetAt := now.Add(36 * time.Hour)
	require.Equal(t, resetAt.Add(-7*24*time.Hour), sevenDayWindowStart(&service.UsageProgress{ResetsAt: &resetAt}, now))

	expiredReset := now.Add(-time.Hour)
	require.Equal(t, now.Add(-7*24*time.Hour), sevenDayWindowStart(&service.UsageProgress{ResetsAt: &expiredReset}, now))
}

func TestAccountHandlerListIncludesCreatedAt(t *testing.T) {
	router, adminSvc := setupAccountListRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts?page=1&page_size=20&sort_by=created_at&sort_order=desc", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "created_at", adminSvc.lastListAccounts.sortBy)

	var payload struct {
		Data struct {
			Items []struct {
				ID        int64  `json:"id"`
				CreatedAt string `json:"created_at"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Items, 1)

	createdAt := payload.Data.Items[0].CreatedAt
	require.NotEmpty(t, createdAt)
	require.True(t, strings.HasSuffix(createdAt, "Z"), "created_at should be serialized as UTC")
	parsed, err := time.Parse(time.RFC3339Nano, createdAt)
	require.NoError(t, err)
	_, offset := parsed.Zone()
	require.Equal(t, 0, offset)
}

func TestAccountHandlerListReturnsSchedulerScoresPerGroup(t *testing.T) {
	router, adminSvc := setupAccountListRouter()
	now := time.Now().UTC()
	groupID := int64(41)
	adminSvc.accounts = []service.Account{
		{
			ID:          101,
			Name:        "account-high-priority",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeAPIKey,
			Status:      service.StatusActive,
			Schedulable: true,
			Concurrency: 10,
			Priority:    1,
			AccountGroups: []service.AccountGroup{
				{AccountID: 101, GroupID: groupID, Priority: 100, Group: &service.Group{ID: groupID, Name: "openai"}},
			},
			GroupIDs:  []int64{groupID},
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:          102,
			Name:        "account-low-priority",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeAPIKey,
			Status:      service.StatusActive,
			Schedulable: true,
			Concurrency: 10,
			Priority:    100000,
			AccountGroups: []service.AccountGroup{
				{AccountID: 102, GroupID: groupID, Priority: 1, Group: &service.Group{ID: groupID, Name: "openai"}},
			},
			GroupIDs:  []int64{groupID},
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts?page=1&page_size=20&platform=openai&include_scheduler_score=1", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var payload struct {
		Data struct {
			Items []struct {
				ID             int64 `json:"id"`
				SchedulerScore struct {
					BaseScore float64 `json:"base_score"`
				} `json:"scheduler_score"`
				SchedulerScores []struct {
					GroupID       *int64  `json:"group_id"`
					GroupName     string  `json:"group_name"`
					GroupPriority *int    `json:"group_priority"`
					BaseScore     float64 `json:"base_score"`
				} `json:"scheduler_scores"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Items, 2)

	var high, low *struct {
		ID             int64 `json:"id"`
		SchedulerScore struct {
			BaseScore float64 `json:"base_score"`
		} `json:"scheduler_score"`
		SchedulerScores []struct {
			GroupID       *int64  `json:"group_id"`
			GroupName     string  `json:"group_name"`
			GroupPriority *int    `json:"group_priority"`
			BaseScore     float64 `json:"base_score"`
		} `json:"scheduler_scores"`
	}
	for i := range payload.Data.Items {
		item := &payload.Data.Items[i]
		switch item.ID {
		case 101:
			high = item
		case 102:
			low = item
		}
	}
	require.NotNil(t, high)
	require.NotNil(t, low)
	require.Len(t, high.SchedulerScores, 1)
	require.Len(t, low.SchedulerScores, 1)
	require.Equal(t, groupID, *high.SchedulerScores[0].GroupID)
	require.Equal(t, "openai", high.SchedulerScores[0].GroupName)
	require.Equal(t, 100, *high.SchedulerScores[0].GroupPriority)
	require.Equal(t, 1, *low.SchedulerScores[0].GroupPriority)
	require.Greater(t, high.SchedulerScores[0].BaseScore, low.SchedulerScores[0].BaseScore)
}

func TestAccountHandlerListSkipsSchedulerScoresByDefault(t *testing.T) {
	router, adminSvc := setupAccountListRouter()
	now := time.Now().UTC()
	adminSvc.accounts = []service.Account{
		{
			ID:          110,
			Name:        "openai-account",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeAPIKey,
			Status:      service.StatusActive,
			Schedulable: true,
			Concurrency: 10,
			Priority:    1,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts?page=1&page_size=20&platform=openai", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Zero(t, adminSvc.schedulerScoreFilterCalls)
	require.Zero(t, adminSvc.openAISchedulerScorePoolCalls)

	var payload struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Items, 1)
	require.NotContains(t, payload.Data.Items[0], "scheduler_score")
	require.NotContains(t, payload.Data.Items[0], "scheduler_scores")
}

func TestAccountHandlerListKeepsSchedulerScoreScopedToFilter(t *testing.T) {
	router, adminSvc := setupAccountListRouter()
	now := time.Now().UTC()
	groupID := int64(42)
	visibleAccount := service.Account{
		ID:          201,
		Name:        "visible-low-priority",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 10,
		Priority:    100000,
		AccountGroups: []service.AccountGroup{
			{AccountID: 201, GroupID: groupID, Priority: 1, Group: &service.Group{ID: groupID, Name: "openai"}},
		},
		GroupIDs:  []int64{groupID},
		CreatedAt: now,
		UpdatedAt: now,
	}
	hiddenGroupPeer := service.Account{
		ID:          202,
		Name:        "hidden-high-priority",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 10,
		Priority:    1,
		AccountGroups: []service.AccountGroup{
			{AccountID: 202, GroupID: groupID, Priority: 2, Group: &service.Group{ID: groupID, Name: "openai"}},
		},
		GroupIDs:  []int64{groupID},
		CreatedAt: now,
		UpdatedAt: now,
	}
	adminSvc.accounts = []service.Account{visibleAccount}
	adminSvc.accountSchedulerScoreFilterAccounts = []service.Account{visibleAccount, hiddenGroupPeer}
	adminSvc.openAISchedulerScorePoolAccounts = []service.Account{visibleAccount, hiddenGroupPeer}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts?page=1&page_size=1&platform=openai&include_scheduler_score=1", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var payload struct {
		Data struct {
			Items []struct {
				ID             int64 `json:"id"`
				SchedulerScore struct {
					BaseScore float64 `json:"base_score"`
				} `json:"scheduler_score"`
				SchedulerScores []struct {
					GroupID   *int64  `json:"group_id"`
					BaseScore float64 `json:"base_score"`
				} `json:"scheduler_scores"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Items, 1)
	item := payload.Data.Items[0]
	require.Equal(t, int64(201), item.ID)
	require.Len(t, item.SchedulerScores, 1)
	require.Equal(t, groupID, *item.SchedulerScores[0].GroupID)
	require.Equal(t, item.SchedulerScores[0].BaseScore, item.SchedulerScore.BaseScore)
}

func TestAccountHandlerListSchedulerScoreIgnoresPagination(t *testing.T) {
	router, adminSvc := setupAccountListRouter()
	now := time.Now().UTC()
	visibleAccount := service.Account{
		ID:          301,
		Name:        "visible-low-priority",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 10,
		Priority:    100000,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	hiddenFilterPeer := service.Account{
		ID:          302,
		Name:        "hidden-high-priority",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 10,
		Priority:    1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	adminSvc.accounts = []service.Account{visibleAccount}
	adminSvc.accountSchedulerScoreFilterAccounts = []service.Account{visibleAccount, hiddenFilterPeer}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts?page=1&page_size=1&platform=openai&include_scheduler_score=1", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var payload struct {
		Data struct {
			Items []struct {
				ID             int64 `json:"id"`
				SchedulerScore struct {
					BaseScore float64 `json:"base_score"`
				} `json:"scheduler_score"`
				SchedulerScores []struct {
					GroupID   *int64  `json:"group_id"`
					BaseScore float64 `json:"base_score"`
				} `json:"scheduler_scores"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Items, 1)
	require.Equal(t, int64(301), payload.Data.Items[0].ID)
	require.Less(t, payload.Data.Items[0].SchedulerScore.BaseScore, 3.75)
	require.Empty(t, payload.Data.Items[0].SchedulerScores)
}
