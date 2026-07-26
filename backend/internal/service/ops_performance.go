package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	opsSlowQueueThresholdMs         int64 = 1_000
	opsSlowInternalPhaseMs          int64 = 2_000
	opsSlowTTFTThresholdMs          int64 = 10_000
	opsSlowStreamGapThresholdMs     int64 = 15_000
	opsSlowNonStreamUpstreamMs      int64 = 30_000
	opsPerformanceRequestIDMaxBytes       = 128
	opsPerformancePlatformMaxBytes        = 32
	opsPerformanceModelMaxBytes           = 100
	opsPerformanceCauseMaxBytes           = 32
	opsPerformanceQueryTimeout            = 5 * time.Second
	opsPerformanceCacheTTL                = 30 * time.Second
)

type OpsRequestPerformanceInput struct {
	CreatedAt         time.Time
	RequestID         string
	UserID            int64
	APIKeyID          int64
	AccountID         int64
	GroupID           *int64
	Platform          string
	Model             string
	Stream            bool
	RequestBodyLane   RequestBodyLane
	RequestBodyBytes  int64
	LogicalStatusCode int
	EndToEndMs        int64
	BodyReadMs        int64
	UserQueueMs       int64
	BodyLaneWaitMs    int64
	AccountQueueMs    int64
	RoutingMs         int64
	UpstreamMs        int64
	// TimeToFirstTokenMs is the end-to-end delay observed by the client. The
	// upstream-only value is kept separately so queueing is not blamed on the
	// provider when classifying a slow request.
	TimeToFirstTokenMs int64
	UpstreamTTFTMs     int64
	StreamDurationMs   int64
	MaxStreamGapMs     int64
	FailoverMs         int64
	AttemptCount       int
	AccountSwitchCount int
	FailureCause       string
	SlowCause          string
}

type OpsPerformanceSummary struct {
	SampleCount int64          `json:"sample_count"`
	SlowCount   int64          `json:"slow_count"`
	SlowRate    float64        `json:"slow_rate"`
	EndToEnd    OpsPercentiles `json:"end_to_end"`
	TTFT        OpsPercentiles `json:"ttft"`
}

type OpsSlowCauseSummary struct {
	Cause      string  `json:"cause"`
	Count      int64   `json:"count"`
	Share      float64 `json:"share"`
	E2EP95Ms   *int    `json:"e2e_p95_ms"`
	QueueP95Ms *int    `json:"queue_p95_ms"`
	TTFTP95Ms  *int    `json:"ttft_p95_ms"`
}

type OpsSlowCauseTrendPoint struct {
	BucketStart time.Time        `json:"bucket_start"`
	Causes      map[string]int64 `json:"causes"`
}

type OpsPerformanceImpact struct {
	Dimension    string  `json:"dimension"`
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	RequestCount int64   `json:"request_count"`
	SlowRate     float64 `json:"slow_rate"`
	E2EP95Ms     *int    `json:"e2e_p95_ms"`
	TTFTP95Ms    *int    `json:"ttft_p95_ms"`
	QueueP95Ms   *int    `json:"queue_p95_ms"`
	MainCause    string  `json:"main_cause"`
}

type OpsPerformanceDiagnosticsResponse struct {
	StartTime       time.Time                       `json:"start_time"`
	EndTime         time.Time                       `json:"end_time"`
	Bucket          string                          `json:"bucket"`
	Summary         OpsPerformanceSummary           `json:"summary"`
	Causes          []OpsSlowCauseSummary           `json:"causes"`
	Trend           []OpsSlowCauseTrendPoint        `json:"trend"`
	Impacts         []OpsPerformanceImpact          `json:"impacts"`
	IngestionHealth OpsRequestPerformanceSinkHealth `json:"ingestion_health"`
}

type opsRequestPerformanceRepository interface {
	InsertRequestPerformance(ctx context.Context, input *OpsRequestPerformanceInput) error
	GetPerformanceDiagnostics(ctx context.Context, filter *OpsDashboardFilter, bucketSeconds int) (*OpsPerformanceDiagnosticsResponse, error)
}

type opsRequestPerformanceBatchRepository interface {
	BatchInsertRequestPerformance(ctx context.Context, inputs []*OpsRequestPerformanceInput) (int64, error)
}

type opsPerformanceCacheEntry struct {
	value     *OpsPerformanceDiagnosticsResponse
	expiresAt time.Time
}

func (s *OpsService) RecordRequestPerformance(ctx context.Context, input *OpsRequestPerformanceInput) error {
	if s == nil || input == nil {
		return nil
	}
	if !s.IsMonitoringEnabled(ctx) {
		return nil
	}
	repo, ok := s.opsRepo.(opsRequestPerformanceRepository)
	if !ok {
		return nil
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}
	input.RequestID = truncateUTF8(strings.TrimSpace(input.RequestID), opsPerformanceRequestIDMaxBytes)
	input.Platform = truncateUTF8(strings.TrimSpace(input.Platform), opsPerformancePlatformMaxBytes)
	input.Model = truncateUTF8(strings.TrimSpace(input.Model), opsPerformanceModelMaxBytes)
	input.FailureCause = truncateUTF8(strings.TrimSpace(input.FailureCause), opsPerformanceCauseMaxBytes)
	if input.SlowCause == "" {
		input.SlowCause = ClassifyOpsSlowCause(input)
	}
	input.SlowCause = truncateUTF8(strings.TrimSpace(input.SlowCause), opsPerformanceCauseMaxBytes)
	if s.performanceSink != nil {
		s.performanceSink.Enqueue(input)
		return nil
	}
	return repo.InsertRequestPerformance(ctx, input)
}

func (s *OpsService) GetPerformanceDiagnostics(ctx context.Context, filter *OpsDashboardFilter, bucketSeconds int) (*OpsPerformanceDiagnosticsResponse, error) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	repo, ok := s.opsRepo.(opsRequestPerformanceRepository)
	if !ok {
		return s.withPerformanceIngestionHealth(&OpsPerformanceDiagnosticsResponse{
			Causes: []OpsSlowCauseSummary{}, Trend: []OpsSlowCauseTrendPoint{}, Impacts: []OpsPerformanceImpact{},
		}), nil
	}
	if filter == nil {
		return nil, fmt.Errorf("performance diagnostics filter is required")
	}
	key := performanceDiagnosticsCacheKey(filter, bucketSeconds)
	now := time.Now()
	s.performanceDiagnosticsMu.RLock()
	entry, found := opsPerformanceCacheEntry{}, false
	if s.performanceDiagnosticsCache != nil {
		entry, found = s.performanceDiagnosticsCache[key]
	}
	s.performanceDiagnosticsMu.RUnlock()
	if found && now.Before(entry.expiresAt) {
		return s.withPerformanceIngestionHealth(entry.value), nil
	}
	filterCopy := *filter
	resultCh := s.performanceDiagnosticsSF.DoChan(key, func() (any, error) {
		queryCtx, cancel := context.WithTimeout(context.Background(), opsPerformanceQueryTimeout)
		defer cancel()
		value, err := repo.GetPerformanceDiagnostics(queryCtx, &filterCopy, bucketSeconds)
		if err != nil {
			return nil, err
		}
		cacheNow := time.Now()
		s.performanceDiagnosticsMu.Lock()
		if s.performanceDiagnosticsCache == nil {
			s.performanceDiagnosticsCache = make(map[string]opsPerformanceCacheEntry)
		}
		for cacheKey, cached := range s.performanceDiagnosticsCache {
			if !cacheNow.Before(cached.expiresAt) {
				delete(s.performanceDiagnosticsCache, cacheKey)
			}
		}
		s.performanceDiagnosticsCache[key] = opsPerformanceCacheEntry{value: value, expiresAt: cacheNow.Add(opsPerformanceCacheTTL)}
		s.performanceDiagnosticsMu.Unlock()
		return value, nil
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return nil, result.Err
		}
		value, _ := result.Val.(*OpsPerformanceDiagnosticsResponse)
		return s.withPerformanceIngestionHealth(value), nil
	}
}

func (s *OpsService) withPerformanceIngestionHealth(value *OpsPerformanceDiagnosticsResponse) *OpsPerformanceDiagnosticsResponse {
	if value == nil {
		return nil
	}
	cloned := *value
	if s != nil && s.performanceSink != nil {
		cloned.IngestionHealth = s.performanceSink.Health()
	}
	return &cloned
}

func ClassifyOpsSlowCause(input *OpsRequestPerformanceInput) string {
	if input == nil {
		return "healthy"
	}
	if cause := strings.TrimSpace(input.FailureCause); cause != "" {
		return cause
	}
	type candidate struct {
		name string
		ms   int64
		min  int64
	}
	laneCause := "request_body_lane_queue"
	switch input.RequestBodyLane {
	case RequestBodyLaneHeavy:
		laneCause = "heavy_queue"
	case RequestBodyLaneRecovery:
		laneCause = "recovery_queue"
	}
	candidates := []candidate{
		{name: "user_queue", ms: input.UserQueueMs, min: opsSlowQueueThresholdMs},
		{name: laneCause, ms: input.BodyLaneWaitMs, min: opsSlowQueueThresholdMs},
		{name: "account_queue", ms: input.AccountQueueMs, min: opsSlowQueueThresholdMs},
		{name: "body_read", ms: input.BodyReadMs, min: opsSlowInternalPhaseMs},
		{name: "routing", ms: input.RoutingMs, min: opsSlowInternalPhaseMs},
	}
	if input.AccountSwitchCount > 0 || input.AttemptCount > 1 {
		candidates = append(candidates, candidate{name: "failover", ms: input.FailoverMs, min: opsSlowQueueThresholdMs})
	}
	if input.Stream {
		candidates = append(candidates,
			candidate{name: "upstream_ttft", ms: input.UpstreamTTFTMs, min: opsSlowTTFTThresholdMs},
			candidate{name: "stream_gap", ms: input.MaxStreamGapMs, min: opsSlowStreamGapThresholdMs},
		)
	} else {
		candidates = append(candidates, candidate{name: "upstream", ms: input.UpstreamMs, min: opsSlowNonStreamUpstreamMs})
	}
	winner := candidate{}
	for _, item := range candidates {
		if item.ms >= item.min && item.ms > winner.ms {
			winner = item
		}
	}
	if winner.name == "" {
		return "healthy"
	}
	return winner.name
}

func performanceDiagnosticsCacheKey(filter *OpsDashboardFilter, bucketSeconds int) string {
	groupID := int64(0)
	if filter.GroupID != nil {
		groupID = *filter.GroupID
	}
	return fmt.Sprintf("%d:%d:%s:%d:%d",
		filter.StartTime.UnixNano(),
		filter.EndTime.UnixNano(),
		strings.TrimSpace(filter.Platform), groupID, bucketSeconds,
	)
}
