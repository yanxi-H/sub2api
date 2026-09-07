package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildOpsInvestigationResponseClassifiesSystemAndClientErrors(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	current := &OpsDashboardFilter{StartTime: now, EndTime: now.Add(time.Hour)}
	baseline := &OpsDashboardFilter{StartTime: now.Add(-7 * time.Hour), EndTime: now}

	response := buildOpsInvestigationResponse(
		current,
		baseline,
		[]*OpsInvestigationErrorGroup{
			{Phase: "routing", Owner: "platform", Type: "routing", StatusCode: 503, Count: 5},
			{Phase: "auth", Owner: "client", Type: "invalid_api_key", StatusCode: 401, Count: 20},
		},
		[]*OpsInvestigationErrorGroup{
			{Phase: "routing", Owner: "platform", Type: "routing", StatusCode: 503, Count: 7},
			{Phase: "auth", Owner: "client", Type: "invalid_api_key", StatusCode: 401, Count: 7},
		},
		nil,
		nil,
	)

	require.Equal(t, int64(25), response.TotalErrors)
	require.Len(t, response.Findings, 2)
	require.Equal(t, "routing_capacity", response.Findings[0].Rule)
	require.Equal(t, "critical", response.Findings[0].Severity)
	require.Equal(t, int64(1), response.Findings[0].BaselineCount)
	require.Equal(t, "client_auth", response.Findings[1].Rule)
	require.Equal(t, "info", response.Findings[1].Severity)
}

func TestBuildOpsInvestigationResponseAddsLatencyRegression(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	current := &OpsDashboardFilter{StartTime: now, EndTime: now.Add(time.Hour)}
	baseline := &OpsDashboardFilter{StartTime: now.Add(-time.Hour), EndTime: now}
	currentP95 := 1800
	baselineP95 := 700

	response := buildOpsInvestigationResponse(
		current,
		baseline,
		nil,
		nil,
		&OpsDashboardOverview{SuccessCount: 100, Duration: OpsPercentiles{P95: &currentP95}},
		&OpsDashboardOverview{SuccessCount: 120, Duration: OpsPercentiles{P95: &baselineP95}},
	)

	require.Len(t, response.Findings, 1)
	require.Equal(t, "duration_p95_regression", response.Findings[0].Rule)
	require.Equal(t, "critical", response.Findings[0].Severity)
	require.Equal(t, int64(1100), response.Findings[0].DeltaCount)
}
