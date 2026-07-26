package service

import (
	"context"
	"net/url"
	"sort"
	"strings"
	"time"
)

const monitorCenterProbeHistoryLimit = 1000
const monitorCenterProbeGroupName = "monitor-center"

type monitorCenterProbeHistoryRepository interface {
	ListHistoryRange(ctx context.Context, monitorID int64, model string, start, end time.Time) ([]*ChannelMonitorHistoryEntry, error)
}

// GetMonitorCenterProbe reuses an existing enabled OpenAI channel monitor.
// It never decrypts credentials or triggers a model request; the regular
// channel-monitor scheduler owns probe execution and persistence.
func (s *ChannelMonitorService) GetMonitorCenterProbe(
	ctx context.Context,
	start, end time.Time,
) (*MonitorCenterProbe, error) {
	empty := &MonitorCenterProbe{
		Configured: false,
		Status:     MonitorCenterStatusUnknown,
		Points:     []MonitorCenterProbePoint{},
	}
	if s == nil || s.repo == nil {
		return empty, nil
	}
	enabled := true
	monitors, _, err := s.repo.List(ctx, ChannelMonitorListParams{
		Page: 1, PageSize: 200, Provider: MonitorProviderOpenAI, Enabled: &enabled,
	})
	if err != nil {
		return nil, err
	}
	if len(monitors) == 0 {
		return empty, nil
	}
	monitor := selectMonitorCenterProbe(monitors)
	if monitor == nil {
		return empty, nil
	}
	latestEntries, err := s.repo.ListHistory(ctx, monitor.ID, monitor.PrimaryModel, monitorCenterProbeHistoryLimit)
	if err != nil {
		return nil, err
	}
	rangeEntries := latestEntries
	if rangeRepo, ok := s.repo.(monitorCenterProbeHistoryRepository); ok {
		rangeEntries, err = rangeRepo.ListHistoryRange(ctx, monitor.ID, monitor.PrimaryModel, start, end)
		if err != nil {
			return nil, err
		}
	}

	probe := &MonitorCenterProbe{
		Configured:   true,
		MonitorID:    monitor.ID,
		MonitorName:  monitor.Name,
		Endpoint:     monitor.Endpoint,
		Model:        monitor.PrimaryModel,
		EndpointKind: monitorCenterProbeEndpointKind(monitor.Endpoint),
		Status:       MonitorCenterStatusUnknown,
		Points:       make([]MonitorCenterProbePoint, 0, len(rangeEntries)),
	}
	for _, entry := range rangeEntries {
		if entry == nil || entry.CheckedAt.Before(start) || entry.CheckedAt.After(end) {
			continue
		}
		mappedStatus := normalizeMonitorCenterStatus(entry.Status)
		probe.Points = append(probe.Points, MonitorCenterProbePoint{
			Timestamp: entry.CheckedAt.UTC(), Status: mappedStatus, LatencyMs: entry.LatencyMs,
		})
	}
	for index, entry := range latestEntries {
		if entry == nil {
			continue
		}
		if index == 0 || probe.LastCheckedAt == nil {
			checkedAt := entry.CheckedAt.UTC()
			probe.LastCheckedAt = &checkedAt
			probe.Status = normalizeMonitorCenterStatus(entry.Status)
			probe.LatencyMs = entry.LatencyMs
		}
		if probe.LastSuccessAt == nil && (entry.Status == MonitorStatusOperational || entry.Status == MonitorStatusDegraded) {
			successAt := entry.CheckedAt.UTC()
			probe.LastSuccessAt = &successAt
		}
	}
	for _, entry := range latestEntries {
		if entry == nil {
			continue
		}
		if entry.Status == MonitorStatusOperational || entry.Status == MonitorStatusDegraded {
			break
		}
		probe.ConsecutiveFailures++
	}
	sort.Slice(probe.Points, func(i, j int) bool { return probe.Points[i].Timestamp.Before(probe.Points[j].Timestamp) })
	return probe, nil
}

func selectMonitorCenterProbe(monitors []*ChannelMonitor) *ChannelMonitor {
	eligible := make([]*ChannelMonitor, 0, len(monitors))
	for _, monitor := range monitors {
		if monitor == nil || !strings.EqualFold(strings.TrimSpace(monitor.GroupName), monitorCenterProbeGroupName) {
			continue
		}
		if monitorCenterProbeEndpointKind(monitor.Endpoint) != "custom_endpoint" {
			continue
		}
		eligible = append(eligible, monitor)
	}
	if len(eligible) == 0 {
		return nil
	}
	// A stable ID-based choice prevents the cockpit probe from changing merely
	// because another monitor happened to run more recently.
	sort.SliceStable(eligible, func(i, j int) bool { return eligible[i].ID < eligible[j].ID })
	return eligible[0]
}

func monitorCenterProbeEndpointKind(endpoint string) string {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return "unknown"
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "api.openai.com" {
		return "openai_direct"
	}
	if host == "" {
		return "unknown"
	}
	return "custom_endpoint"
}
