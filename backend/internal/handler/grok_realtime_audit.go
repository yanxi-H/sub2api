package handler

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type grokRealtimeAuditError struct {
	decision *securityaudit.Decision
}

func (e *grokRealtimeAuditError) Error() string {
	if e == nil {
		return ""
	}
	return securityAuditErrorCode(e.decision)
}

func (h *OpenAIGatewayHandler) rejectGrokRealtimeWithoutPreRoutingAudit(c *gin.Context, apiKey *service.APIKey, model string) bool {
	if h == nil || c == nil || c.Request == nil || apiKey == nil {
		return false
	}
	blockedByPromptGuard := h.securityAuditCoordinator != nil && h.securityAuditCoordinator.RequiresPreRoutingBody(apiKey.GroupID)
	blockedByLegacyModeration := h.contentModerationService != nil && h.contentModerationService.RequiresPreRoutingBody(c.Request.Context(), apiKey.GroupID, model)
	if !blockedByPromptGuard && !blockedByLegacyModeration {
		return false
	}
	h.errorResponse(c, http.StatusServiceUnavailable, "realtime_blocking_audit_unsupported", "Grok Realtime is unavailable while blocking prompt audit is enabled")
	return true
}

func (h *OpenAIGatewayHandler) auditGrokRealtimeEvent(
	c *gin.Context,
	reqLog *zap.Logger,
	apiKey *service.APIKey,
	subject middleware2.AuthSubject,
	model string,
	event []byte,
	stage string,
	turn int,
) error {
	auditBody, hasText, err := buildGrokRealtimeAuditBody(event)
	if err != nil {
		return service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "invalid realtime event", err)
	}
	if !hasText {
		return nil
	}
	c.Set(securityAuditWSTurnContextKey, turn)
	decision := h.checkSecurityAuditStage(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIChat, model, auditBody, stage)
	if decision != nil && !decision.AllowNextStage {
		return &grokRealtimeAuditError{decision: decision}
	}
	return nil
}

func buildGrokRealtimeAuditBody(event []byte) ([]byte, bool, error) {
	var document any
	if err := json.Unmarshal(event, &document); err != nil {
		return nil, false, err
	}
	texts := make([]string, 0, 4)
	seen := make(map[string]struct{})
	collectGrokRealtimeText(document, "", &texts, seen)
	if len(texts) == 0 {
		return nil, false, nil
	}
	body, err := json.Marshal(map[string]any{
		"messages": []map[string]any{{"role": "user", "content": strings.Join(texts, "\n")}},
	})
	return body, err == nil, err
}

func collectGrokRealtimeText(value any, key string, texts *[]string, seen map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for childKey := range typed {
			keys = append(keys, childKey)
		}
		sort.Strings(keys)
		for _, childKey := range keys {
			child := typed[childKey]
			collectGrokRealtimeText(child, strings.ToLower(strings.TrimSpace(childKey)), texts, seen)
		}
	case []any:
		for _, child := range typed {
			collectGrokRealtimeText(child, key, texts, seen)
		}
	case string:
		if !isGrokRealtimeTextField(key) {
			return
		}
		text := strings.TrimSpace(typed)
		if text == "" {
			return
		}
		if _, exists := seen[text]; exists {
			return
		}
		seen[text] = struct{}{}
		*texts = append(*texts, text)
	}
}

func isGrokRealtimeTextField(key string) bool {
	switch key {
	case "instructions", "prompt", "input", "text", "transcript", "content", "output":
		return true
	default:
		return false
	}
}

var _ error = (*grokRealtimeAuditError)(nil)
