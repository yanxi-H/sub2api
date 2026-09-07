package service

import "time"

type OpsThroughputTrendPoint struct {
	BucketStart   time.Time `json:"bucket_start"`
	RequestCount  int64     `json:"request_count"`
	TokenConsumed int64     `json:"token_consumed"`
	SwitchCount   int64     `json:"switch_count"`
	QPS           float64   `json:"qps"`
	TPS           float64   `json:"tps"`
}

type OpsThroughputPlatformBreakdownItem struct {
	Platform      string `json:"platform"`
	RequestCount  int64  `json:"request_count"`
	TokenConsumed int64  `json:"token_consumed"`
}

type OpsThroughputGroupBreakdownItem struct {
	GroupID       int64  `json:"group_id"`
	GroupName     string `json:"group_name"`
	RequestCount  int64  `json:"request_count"`
	TokenConsumed int64  `json:"token_consumed"`
}

type OpsThroughputTrendResponse struct {
	Bucket string `json:"bucket"`

	Points []*OpsThroughputTrendPoint `json:"points"`

	// Optional drilldown helpers:
	// - When no platform/group is selected: returns totals by platform.
	// - When platform is selected but group is not: returns top groups in that platform.
	ByPlatform []*OpsThroughputPlatformBreakdownItem `json:"by_platform,omitempty"`
	TopGroups  []*OpsThroughputGroupBreakdownItem    `json:"top_groups,omitempty"`
}

type OpsLatencyTrendPoint struct {
	BucketStart time.Time      `json:"bucket_start"`
	P50         *int           `json:"p50_ms"`
	P90         *int           `json:"p90_ms"`
	P95         *int           `json:"p95_ms"`
	Avg         *int           `json:"avg_ms"`
	Max         *int           `json:"max_ms"`
	TTFT        OpsPercentiles `json:"ttft"`
	SampleCount int64          `json:"sample_count"`
}

type OpsLatencyTrendResponse struct {
	Bucket string                  `json:"bucket"`
	Points []*OpsLatencyTrendPoint `json:"points"`
}

type OpsErrorTrendPoint struct {
	BucketStart time.Time `json:"bucket_start"`

	ErrorCountTotal      int64 `json:"error_count_total"`
	BusinessLimitedCount int64 `json:"business_limited_count"`
	ErrorCountSLA        int64 `json:"error_count_sla"`

	UpstreamErrorCountExcl429529 int64 `json:"upstream_error_count_excl_429_529"`
	Upstream429Count             int64 `json:"upstream_429_count"`
	Upstream529Count             int64 `json:"upstream_529_count"`
}

type OpsErrorTrendResponse struct {
	Bucket string                `json:"bucket"`
	Points []*OpsErrorTrendPoint `json:"points"`
}

type OpsErrorDistributionItem struct {
	StatusCode      int   `json:"status_code"`
	Total           int64 `json:"total"`
	SLA             int64 `json:"sla"`
	BusinessLimited int64 `json:"business_limited"`
}

type OpsErrorDistributionResponse struct {
	Total int64                       `json:"total"`
	Items []*OpsErrorDistributionItem `json:"items"`
}

type OpsUserErrorTypeCount struct {
	ErrorType string `json:"error_type"`
	Count     int64  `json:"count"`
}

type OpsUserErrorDistributionItem struct {
	UserID   *int64                   `json:"user_id"`
	Username string                   `json:"username"`
	Email    string                   `json:"email"`
	Deleted  bool                     `json:"deleted"`
	Total    int64                    `json:"total"`
	Errors   []*OpsUserErrorTypeCount `json:"errors"`
}

type OpsUserErrorDistributionResponse struct {
	Total      int64                           `json:"total"`
	TotalUsers int                             `json:"total_users"`
	UserLimit  int                             `json:"user_limit"`
	Items      []*OpsUserErrorDistributionItem `json:"items"`
}
