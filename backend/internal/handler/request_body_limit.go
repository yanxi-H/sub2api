package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	requestBodyRecoveryLimitExceededCode = "request_body_recovery_limit_exceeded"
	requestBodyHeavyLimitExceededCode    = "request_body_heavy_limit_exceeded"
	requestBodyAdmissionUnavailableCode  = "request_body_admission_unavailable"
	largeRequestQueueTimeoutCode         = "large_request_queue_timeout"
	requestBodyRecoveryExecutionLimit    = 15 * time.Minute
)

type responsesRecoveryDeadline struct {
	deadline time.Time
}

func (d *responsesRecoveryDeadline) apply(c *gin.Context, lane service.RequestBodyLane) context.CancelFunc {
	if c == nil || c.Request == nil || d == nil || lane != service.RequestBodyLaneRecovery {
		return nil
	}
	if d.deadline.IsZero() {
		d.deadline = time.Now().Add(requestBodyRecoveryExecutionLimit)
	}
	requestCtx, requestCancel := context.WithDeadline(c.Request.Context(), d.deadline)
	upstreamParent, ok := service.RequestBodyAdmissionUpstreamContext(c.Request.Context())
	if !ok {
		upstreamParent = context.WithoutCancel(c.Request.Context())
	}
	upstreamCtx, upstreamCancel := context.WithDeadline(upstreamParent, d.deadline)
	requestCtx = service.WithRequestBodyAdmissionUpstreamContext(requestCtx, upstreamCtx)
	c.Request = c.Request.WithContext(requestCtx)
	return func() {
		requestCancel()
		upstreamCancel()
	}
}

func installRequestBodyAdmissionContexts(c *gin.Context) (context.Context, func()) {
	parent := c.Request.Context()
	handlerCtx, handlerCancel := context.WithCancel(parent)
	upstreamCtx, upstreamCancel := context.WithCancel(context.WithoutCancel(parent))
	var cancelOnce sync.Once
	cancel := func() {
		cancelOnce.Do(func() {
			handlerCancel()
			upstreamCancel()
		})
	}
	upstreamCtx = service.WithRequestBodyAdmissionLeaseLossCancel(upstreamCtx, cancel)
	handlerCtx = service.WithRequestBodyAdmissionLeaseLossCancel(handlerCtx, cancel)
	handlerCtx = service.WithRequestBodyAdmissionUpstreamContext(handlerCtx, upstreamCtx)
	c.Request = c.Request.WithContext(handlerCtx)
	return parent, cancel
}

func responsesAccountSlotLifecycleContext(ctx context.Context, lane service.RequestBodyLane) context.Context {
	if lane != service.RequestBodyLaneHeavy && lane != service.RequestBodyLaneRecovery {
		return ctx
	}
	if upstreamCtx, ok := service.RequestBodyAdmissionUpstreamContext(ctx); ok {
		return upstreamCtx
	}
	return ctx
}

func releaseAcquiredAccountSelection(selection *service.AccountSelectionResult) {
	if selection != nil && selection.Acquired && selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func combineReleaseFuncs(releases ...func()) func() {
	return func() {
		for _, release := range releases {
			if release != nil {
				release()
			}
		}
	}
}

func releaseSelectionForRequestBodyLaneWait(selection *service.AccountSelectionResult) {
	if selection == nil || selection.Account == nil || !selection.Acquired {
		return
	}
	releaseAcquiredAccountSelection(selection)
	selection.Acquired = false
	selection.ReleaseFunc = nil
	if selection.WaitPlan == nil {
		selection.WaitPlan = &service.AccountWaitPlan{
			AccountID:      selection.Account.ID,
			MaxConcurrency: selection.Account.Concurrency,
			Timeout:        maxConcurrencyWait,
			MaxWaiting:     1,
		}
	}
}

func requestBodyPolicyLimit(policy service.RequestBodyAdmissionPolicy, compactRequest bool) int64 {
	if compactRequest {
		return policy.RecoveryLimitBytes
	}
	return policy.HeavyLimitBytes
}

func (h *OpenAIGatewayHandler) rejectResponsesRequestBodyAcrossAccounts(
	c *gin.Context,
	bodyBytes int64,
	compactRequest bool,
	limit int64,
	streamStarted bool,
) {
	markResponsesPerformanceFailure(c, "request_too_large")
	if c != nil {
		c.Header("X-Sub2API-Request-Body-Policy", "tiered-admission")
		c.Header("X-Sub2API-Request-Body-Lane", string(service.RequestBodyLaneRejected))
	}
	code := requestBodyRecoveryLimitExceededCode
	message := fmt.Sprintf("Compact request body is %d bytes and exceeds every eligible account's recovery limit; the largest configured limit is %d bytes", bodyBytes, limit)
	if !compactRequest {
		code = requestBodyHeavyLimitExceededCode
		message = fmt.Sprintf("Ordinary request body is %d bytes and exceeds every eligible account's heavy request limit; the largest configured limit is %d bytes; compact the conversation before retrying", bodyBytes, limit)
	}
	h.handleStreamingAwareErrorWithCode(
		c, http.StatusRequestEntityTooLarge, "invalid_request_error", code, message, streamStarted, false,
	)
}

func (h *OpenAIGatewayHandler) acquireResponsesRequestBodyLane(
	c *gin.Context,
	reqLog *zap.Logger,
	selection *service.AccountSelectionResult,
	userID int64,
	bodyBytes int64,
	compactRequest bool,
	isStream bool,
	streamStarted *bool,
	resolveNormal func(),
	reserveNonNormal func(),
	waitOnBusyLane bool,
) (func(), service.RequestBodyLane, bool, bool) {
	if selection == nil || selection.Account == nil {
		if resolveNormal != nil {
			resolveNormal()
		}
		return nil, service.RequestBodyLaneNormal, true, false
	}
	account := selection.Account
	if reserveNonNormal != nil {
		reserveNonNormal()
	}
	policy := account.GetRequestBodyAdmissionPolicy()
	lane := policy.Classify(bodyBytes, compactRequest)
	if lane == service.RequestBodyLaneDisabled {
		if resolveNormal != nil {
			resolveNormal()
		}
		return nil, service.RequestBodyLaneNormal, true, false
	}
	c.Header("X-Sub2API-Request-Body-Policy", "tiered-admission")
	c.Header("X-Sub2API-Request-Body-Lane", string(lane))
	if reqLog != nil {
		fields := []zap.Field{
			zap.Int64("account_id", account.ID),
			zap.Int64("user_id", userID),
			zap.Int64("request_body_bytes", bodyBytes),
			zap.Int64("normal_limit_bytes", policy.NormalLimitBytes),
			zap.Int64("heavy_limit_bytes", policy.HeavyLimitBytes),
			zap.Int64("recovery_limit_bytes", policy.RecoveryLimitBytes),
			zap.String("request_body_lane", string(lane)),
			zap.Bool("compact_request", compactRequest),
		}
		if lane == service.RequestBodyLaneNormal {
			reqLog.Debug("openai.request_body_lane_classified", fields...)
		} else {
			reqLog.Info("openai.request_body_lane_classified", fields...)
		}
	}

	if lane == service.RequestBodyLaneRejected {
		markResponsesPerformanceFailure(c, "request_too_large")
		releaseAcquiredAccountSelection(selection)
		code := requestBodyRecoveryLimitExceededCode
		limit := policy.RecoveryLimitBytes
		message := fmt.Sprintf("Compact request body is %d bytes and exceeds the configured recovery limit of %d bytes", bodyBytes, limit)
		if !compactRequest {
			code = requestBodyHeavyLimitExceededCode
			limit = policy.HeavyLimitBytes
			message = fmt.Sprintf("Ordinary request body is %d bytes and exceeds the heavy request limit of %d bytes; compact the conversation before retrying", bodyBytes, limit)
		}
		h.handleStreamingAwareErrorWithCode(
			c, http.StatusRequestEntityTooLarge, "invalid_request_error", code, message, *streamStarted, false,
		)
		return nil, lane, false, false
	}
	if lane == service.RequestBodyLaneNormal {
		if resolveNormal != nil {
			resolveNormal()
		}
		return nil, lane, true, false
	}
	requestParent, cancelAdmissionContexts := installRequestBodyAdmissionContexts(c)

	scopeID := account.ID
	maxPermits := service.RequestBodyHeavyConcurrencyLimit(account.Concurrency)
	if lane == service.RequestBodyLaneRecovery {
		maxPermits = service.RequestBodyRecoveryGlobalConcurrency
	}
	release, acquired, err := h.concurrencyHelper.TryAcquireRequestBodyLane(
		c.Request.Context(), lane, scopeID, userID, maxPermits, 1,
	)
	if err != nil {
		markResponsesPerformanceFailure(c, "queue_rejected")
		cancelAdmissionContexts()
		c.Request = c.Request.WithContext(requestParent)
		releaseAcquiredAccountSelection(selection)
		h.handleStreamingAwareErrorWithCode(
			c, http.StatusServiceUnavailable, "api_error", requestBodyAdmissionUnavailableCode, "Request body admission is temporarily unavailable", *streamStarted, false,
		)
		return nil, lane, false, false
	}
	if acquired {
		return combineReleaseFuncs(release, cancelAdmissionContexts), lane, true, false
	}

	// The scheduler may have optimistically reserved an account slot. Large
	// requests must release it before waiting so ordinary traffic stays isolated.
	releaseSelectionForRequestBodyLaneWait(selection)
	if !waitOnBusyLane {
		cancelAdmissionContexts()
		c.Request = c.Request.WithContext(requestParent)
		return nil, lane, false, true
	}
	waitStartedAt := time.Now()
	release, err = h.concurrencyHelper.AcquireRequestBodyLaneWithWait(
		c, lane, scopeID, userID, maxPermits, service.RequestBodyLaneWaitLimit(maxPermits), 1, isStream, streamStarted,
	)
	if err != nil {
		requestContextErr := c.Request.Context().Err()
		cancelAdmissionContexts()
		c.Request = c.Request.WithContext(requestParent)
		if reqLog != nil {
			reqLog.Warn("openai.request_body_lane_wait_failed",
				zap.Int64("account_id", account.ID),
				zap.Int64("user_id", userID),
				zap.String("request_body_lane", string(lane)),
				zap.Int64("wait_ms", time.Since(waitStartedAt).Milliseconds()),
				zap.Error(err),
			)
		}
		if requestContextErr != nil {
			if lane == service.RequestBodyLaneRecovery && errors.Is(requestContextErr, context.DeadlineExceeded) {
				markResponsesPerformanceFailure(c, "recovery_deadline")
			}
			return nil, lane, false, false
		}
		var queueFullErr *WaitQueueFullError
		var concurrencyErr *ConcurrencyError
		if errors.As(err, &queueFullErr) || (errors.As(err, &concurrencyErr) && concurrencyErr.IsTimeout) {
			markResponsesPerformanceFailure(c, "queue_rejected")
			c.Header("Retry-After", "5")
			h.handleStreamingAwareErrorWithCode(
				c, http.StatusTooManyRequests, "rate_limit_error", largeRequestQueueTimeoutCode, "Large request queue is full or timed out; retry later", *streamStarted, true,
			)
			return nil, lane, false, false
		}
		markResponsesPerformanceFailure(c, "queue_rejected")
		h.handleStreamingAwareErrorWithCode(
			c, http.StatusServiceUnavailable, "api_error", requestBodyAdmissionUnavailableCode, "Request body admission is temporarily unavailable", *streamStarted, false,
		)
		return nil, lane, false, false
	}
	if reqLog != nil {
		reqLog.Info("openai.request_body_lane_wait_succeeded",
			zap.Int64("account_id", account.ID),
			zap.Int64("user_id", userID),
			zap.String("request_body_lane", string(lane)),
			zap.Int64("wait_ms", time.Since(waitStartedAt).Milliseconds()),
		)
	}
	return combineReleaseFuncs(release, cancelAdmissionContexts), lane, true, false
}

func extractMaxBytesError(err error) (*http.MaxBytesError, bool) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return maxErr, true
	}
	return nil, false
}

func buildBodyTooLargeMessage(limit int64) string {
	return fmt.Sprintf("Request body too large: maximum size is %d bytes", limit)
}

func readLenientJSONRequestBodyWithPrealloc(req *http.Request, cfg *config.Config) ([]byte, error) {
	return pkghttputil.ReadLenientJSONRequestBodyWithPrealloc(req, gatewayMaxBodySize(cfg))
}

func readOpenAIResponsesRequestBodyWithPrealloc(req *http.Request, cfg *config.Config) ([]byte, error) {
	limit := service.MaxRequestBodyRecoveryLimitBytes
	if cfg != nil {
		if configured := cfg.Gateway.TextMaxBodySize; configured > 0 && configured < limit {
			limit = configured
		}
		if configured := cfg.Gateway.MaxBodySize; configured > 0 && configured < limit {
			limit = configured
		}
	}
	return pkghttputil.ReadLenientJSONRequestBodyWithPrealloc(req, limit)
}

func gatewayMaxBodySize(cfg *config.Config) int64 {
	if cfg == nil {
		return 0
	}
	return cfg.Gateway.MaxBodySize
}
