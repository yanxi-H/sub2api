package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

const codexRadarStationFixture = `{
  "schema": 1,
  "source_updated_at": "2026-08-03T10:00:00Z",
  "recommendations": [
    {
      "key": "daily_development",
      "title": "Daily development",
      "items": [
        {"model": "model-a", "effort": "low", "iq": 75, "average_cost_usd": 1.25, "average_duration_minutes": 4},
        {"model": "model-b", "effort": "high", "iq": 90, "average_cost_usd": 2.5, "average_duration_minutes": 8},
        {"model": "model-c", "effort": "max", "iq": 100, "average_cost_usd": 3.5, "average_duration_minutes": 12}
      ]
    }
  ]
}`

const codexRadarIntelligenceFixture = `{
  "schema": 1,
  "combos": [
    {"model": "model-a", "effort": "low"},
    {"model": "model-a", "effort": "ultra"}
  ],
  "tasks": [{"id": 1}, {"id": 2}],
  "cells": {
    "1|model-a|low": {"ran_by": [{"passed": true, "duration_sec": 60, "actual_cost_usd": 2, "graded_at": "2026-08-03T11:00:00Z"}]},
    "2|model-a|low": {"ran_by": [{"passed": false, "duration_sec": 120, "actual_cost_usd": 4, "graded_at": "2026-08-03T12:00:00Z"}]},
    "1|model-a|ultra": {"ran_by": [{"passed": true, "duration_sec": 180, "actual_cost_usd": 10, "cost_complete": false, "graded_at": "2026-08-03T13:00:00Z"}]},
    "2|model-a|ultra": {"ran_by": [{"passed": true, "duration_sec": 240, "actual_cost_usd": 12, "cost_complete": true, "graded_at": "2026-08-03T13:30:00Z"}]}
  }
}`

func TestCodexRadarServiceAggregatesDashboardData(t *testing.T) {
	var invalidRequest atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Accept") != "application/json" {
			invalidRequest.Store(true)
		}
		switch r.URL.Path {
		case "/insights":
			_, _ = w.Write([]byte(codexRadarStationFixture))
		case "/intelligence":
			_, _ = w.Write([]byte(codexRadarIntelligenceFixture))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := newCodexRadarService(server.Client(), server.URL+"/insights", server.URL+"/intelligence")
	result, err := service.GetDashboardRecommendations(context.Background())
	require.NoError(t, err)
	require.True(t, result.StationAvailable)
	require.True(t, result.IntelligenceAvailable)
	require.False(t, invalidRequest.Load())
	require.Equal(t, "2026-08-03T13:30:00Z", result.SourceUpdatedAt)

	require.Len(t, result.StationRecommendations, 1)
	require.Len(t, result.StationRecommendations[0].Items, 2)
	require.Equal(t, "model-a", result.StationRecommendations[0].Items[0].Model)
	require.Equal(t, "model-b", result.StationRecommendations[0].Items[1].Model)

	require.Len(t, result.IntelligenceRecommendations, 2)
	low := result.IntelligenceRecommendations[0]
	require.Equal(t, "model-a", low.Model)
	require.Equal(t, "low", low.Effort)
	require.Equal(t, 2, low.Samples)
	require.InDelta(t, 75, low.IQ, 0.001)
	require.NotNil(t, low.AverageCostUSD)
	require.InDelta(t, 3, *low.AverageCostUSD, 0.001)
	require.NotNil(t, low.AverageDurationMinutes)
	require.InDelta(t, 1.5, *low.AverageDurationMinutes, 0.001)

	ultra := result.IntelligenceRecommendations[1]
	require.InDelta(t, 150, ultra.IQ, 0.001)
	require.NotNil(t, ultra.AverageCostUSD)
	require.InDelta(t, 12, *ultra.AverageCostUSD, 0.001)
	require.NotNil(t, ultra.AverageDurationMinutes)
	require.InDelta(t, 3.5, *ultra.AverageDurationMinutes, 0.001)
}

func TestCodexRadarServiceKeepsAvailableSourceWhenTheOtherFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/insights":
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		case "/intelligence":
			_, _ = w.Write([]byte(codexRadarIntelligenceFixture))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := newCodexRadarService(server.Client(), server.URL+"/insights", server.URL+"/intelligence")
	result, err := service.GetDashboardRecommendations(context.Background())
	require.NoError(t, err)
	require.False(t, result.StationAvailable)
	require.Empty(t, result.StationRecommendations)
	require.True(t, result.IntelligenceAvailable)
	require.Len(t, result.IntelligenceRecommendations, 2)
}

func TestCodexRadarServiceDoesNotFollowRedirects(t *testing.T) {
	var redirected atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/insights":
			http.Redirect(w, r, "/redirect-target", http.StatusFound)
		case "/redirect-target":
			redirected.Store(true)
			_, _ = w.Write([]byte(codexRadarStationFixture))
		case "/intelligence":
			_, _ = w.Write([]byte(codexRadarIntelligenceFixture))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := newCodexRadarService(server.Client(), server.URL+"/insights", server.URL+"/intelligence")
	result, err := service.GetDashboardRecommendations(context.Background())
	require.NoError(t, err)
	require.False(t, result.StationAvailable)
	require.True(t, result.IntelligenceAvailable)
	require.False(t, redirected.Load())
}
