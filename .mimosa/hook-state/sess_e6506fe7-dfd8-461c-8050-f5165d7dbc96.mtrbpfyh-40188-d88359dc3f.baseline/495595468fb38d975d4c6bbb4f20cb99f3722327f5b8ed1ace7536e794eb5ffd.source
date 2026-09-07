package service

import "time"

const (
	MonitorCenterStatusOperational         = "operational"
	MonitorCenterStatusDegradedPerformance = "degraded_performance"
	MonitorCenterStatusPartialOutage       = "partial_outage"
	MonitorCenterStatusMajorOutage         = "major_outage"
	MonitorCenterStatusUnderMaintenance    = "under_maintenance"
	MonitorCenterStatusUnknown             = "unknown"
)

type MonitorCenterComponentStatus struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Matched bool   `json:"matched"`
}

type MonitorCenterServiceGroup struct {
	Key        string                         `json:"key"`
	Name       string                         `json:"name"`
	Status     string                         `json:"status"`
	Components []MonitorCenterComponentStatus `json:"components"`
}

type MonitorCenterIncidentUpdate struct {
	Status    string    `json:"status"`
	Body      string    `json:"body"`
	UpdatedAt time.Time `json:"updated_at"`
}

type MonitorCenterIncident struct {
	ID                 string                        `json:"id"`
	Name               string                        `json:"name"`
	Status             string                        `json:"status"`
	Impact             string                        `json:"impact"`
	AffectedComponents []string                      `json:"affected_components"`
	AffectedGroups     []string                      `json:"affected_groups"`
	StartedAt          *time.Time                    `json:"started_at,omitempty"`
	CreatedAt          *time.Time                    `json:"created_at,omitempty"`
	UpdatedAt          time.Time                     `json:"updated_at"`
	ResolvedAt         *time.Time                    `json:"resolved_at,omitempty"`
	URL                string                        `json:"url,omitempty"`
	LatestUpdate       *MonitorCenterIncidentUpdate  `json:"latest_update,omitempty"`
	Updates            []MonitorCenterIncidentUpdate `json:"updates"`
}

type MonitorCenterOpenAIStatus struct {
	OverallStatus      string                      `json:"overall_status"`
	OverallDescription string                      `json:"overall_description"`
	Groups             []MonitorCenterServiceGroup `json:"groups"`
	Incidents          []MonitorCenterIncident     `json:"incidents"`
	LastAttemptAt      *time.Time                  `json:"last_attempt_at,omitempty"`
	LastSuccessAt      *time.Time                  `json:"last_success_at,omitempty"`
	FetchStatus        string                      `json:"fetch_status"`
	FetchLatencyMs     int                         `json:"fetch_latency_ms"`
	Stale              bool                        `json:"stale"`
}

type MonitorCenterOpenAIHistoryPoint struct {
	Timestamp           time.Time           `json:"timestamp"`
	OverallStatus       string              `json:"overall_status"`
	APIStatus           string              `json:"api_status"`
	ChatGPTStatus       string              `json:"chatgpt_status"`
	CodexStatus         string              `json:"codex_status"`
	ActiveIncidentCount int                 `json:"active_incident_count"`
	FetchStatus         string              `json:"fetch_status"`
	LatencyMs           int                 `json:"latency_ms"`
	FailureReason       string              `json:"failure_reason,omitempty"`
	IncidentRefs        map[string][]string `json:"incident_refs,omitempty"`
}

type MonitorCenterOpenAIGroupStatistics struct {
	SampleCount      int     `json:"sample_count"`
	KnownSampleCount int     `json:"known_sample_count"`
	OperationalCount int     `json:"operational_count"`
	AvailabilityPct  float64 `json:"availability_pct"`
}

type MonitorCenterOpenAIHistoryStatistics struct {
	SampleCount      int                                           `json:"sample_count"`
	SuccessfulCount  int                                           `json:"successful_count"`
	FetchSuccessPct  float64                                       `json:"fetch_success_pct"`
	AverageLatencyMs float64                                       `json:"average_latency_ms"`
	AnomalyCount     int                                           `json:"anomaly_count"`
	Groups           map[string]MonitorCenterOpenAIGroupStatistics `json:"groups"`
}

type MonitorCenterOpenAIHistory struct {
	StartTime  time.Time                            `json:"start_time"`
	EndTime    time.Time                            `json:"end_time"`
	Bucket     string                               `json:"bucket"`
	Points     []MonitorCenterOpenAIHistoryPoint    `json:"points"`
	Statistics MonitorCenterOpenAIHistoryStatistics `json:"statistics"`
	Incidents  []MonitorCenterIncident              `json:"incidents"`
}

type MonitorCenterProbePoint struct {
	Timestamp     time.Time `json:"timestamp"`
	Status        string    `json:"status"`
	LatencyMs     *int      `json:"latency_ms"`
	FailureReason string    `json:"failure_reason,omitempty"`
}

type MonitorCenterProbe struct {
	Configured          bool                      `json:"configured"`
	MonitorID           int64                     `json:"monitor_id,omitempty"`
	MonitorName         string                    `json:"monitor_name,omitempty"`
	Endpoint            string                    `json:"endpoint,omitempty"`
	Model               string                    `json:"model,omitempty"`
	EndpointKind        string                    `json:"endpoint_kind,omitempty"`
	Status              string                    `json:"status"`
	LatencyMs           *int                      `json:"latency_ms,omitempty"`
	FailureReason       string                    `json:"failure_reason,omitempty"`
	ConsecutiveFailures int                       `json:"consecutive_failures"`
	LastCheckedAt       *time.Time                `json:"last_checked_at,omitempty"`
	LastSuccessAt       *time.Time                `json:"last_success_at,omitempty"`
	Points              []MonitorCenterProbePoint `json:"points"`
}
