package service

import (
	"context"
	"time"
)

func (s *OpsService) SetMonitorCenterService(monitorCenter *MonitorCenterService) {
	if s == nil {
		return
	}
	s.monitorCenter = monitorCenter
}

func (s *OpsService) StopMonitorCenter() {
	if s == nil || s.monitorCenter == nil {
		return
	}
	s.monitorCenter.Stop()
}

func (s *OpsService) GetMonitorCenterOpenAIStatus(ctx context.Context) (*MonitorCenterOpenAIStatus, error) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	if s.monitorCenter == nil {
		return unknownMonitorCenterOpenAIStatus(), nil
	}
	return s.monitorCenter.GetOpenAIStatus(ctx)
}

func (s *OpsService) GetMonitorCenterOpenAIHistory(
	ctx context.Context,
	start, end time.Time,
) (*MonitorCenterOpenAIHistory, error) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	if s.monitorCenter == nil {
		return emptyMonitorCenterOpenAIHistory(start, end), nil
	}
	return s.monitorCenter.GetOpenAIHistory(ctx, start, end)
}

func (s *OpsService) GetMonitorCenterProbe(
	ctx context.Context,
	start, end time.Time,
) (*MonitorCenterProbe, error) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	if s.monitorCenter == nil {
		return &MonitorCenterProbe{Configured: false, Status: MonitorCenterStatusUnknown, Points: []MonitorCenterProbePoint{}}, nil
	}
	return s.monitorCenter.GetProbe(ctx, start, end)
}
