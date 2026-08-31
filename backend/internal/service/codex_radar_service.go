package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
)

const (
	codexRadarInsightsURL           = "https://codexradar.com/api/radar-insights"
	codexRadarSoftwareMetricsURL    = "https://codexradar.com/api/intelligence-efficiency-metrics"
	codexRadarVisualSpatialURL      = "https://codexradar.com/api/visual-spatial-reasoning"
	codexRadarRequestTimeout        = 12 * time.Second
	codexRadarResponseHeaderTTL     = 8 * time.Second
	codexRadarSoftwareMetricsSchema = 3
	codexRadarVisualSpatialSchema   = 1

	codexRadarInsightsBodyLimit        int64 = 512 << 10
	codexRadarSoftwareMetricsBodyLimit int64 = 256 << 10
	codexRadarVisualSpatialBodyLimit   int64 = 1 << 20

	codexRadarMaxStationItems = 2
)

// CodexRadarDashboardRecommendations contains the limited public data shown on
// the user dashboard. It deliberately excludes raw task and runner data.
type CodexRadarDashboardRecommendations struct {
	SourceUpdatedAt                    string                               `json:"source_updated_at,omitempty"`
	StationAvailable                   bool                                 `json:"station_available"`
	IntelligenceAvailable              bool                                 `json:"intelligence_available"`
	SoftwareEngineeringAvailable       bool                                 `json:"software_engineering_available"`
	VisualSpatialAvailable             bool                                 `json:"visual_spatial_available"`
	StationRecommendations             []CodexRadarStationRecommendationSet `json:"station_recommendations"`
	IntelligenceRecommendations        []CodexRadarIntelligenceMetric       `json:"intelligence_recommendations"`
	SoftwareEngineeringRecommendations []CodexRadarIntelligenceMetric       `json:"software_engineering_recommendations"`
	VisualSpatialRecommendations       []CodexRadarIntelligenceMetric       `json:"visual_spatial_recommendations"`
}

type CodexRadarStationRecommendationSet struct {
	Key   string                            `json:"key"`
	Title string                            `json:"title"`
	Items []CodexRadarStationRecommendation `json:"items"`
}

type CodexRadarStationRecommendation struct {
	Model                  string   `json:"model"`
	Effort                 string   `json:"effort"`
	IQ                     *float64 `json:"iq"`
	AverageCostUSD         *float64 `json:"average_cost_usd"`
	AverageDurationMinutes *float64 `json:"average_duration_minutes"`
}

type CodexRadarIntelligenceMetric struct {
	Model                  string                `json:"model"`
	Effort                 string                `json:"effort"`
	IQ                     float64               `json:"iq"`
	Samples                int                   `json:"samples"`
	AverageCostUSD         *float64              `json:"average_cost_usd"`
	AverageCostUSDByBand   *CodexRadarPriceBands `json:"average_cost_usd_by_band,omitempty"`
	AverageDurationMinutes *float64              `json:"average_duration_minutes"`
}

type CodexRadarPriceBands struct {
	OffPeak *float64 `json:"off_peak"`
	Peak    *float64 `json:"peak"`
}

type CodexRadarService struct {
	httpClient         *http.Client
	insightsURL        string
	softwareMetricsURL string
	visualSpatialURL   string
}

// NewCodexRadarService builds a direct, DNS-rebinding-protected client for the
// fixed CodexRadar endpoints. It never inherits an environment proxy.
func NewCodexRadarService() (*CodexRadarService, error) {
	client, err := httpclient.GetClient(httpclient.Options{
		Timeout:               codexRadarRequestTimeout,
		ResponseHeaderTimeout: codexRadarResponseHeaderTTL,
		ValidateResolvedIP:    true,
		MaxConnsPerHost:       4,
	})
	if err != nil {
		return nil, fmt.Errorf("create CodexRadar HTTP client: %w", err)
	}

	return newCodexRadarService(client, codexRadarInsightsURL, codexRadarSoftwareMetricsURL, codexRadarVisualSpatialURL), nil
}

func newCodexRadarService(client *http.Client, insightsURL, softwareMetricsURL, visualSpatialURL string) *CodexRadarService {
	if client == nil {
		client = &http.Client{Timeout: codexRadarRequestTimeout}
	}

	clientCopy := *client
	clientCopy.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &CodexRadarService{
		httpClient:         &clientCopy,
		insightsURL:        insightsURL,
		softwareMetricsURL: softwareMetricsURL,
		visualSpatialURL:   visualSpatialURL,
	}
}

// GetDashboardRecommendations loads the independent public sources in
// parallel. A temporary failure in one source still leaves the others
// available to the dashboard.
func (s *CodexRadarService) GetDashboardRecommendations(ctx context.Context) (*CodexRadarDashboardRecommendations, error) {
	if s == nil || s.httpClient == nil {
		return nil, infraerrors.ServiceUnavailable("CODEX_RADAR_UNAVAILABLE", "Model recommendations are temporarily unavailable")
	}

	type insightsResult struct {
		groups              []CodexRadarStationRecommendationSet
		comprehensivePoints []codexRadarComprehensivePointPayload
		updatedAt           time.Time
		err                 error
	}
	type metricsResult struct {
		metrics   []CodexRadarIntelligenceMetric
		updatedAt time.Time
		err       error
	}

	insightsCh := make(chan insightsResult, 1)
	softwareCh := make(chan metricsResult, 1)
	visualCh := make(chan metricsResult, 1)

	go func() {
		groups, points, updatedAt, err := s.fetchInsights(ctx)
		insightsCh <- insightsResult{groups: groups, comprehensivePoints: points, updatedAt: updatedAt, err: err}
	}()
	go func() {
		metrics, updatedAt, err := s.fetchMetricPoints(ctx, s.softwareMetricsURL, codexRadarSoftwareMetricsBodyLimit, codexRadarSoftwareMetricsSchema)
		softwareCh <- metricsResult{metrics: metrics, updatedAt: updatedAt, err: err}
	}()
	go func() {
		metrics, updatedAt, err := s.fetchMetricPoints(ctx, s.visualSpatialURL, codexRadarVisualSpatialBodyLimit, codexRadarVisualSpatialSchema)
		visualCh <- metricsResult{metrics: metrics, updatedAt: updatedAt, err: err}
	}()

	insights := <-insightsCh
	software := <-softwareCh
	visual := <-visualCh
	if insights.err != nil && software.err != nil && visual.err != nil {
		return nil, infraerrors.ServiceUnavailable("CODEX_RADAR_UNAVAILABLE", "Model recommendations are temporarily unavailable")
	}

	comprehensive := buildCodexRadarComprehensiveMetrics(insights.comprehensivePoints, software.metrics, visual.metrics)
	updatedAt := latestCodexRadarTime(insights.updatedAt, software.updatedAt)
	updatedAt = latestCodexRadarTime(updatedAt, visual.updatedAt)
	result := &CodexRadarDashboardRecommendations{
		StationAvailable:                   insights.err == nil && len(insights.groups) > 0,
		IntelligenceAvailable:              len(comprehensive) > 0,
		SoftwareEngineeringAvailable:       software.err == nil && len(software.metrics) > 0,
		VisualSpatialAvailable:             visual.err == nil && len(visual.metrics) > 0,
		StationRecommendations:             insights.groups,
		IntelligenceRecommendations:        comprehensive,
		SoftwareEngineeringRecommendations: software.metrics,
		VisualSpatialRecommendations:       visual.metrics,
	}
	if !updatedAt.IsZero() {
		result.SourceUpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	}

	return result, nil
}

func (s *CodexRadarService) fetchInsights(ctx context.Context) ([]CodexRadarStationRecommendationSet, []codexRadarComprehensivePointPayload, time.Time, error) {
	var payload codexRadarInsightsPayload
	if err := s.fetchJSON(ctx, s.insightsURL, codexRadarInsightsBodyLimit, &payload); err != nil {
		return nil, nil, time.Time{}, err
	}
	if payload.Schema != 1 {
		return nil, nil, time.Time{}, fmt.Errorf("unsupported CodexRadar insights schema")
	}

	sets := make([]CodexRadarStationRecommendationSet, 0, len(payload.Recommendations))
	seenKeys := make(map[string]struct{}, len(payload.Recommendations))
	for _, rawSet := range payload.Recommendations {
		key := codexRadarText(rawSet.Key, 64)
		if key == "" {
			continue
		}
		if _, seen := seenKeys[key]; seen {
			continue
		}

		items := make([]CodexRadarStationRecommendation, 0, codexRadarMaxStationItems)
		for _, rawItem := range rawSet.Items {
			if len(items) >= codexRadarMaxStationItems {
				break
			}
			model := codexRadarText(rawItem.Model, 128)
			effort := codexRadarText(rawItem.Effort, 32)
			if model == "" || effort == "" {
				continue
			}
			items = append(items, CodexRadarStationRecommendation{
				Model:                  model,
				Effort:                 effort,
				IQ:                     codexRadarNonNegative(rawItem.IQ),
				AverageCostUSD:         codexRadarNonNegative(rawItem.AverageCostUSD),
				AverageDurationMinutes: codexRadarNonNegative(rawItem.AverageDurationMinutes),
			})
		}
		if len(items) == 0 {
			continue
		}

		seenKeys[key] = struct{}{}
		sets = append(sets, CodexRadarStationRecommendationSet{
			Key:   key,
			Title: codexRadarText(rawSet.Title, 120),
			Items: items,
		})
	}
	if len(sets) == 0 {
		return nil, nil, time.Time{}, fmt.Errorf("CodexRadar returned no station recommendations")
	}

	return sets, payload.ComprehensivePoints, parseCodexRadarTime(payload.SourceUpdatedAt), nil
}

func (s *CodexRadarService) fetchMetricPoints(ctx context.Context, endpoint string, bodyLimit int64, schema int) ([]CodexRadarIntelligenceMetric, time.Time, error) {
	var payload codexRadarMetricsPayload
	if err := s.fetchJSON(ctx, endpoint, bodyLimit, &payload); err != nil {
		return nil, time.Time{}, err
	}
	if payload.Schema != schema || len(payload.Points) == 0 {
		return nil, time.Time{}, fmt.Errorf("invalid CodexRadar metrics payload")
	}

	metrics := make([]CodexRadarIntelligenceMetric, 0, len(payload.Points))
	seenCombinations := make(map[string]struct{}, len(payload.Points))
	for _, point := range payload.Points {
		metric, ok := codexRadarMetricFromPoint(point)
		if !ok {
			continue
		}
		key := codexRadarCombinationKey(metric.Model, metric.Effort)
		if _, seen := seenCombinations[key]; seen {
			continue
		}
		seenCombinations[key] = struct{}{}
		metrics = append(metrics, metric)
	}
	if len(metrics) == 0 {
		return nil, time.Time{}, fmt.Errorf("CodexRadar returned no metrics")
	}

	return metrics, parseCodexRadarTime(payload.SourceUpdatedAt), nil
}

func (s *CodexRadarService) fetchJSON(ctx context.Context, endpoint string, bodyLimit int64, target any) error {
	requestCtx, cancel := context.WithTimeout(ctx, codexRadarRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create CodexRadar request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request CodexRadar: %w", err)
	}
	if resp == nil || resp.Body == nil {
		return fmt.Errorf("CodexRadar returned an empty response")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("CodexRadar returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, bodyLimit+1))
	if err != nil {
		return fmt.Errorf("read CodexRadar response: %w", err)
	}
	if int64(len(body)) > bodyLimit {
		return fmt.Errorf("CodexRadar response exceeds %d bytes", bodyLimit)
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode CodexRadar response: %w", err)
	}
	return nil
}

type codexRadarInsightsPayload struct {
	Schema              int                                   `json:"schema"`
	SourceUpdatedAt     string                                `json:"source_updated_at"`
	Recommendations     []codexRadarStationSetPayload         `json:"recommendations"`
	ComprehensivePoints []codexRadarComprehensivePointPayload `json:"comprehensive_points"`
}

type codexRadarStationSetPayload struct {
	Key   string                         `json:"key"`
	Title string                         `json:"title"`
	Items []codexRadarStationItemPayload `json:"items"`
}

type codexRadarStationItemPayload struct {
	Model                  string   `json:"model"`
	Effort                 string   `json:"effort"`
	IQ                     *float64 `json:"iq"`
	AverageCostUSD         *float64 `json:"average_cost_usd"`
	AverageDurationMinutes *float64 `json:"average_duration_minutes"`
}

type codexRadarComprehensivePointPayload struct {
	Model   string   `json:"model"`
	Effort  string   `json:"effort"`
	IQ      *float64 `json:"iq"`
	Samples int      `json:"samples"`
}

type codexRadarMetricsPayload struct {
	Schema          int                            `json:"schema"`
	SourceUpdatedAt string                         `json:"source_updated_at"`
	Points          []codexRadarMetricPointPayload `json:"points"`
}

type codexRadarMetricPointPayload struct {
	Model                 string                       `json:"model"`
	Effort                string                       `json:"effort"`
	IQ                    *float64                     `json:"iq"`
	Total                 int                          `json:"total"`
	ValidTasks            int                          `json:"valid_tasks"`
	AveragePriceUSD       *float64                     `json:"average_price_usd"`
	AveragePriceUSDByBand *codexRadarPriceBandsPayload `json:"average_price_usd_by_band"`
	AverageMinutes        *float64                     `json:"average_minutes"`
}

type codexRadarPriceBandsPayload struct {
	OffPeak *float64 `json:"off_peak"`
	Peak    *float64 `json:"peak"`
}

func codexRadarMetricFromPoint(point codexRadarMetricPointPayload) (CodexRadarIntelligenceMetric, bool) {
	model := codexRadarText(point.Model, 128)
	effort := codexRadarText(point.Effort, 32)
	iq := codexRadarNonNegative(point.IQ)
	if model == "" || effort == "" || iq == nil {
		return CodexRadarIntelligenceMetric{}, false
	}

	samples := point.Total
	if samples <= 0 {
		samples = point.ValidTasks
	}
	metric := CodexRadarIntelligenceMetric{
		Model:                  model,
		Effort:                 effort,
		IQ:                     *iq,
		Samples:                samples,
		AverageCostUSD:         codexRadarNonNegative(point.AveragePriceUSD),
		AverageDurationMinutes: codexRadarNonNegative(point.AverageMinutes),
	}
	if point.AveragePriceUSDByBand != nil {
		bands := &CodexRadarPriceBands{
			OffPeak: codexRadarNonNegative(point.AveragePriceUSDByBand.OffPeak),
			Peak:    codexRadarNonNegative(point.AveragePriceUSDByBand.Peak),
		}
		if bands.OffPeak != nil || bands.Peak != nil {
			metric.AverageCostUSDByBand = bands
		}
	}
	return metric, true
}

func buildCodexRadarComprehensiveMetrics(
	points []codexRadarComprehensivePointPayload,
	software []CodexRadarIntelligenceMetric,
	visual []CodexRadarIntelligenceMetric,
) []CodexRadarIntelligenceMetric {
	softwareByKey := codexRadarMetricsByCombination(software)
	visualByKey := codexRadarMetricsByCombination(visual)
	metrics := make([]CodexRadarIntelligenceMetric, 0, len(points))
	seen := make(map[string]struct{}, len(points))
	for _, point := range points {
		model := codexRadarText(point.Model, 128)
		effort := codexRadarText(point.Effort, 32)
		iq := codexRadarNonNegative(point.IQ)
		key := codexRadarCombinationKey(model, effort)
		if model == "" || effort == "" || iq == nil {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		softwareMetric, hasSoftware := softwareByKey[key]
		visualMetric, hasVisual := visualByKey[key]
		metric := CodexRadarIntelligenceMetric{
			Model:   model,
			Effort:  effort,
			IQ:      *iq,
			Samples: point.Samples,
		}
		if hasSoftware && hasVisual {
			metric.AverageCostUSD = averageCodexRadarValues(softwareMetric.AverageCostUSD, visualMetric.AverageCostUSD)
			metric.AverageDurationMinutes = averageCodexRadarValues(softwareMetric.AverageDurationMinutes, visualMetric.AverageDurationMinutes)
		}
		metrics = append(metrics, metric)
	}
	return metrics
}

func codexRadarMetricsByCombination(metrics []CodexRadarIntelligenceMetric) map[string]CodexRadarIntelligenceMetric {
	byKey := make(map[string]CodexRadarIntelligenceMetric, len(metrics))
	for _, metric := range metrics {
		byKey[codexRadarCombinationKey(metric.Model, metric.Effort)] = metric
	}
	return byKey
}

func codexRadarCombinationKey(model, effort string) string {
	return model + "|" + effort
}

func averageCodexRadarValues(left, right *float64) *float64 {
	if left == nil || right == nil {
		return nil
	}
	average := (*left + *right) / 2
	return &average
}

func codexRadarText(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > maxRunes {
		return ""
	}
	return value
}

func codexRadarNonNegative(value *float64) *float64 {
	if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 || *value > 1e12 {
		return nil
	}
	copy := *value
	return &copy
}

func parseCodexRadarTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func latestCodexRadarTime(left, right time.Time) time.Time {
	if right.After(left) {
		return right
	}
	return left
}
