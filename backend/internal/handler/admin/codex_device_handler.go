package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
)

// CodexDeviceHandler 管理网关观测到的 Codex 真实设备 ID 列表
// （供账号指纹收敛 device 档选择，避免手工抓包）。
type CodexDeviceHandler struct {
	recorder service.CodexSeenDeviceRecorder
}

func NewCodexDeviceHandler(recorder service.CodexSeenDeviceRecorder) *CodexDeviceHandler {
	return &CodexDeviceHandler{recorder: recorder}
}

// List GET /admin/codex/seen-devices?api_key_id=1
func (h *CodexDeviceHandler) List(c *gin.Context) {
	if c.Query("api_key_id") == "" {
		entries, err := h.recorder.ListAll(c.Request.Context())
		if err != nil {
			response.Error(c, 500, "Failed to list seen devices")
			return
		}
		response.Success(c, gin.H{"devices": entries})
		return
	}
	apiKeyID, err := strconv.ParseInt(c.Query("api_key_id"), 10, 64)
	if err != nil || apiKeyID <= 0 {
		response.BadRequest(c, "Invalid api_key_id")
		return
	}
	devices, err := h.recorder.ListByAPIKey(c.Request.Context(), apiKeyID)
	if err != nil {
		response.Error(c, 500, "Failed to list seen devices")
		return
	}
	response.Success(c, gin.H{"devices": devices})
}

// Delete DELETE /admin/codex/seen-devices?api_key_id=1&device_id=xxx
func (h *CodexDeviceHandler) Delete(c *gin.Context) {
	apiKeyID, err := strconv.ParseInt(c.Query("api_key_id"), 10, 64)
	if err != nil || apiKeyID <= 0 {
		response.BadRequest(c, "Invalid api_key_id")
		return
	}
	deviceID := c.Query("device_id")
	if deviceID == "" {
		response.BadRequest(c, "device_id is required")
		return
	}
	if err := h.recorder.Delete(c.Request.Context(), apiKeyID, deviceID); err != nil {
		response.Error(c, 500, "Failed to delete seen device")
		return
	}
	response.Success(c, gin.H{"deleted": deviceID})
}
