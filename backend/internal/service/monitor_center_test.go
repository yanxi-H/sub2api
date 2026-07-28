package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
