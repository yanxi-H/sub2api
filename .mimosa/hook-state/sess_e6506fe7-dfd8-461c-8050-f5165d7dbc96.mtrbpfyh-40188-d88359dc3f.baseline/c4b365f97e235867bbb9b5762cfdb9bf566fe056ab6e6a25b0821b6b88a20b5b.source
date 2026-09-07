package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type responsesPerformanceCaptureRepo struct {
	service.OpsRepository
	mu     sync.Mutex
	inputs []service.OpsRequestPerformanceInput
}

func (r *responsesPerformanceCaptureRepo) InsertRequestPerformance(_ context.Context, input *service.OpsRequestPerformanceInput) error {
	if input == nil {
		return nil
	}
	cloned := *input
	if input.GroupID != nil {
		groupID := *input.GroupID
		cloned.GroupID = &groupID
	}
	r.mu.Lock()
	r.inputs = append(r.inputs, cloned)
	r.mu.Unlock()
	return nil
}

func (r *responsesPerformanceCaptureRepo) GetPerformanceDiagnostics(_ context.Context, filter *service.OpsDashboardFilter, _ int) (*service.OpsPerformanceDiagnosticsResponse, error) {
	return &service.OpsPerformanceDiagnosticsResponse{StartTime: filter.StartTime, EndTime: filter.EndTime}, nil
}

func (r *responsesPerformanceCaptureRepo) last(t *testing.T) service.OpsRequestPerformanceInput {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	require.NotEmpty(t, r.inputs)
	return r.inputs[len(r.inputs)-1]
}

func newResponsesPerformanceTestHandler(repo *responsesPerformanceCaptureRepo) *OpenAIGatewayHandler {
	return &OpenAIGatewayHandler{opsService: service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)}
}

func TestFinalizeResponsesPerformanceRecordsGatewayFailureStatuses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		status     int
		cause      string
		wantStatus int
	}{
		{name: "request too large", status: http.StatusRequestEntityTooLarge, cause: "request_too_large", wantStatus: http.StatusRequestEntityTooLarge},
		{name: "queue rejected", status: http.StatusTooManyRequests, cause: "queue_rejected", wantStatus: http.StatusTooManyRequests},
		{name: "no available account", status: http.StatusServiceUnavailable, cause: "no_available_account", wantStatus: http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &responsesPerformanceCaptureRepo{}
			h := newResponsesPerformanceTestHandler(repo)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			c.Status(test.status)
			markResponsesPerformanceFailure(c, test.cause)
			input := &service.OpsRequestPerformanceInput{CreatedAt: time.Now().Add(-10 * time.Millisecond), RequestID: test.name}

			h.finalizeResponsesPerformance(c, c.Request.Context(), input, false, time.Time{})

			captured := repo.last(t)
			require.Equal(t, test.wantStatus, captured.LogicalStatusCode)
			require.Equal(t, test.cause, captured.FailureCause)
		})
	}
}

func TestFinalizeResponsesPerformanceUsesInBandSemanticStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &responsesPerformanceCaptureRepo{}
	h := newResponsesPerformanceTestHandler(repo)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Status(http.StatusOK)
	service.MarkOpsStreamFailure(c, "invalid_request_error", "context_length_exceeded", "context window exceeded", http.StatusBadRequest)
	input := &service.OpsRequestPerformanceInput{CreatedAt: time.Now().Add(-10 * time.Millisecond), RequestID: "stream-failed", Stream: true}

	// WS transport can return a result with a response.failed terminal event, so
	// the stream marker must override an otherwise successful forward result.
	h.finalizeResponsesPerformance(c, c.Request.Context(), input, true, time.Time{})

	captured := repo.last(t)
	require.Equal(t, http.StatusBadRequest, captured.LogicalStatusCode)
	require.Equal(t, "upstream_error", captured.FailureCause)
}

func TestFinalizeResponsesPerformanceRecordsClientCancellationAs499(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &responsesPerformanceCaptureRepo{}
	h := newResponsesPerformanceTestHandler(repo)
	clientCtx, cancel := context.WithCancel(context.Background())
	cancel()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(clientCtx)
	input := &service.OpsRequestPerformanceInput{CreatedAt: time.Now().Add(-10 * time.Millisecond), RequestID: "client-canceled"}

	h.finalizeResponsesPerformance(c, clientCtx, input, false, time.Time{})

	captured := repo.last(t)
	require.Equal(t, statusClientClosedRequest, captured.LogicalStatusCode)
	require.Equal(t, "client_canceled", captured.FailureCause)
}

func TestFinalizeResponsesPerformanceRecordsRecoveryDeadline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &responsesPerformanceCaptureRepo{}
	h := newResponsesPerformanceTestHandler(repo)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	input := &service.OpsRequestPerformanceInput{CreatedAt: time.Now().Add(-10 * time.Millisecond), RequestID: "recovery-timeout"}

	h.finalizeResponsesPerformance(c, c.Request.Context(), input, false, time.Now().Add(-time.Millisecond))

	captured := repo.last(t)
	require.Equal(t, http.StatusGatewayTimeout, captured.LogicalStatusCode)
	require.Equal(t, "recovery_deadline", captured.FailureCause)
}

func TestFinalizeResponsesPerformancePreservesAttemptAndTimingTelemetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &responsesPerformanceCaptureRepo{}
	h := newResponsesPerformanceTestHandler(repo)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Status(http.StatusOK)
	input := &service.OpsRequestPerformanceInput{
		CreatedAt:          time.Now().Add(-20 * time.Millisecond),
		RequestID:          "failover-success",
		Stream:             true,
		AttemptCount:       3,
		AccountSwitchCount: 2,
		FailoverMs:         123,
		TimeToFirstTokenMs: 456,
		UpstreamTTFTMs:     45,
		MaxStreamGapMs:     78,
	}

	h.finalizeResponsesPerformance(c, c.Request.Context(), input, true, time.Time{})

	captured := repo.last(t)
	require.Equal(t, http.StatusOK, captured.LogicalStatusCode)
	require.Equal(t, 3, captured.AttemptCount)
	require.Equal(t, 2, captured.AccountSwitchCount)
	require.Equal(t, int64(123), captured.FailoverMs)
	require.Equal(t, int64(456), captured.TimeToFirstTokenMs)
	require.Equal(t, int64(45), captured.UpstreamTTFTMs)
	require.Equal(t, int64(78), captured.MaxStreamGapMs)
}

func TestResponsesPerformanceTimingCalculations(t *testing.T) {
	requestStartedAt := time.Date(2026, 7, 26, 8, 0, 0, 0, time.UTC)
	firstAttempt := requestStartedAt.Add(250 * time.Millisecond)
	finalAttempt := firstAttempt.Add(3 * time.Second)
	attemptTTFT := 750

	require.Equal(t, int64(3_000), responsesFailoverElapsedMs(firstAttempt, finalAttempt))
	require.Equal(t, int64(4_000), responsesEndToEndTTFTMs(requestStartedAt, finalAttempt, &attemptTTFT))
	require.Zero(t, responsesFailoverElapsedMs(time.Time{}, finalAttempt))
	require.Zero(t, responsesEndToEndTTFTMs(requestStartedAt, finalAttempt, nil))
}
