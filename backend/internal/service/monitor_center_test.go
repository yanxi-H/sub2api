package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type monitorCenterProbeRepoStub struct {
	ChannelMonitorRepository
	monitor       *ChannelMonitor
	bucketed      []*ChannelMonitorHistoryEntry
	recentRaw     []*ChannelMonitorHistoryEntry
	historyCalled bool
}

func (r *monitorCenterProbeRepoStub) List(_ context.Context, _ ChannelMonitorListParams) ([]*ChannelMonitor, int64, error) {
	return []*ChannelMonitor{r.monitor}, 1, nil
}

func (r *monitorCenterProbeRepoStub) ListHistory(_ context.Context, _ int64, _ string, _ int) ([]*ChannelMonitorHistoryEntry, error) {
	r.historyCalled = true
	return nil, nil
}

func (r *monitorCenterProbeRepoStub) ListHistoryRange(_ context.Context, _ int64, _ string, _, _ time.Time, _ time.Duration) ([]*ChannelMonitorHistoryEntry, error) {
	return r.bucketed, nil
}

func (r *monitorCenterProbeRepoStub) ListRecentHistoryRange(_ context.Context, _ int64, _ string, _, _ time.Time, _ int) ([]*ChannelMonitorHistoryEntry, error) {
	return r.recentRaw, nil
}

func TestNormalizeMonitorCenterOpenAIStatusUsesExactRulesAndUnknownForMissingComponents(t *testing.T) {
	now := time.Date(2026, 7, 26, 8, 0, 0, 0, time.UTC)
	payload := &openAISummaryPayload{}
	payload.Status.Indicator = "none"
	payload.Status.Description = "All Systems Operational"
	payload.Components = append(payload.Components,
		struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		}{ID: "responses", Name: "Responses", Status: "operational"},
		struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		}{ID: "files", Name: "Files", Status: "degraded_performance"},
		struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		}{ID: "login-1", Name: "Login", Status: "operational"},
		struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		}{ID: "login-2", Name: "Login", Status: "partial_outage"},
	)

	status := normalizeMonitorCenterOpenAIStatus(payload, now, 123)
	require.Equal(t, MonitorCenterStatusOperational, status.OverallStatus)
	require.Equal(t, MonitorCenterStatusDegradedPerformance, monitorCenterGroupStatus(status.Groups, "api"))
	require.Equal(t, MonitorCenterStatusPartialOutage, monitorCenterGroupStatus(status.Groups, "chatgpt"))
	require.Equal(t, MonitorCenterStatusUnknown, monitorCenterGroupStatus(status.Groups, "codex"))

	api := status.Groups[0]
	require.Equal(t, MonitorCenterStatusUnknown, api.Components[1].Status)
	require.False(t, api.Components[1].Matched)
}

func TestNormalizeMonitorCenterStatusPreservesMaintenanceAndUnknown(t *testing.T) {
	require.Equal(t, MonitorCenterStatusUnderMaintenance, normalizeMonitorCenterStatus("under_maintenance"))
	require.Equal(t, MonitorCenterStatusUnknown, normalizeMonitorCenterStatus("new_status"))
	require.Equal(t, MonitorCenterStatusMajorOutage, worstMonitorCenterStatus([]string{
		MonitorCenterStatusOperational,
		MonitorCenterStatusMajorOutage,
		MonitorCenterStatusDegradedPerformance,
	}))
}

func TestSelectMonitorCenterProbeRequiresExplicitNonDirectMonitor(t *testing.T) {
	selected := selectMonitorCenterProbe([]*ChannelMonitor{
		{ID: 1, GroupName: "default", Endpoint: "https://sub2api.example.com"},
		{ID: 2, GroupName: "monitor-center", Endpoint: "https://api.openai.com"},
		{ID: 9, GroupName: " Monitor-Center ", Endpoint: "https://sub2api.example.com"},
		{ID: 4, GroupName: "monitor-center", Endpoint: "https://gateway.example.com"},
	})

	require.NotNil(t, selected)
	require.Equal(t, int64(4), selected.ID)
	require.Nil(t, selectMonitorCenterProbe([]*ChannelMonitor{
		{ID: 1, GroupName: "monitor-center", Endpoint: "https://api.openai.com"},
	}))
}

func TestMonitorCenterIncidentGroupsOnlyMatchConfiguredComponents(t *testing.T) {
	require.ElementsMatch(t, []string{"api", "codex"}, monitorCenterIncidentGroups([]string{
		"Responses API",
		"Codex CLI",
		"Unrelated OpenAI Product",
	}))
	require.Empty(t, monitorCenterIncidentGroups([]string{"Unrelated OpenAI Product"}))
}

func TestMonitorCenterHistoryStatisticsUseRelevantGroupsNotOverallStatus(t *testing.T) {
	points := []MonitorCenterOpenAIHistoryPoint{
		{FetchStatus: monitorCenterFetchStatusSuccess, OverallStatus: MonitorCenterStatusPartialOutage, APIStatus: MonitorCenterStatusOperational, ChatGPTStatus: MonitorCenterStatusOperational, CodexStatus: MonitorCenterStatusOperational, LatencyMs: 100},
		{FetchStatus: monitorCenterFetchStatusSuccess, OverallStatus: MonitorCenterStatusOperational, APIStatus: MonitorCenterStatusDegradedPerformance, ChatGPTStatus: MonitorCenterStatusOperational, CodexStatus: MonitorCenterStatusOperational, LatencyMs: 200},
		{FetchStatus: monitorCenterFetchStatusFailed, APIStatus: MonitorCenterStatusOperational, ChatGPTStatus: MonitorCenterStatusOperational, CodexStatus: MonitorCenterStatusOperational, LatencyMs: 300},
	}

	stats := monitorCenterHistoryStatistics(points)
	require.Equal(t, 3, stats.SampleCount)
	require.Equal(t, 2, stats.SuccessfulCount)
	require.InDelta(t, 150, stats.AverageLatencyMs, 0.01)
	require.Equal(t, 2, stats.AnomalyCount)
	require.InDelta(t, 50, stats.Groups["api"].AvailabilityPct, 0.01)
	require.InDelta(t, 100, stats.Groups["chatgpt"].AvailabilityPct, 0.01)
}

func TestMonitorCenterProbeHistoryBucketBoundsLongRanges(t *testing.T) {
	require.Equal(t, time.Minute, monitorCenterProbeHistoryBucket(time.Hour))
	require.Equal(t, 2*time.Minute, monitorCenterProbeHistoryBucket(24*time.Hour))
	require.Equal(t, 44*time.Minute, monitorCenterProbeHistoryBucket(30*24*time.Hour))
	require.LessOrEqual(t, int((30*24*time.Hour)/monitorCenterProbeHistoryBucket(30*24*time.Hour))+1, monitorCenterProbeHistoryLimit+1)
}

func TestGetMonitorCenterProbeUsesRawLatestStatusInsteadOfWorstBucketRepresentative(t *testing.T) {
	start := time.Date(2026, 7, 27, 8, 0, 0, 0, time.UTC)
	failedAt := start.Add(10 * time.Minute)
	recoveredAt := start.Add(11 * time.Minute)
	repo := &monitorCenterProbeRepoStub{
		monitor: &ChannelMonitor{
			ID: 7, Name: "probe", Provider: MonitorProviderOpenAI, GroupName: monitorCenterProbeGroupName,
			Endpoint: "https://gateway.example.com", PrimaryModel: "gpt-test", Enabled: true,
		},
		bucketed: []*ChannelMonitorHistoryEntry{
			{ID: 1, Status: MonitorStatusFailed, Message: "timeout", CheckedAt: failedAt},
		},
		recentRaw: []*ChannelMonitorHistoryEntry{
			{ID: 2, Status: MonitorStatusOperational, CheckedAt: recoveredAt},
			{ID: 1, Status: MonitorStatusFailed, Message: "timeout", CheckedAt: failedAt},
		},
	}
	service := NewChannelMonitorService(repo, nil)

	probe, err := service.GetMonitorCenterProbe(context.Background(), start, start.Add(time.Hour))

	require.NoError(t, err)
	require.True(t, repo.historyCalled)
	require.Equal(t, MonitorCenterStatusOperational, probe.Status)
	require.Empty(t, probe.FailureReason)
	require.Zero(t, probe.ConsecutiveFailures)
	require.Equal(t, recoveredAt, *probe.LastSuccessAt)
	require.Len(t, probe.Points, 1)
	require.Equal(t, MonitorCenterStatusPartialOutage, probe.Points[0].Status)
}
