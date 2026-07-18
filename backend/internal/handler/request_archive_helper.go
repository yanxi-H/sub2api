package handler

import (
	"strings"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// archiveRequest 是统一的请求存档捕获点。在网关 handler 读取 body 后调用。
// 如果存档未启用或 service 为 nil,直接返回(零开销)。
// 注意:这里用独立的 extractArchivePrompt 提取更完整的文本(所有 user 消息),
// 不复用风控的 ExtractContentModerationInput(它只取最后一条 user message)。
func archiveRequest(h interface{ archiveSvc() *service.RequestArchiveService }, c *gin.Context, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, body []byte) {
	svc := h.archiveSvc()
	if svc == nil || !svc.IsEnabled() {
		return
	}
	if len(body) == 0 {
		return
	}

	promptText := extractArchivePrompt(protocol, body)

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

// extractArchivePrompt 从请求体提取所有 user 的文本内容(不限于最后一条)。
// 这是存档专用的提取逻辑,比风控的更完整,便于事后筛查完整对话。
func extractArchivePrompt(protocol string, body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	var parts []string

	switch protocol {
	case service.ContentModerationProtocolAnthropicMessages:
		// Anthropic Messages: 遍历 messages,取所有 role=user 的文本
		messages := gjson.GetBytes(body, "messages")
		if messages.IsArray() {
			for _, msg := range messages.Array() {
				role := strings.ToLower(strings.TrimSpace(msg.Get("role").String()))
				if role != "user" && role != "assistant" {
					continue
				}
				text := extractContentText(msg.Get("content"))
				if text != "" {
					if role == "assistant" {
						parts = append(parts, "[assistant] "+text)
					} else {
						parts = append(parts, text)
					}
				}
			}
		}
		// 也提取 system
		if sys := gjson.GetBytes(body, "system"); sys.Exists() {
			sysText := extractContentText(sys)
			if sysText != "" {
				parts = append([]string{"[system] " + sysText}, parts...)
			}
		}

	case service.ContentModerationProtocolOpenAIChat:
		// OpenAI Chat Completions: messages 数组
		messages := gjson.GetBytes(body, "messages")
		if messages.IsArray() {
			for _, msg := range messages.Array() {
				role := strings.ToLower(strings.TrimSpace(msg.Get("role").String()))
				text := ""
				if msg.Get("content").Type == gjson.String {
					text = msg.Get("content").String()
				} else {
					text = extractContentText(msg.Get("content"))
				}
				if text != "" {
					if role == "user" {
						parts = append(parts, text)
					} else if role != "" {
						parts = append(parts, "["+role+"] "+text)
					}
				}
			}
		}

	case service.ContentModerationProtocolOpenAIResponses:
		// OpenAI Responses (Codex): input 数组,取所有 user input_text
		input := gjson.GetBytes(body, "input")
		if input.Type == gjson.String {
			parts = append(parts, input.String())
		} else if input.IsArray() {
			for _, item := range input.Array() {
				role := strings.ToLower(strings.TrimSpace(item.Get("role").String()))
				if role == "assistant" {
					continue // 跳过 assistant 回复
				}
				// input_text 类型或 content 数组
				if item.Get("type").String() == "input_text" || item.Get("text").Exists() {
					text := item.Get("text").String()
					if text == "" {
						text = extractContentText(item.Get("content"))
					}
					if text != "" {
						parts = append(parts, text)
					}
				} else if item.Get("content").Exists() {
					text := extractContentText(item.Get("content"))
					if text != "" {
						if role != "" && role != "user" {
							parts = append(parts, "["+role+"] "+text)
						} else {
							parts = append(parts, text)
						}
					}
				}
			}
		}
		// 也提取 instructions
		if ins := gjson.GetBytes(body, "instructions"); ins.Exists() && ins.Type == gjson.String {
			parts = append([]string{"[instructions] " + ins.String()}, parts...)
		}

	default:
		// 兜底:尝试所有可能的字段
		for _, key := range []string{"messages", "input", "contents"} {
			arr := gjson.GetBytes(body, key)
			if arr.IsArray() {
				for _, item := range arr.Array() {
					text := extractContentText(item.Get("content"))
					if text == "" && item.Get("text").Exists() {
						text = item.Get("text").String()
					}
					if text != "" {
						parts = append(parts, text)
					}
				}
			}
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// extractContentText 从 content 字段提取纯文本。
// content 可能是字符串,也可能是数组(含 text/image_url 类型)。
func extractContentText(content gjson.Result) string {
	if !content.Exists() {
		return ""
	}
	if content.Type == gjson.String {
		return content.String()
	}
	if !content.IsArray() {
		return ""
	}
	var texts []string
	for _, item := range content.Array() {
		// text 类型
		if t := item.Get("text"); t.Exists() && t.Type == gjson.String {
			texts = append(texts, t.String())
		}
		// input_text / output_text 类型
		if item.Get("type").String() == "input_text" || item.Get("type").String() == "output_text" {
			if t := item.Get("text"); t.Exists() {
				texts = append(texts, t.String())
			}
		}
	}
	return strings.Join(texts, "\n")
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
