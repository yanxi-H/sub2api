package handler

import (
	"strings"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// archiveRequest 是统一的请求存档捕获点。在网关 handler 读取 body 后调用。
// 如果存档未启用或 service 为 nil,直接返回(零开销)。
// 复用 ExtractContentModerationInput 提取 prompt 文本,保持协议一致性。
func archiveRequest(h interface{ archiveSvc() *service.RequestArchiveService }, c *gin.Context, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, body []byte) {
	svc := h.archiveSvc()
	if svc == nil || !svc.IsEnabled() {
		return
	}
	if len(body) == 0 {
		return
	}

	// 复用风控的文本提取逻辑(协议感知)
	promptText := service.ExtractContentModerationInput(protocol, body).Text

	entry := service.RequestArchiveEntry{
		RequestID:  contentModerationRequestID(c.Request.Context()),
		UserID:     subject.UserID,
		APIKeyID:   apiKey.ID,
		APIKeyName: apiKey.Name,
		Endpoint:   GetInboundEndpoint(c),
		Protocol:   protocol,
		Model:      strings.TrimSpace(model),
		IPAddress:  c.ClientIP(),
		PromptText: promptText,
	}
	if apiKey.User != nil {
		entry.UserEmail = apiKey.User.Email
	}
	if apiKey.GroupID != nil {
		gid := *apiKey.GroupID
		entry.GroupID = &gid
	}
	if entry.Endpoint == "" && c.Request != nil && c.Request.URL != nil {
		entry.Endpoint = c.Request.URL.Path
	}

	svc.Archive(entry)
}

// archiveSvcAdapter 让 GatewayHandler 和 OpenAIGatewayHandler 都能通过 archiveRequest 调用。
type gatewayArchiveAdapter struct{ h *GatewayHandler }

func (a gatewayArchiveAdapter) archiveSvc() *service.RequestArchiveService {
	if a.h == nil {
		return nil
	}
	return a.h.requestArchiveService
}

type openAIArchiveAdapter struct{ h *OpenAIGatewayHandler }

func (a openAIArchiveAdapter) archiveSvc() *service.RequestArchiveService {
	if a.h == nil {
		return nil
	}
	return a.h.requestArchiveService
}

// archiveRequestForGateway 供 GatewayHandler 调用。
func (h *GatewayHandler) archiveRequestForGateway(c *gin.Context, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, body []byte) {
	archiveRequest(gatewayArchiveAdapter{h}, c, apiKey, subject, protocol, model, body)
}

// archiveRequestForOpenAI 供 OpenAIGatewayHandler 调用。
func (h *OpenAIGatewayHandler) archiveRequestForOpenAI(c *gin.Context, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, body []byte) {
	archiveRequest(openAIArchiveAdapter{h}, c, apiKey, subject, protocol, model, body)
}
