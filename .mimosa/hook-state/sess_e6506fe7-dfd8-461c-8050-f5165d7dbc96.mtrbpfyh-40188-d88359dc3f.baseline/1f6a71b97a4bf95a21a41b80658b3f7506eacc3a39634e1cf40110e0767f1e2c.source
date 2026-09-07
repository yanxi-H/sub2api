package service

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type recordingPerformanceOpsRepo struct {
	opsRepoMock
	inserts int
}

type diagnosticsPerformanceOpsRepo struct {
	opsRepoMock
	mu    sync.Mutex
	calls int
	getFn func(context.Context, *OpsDashboardFilter, int) (*OpsPerformanceDiagnosticsResponse, error)
}

func (r *diagnosticsPerformanceOpsRepo) InsertRequestPerformance(context.Context, *OpsRequestPerformanceInput) error {
	return nil
}

func (r *diagnosticsPerformanceOpsRepo) GetPerformanceDiagnostics(ctx context.Context, filter *OpsDashboardFilter, bucketSeconds int) (*OpsPerformanceDiagnosticsResponse, error) {
	r.mu.Lock()
	r.calls++
	r.mu.Unlock()
	if r.getFn != nil {
		return r.getFn(ctx, filter, bucketSeconds)
	}
	return &OpsPerformanceDiagnosticsResponse{StartTime: filter.StartTime, EndTime: filter.EndTime}, nil
}

func (r *diagnosticsPerformanceOpsRepo) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func (r *recordingPerformanceOpsRepo) InsertRequestPerformance(context.Context, *OpsRequestPerformanceInput) error {
	r.inserts++
	return nil
}

func (r *recordingPerformanceOpsRepo) GetPerformanceDiagnostics(context.Context, *OpsDashboardFilter, int) (*OpsPerformanceDiagnosticsResponse, error) {
	return &OpsPerformanceDiagnosticsResponse{}, nil
}

func TestClassifyOpsSlowCause(t *testing.T) {
	tests := []struct {
		name  string
		input OpsRequestPerformanceInput
		want  string
	}{
		{name: "healthy", input: OpsRequestPerformanceInput{EndToEndMs: 45_000, Stream: true, TimeToFirstTokenMs: 2_000, StreamDurationMs: 43_000, MaxStreamGapMs: 2_000}, want: "healthy"},
		{name: "heavy queue", input: OpsRequestPerformanceInput{EndToEndMs: 20_000, RequestBodyLane: RequestBodyLaneHeavy, BodyLaneWaitMs: 8_000, UpstreamMs: 5_000}, want: "heavy_queue"},
		{name: "recovery queue", input: OpsRequestPerformanceInput{EndToEndMs: 20_000, RequestBodyLane: RequestBodyLaneRecovery, BodyLaneWaitMs: 8_000, UpstreamMs: 5_000}, want: "recovery_queue"},
		{name: "user queue", input: OpsRequestPerformanceInput{EndToEndMs: 20_000, UserQueueMs: 9_000, UpstreamMs: 5_000}, want: "user_queue"},
		{name: "account queue", input: OpsRequestPerformanceInput{EndToEndMs: 20_000, AccountQueueMs: 9_000, UpstreamMs: 5_000}, want: "account_queue"},
		{name: "ttft", input: OpsRequestPerformanceInput{EndToEndMs: 20_000, Stream: true, UpstreamMs: 20_000, TimeToFirstTokenMs: 12_000, UpstreamTTFTMs: 12_000, StreamDurationMs: 8_000}, want: "upstream_ttft"},
		{name: "queue is not upstream ttft", input: OpsRequestPerformanceInput{EndToEndMs: 20_000, Stream: true, UserQueueMs: 12_000, TimeToFirstTokenMs: 12_500, UpstreamTTFTMs: 500}, want: "user_queue"},
		{name: "stream gap", input: OpsRequestPerformanceInput{EndToEndMs: 30_000, Stream: true, TimeToFirstTokenMs: 4_000, StreamDurationMs: 26_000, MaxStreamGapMs: 18_000}, want: "stream_gap"},
		{name: "upstream", input: OpsRequestPerformanceInput{EndToEndMs: 35_000, UpstreamMs: 31_000}, want: "upstream"},
		{name: "same account retry", input: OpsRequestPerformanceInput{AttemptCount: 2, FailoverMs: 3_000}, want: "failover"},
		{name: "failure wins", input: OpsRequestPerformanceInput{FailureCause: "queue_rejected", UpstreamMs: 50_000}, want: "queue_rejected"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, ClassifyOpsSlowCause(&test.input))
		})
	}
}

func TestRecordRequestPerformanceRespectsMonitoringSwitch(t *testing.T) {
	repo := &recordingPerformanceOpsRepo{}
	disabledConfig := &config.Config{}
	disabledConfig.Ops.Enabled = false
	disabled := &OpsService{opsRepo: repo, cfg: disabledConfig}

	require.NoError(t, disabled.RecordRequestPerformance(context.Background(), &OpsRequestPerformanceInput{RequestID: "disabled"}))
	require.Zero(t, repo.inserts)

	enabled := &OpsService{opsRepo: repo}
	require.NoError(t, enabled.RecordRequestPerformance(context.Background(), &OpsRequestPerformanceInput{RequestID: "enabled"}))
	require.Equal(t, 1, repo.inserts)
}

func TestRecordRequestPerformanceBoundsBatchStringColumns(t *testing.T) {
	repo := &recordingPerformanceOpsRepo{}
	svc := &OpsService{opsRepo: repo}
	input := &OpsRequestPerformanceInput{
		RequestID:    strings.Repeat("r", opsPerformanceRequestIDMaxBytes+10),
		Platform:     strings.Repeat("p", opsPerformancePlatformMaxBytes+10),
		Model:        strings.Repeat("模", opsPerformanceModelMaxBytes),
		FailureCause: strings.Repeat("f", opsPerformanceCauseMaxBytes+10),
		SlowCause:    strings.Repeat("s", opsPerformanceCauseMaxBytes+10),
	}

	require.NoError(t, svc.RecordRequestPerformance(t.Context(), input))
	require.Equal(t, 1, repo.inserts)
	require.LessOrEqual(t, len(input.RequestID), opsPerformanceRequestIDMaxBytes)
	require.LessOrEqual(t, len(input.Platform), opsPerformancePlatformMaxBytes)
	require.LessOrEqual(t, len(input.Model), opsPerformanceModelMaxBytes)
	require.LessOrEqual(t, len(input.FailureCause), opsPerformanceCauseMaxBytes)
	require.LessOrEqual(t, len(input.SlowCause), opsPerformanceCauseMaxBytes)
	require.True(t, utf8.ValidString(input.Model))
}

func TestPerformanceDiagnosticsCacheUsesExactRangeAndInitializesNilMap(t *testing.T) {
	repo := &diagnosticsPerformanceOpsRepo{}
	svc := &OpsService{opsRepo: repo}
	start := time.Date(2026, 7, 26, 8, 0, 1, 0, time.UTC)
	first := &OpsDashboardFilter{StartTime: start, EndTime: start.Add(time.Hour), Platform: PlatformOpenAI}
	second := &OpsDashboardFilter{StartTime: start.Add(time.Second), EndTime: start.Add(time.Hour + time.Second), Platform: PlatformOpenAI}

	firstResult, err := svc.GetPerformanceDiagnostics(t.Context(), first, 60)
	require.NoError(t, err)
	secondResult, err := svc.GetPerformanceDiagnostics(t.Context(), second, 60)
	require.NoError(t, err)
	firstCached, err := svc.GetPerformanceDiagnostics(t.Context(), first, 60)
	require.NoError(t, err)

	require.Equal(t, 2, repo.callCount(), "distinct custom ranges inside one 30-second window must not share cached data")
	require.Equal(t, first.StartTime, firstResult.StartTime)
	require.Equal(t, second.StartTime, secondResult.StartTime)
	require.NotSame(t, firstResult, firstCached)
	require.Equal(t, firstResult.StartTime, firstCached.StartTime)
	require.NotNil(t, svc.performanceDiagnosticsCache)
}

func TestPerformanceDiagnosticsReportsCurrentIngestionHealthOnCachedResponses(t *testing.T) {
	repo := &performanceSinkTestRepo{}
	sink := newPerformanceSinkForTest(repo, 1, 10, time.Hour)
	svc := &OpsService{opsRepo: repo, performanceSink: sink}
	filter := &OpsDashboardFilter{StartTime: time.Now().Add(-time.Hour), EndTime: time.Now()}

	first, err := svc.GetPerformanceDiagnostics(t.Context(), filter, 60)
	require.NoError(t, err)
	require.Zero(t, first.IngestionHealth.DroppedCount)

	require.True(t, sink.Enqueue(&OpsRequestPerformanceInput{RequestID: "accepted"}))
	require.False(t, sink.Enqueue(&OpsRequestPerformanceInput{RequestID: "dropped"}))
	second, err := svc.GetPerformanceDiagnostics(t.Context(), filter, 60)
	require.NoError(t, err)

	require.Equal(t, 1, repo.callCount(), "health updates must not bypass the diagnostics cache")
	require.Equal(t, uint64(1), second.IngestionHealth.DroppedCount)
	require.Equal(t, int64(1), second.IngestionHealth.QueueDepth)
}

func TestPerformanceDiagnosticsSingleflightCoalescesConcurrentQueries(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	repo := &diagnosticsPerformanceOpsRepo{getFn: func(_ context.Context, filter *OpsDashboardFilter, _ int) (*OpsPerformanceDiagnosticsResponse, error) {
		startedOnce.Do(func() { close(started) })
		<-release
		return &OpsPerformanceDiagnosticsResponse{StartTime: filter.StartTime, EndTime: filter.EndTime}, nil
	}}
	svc := &OpsService{opsRepo: repo}
	filter := &OpsDashboardFilter{StartTime: time.Now().Add(-time.Hour), EndTime: time.Now()}

	const callers = 8
	startCalls := make(chan struct{})
	results := make(chan error, callers)
	var ready sync.WaitGroup
	ready.Add(callers)
	for range callers {
		go func() {
			ready.Done()
			<-startCalls
			_, err := svc.GetPerformanceDiagnostics(context.Background(), filter, 60)
			results <- err
		}()
	}
	ready.Wait()
	close(startCalls)
	<-started
	time.Sleep(20 * time.Millisecond)
	close(release)
	for range callers {
		require.NoError(t, <-results)
	}
	require.Equal(t, 1, repo.callCount())
}

func TestPerformanceDiagnosticsQueryHasBoundedTimeout(t *testing.T) {
	deadlineRemaining := make(chan time.Duration, 1)
	repo := &diagnosticsPerformanceOpsRepo{getFn: func(ctx context.Context, filter *OpsDashboardFilter, _ int) (*OpsPerformanceDiagnosticsResponse, error) {
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		deadlineRemaining <- time.Until(deadline)
		return &OpsPerformanceDiagnosticsResponse{StartTime: filter.StartTime, EndTime: filter.EndTime}, nil
	}}
	svc := &OpsService{opsRepo: repo}
	filter := &OpsDashboardFilter{StartTime: time.Now().Add(-time.Hour), EndTime: time.Now()}

	_, err := svc.GetPerformanceDiagnostics(t.Context(), filter, 60)
	require.NoError(t, err)
	remaining := <-deadlineRemaining
	require.Greater(t, remaining, 4*time.Second)
	require.LessOrEqual(t, remaining, opsPerformanceQueryTimeout)
}

func TestPerformanceDiagnosticsCallerCancellationDoesNotWaitForSharedQuery(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	repo := &diagnosticsPerformanceOpsRepo{getFn: func(_ context.Context, filter *OpsDashboardFilter, _ int) (*OpsPerformanceDiagnosticsResponse, error) {
		close(started)
		<-release
		close(finished)
		return &OpsPerformanceDiagnosticsResponse{StartTime: filter.StartTime, EndTime: filter.EndTime}, nil
	}}
	svc := &OpsService{opsRepo: repo}
	filter := &OpsDashboardFilter{StartTime: time.Now().Add(-time.Hour), EndTime: time.Now()}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := svc.GetPerformanceDiagnostics(ctx, filter, 60)
		result <- err
	}()

	<-started
	cancel()
	require.ErrorIs(t, <-result, context.Canceled)
	close(release)
	<-finished
}
