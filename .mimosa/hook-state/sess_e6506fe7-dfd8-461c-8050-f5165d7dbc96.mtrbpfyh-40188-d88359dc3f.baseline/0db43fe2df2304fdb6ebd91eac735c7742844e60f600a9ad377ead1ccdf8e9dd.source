package service

import "time"

type OpsDashboardFilter struct {
	StartTime time.Time
	EndTime   time.Time

	Platform string
	GroupID  *int64
	UserID   *int64

	// QueryMode controls whether dashboard queries should use raw logs or pre-aggregated tables.
	// Expected values: auto/raw/preagg (see OpsQueryMode).
	QueryMode OpsQueryMode
}

type OpsRateSummary struct {
	Current float64 `json:"current"`
	Peak    float64 `json:"peak"`
	Avg     float64 `json:"avg"`
}

type OpsPercentiles struct {
	P50 *int `json:"p50_ms"`
	P90 *int `json:"p90_ms"`
	P95 *int `json:"p95_ms"`
	P99 *int `json:"p99_ms"`
	Avg *int `json:"avg_ms"`
	Max *int `json:"max_ms"`
}

type OpsDashboardOverview struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Platform  string    `json:"platform"`
	GroupID   *int64    `json:"group_id"`

	// HealthScore is a backend-computed overall health score (0-100).
	// It is derived from the monitored metrics in this overview, plus best-effort system metrics/job heartbeats.
	HealthScore int `json:"health_score"`

	// Latest system-level snapshot (window=1m, global).
	SystemMetrics *OpsSystemMetricsSnapshot `json:"system_metrics"`

	// Background jobs health (heartbeats).
	JobHeartbeats []*OpsJobHeartbeat `json:"job_heartbeats"`

	SuccessCount         int64 `json:"success_count"`
	ErrorCountTotal      int64 `json:"error_count_total"`
	BusinessLimitedCount int64 `json:"business_limited_count"`

	ErrorCountSLA     int64 `json:"error_count_sla"`
	RequestCountTotal int64 `json:"request_count_total"`
	RequestCountSLA   int64 `json:"request_count_sla"`

	TokenConsumed int64 `json:"token_consumed"`

	SLA                          float64 `json:"sla"`
	ErrorRate                    float64 `json:"error_rate"`
	UpstreamErrorRate            float64 `json:"upstream_error_rate"`
	UpstreamErrorCountExcl429529 int64   `json:"upstream_error_count_excl_429_529"`
	Upstream429Count             int64   `json:"upstream_429_count"`
	Upstream529Count             int64   `json:"upstream_529_count"`

	QPS OpsRateSummary `json:"qps"`
	TPS OpsRateSummary `json:"tps"`

	Duration OpsPercentiles `json:"duration"`
	TTFT     OpsPercentiles `json:"ttft"`
}

// OpsInvestigationErrorGroup is a privacy-safe aggregation used by the Ops
// investigation panel. It intentionally excludes request bodies and messages.
type OpsInvestigationErrorGroup struct {
	Phase      string `json:"phase"`
	Owner      string `json:"owner"`
	Type       string `json:"type"`
	StatusCode int    `json:"status_code"`
	Platform   string `json:"platform"`
	GroupID    *int64 `json:"group_id,omitempty"`
	Count      int64  `json:"count"`
	Total      int64  `json:"-"`
}

// OpsInvestigationFinding is a deterministic, evidence-backed observation.
// Rule is a stable machine-readable identifier; presentation is localized by
// the frontend so API responses do not encode a language preference.
type OpsInvestigationFinding struct {
	Rule          string `json:"rule"`
	Kind          string `json:"kind"` // error | latency
	Severity      string `json:"severity"`
	Phase         string `json:"phase,omitempty"`
	Owner         string `json:"owner,omitempty"`
	StatusCode    int    `json:"status_code,omitempty"`
	Platform      string `json:"platform,omitempty"`
	GroupID       *int64 `json:"group_id,omitempty"`
	CurrentCount  int64  `json:"current_count,omitempty"`
	BaselineCount int64  `json:"baseline_count,omitempty"`
	DeltaCount    int64  `json:"delta_count,omitempty"`
	ChangePercent int64  `json:"change_percent,omitempty"`
	SharePercent  int64  `json:"share_percent,omitempty"`
	CurrentValue  *int   `json:"current_value_ms,omitempty"`
	BaselineValue *int   `json:"baseline_value_ms,omitempty"`
}

type OpsInvestigationResponse struct {
	StartTime     time.Time                  `json:"start_time"`
	EndTime       time.Time                  `json:"end_time"`
	BaselineStart time.Time                  `json:"baseline_start"`
	BaselineEnd   time.Time                  `json:"baseline_end"`
	TotalErrors   int64                      `json:"total_errors"`
	Findings      []*OpsInvestigationFinding `json:"findings"`
}

type OpsLatencyHistogramBucket struct {
	Range string `json:"range"`
	MinMs int    `json:"min_ms"`
	MaxMs *int   `json:"max_ms"`
	Count int64  `json:"count"`
}

type OpsLatencyUserSummary struct {
	UserID        int64  `json:"user_id"`
	Username      string `json:"username"`
	Email         string `json:"email"`
	Deleted       bool   `json:"deleted"`
	RequestCount  int64  `json:"request_count"`
	AvgDurationMs int    `json:"avg_duration_ms"`
}

// OpsLatencyHistogramResponse is a coarse latency distribution histogram (success requests only).
// It is used by the Ops dashboard to quickly identify tail latency regressions.
type OpsLatencyHistogramResponse struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Platform  string    `json:"platform"`
	GroupID   *int64    `json:"group_id"`
	UserID    *int64    `json:"user_id"`

	TotalRequests  int64                        `json:"total_requests"`
	AvgDurationMs  *int                         `json:"avg_duration_ms"`
	TopAvgUsers    []*OpsLatencyUserSummary     `json:"top_avg_users"`
	AvailableUsers []*OpsLatencyUserSummary     `json:"available_users"`
	Buckets        []*OpsLatencyHistogramBucket `json:"buckets"`
}
