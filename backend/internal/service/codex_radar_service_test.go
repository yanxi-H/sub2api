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
  "comprehensive_points": [
    {"model": "model-a", "effort": "low", "iq": 75, "samples": 4},
    {"model": "model-a", "effort": "ultra", "iq": 105, "samples": 4}
  ],
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

const codexRadarSoftwareMetricsFixture = `{
  "schema": 3,
  "source_updated_at": "2026-08-03T12:00:00Z",
  "points": [
    {"model": "model-a", "effort": "low", "iq": 80, "total": 2, "average_price_usd": 2, "average_price_usd_by_band": {"off_peak": 2, "peak": 4}, "average_minutes": 1},
    {"model": "model-a", "effort": "ultra", "iq": 100, "total": 2, "average_price_usd": 4, "average_minutes": 3}
  ]
}`

const codexRadarVisualSpatialFixture = `{
  "schema": 1,
  "source_updated_at": "2026-08-03T13:30:00Z",
  "points": [
    {"model": "model-a", "effort": "low", "iq": 70, "valid_tasks": 2, "average_price_usd": 4, "average_minutes": 2},
    {"model": "model-a", "effort": "ultra", "iq": 110, "valid_tasks": 2, "average_price_usd": 12, "average_minutes": 4}
  ]
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
		case "/software":
			_, _ = w.Write([]byte(codexRadarSoftwareMetricsFixture))
		case "/visual":
			_, _ = w.Write([]byte(codexRadarVisualSpatialFixture))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := newCodexRadarService(server.Client(), server.URL+"/insights", server.URL+"/software", server.URL+"/visual")
	result, err := service.GetDashboardRecommendations(context.Background())
	require.NoError(t, err)
	require.True(t, result.StationAvailable)
	require.True(t, result.IntelligenceAvailable)
	require.True(t, result.SoftwareEngineeringAvailable)
	require.True(t, result.VisualSpatialAvailable)
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
	require.Equal(t, 4, low.Samples)
	require.InDelta(t, 75, low.IQ, 0.001)
	require.NotNil(t, low.AverageCostUSD)
	require.InDelta(t, 3, *low.AverageCostUSD, 0.001)
	require.NotNil(t, low.AverageDurationMinutes)
	require.InDelta(t, 1.5, *low.AverageDurationMinutes, 0.001)

	ultra := result.IntelligenceRecommendations[1]
	require.InDelta(t, 105, ultra.IQ, 0.001)
	require.NotNil(t, ultra.AverageCostUSD)
	require.InDelta(t, 8, *ultra.AverageCostUSD, 0.001)
	require.NotNil(t, ultra.AverageDurationMinutes)
	require.InDelta(t, 3.5, *ultra.AverageDurationMinutes, 0.001)

	require.Len(t, result.SoftwareEngineeringRecommendations, 2)
	softwareLow := result.SoftwareEngineeringRecommendations[0]
	require.InDelta(t, 80, softwareLow.IQ, 0.001)
	require.NotNil(t, softwareLow.AverageCostUSDByBand)
	require.InDelta(t, 2, *softwareLow.AverageCostUSDByBand.OffPeak, 0.001)
	require.InDelta(t, 4, *softwareLow.AverageCostUSDByBand.Peak, 0.001)

	require.Len(t, result.VisualSpatialRecommendations, 2)
	require.InDelta(t, 70, result.VisualSpatialRecommendations[0].IQ, 0.001)
}

func TestCodexRadarServiceKeepsAvailableSourceWhenTheOtherFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/insights":
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		case "/software":
			_, _ = w.Write([]byte(codexRadarSoftwareMetricsFixture))
		case "/visual":
			_, _ = w.Write([]byte(codexRadarVisualSpatialFixture))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := newCodexRadarService(server.Client(), server.URL+"/insights", server.URL+"/software", server.URL+"/visual")
	result, err := service.GetDashboardRecommendations(context.Background())
	require.NoError(t, err)
	require.False(t, result.StationAvailable)
	require.Empty(t, result.StationRecommendations)
	require.False(t, result.IntelligenceAvailable)
	require.Empty(t, result.IntelligenceRecommendations)
	require.True(t, result.SoftwareEngineeringAvailable)
	require.Len(t, result.SoftwareEngineeringRecommendations, 2)
	require.True(t, result.VisualSpatialAvailable)
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
		case "/software":
			_, _ = w.Write([]byte(codexRadarSoftwareMetricsFixture))
		case "/visual":
			_, _ = w.Write([]byte(codexRadarVisualSpatialFixture))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := newCodexRadarService(server.Client(), server.URL+"/insights", server.URL+"/software", server.URL+"/visual")
	result, err := service.GetDashboardRecommendations(context.Background())
	require.NoError(t, err)
	require.False(t, result.StationAvailable)
	require.False(t, result.IntelligenceAvailable)
	require.True(t, result.SoftwareEngineeringAvailable)
	require.True(t, result.VisualSpatialAvailable)
	require.False(t, redirected.Load())
}
