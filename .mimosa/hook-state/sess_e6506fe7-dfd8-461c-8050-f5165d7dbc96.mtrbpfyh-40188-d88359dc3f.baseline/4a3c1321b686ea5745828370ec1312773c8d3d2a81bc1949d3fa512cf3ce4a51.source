package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	monitorCenterOpenAIURL          = "https://status.openai.com/api/v2/summary.json"
	monitorCenterPollInterval       = time.Minute
	monitorCenterHTTPTimeout        = 5 * time.Second
	monitorCenterRetryDelay         = 300 * time.Millisecond
	monitorCenterCacheTTL           = 3 * time.Minute
	monitorCenterRetention          = 72 * time.Hour
	monitorCenterRedisPollLockTTL   = 55 * time.Second
	monitorCenterMaxResponseBytes   = 2 << 20
	monitorCenterFetchStatusSuccess = "success"
	monitorCenterFetchStatusFailed  = "failed"
)

type monitorCenterRepository interface {
	UpsertMonitorCenterOpenAIHistory(ctx context.Context, point *MonitorCenterOpenAIHistoryPoint) error
	InsertMonitorCenterOpenAIEvent(ctx context.Context, observedAt time.Time, contentHash string, status *MonitorCenterOpenAIStatus) error
	UpsertMonitorCenterIncidentUpdates(ctx context.Context, incidents []MonitorCenterIncident) error
	UpsertMonitorCenterIncidents(ctx context.Context, observedAt time.Time, incidents []MonitorCenterIncident) error
	ResolveMissingMonitorCenterIncidents(ctx context.Context, observedAt time.Time, activeIncidentIDs []string) error
	ListMonitorCenterOpenAIHistory(ctx context.Context, start, end time.Time) ([]MonitorCenterOpenAIHistoryPoint, error)
	ListMonitorCenterIncidents(ctx context.Context, start, end time.Time) ([]MonitorCenterIncident, error)
	DeleteMonitorCenterOpenAIBefore(ctx context.Context, before time.Time) error
	LoadLatestMonitorCenterOpenAIEventHash(ctx context.Context) (string, error)
}

type MonitorCenterCache interface {
	TryAcquireMonitorCenterPollLock(ctx context.Context, owner string, ttl time.Duration) (bool, error)
	StoreMonitorCenterOpenAIStatus(ctx context.Context, payload []byte, ttl time.Duration) error
	LoadMonitorCenterOpenAIStatus(ctx context.Context) ([]byte, error)
}

type monitorCenterCacheEnvelope struct {
	Status       *MonitorCenterOpenAIStatus `json:"status"`
	ContentHash  string                     `json:"content_hash"`
	ETag         string                     `json:"etag,omitempty"`
	LastModified string                     `json:"last_modified,omitempty"`
}

type monitorCenterComponentRule struct {
	Key     string
	Name    string
	Aliases []string
}

type monitorCenterGroupRule struct {
	Key        string
	Name       string
	Components []monitorCenterComponentRule
}

// Keep component matching in one place. Aliases are exact, case-insensitive
// official names; missing components remain unknown instead of being guessed.
var monitorCenterOpenAIGroupRules = []monitorCenterGroupRule{
	{
		Key: "api", Name: "API",
		Components: []monitorCenterComponentRule{
			{Key: "responses", Name: "Responses", Aliases: []string{"Responses", "Responses API"}},
			{Key: "chat_completions", Name: "Chat Completions", Aliases: []string{"Chat Completions", "Chat Completion"}},
			{Key: "files", Name: "Files", Aliases: []string{"Files"}},
		},
	},
	{
		Key: "chatgpt", Name: "ChatGPT",
		Components: []monitorCenterComponentRule{
			{Key: "login", Name: "Login", Aliases: []string{"Login"}},
			{Key: "conversations", Name: "Conversations", Aliases: []string{"Conversations"}},
			{Key: "file_uploads", Name: "File Uploads", Aliases: []string{"File Uploads", "Uploads"}},
		},
	},
	{
		Key: "codex", Name: "Codex",
		Components: []monitorCenterComponentRule{
			{Key: "codex_api", Name: "Codex API", Aliases: []string{"Codex API"}},
			{Key: "codex_web", Name: "Codex Web", Aliases: []string{"Codex Web"}},
			{Key: "codex_desktop", Name: "Codex in ChatGPT Desktop", Aliases: []string{"Codex in ChatGPT Desktop"}},
			{Key: "codex_cli", Name: "Codex CLI", Aliases: []string{"Codex CLI", "CLI"}},
		},
	},
}

type openAISummaryPayload struct {
	Status struct {
		Indicator   string `json:"indicator"`
		Description string `json:"description"`
	} `json:"status"`
	Components []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"components"`
	Incidents []struct {
		ID         string     `json:"id"`
		Name       string     `json:"name"`
		Status     string     `json:"status"`
		Impact     string     `json:"impact"`
		Shortlink  string     `json:"shortlink"`
		CreatedAt  *time.Time `json:"created_at"`
		StartedAt  *time.Time `json:"started_at"`
		UpdatedAt  time.Time  `json:"updated_at"`
		ResolvedAt *time.Time `json:"resolved_at"`
		Components []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"components"`
		IncidentUpdates []struct {
			Status    string    `json:"status"`
			Body      string    `json:"body"`
			UpdatedAt time.Time `json:"updated_at"`
		} `json:"incident_updates"`
	} `json:"incidents"`
}

type MonitorCenterService struct {
	repo           monitorCenterRepository
	channelMonitor *ChannelMonitorService
	cache          MonitorCenterCache
	httpClient     *http.Client
	instanceID     string

	mu           sync.RWMutex
	current      *MonitorCenterOpenAIStatus
	contentHash  string
	etag         string
	lastModified string

	refreshMu sync.Mutex
	stopCh    chan struct{}
	doneCh    chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once
}

func NewMonitorCenterService(
	opsRepo OpsRepository,
	channelMonitor *ChannelMonitorService,
	cache MonitorCenterCache,
) *MonitorCenterService {
	repo, _ := opsRepo.(monitorCenterRepository)
	return &MonitorCenterService{
		repo:           repo,
		channelMonitor: channelMonitor,
		cache:          cache,
		httpClient:     &http.Client{Timeout: monitorCenterHTTPTimeout},
		instanceID:     uuid.NewString(),
		stopCh:         make(chan struct{}),
		doneCh:         make(chan struct{}),
	}
}

func ProvideMonitorCenterService(
	opsRepo OpsRepository,
	channelMonitor *ChannelMonitorService,
	cache MonitorCenterCache,
) *MonitorCenterService {
	svc := NewMonitorCenterService(opsRepo, channelMonitor, cache)
	svc.Start()
	return svc
}

func (s *MonitorCenterService) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		go s.loop()
	})
}

func (s *MonitorCenterService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	select {
	case <-s.doneCh:
	case <-time.After(monitorCenterHTTPTimeout + time.Second):
	}
}

func (s *MonitorCenterService) loop() {
	defer close(s.doneCh)
	defer func() {
		if r := recover(); r != nil {
			slog.Error("monitor center: background loop panic recovered", "panic", r)
		}
	}()
	s.loadRedisCache(context.Background())
	if s.currentContentHash() == "" && s.repo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), monitorCenterHTTPTimeout)
		if hash, err := s.repo.LoadLatestMonitorCenterOpenAIEventHash(ctx); err != nil {
			slog.Warn("monitor center: load latest OpenAI event hash failed", "error", err)
		} else if hash != "" {
			s.mu.Lock()
			s.contentHash = hash
			s.mu.Unlock()
		}
		cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*monitorCenterHTTPTimeout+monitorCenterRetryDelay)
	_ = s.RefreshOpenAIStatus(ctx)
	cancel()

	ticker := time.NewTicker(monitorCenterPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("monitor center: refresh tick panic recovered", "panic", r)
					}
				}()
				ctx, cancel := context.WithTimeout(context.Background(), 2*monitorCenterHTTPTimeout+monitorCenterRetryDelay)
				_ = s.RefreshOpenAIStatus(ctx)
				cancel()
			}()
		case <-s.stopCh:
			return
		}
	}
}

func (s *MonitorCenterService) GetOpenAIStatus(ctx context.Context) (*MonitorCenterOpenAIStatus, error) {
	if s == nil {
		return unknownMonitorCenterOpenAIStatus(), nil
	}
	if current := s.currentStatus(); current != nil {
		markMonitorCenterStale(current)
		return current, nil
	}
	s.loadRedisCache(ctx)
	if current := s.currentStatus(); current != nil {
		markMonitorCenterStale(current)
		return current, nil
	}
	if err := s.RefreshOpenAIStatus(ctx); err != nil {
		return unknownMonitorCenterOpenAIStatus(), nil
	}
	current := s.currentStatus()
	if current == nil {
		return unknownMonitorCenterOpenAIStatus(), nil
	}
	markMonitorCenterStale(current)
	return current, nil
}

func (s *MonitorCenterService) GetOpenAIHistory(ctx context.Context, start, end time.Time) (*MonitorCenterOpenAIHistory, error) {
	if s == nil || s.repo == nil {
		return emptyMonitorCenterOpenAIHistory(start, end), nil
	}
	points, err := s.repo.ListMonitorCenterOpenAIHistory(ctx, start, end)
	if err != nil {
		return nil, err
	}
	incidents, err := s.repo.ListMonitorCenterIncidents(ctx, start, end)
	if err != nil {
		return nil, err
	}
	return &MonitorCenterOpenAIHistory{
		StartTime:  start,
		EndTime:    end,
		Bucket:     "minute",
		Points:     points,
		Statistics: monitorCenterHistoryStatistics(points),
		Incidents:  incidents,
	}, nil
}

func (s *MonitorCenterService) GetProbe(ctx context.Context, start, end time.Time) (*MonitorCenterProbe, error) {
	if s == nil || s.channelMonitor == nil {
		return &MonitorCenterProbe{Configured: false, Status: MonitorCenterStatusUnknown, Points: []MonitorCenterProbePoint{}}, nil
	}
	return s.channelMonitor.GetMonitorCenterProbe(ctx, start, end)
}

func (s *MonitorCenterService) RefreshOpenAIStatus(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	if !s.acquirePollLock(ctx) {
		s.loadRedisCache(ctx)
		return nil
	}

	startedAt := time.Now()
	payload, etag, lastModified, notModified, err := s.fetchOpenAISummaryWithRetry(ctx)
	latencyMs := int(time.Since(startedAt) / time.Millisecond)
	now := time.Now().UTC()
	if err != nil {
		s.recordFetchFailure(ctx, now, latencyMs, err)
		return err
	}

	if notModified {
		current := s.currentStatus()
		if current == nil {
			s.loadRedisCache(ctx)
			current = s.currentStatus()
		}
		if current == nil {
			err := fmt.Errorf("openai status returned 304 without a cached snapshot")
			s.recordFetchFailure(ctx, now, latencyMs, err)
			return err
		}
		current.LastAttemptAt = timePointer(now)
		current.LastSuccessAt = timePointer(now)
		current.FetchStatus = monitorCenterFetchStatusSuccess
		current.FetchLatencyMs = latencyMs
		current.Stale = false
		s.publish(current, s.currentContentHash(), monitorCenterFirstNonEmpty(etag, s.currentETag()), monitorCenterFirstNonEmpty(lastModified, s.currentLastModified()))
		s.persistSample(ctx, current, false)
		return nil
	}

	status := normalizeMonitorCenterOpenAIStatus(payload, now, latencyMs)
	hash, err := hashMonitorCenterStatus(status)
	if err != nil {
		return err
	}
	changed := hash != s.currentContentHash()
	s.publish(status, hash, etag, lastModified)
	s.persistSample(ctx, status, changed)
	return nil
}

func (s *MonitorCenterService) fetchOpenAISummaryWithRetry(ctx context.Context) (
	*openAISummaryPayload,
	string,
	string,
	bool,
	error,
) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		payload, etag, modified, notModified, err := s.fetchOpenAISummary(ctx)
		if err == nil {
			return payload, etag, modified, notModified, nil
		}
		lastErr = err
		if attempt == 0 {
			select {
			case <-time.After(monitorCenterRetryDelay):
			case <-ctx.Done():
				return nil, "", "", false, ctx.Err()
			}
		}
	}
	return nil, "", "", false, lastErr
}

func (s *MonitorCenterService) fetchOpenAISummary(ctx context.Context) (
	*openAISummaryPayload,
	string,
	string,
	bool,
	error,
) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, monitorCenterOpenAIURL, nil)
	if err != nil {
		return nil, "", "", false, err
	}
	if etag := s.currentETag(); etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if modified := s.currentLastModified(); modified != "" {
		req.Header.Set("If-Modified-Since", modified)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", "", false, err
	}
	defer func() { _ = resp.Body.Close() }()
	etag := strings.TrimSpace(resp.Header.Get("ETag"))
	modified := strings.TrimSpace(resp.Header.Get("Last-Modified"))
	if resp.StatusCode == http.StatusNotModified {
		return nil, etag, modified, true, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return nil, etag, modified, false, fmt.Errorf("openai status returned HTTP %d", resp.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, monitorCenterMaxResponseBytes))
	var payload openAISummaryPayload
	if err := decoder.Decode(&payload); err != nil {
		return nil, etag, modified, false, err
	}
	return &payload, etag, modified, false, nil
}

func (s *MonitorCenterService) persistSample(ctx context.Context, status *MonitorCenterOpenAIStatus, changed bool) {
	if status == nil {
		return
	}
	s.storeRedisCache(ctx)
	if s.repo == nil {
		return
	}
	point := monitorCenterHistoryPoint(status)
	if err := s.repo.UpsertMonitorCenterOpenAIHistory(ctx, point); err != nil {
		slog.Warn("monitor center: persist OpenAI history failed", "error", err)
	}
	if changed {
		observedAt := time.Now().UTC()
		if status.LastSuccessAt != nil {
			observedAt = status.LastSuccessAt.UTC()
		}
		if err := s.repo.InsertMonitorCenterOpenAIEvent(ctx, observedAt, s.currentContentHash(), status); err != nil {
			slog.Warn("monitor center: persist OpenAI status event failed", "error", err)
		}
	}
	if err := s.repo.UpsertMonitorCenterIncidentUpdates(ctx, status.Incidents); err != nil {
		slog.Warn("monitor center: persist OpenAI incident updates failed", "error", err)
	}
	observedAt := time.Now().UTC()
	if status.LastAttemptAt != nil {
		observedAt = status.LastAttemptAt.UTC()
	}
	if err := s.repo.UpsertMonitorCenterIncidents(ctx, observedAt, status.Incidents); err != nil {
		slog.Warn("monitor center: persist OpenAI incidents failed", "error", err)
	}
	activeIncidentIDs := make([]string, 0, len(status.Incidents))
	for _, incident := range status.Incidents {
		if incident.ID != "" && !strings.EqualFold(incident.Status, "resolved") {
			activeIncidentIDs = append(activeIncidentIDs, incident.ID)
		}
	}
	if err := s.repo.ResolveMissingMonitorCenterIncidents(ctx, observedAt, activeIncidentIDs); err != nil {
		slog.Warn("monitor center: reconcile resolved OpenAI incidents failed", "error", err)
	}
	if time.Now().UTC().Minute() == 0 {
		if err := s.repo.DeleteMonitorCenterOpenAIBefore(ctx, time.Now().UTC().Add(-monitorCenterRetention)); err != nil {
			slog.Warn("monitor center: prune OpenAI history failed", "error", err)
		}
	}
}

func (s *MonitorCenterService) recordFetchFailure(ctx context.Context, now time.Time, latencyMs int, fetchErr error) {
	current := s.currentStatus()
	if current == nil {
		current = unknownMonitorCenterOpenAIStatus()
	}
	current.LastAttemptAt = timePointer(now)
	current.FetchStatus = monitorCenterFetchStatusFailed
	current.FetchLatencyMs = latencyMs
	current.Stale = true
	s.publish(current, s.currentContentHash(), s.currentETag(), s.currentLastModified())
	s.storeRedisCache(ctx)
	if s.repo != nil {
		point := monitorCenterHistoryPoint(current)
		if fetchErr != nil {
			point.FailureReason = monitorCenterFailureReason(fetchErr)
		}
		if err := s.repo.UpsertMonitorCenterOpenAIHistory(ctx, point); err != nil {
			slog.Warn("monitor center: persist OpenAI fetch failure failed", "error", err)
		}
	}
}

func (s *MonitorCenterService) publish(status *MonitorCenterOpenAIStatus, hash, etag, modified string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = cloneMonitorCenterStatus(status)
	s.contentHash = hash
	s.etag = etag
	s.lastModified = modified
}

func (s *MonitorCenterService) currentStatus() *MonitorCenterOpenAIStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneMonitorCenterStatus(s.current)
}

func (s *MonitorCenterService) currentContentHash() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.contentHash
}

func (s *MonitorCenterService) currentETag() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.etag
}

func (s *MonitorCenterService) currentLastModified() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastModified
}

func (s *MonitorCenterService) acquirePollLock(ctx context.Context) bool {
	if s.cache == nil {
		return true
	}
	ok, err := s.cache.TryAcquireMonitorCenterPollLock(ctx, s.instanceID, monitorCenterRedisPollLockTTL)
	if err != nil {
		slog.Warn("monitor center: Redis poll lock unavailable; using process-local lock", "error", err)
		return true
	}
	return ok
}

func (s *MonitorCenterService) storeRedisCache(ctx context.Context) {
	if s.cache == nil {
		return
	}
	envelope := monitorCenterCacheEnvelope{
		Status:       s.currentStatus(),
		ContentHash:  s.currentContentHash(),
		ETag:         s.currentETag(),
		LastModified: s.currentLastModified(),
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		return
	}
	if err := s.cache.StoreMonitorCenterOpenAIStatus(ctx, payload, monitorCenterCacheTTL); err != nil {
		slog.Warn("monitor center: Redis status cache write failed", "error", err)
	}
}

func (s *MonitorCenterService) loadRedisCache(ctx context.Context) {
	if s == nil || s.cache == nil {
		return
	}
	payload, err := s.cache.LoadMonitorCenterOpenAIStatus(ctx)
	if err != nil {
		return
	}
	var envelope monitorCenterCacheEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil || envelope.Status == nil {
		return
	}
	s.publish(envelope.Status, envelope.ContentHash, envelope.ETag, envelope.LastModified)
}

func normalizeMonitorCenterOpenAIStatus(payload *openAISummaryPayload, now time.Time, latencyMs int) *MonitorCenterOpenAIStatus {
	status := unknownMonitorCenterOpenAIStatus()
	status.OverallStatus = normalizeMonitorCenterStatus(payload.Status.Indicator)
	status.OverallDescription = strings.TrimSpace(payload.Status.Description)
	status.LastAttemptAt = timePointer(now)
	status.LastSuccessAt = timePointer(now)
	status.FetchStatus = monitorCenterFetchStatusSuccess
	status.FetchLatencyMs = latencyMs
	status.Stale = false

	componentsByName := make(map[string][]string, len(payload.Components))
	for _, component := range payload.Components {
		name := strings.ToLower(strings.TrimSpace(component.Name))
		if name == "" {
			continue
		}
		componentsByName[name] = append(componentsByName[name], normalizeMonitorCenterStatus(component.Status))
	}
	status.Groups = make([]MonitorCenterServiceGroup, 0, len(monitorCenterOpenAIGroupRules))
	for _, groupRule := range monitorCenterOpenAIGroupRules {
		group := MonitorCenterServiceGroup{Key: groupRule.Key, Name: groupRule.Name}
		for _, componentRule := range groupRule.Components {
			componentStatus, matched := matchMonitorCenterComponent(componentsByName, componentRule.Aliases)
			group.Components = append(group.Components, MonitorCenterComponentStatus{
				Key: componentRule.Key, Name: componentRule.Name, Status: componentStatus, Matched: matched,
			})
		}
		group.Status = aggregateMonitorCenterComponents(group.Components)
		status.Groups = append(status.Groups, group)
	}

	status.Incidents = make([]MonitorCenterIncident, 0, len(payload.Incidents))
	for _, source := range payload.Incidents {
		incident := MonitorCenterIncident{
			ID: source.ID, Name: source.Name, Status: source.Status, Impact: source.Impact, UpdatedAt: source.UpdatedAt.UTC(),
			AffectedComponents: make([]string, 0, len(source.Components)),
			AffectedGroups:     make([]string, 0, len(monitorCenterOpenAIGroupRules)),
			CreatedAt:          utcTimePointer(source.CreatedAt),
			StartedAt:          utcTimePointer(source.StartedAt),
			ResolvedAt:         utcTimePointer(source.ResolvedAt),
			URL:                strings.TrimSpace(source.Shortlink),
			Updates:            make([]MonitorCenterIncidentUpdate, 0, len(source.IncidentUpdates)),
		}
		for _, component := range source.Components {
			if name := strings.TrimSpace(component.Name); name != "" {
				incident.AffectedComponents = append(incident.AffectedComponents, name)
			}
		}
		incident.AffectedGroups = monitorCenterIncidentGroups(incident.AffectedComponents)
		for _, sourceUpdate := range source.IncidentUpdates {
			incident.Updates = append(incident.Updates, MonitorCenterIncidentUpdate{
				Status: sourceUpdate.Status, Body: sourceUpdate.Body, UpdatedAt: sourceUpdate.UpdatedAt.UTC(),
			})
		}
		sort.Slice(incident.Updates, func(i, j int) bool { return incident.Updates[i].UpdatedAt.After(incident.Updates[j].UpdatedAt) })
		if len(incident.Updates) > 0 {
			latest := incident.Updates[0]
			incident.LatestUpdate = &latest
		}
		status.Incidents = append(status.Incidents, incident)
	}
	sort.Slice(status.Incidents, func(i, j int) bool { return status.Incidents[i].UpdatedAt.After(status.Incidents[j].UpdatedAt) })
	return status
}

func matchMonitorCenterComponent(byName map[string][]string, aliases []string) (string, bool) {
	statuses := make([]string, 0, 2)
	for _, alias := range aliases {
		statuses = append(statuses, byName[strings.ToLower(strings.TrimSpace(alias))]...)
	}
	if len(statuses) == 0 {
		return MonitorCenterStatusUnknown, false
	}
	return worstMonitorCenterStatus(statuses), true
}

func aggregateMonitorCenterComponents(components []MonitorCenterComponentStatus) string {
	known := make([]string, 0, len(components))
	for _, component := range components {
		if component.Matched && component.Status != MonitorCenterStatusUnknown {
			known = append(known, component.Status)
		}
	}
	if len(known) == 0 {
		return MonitorCenterStatusUnknown
	}
	return worstMonitorCenterStatus(known)
}

func normalizeMonitorCenterStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "none", "operational":
		return MonitorCenterStatusOperational
	case "minor", "degraded", "degraded_performance":
		return MonitorCenterStatusDegradedPerformance
	case "major", "partial_outage", "failed", "error":
		return MonitorCenterStatusPartialOutage
	case "critical", "major_outage":
		return MonitorCenterStatusMajorOutage
	case "under_maintenance", "maintenance":
		return MonitorCenterStatusUnderMaintenance
	default:
		return MonitorCenterStatusUnknown
	}
}

func worstMonitorCenterStatus(statuses []string) string {
	worst := MonitorCenterStatusUnknown
	worstSeverity := -1
	for _, status := range statuses {
		severity := monitorCenterStatusSeverity(status)
		if severity > worstSeverity {
			worst = status
			worstSeverity = severity
		}
	}
	return worst
}

func monitorCenterStatusSeverity(status string) int {
	switch status {
	case MonitorCenterStatusOperational:
		return 0
	case MonitorCenterStatusUnderMaintenance:
		return 1
	case MonitorCenterStatusDegradedPerformance:
		return 2
	case MonitorCenterStatusPartialOutage:
		return 3
	case MonitorCenterStatusMajorOutage:
		return 4
	default:
		return -1
	}
}

func monitorCenterHistoryPoint(status *MonitorCenterOpenAIStatus) *MonitorCenterOpenAIHistoryPoint {
	timestamp := time.Now().UTC()
	if status.LastAttemptAt != nil {
		timestamp = status.LastAttemptAt.UTC()
	}
	point := &MonitorCenterOpenAIHistoryPoint{
		Timestamp:           timestamp,
		OverallStatus:       status.OverallStatus,
		APIStatus:           monitorCenterGroupStatus(status.Groups, "api"),
		ChatGPTStatus:       monitorCenterGroupStatus(status.Groups, "chatgpt"),
		CodexStatus:         monitorCenterGroupStatus(status.Groups, "codex"),
		ActiveIncidentCount: len(status.Incidents),
		FetchStatus:         status.FetchStatus,
		LatencyMs:           status.FetchLatencyMs,
		IncidentRefs:        monitorCenterIncidentRefs(status.Incidents),
	}
	return point
}

func monitorCenterIncidentGroups(componentNames []string) []string {
	groups := make([]string, 0, len(monitorCenterOpenAIGroupRules))
	for _, groupRule := range monitorCenterOpenAIGroupRules {
		matched := false
		for _, componentRule := range groupRule.Components {
			for _, affectedName := range componentNames {
				for _, alias := range componentRule.Aliases {
					if strings.EqualFold(strings.TrimSpace(affectedName), strings.TrimSpace(alias)) {
						matched = true
						break
					}
				}
			}
		}
		if matched {
			groups = append(groups, groupRule.Key)
		}
	}
	return groups
}

func monitorCenterIncidentRefs(incidents []MonitorCenterIncident) map[string][]string {
	refs := map[string][]string{}
	for _, incident := range incidents {
		if incident.ID == "" || strings.EqualFold(incident.Status, "resolved") {
			continue
		}
		refs["all"] = append(refs["all"], incident.ID)
		for _, group := range incident.AffectedGroups {
			refs[group] = append(refs[group], incident.ID)
		}
	}
	return refs
}

func monitorCenterHistoryStatistics(points []MonitorCenterOpenAIHistoryPoint) MonitorCenterOpenAIHistoryStatistics {
	stats := MonitorCenterOpenAIHistoryStatistics{Groups: map[string]MonitorCenterOpenAIGroupStatistics{}}
	stats.SampleCount = len(points)
	latencyTotal := 0
	for _, point := range points {
		if point.FetchStatus == monitorCenterFetchStatusSuccess {
			stats.SuccessfulCount++
			latencyTotal += point.LatencyMs
		}
		if point.FetchStatus != monitorCenterFetchStatusSuccess || point.APIStatus != MonitorCenterStatusOperational || point.ChatGPTStatus != MonitorCenterStatusOperational || point.CodexStatus != MonitorCenterStatusOperational {
			stats.AnomalyCount++
		}
		for key, value := range map[string]string{"api": point.APIStatus, "chatgpt": point.ChatGPTStatus, "codex": point.CodexStatus} {
			group := stats.Groups[key]
			group.SampleCount++
			if point.FetchStatus == monitorCenterFetchStatusSuccess && value != MonitorCenterStatusUnknown {
				group.KnownSampleCount++
				if value == MonitorCenterStatusOperational {
					group.OperationalCount++
				}
			}
			stats.Groups[key] = group
		}
	}
	if stats.SampleCount > 0 {
		stats.FetchSuccessPct = float64(stats.SuccessfulCount) / float64(stats.SampleCount) * 100
	}
	if stats.SuccessfulCount > 0 {
		stats.AverageLatencyMs = float64(latencyTotal) / float64(stats.SuccessfulCount)
	}
	for key, group := range stats.Groups {
		if group.KnownSampleCount > 0 {
			group.AvailabilityPct = float64(group.OperationalCount) / float64(group.KnownSampleCount) * 100
		}
		stats.Groups[key] = group
	}
	return stats
}

func emptyMonitorCenterOpenAIHistory(start, end time.Time) *MonitorCenterOpenAIHistory {
	return &MonitorCenterOpenAIHistory{
		StartTime: start, EndTime: end, Bucket: "minute",
		Points:     []MonitorCenterOpenAIHistoryPoint{},
		Statistics: monitorCenterHistoryStatistics(nil),
		Incidents:  []MonitorCenterIncident{},
	}
}

func monitorCenterFailureReason(err error) string {
	if err == nil {
		return ""
	}
	value := strings.TrimSpace(err.Error())
	if len(value) > 300 {
		value = value[:300]
	}
	return value
}

func utcTimePointer(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	return timePointer(value.UTC())
}

func monitorCenterGroupStatus(groups []MonitorCenterServiceGroup, key string) string {
	for _, group := range groups {
		if group.Key == key {
			return group.Status
		}
	}
	return MonitorCenterStatusUnknown
}

func hashMonitorCenterStatus(status *MonitorCenterOpenAIStatus) (string, error) {
	if status == nil {
		return "", nil
	}
	stable := struct {
		OverallStatus      string                      `json:"overall_status"`
		OverallDescription string                      `json:"overall_description"`
		Groups             []MonitorCenterServiceGroup `json:"groups"`
		Incidents          []MonitorCenterIncident     `json:"incidents"`
	}{status.OverallStatus, status.OverallDescription, status.Groups, status.Incidents}
	payload, err := json.Marshal(stable)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:]), nil
}

func unknownMonitorCenterOpenAIStatus() *MonitorCenterOpenAIStatus {
	groups := make([]MonitorCenterServiceGroup, 0, len(monitorCenterOpenAIGroupRules))
	for _, groupRule := range monitorCenterOpenAIGroupRules {
		group := MonitorCenterServiceGroup{Key: groupRule.Key, Name: groupRule.Name, Status: MonitorCenterStatusUnknown}
		for _, componentRule := range groupRule.Components {
			group.Components = append(group.Components, MonitorCenterComponentStatus{
				Key: componentRule.Key, Name: componentRule.Name, Status: MonitorCenterStatusUnknown, Matched: false,
			})
		}
		groups = append(groups, group)
	}
	return &MonitorCenterOpenAIStatus{
		OverallStatus:      MonitorCenterStatusUnknown,
		OverallDescription: "OpenAI status is not available yet",
		Groups:             groups,
		Incidents:          []MonitorCenterIncident{},
		FetchStatus:        monitorCenterFetchStatusFailed,
		Stale:              true,
	}
}

func markMonitorCenterStale(status *MonitorCenterOpenAIStatus) {
	if status == nil {
		return
	}
	status.Stale = status.FetchStatus != monitorCenterFetchStatusSuccess ||
		status.LastSuccessAt == nil || time.Since(status.LastSuccessAt.UTC()) > monitorCenterCacheTTL
}

func cloneMonitorCenterStatus(status *MonitorCenterOpenAIStatus) *MonitorCenterOpenAIStatus {
	if status == nil {
		return nil
	}
	payload, err := json.Marshal(status)
	if err != nil {
		return nil
	}
	var cloned MonitorCenterOpenAIStatus
	if err := json.Unmarshal(payload, &cloned); err != nil {
		return nil
	}
	return &cloned
}

func timePointer(value time.Time) *time.Time {
	value = value.UTC()
	return &value
}

func monitorCenterFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
