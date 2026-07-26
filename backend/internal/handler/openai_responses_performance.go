package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const responsesPerformanceFailureCauseKey = "responses_performance_failure_cause"

func responsesFailoverElapsedMs(firstAttemptStartedAt, currentAttemptStartedAt time.Time) int64 {
	if firstAttemptStartedAt.IsZero() || currentAttemptStartedAt.IsZero() {
		return 0
	}
	return max(currentAttemptStartedAt.Sub(firstAttemptStartedAt).Milliseconds(), 0)
}

func responsesEndToEndTTFTMs(requestStartedAt, attemptStartedAt time.Time, attemptTTFTMs *int) int64 {
	if requestStartedAt.IsZero() || attemptStartedAt.IsZero() || attemptTTFTMs == nil {
		return 0
	}
	return max(attemptStartedAt.Sub(requestStartedAt).Milliseconds()+int64(*attemptTTFTMs), 0)
}

func markResponsesPerformanceFailure(c *gin.Context, cause string) {
	if c == nil || strings.TrimSpace(cause) == "" {
		return
	}
	if _, exists := c.Get(responsesPerformanceFailureCauseKey); exists {
		return
	}
	c.Set(responsesPerformanceFailureCauseKey, strings.TrimSpace(cause))
}

func responsesPerformanceFailure(c *gin.Context) string {
	if c == nil {
		return ""
	}
	value, _ := c.Get(responsesPerformanceFailureCauseKey)
	cause, _ := value.(string)
	return strings.TrimSpace(cause)
}

func classifyResponsesStreamFailure(streamErr service.OpsStreamError) string {
	combined := strings.ToLower(strings.TrimSpace(streamErr.Code + " " + streamErr.ErrType + " " + streamErr.Message))
	switch {
	case strings.Contains(combined, "too_large"), strings.Contains(combined, "too large"):
		return "request_too_large"
	case strings.Contains(combined, "queue"), strings.Contains(combined, "concurrency"):
		return "queue_rejected"
	default:
		return "upstream_error"
	}
}

func (h *OpenAIGatewayHandler) finalizeResponsesPerformance(
	c *gin.Context,
	clientCtx context.Context,
	input *service.OpsRequestPerformanceInput,
	succeeded bool,
	recoveryDeadline time.Time,
) {
	if h == nil || h.opsService == nil || input == nil {
		return
	}
	input.EndToEndMs = max(time.Since(input.CreatedAt).Milliseconds(), 0)
	input.FailureCause = responsesPerformanceFailure(c)
	status := 0
	if c != nil && c.Writer != nil {
		status = c.Writer.Status()
	}

	if streamErr, ok := service.GetOpsStreamError(c); ok {
		if streamErr.IntendedStatus > 0 {
			status = streamErr.IntendedStatus
		}
		if input.FailureCause == "" {
			input.FailureCause = classifyResponsesStreamFailure(streamErr)
		}
		succeeded = false
	}
	if clientCtx != nil && clientCtx.Err() != nil {
		status = statusClientClosedRequest
		input.FailureCause = "client_canceled"
		succeeded = false
	} else if !succeeded && !recoveryDeadline.IsZero() && !time.Now().Before(recoveryDeadline) {
		status = http.StatusGatewayTimeout
		input.FailureCause = "recovery_deadline"
	}
	if succeeded {
		if status < http.StatusOK || status >= http.StatusBadRequest {
			status = http.StatusOK
		}
		input.FailureCause = ""
	} else {
		if status < 100 || status < http.StatusBadRequest {
			status = http.StatusInternalServerError
		}
		if input.FailureCause == "" {
			switch {
			case status == http.StatusRequestEntityTooLarge:
				input.FailureCause = "request_too_large"
			case input.AttemptCount > 0:
				input.FailureCause = "upstream_error"
			default:
				input.FailureCause = "request_rejected"
			}
		}
	}
	input.LogicalStatusCode = status
	if input.Stream && input.TimeToFirstTokenMs > 0 {
		input.StreamDurationMs = max(input.EndToEndMs-input.TimeToFirstTokenMs, 0)
	}
	input.SlowCause = service.ClassifyOpsSlowCause(input)

	if err := h.opsService.RecordRequestPerformance(context.Background(), input); err != nil {
		logger.L().With(
			zap.String("component", "handler.openai_gateway.responses"),
			zap.String("request_id", input.RequestID),
		).Warn("openai.record_performance_failed", zap.Error(err))
	}
}
