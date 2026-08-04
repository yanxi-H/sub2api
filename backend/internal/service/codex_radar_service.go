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
	codexRadarInsightsURL       = "https://codexradar.com/api/radar-insights"
	codexRadarIntelligenceURL   = "https://codexradar.com/api/intelligence-efficiency"
	codexRadarRequestTimeout    = 12 * time.Second
	codexRadarResponseHeaderTTL = 8 * time.Second

	codexRadarInsightsBodyLimit     int64 = 512 << 10
	codexRadarIntelligenceBodyLimit int64 = 5 << 20

	codexRadarMaxStationItems = 2
)

// CodexRadarDashboardRecommendations contains the limited public data shown on
// the user dashboard. It deliberately excludes raw task and runner data.
type CodexRadarDashboardRecommendations struct {
	SourceUpdatedAt             string                               `json:"source_updated_at,omitempty"`
	StationAvailable            bool                                 `json:"station_available"`
	IntelligenceAvailable       bool                                 `json:"intelligence_available"`
	StationRecommendations      []CodexRadarStationRecommendationSet `json:"station_recommendations"`
	IntelligenceRecommendations []CodexRadarIntelligenceMetric       `json:"intelligence_recommendations"`
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
	Model                  string   `json:"model"`
	Effort                 string   `json:"effort"`
	IQ                     float64  `json:"iq"`
	Samples                int      `json:"samples"`
	AverageCostUSD         *float64 `json:"average_cost_usd"`
	AverageDurationMinutes *float64 `json:"average_duration_minutes"`
}

type CodexRadarService struct {
	httpClient                *http.Client
	insightsURL               string
	intelligenceEfficiencyURL string
}

// NewCodexRadarService builds a direct, DNS-rebinding-protected client for the
// two fixed CodexRadar endpoints. It never inherits an environment proxy.
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

	return newCodexRadarService(client, codexRadarInsightsURL, codexRadarIntelligenceURL), nil
}

func newCodexRadarService(client *http.Client, insightsURL, intelligenceEfficiencyURL string) *CodexRadarService {
	if client == nil {
		client = &http.Client{Timeout: codexRadarRequestTimeout}
	}

	clientCopy := *client
	clientCopy.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &CodexRadarService{
		httpClient:                &clientCopy,
		insightsURL:               insightsURL,
		intelligenceEfficiencyURL: intelligenceEfficiencyURL,
	}
}

// GetDashboardRecommendations loads both independent public sources in
// parallel. A temporary failure in either source still leaves the other source
// available to the dashboard.
func (s *CodexRadarService) GetDashboardRecommendations(ctx context.Context) (*CodexRadarDashboardRecommendations, error) {
	if s == nil || s.httpClient == nil {
		return nil, infraerrors.ServiceUnavailable("CODEX_RADAR_UNAVAILABLE", "Model recommendations are temporarily unavailable")
	}

	type stationResult struct {
		groups    []CodexRadarStationRecommendationSet
		updatedAt time.Time
		err       error
	}
	type intelligenceResult struct {
		metrics   []CodexRadarIntelligenceMetric
		updatedAt time.Time
		err       error
	}

	stationCh := make(chan stationResult, 1)
	intelligenceCh := make(chan intelligenceResult, 1)

	go func() {
		groups, updatedAt, err := s.fetchStationRecommendations(ctx)
		stationCh <- stationResult{groups: groups, updatedAt: updatedAt, err: err}
	}()
	go func() {
		metrics, updatedAt, err := s.fetchIntelligenceMetrics(ctx)
		intelligenceCh <- intelligenceResult{metrics: metrics, updatedAt: updatedAt, err: err}
	}()

	station := <-stationCh
	intelligence := <-intelligenceCh
	if station.err != nil && intelligence.err != nil {
		return nil, infraerrors.ServiceUnavailable("CODEX_RADAR_UNAVAILABLE", "Model recommendations are temporarily unavailable")
	}

	updatedAt := latestCodexRadarTime(station.updatedAt, intelligence.updatedAt)
	result := &CodexRadarDashboardRecommendations{
		StationAvailable:            station.err == nil,
		IntelligenceAvailable:       intelligence.err == nil,
		StationRecommendations:      station.groups,
		IntelligenceRecommendations: intelligence.metrics,
	}
	if !updatedAt.IsZero() {
		result.SourceUpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	}

	return result, nil
}

func (s *CodexRadarService) fetchStationRecommendations(ctx context.Context) ([]CodexRadarStationRecommendationSet, time.Time, error) {
	var payload codexRadarInsightsPayload
	if err := s.fetchJSON(ctx, s.insightsURL, codexRadarInsightsBodyLimit, &payload); err != nil {
		return nil, time.Time{}, err
	}
	if payload.Schema != 1 {
		return nil, time.Time{}, fmt.Errorf("unsupported CodexRadar insights schema")
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
		return nil, time.Time{}, fmt.Errorf("CodexRadar returned no station recommendations")
	}

	return sets, parseCodexRadarTime(payload.SourceUpdatedAt), nil
}

func (s *CodexRadarService) fetchIntelligenceMetrics(ctx context.Context) ([]CodexRadarIntelligenceMetric, time.Time, error) {
	var payload codexRadarIntelligencePayload
	if err := s.fetchJSON(ctx, s.intelligenceEfficiencyURL, codexRadarIntelligenceBodyLimit, &payload); err != nil {
		return nil, time.Time{}, err
	}
	if payload.Schema != 1 || len(payload.Combos) == 0 || len(payload.Tasks) == 0 || len(payload.Cells) == 0 {
		return nil, time.Time{}, fmt.Errorf("invalid CodexRadar intelligence payload")
	}

	metrics := make([]CodexRadarIntelligenceMetric, 0, len(payload.Combos))
	seenCombinations := make(map[string]struct{}, len(payload.Combos))
	var updatedAt time.Time
	for _, combo := range payload.Combos {
		model := codexRadarText(combo.Model, 128)
		effort := codexRadarText(combo.Effort, 32)
		if model == "" || effort == "" {
			continue
		}
		combinationKey := model + "|" + effort
		if _, seen := seenCombinations[combinationKey]; seen {
			continue
		}

		passed := 0
		validTasks := 0
		priceSum := 0.0
		priceSamples := 0
		durationSum := 0.0
		durationSamples := 0
		for _, task := range payload.Tasks {
			taskID := codexRadarTaskID(task.ID)
			if taskID == "" {
				continue
			}
			cell, ok := payload.Cells[taskID+"|"+combinationKey]
			if !ok || len(cell.RanBy) == 0 {
				continue
			}
			runner := cell.RanBy[0]
			if runner.Passed == nil {
				continue
			}

			validTasks++
			if *runner.Passed {
				passed++
			}
			if duration := codexRadarPositive(runner.DurationSeconds); duration != nil {
				durationSum += *duration / 60
				durationSamples++
			}
			if price := codexRadarNonNegative(runner.ActualCostUSD); price != nil && (effort != "ultra" || runner.CostComplete) {
				priceSum += *price
				priceSamples++
			}
			updatedAt = latestCodexRadarTime(updatedAt, parseCodexRadarTime(runner.GradedAt))
		}
		if validTasks == 0 {
			continue
		}

		seenCombinations[combinationKey] = struct{}{}
		metric := CodexRadarIntelligenceMetric{
			Model:   model,
			Effort:  effort,
			IQ:      float64(passed) / float64(validTasks) * 150,
			Samples: validTasks,
		}
		if priceSamples > 0 {
			average := priceSum / float64(priceSamples)
			metric.AverageCostUSD = &average
		}
		if durationSamples > 0 {
			average := durationSum / float64(durationSamples)
			metric.AverageDurationMinutes = &average
		}
		metrics = append(metrics, metric)
	}
	if len(metrics) == 0 {
		return nil, time.Time{}, fmt.Errorf("CodexRadar returned no intelligence metrics")
	}

	return metrics, updatedAt, nil
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
	Schema          int                           `json:"schema"`
	SourceUpdatedAt string                        `json:"source_updated_at"`
	Recommendations []codexRadarStationSetPayload `json:"recommendations"`
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

type codexRadarIntelligencePayload struct {
	Schema int                              `json:"schema"`
	Combos []codexRadarCombinationPayload   `json:"combos"`
	Tasks  []codexRadarTaskPayload          `json:"tasks"`
	Cells  map[string]codexRadarCellPayload `json:"cells"`
}

type codexRadarCombinationPayload struct {
	Model  string `json:"model"`
	Effort string `json:"effort"`
}

type codexRadarTaskPayload struct {
	ID any `json:"id"`
}

type codexRadarCellPayload struct {
	RanBy []codexRadarRunnerPayload `json:"ran_by"`
}

type codexRadarRunnerPayload struct {
	Passed          *bool    `json:"passed"`
	DurationSeconds *float64 `json:"duration_sec"`
	ActualCostUSD   *float64 `json:"actual_cost_usd"`
	CostComplete    bool     `json:"cost_complete"`
	GradedAt        string   `json:"graded_at"`
}

func codexRadarText(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > maxRunes {
		return ""
	}
	return value
}

func codexRadarTaskID(value any) string {
	switch id := value.(type) {
	case string:
		return codexRadarText(id, 128)
	case float64:
		if math.IsNaN(id) || math.IsInf(id, 0) {
			return ""
		}
		return fmt.Sprintf("%v", id)
	default:
		return ""
	}
}

func codexRadarNonNegative(value *float64) *float64 {
	if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 || *value > 1e12 {
		return nil
	}
	copy := *value
	return &copy
}

func codexRadarPositive(value *float64) *float64 {
	if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) || *value <= 0 || *value > 1e12 {
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
