package service

import (
	"context"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const opsInvestigationMaxBaselineWindow = 7 * 24 * time.Hour

// GetDashboardInvestigation compares the selected window with up to seven
// preceding equivalent windows. It intentionally uses only aggregates, making
// every finding reproducible without exposing request content to an AI model.
func (s *OpsService) GetDashboardInvestigation(ctx context.Context, filter *OpsDashboardFilter) (*OpsInvestigationResponse, error) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	if s.opsRepo == nil {
		return nil, infraerrors.ServiceUnavailable("OPS_REPO_UNAVAILABLE", "Ops repository not available")
	}
	if filter == nil || filter.StartTime.IsZero() || filter.EndTime.IsZero() || !filter.StartTime.Before(filter.EndTime) {
		return nil, infraerrors.BadRequest("OPS_TIME_RANGE_INVALID", "a valid start_time/end_time range is required")
	}

	current := cloneOpsFilterWithMode(filter, s.resolveOpsQueryMode(ctx, filter.QueryMode))
	window := current.EndTime.Sub(current.StartTime)
	baselineWindow := window * 7
	if baselineWindow > opsInvestigationMaxBaselineWindow {
		baselineWindow = opsInvestigationMaxBaselineWindow
	}
	baseline := cloneOpsFilterWithMode(current, current.QueryMode)
	baseline.EndTime = current.StartTime
	baseline.StartTime = baseline.EndTime.Add(-baselineWindow)

	currentGroups, err := s.opsRepo.GetInvestigationErrorGroups(ctx, current)
	if err != nil {
		return nil, err
	}
	baselineGroups, err := s.opsRepo.GetInvestigationErrorGroups(ctx, baseline)
	if err != nil {
		return nil, err
	}

	currentOverview, _ := s.getInvestigationOverview(ctx, current)
	baselineOverview, _ := s.getInvestigationOverview(ctx, baseline)

	return buildOpsInvestigationResponse(current, baseline, currentGroups, baselineGroups, currentOverview, baselineOverview), nil
}

func (s *OpsService) getInvestigationOverview(ctx context.Context, filter *OpsDashboardFilter) (*OpsDashboardOverview, error) {
	overview, err := s.opsRepo.GetDashboardOverview(ctx, filter)
	if err != nil && shouldFallbackOpsPreagg(filter, err) {
		rawFilter := cloneOpsFilterWithMode(filter, OpsQueryModeRaw)
		return s.opsRepo.GetDashboardOverview(ctx, rawFilter)
	}
	return overview, err
}

func buildOpsInvestigationResponse(current, baseline *OpsDashboardFilter, currentGroups, baselineGroups []*OpsInvestigationErrorGroup, currentOverview, baselineOverview *OpsDashboardOverview) *OpsInvestigationResponse {
	response := &OpsInvestigationResponse{
		StartTime:     current.StartTime.UTC(),
		EndTime:       current.EndTime.UTC(),
		BaselineStart: baseline.StartTime.UTC(),
		BaselineEnd:   baseline.EndTime.UTC(),
		Findings:      make([]*OpsInvestigationFinding, 0, 8),
	}

	baselineCounts := make(map[string]int64, len(baselineGroups))
	for _, group := range baselineGroups {
		if group != nil {
			baselineCounts[opsInvestigationGroupKey(group)] += group.Count
		}
	}
	for _, group := range currentGroups {
		if group != nil {
			if group.Total > 0 {
				response.TotalErrors = group.Total
			} else {
				response.TotalErrors += group.Count
			}
		}
	}

	currentWindow := current.EndTime.Sub(current.StartTime)
	baselineWindow := baseline.EndTime.Sub(baseline.StartTime)
	for _, group := range currentGroups {
		if group == nil || group.Count <= 0 {
			continue
		}
		baselineCount := normalizeOpsInvestigationCount(baselineCounts[opsInvestigationGroupKey(group)], currentWindow, baselineWindow)
		if finding := buildOpsErrorInvestigationFinding(group, baselineCount, response.TotalErrors); finding != nil {
			response.Findings = append(response.Findings, finding)
		}
	}

	appendOpsLatencyInvestigationFindings(response, currentOverview, baselineOverview)
	sort.SliceStable(response.Findings, func(i, j int) bool {
		left, right := response.Findings[i], response.Findings[j]
		if opsInvestigationSeverityRank(left.Severity) != opsInvestigationSeverityRank(right.Severity) {
			return opsInvestigationSeverityRank(left.Severity) > opsInvestigationSeverityRank(right.Severity)
		}
		if left.CurrentCount != right.CurrentCount {
			return left.CurrentCount > right.CurrentCount
		}
		return left.Rule < right.Rule
	})
	if len(response.Findings) > 6 {
		response.Findings = response.Findings[:6]
	}
	return response
}

func buildOpsErrorInvestigationFinding(group *OpsInvestigationErrorGroup, baselineCount, total int64) *OpsInvestigationFinding {
	rule := opsInvestigationRule(group)
	if rule == "" {
		return nil
	}
	severity := opsInvestigationSeverity(rule, group.Count, baselineCount)
	if severity == "" {
		return nil
	}
	delta := group.Count - baselineCount
	change := int64(0)
	if baselineCount > 0 {
		change = int64(math.Round(float64(delta) * 100 / float64(baselineCount)))
	} else if group.Count > 0 {
		change = 100
	}
	share := int64(0)
	if total > 0 {
		share = int64(math.Round(float64(group.Count) * 100 / float64(total)))
	}
	return &OpsInvestigationFinding{
		Rule:          rule,
		Kind:          "error",
		Severity:      severity,
		Phase:         group.Phase,
		Owner:         group.Owner,
		StatusCode:    group.StatusCode,
		Platform:      group.Platform,
		GroupID:       group.GroupID,
		CurrentCount:  group.Count,
		BaselineCount: baselineCount,
		DeltaCount:    delta,
		ChangePercent: change,
		SharePercent:  share,
	}
}

func opsInvestigationRule(group *OpsInvestigationErrorGroup) string {
	owner := strings.ToLower(strings.TrimSpace(group.Owner))
	phase := strings.ToLower(strings.TrimSpace(group.Phase))
	typ := strings.ToLower(strings.TrimSpace(group.Type))
	status := group.StatusCode
	switch {
	case owner == "provider" && (status == 429 || status == 529):
		return "provider_throttle"
	case owner == "provider" && strings.Contains(typ, "cyber"):
		return "provider_policy"
	case owner == "provider" && (status >= 500 || phase == "network" || strings.Contains(typ, "timeout")):
		return "provider_failure"
	case phase == "routing" && status == 503:
		return "routing_capacity"
	case owner == "platform" && status == 413:
		return "request_body_limit"
	case owner == "platform" && (status == 500 || status == 499 || status == 404 || phase == "internal"):
		return "platform_failure"
	case owner == "client" && (status == 401 || status == 403 || phase == "auth"):
		return "client_auth"
	case owner == "client" && (status == 400 || status == 422):
		return "client_request"
	}
	return ""
}

func opsInvestigationSeverity(rule string, current, baseline int64) string {
	spike := current >= 3 && (baseline == 0 || current >= baseline*2)
	switch rule {
	case "provider_failure", "routing_capacity", "platform_failure":
		if current >= 5 && spike {
			return "critical"
		}
		if current > 0 {
			return "warning"
		}
	case "provider_throttle", "provider_policy":
		if current >= 10 && spike {
			return "critical"
		}
		if current >= 3 {
			return "warning"
		}
	case "request_body_limit":
		if current >= 3 {
			return "info"
		}
	case "client_auth", "client_request":
		if current >= 5 && (spike || current >= 15) {
			return "info"
		}
	}
	return ""
}

func appendOpsLatencyInvestigationFindings(response *OpsInvestigationResponse, current, baseline *OpsDashboardOverview) {
	if current == nil || baseline == nil {
		return
	}
	appendFinding := func(rule string, currentValue, baselineValue *int, minimum, deltaMinimum int) {
		if currentValue == nil || baselineValue == nil || *baselineValue <= 0 || *currentValue < minimum {
			return
		}
		delta := *currentValue - *baselineValue
		if delta < deltaMinimum || float64(*currentValue) < float64(*baselineValue)*1.5 {
			return
		}
		severity := "warning"
		if float64(*currentValue) >= float64(*baselineValue)*2.5 {
			severity = "critical"
		}
		change := int64(math.Round(float64(delta) * 100 / float64(*baselineValue)))
		response.Findings = append(response.Findings, &OpsInvestigationFinding{
			Rule:          rule,
			Kind:          "latency",
			Severity:      severity,
			CurrentCount:  current.SuccessCount,
			BaselineCount: baseline.SuccessCount,
			CurrentValue:  currentValue,
			BaselineValue: baselineValue,
			DeltaCount:    int64(delta),
			ChangePercent: change,
		})
	}
	appendFinding("duration_p95_regression", current.Duration.P95, baseline.Duration.P95, 1000, 300)
	appendFinding("ttft_p95_regression", current.TTFT.P95, baseline.TTFT.P95, 500, 200)
}

func normalizeOpsInvestigationCount(count int64, currentWindow, baselineWindow time.Duration) int64 {
	if count <= 0 || currentWindow <= 0 || baselineWindow <= 0 {
		return 0
	}
	return int64(math.Round(float64(count) * float64(currentWindow) / float64(baselineWindow)))
}

func opsInvestigationGroupKey(group *OpsInvestigationErrorGroup) string {
	groupID := int64(0)
	if group.GroupID != nil {
		groupID = *group.GroupID
	}
	return strings.Join([]string{
		strings.ToLower(strings.TrimSpace(group.Phase)),
		strings.ToLower(strings.TrimSpace(group.Owner)),
		strings.ToLower(strings.TrimSpace(group.Type)),
		strconv.Itoa(group.StatusCode),
		strings.ToLower(strings.TrimSpace(group.Platform)),
		strconv.FormatInt(groupID, 10),
	}, "|")
}

func opsInvestigationSeverityRank(severity string) int {
	switch severity {
	case "critical":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}
