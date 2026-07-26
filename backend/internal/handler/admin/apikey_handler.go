package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type AdminBatchAPIKeyRequest struct {
	APIKeyIDs []int64 `json:"api_key_ids" binding:"required"`
	GroupID   *int64  `json:"group_id" binding:"required"`
}

type AdminBatchSyncAPIKey7dWindowRequest struct {
	APIKeyIDs []int64 `json:"api_key_ids" binding:"required"`
	GroupID   *int64  `json:"group_id" binding:"required"`
	AccountID int64   `json:"account_id" binding:"required"`
}

// AdminAPIKeyHandler handles admin API key management
type AdminAPIKeyHandler struct {
	adminService service.AdminService
}

// NewAdminAPIKeyHandler creates a new admin API key handler
func NewAdminAPIKeyHandler(adminService service.AdminService) *AdminAPIKeyHandler {
	return &AdminAPIKeyHandler{
		adminService: adminService,
	}
}

// AdminUpdateAPIKeyGroupRequest represents the request to update an API key.
type AdminUpdateAPIKeyGroupRequest struct {
	GroupID             *int64 `json:"group_id"`               // nil=不修改, 0=解绑, >0=绑定到目标分组
	ResetRateLimitUsage *bool  `json:"reset_rate_limit_usage"` // true=重置 5h/1d/7d 限速用量
}

// UpdateGroup handles updating an API key's admin-managed fields.
// PUT /api/v1/admin/api-keys/:id
func (h *AdminAPIKeyHandler) UpdateGroup(c *gin.Context) {
	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid API key ID")
		return
	}

	var req AdminUpdateAPIKeyGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	var resetKey *service.APIKey
	if req.ResetRateLimitUsage != nil && *req.ResetRateLimitUsage {
		resetKey, err = h.adminService.AdminResetAPIKeyRateLimitUsage(c.Request.Context(), keyID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	result, err := h.adminService.AdminUpdateAPIKeyGroupID(c.Request.Context(), keyID, req.GroupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if resetKey != nil && req.GroupID == nil {
		result.APIKey = resetKey
	}

	resp := struct {
		APIKey                 *dto.APIKey `json:"api_key"`
		AutoGrantedGroupAccess bool        `json:"auto_granted_group_access"`
		GrantedGroupID         *int64      `json:"granted_group_id,omitempty"`
		GrantedGroupName       string      `json:"granted_group_name,omitempty"`
	}{
		APIKey:                 dto.APIKeyFromService(result.APIKey),
		AutoGrantedGroupAccess: result.AutoGrantedGroupAccess,
		GrantedGroupID:         result.GrantedGroupID,
		GrantedGroupName:       result.GrantedGroupName,
	}
	response.Success(c, resp)
}

// BatchSync7dWindow aligns selected API keys to an upstream account's known 7-day window.
func (h *AdminAPIKeyHandler) BatchSync7dWindow(c *gin.Context) {
	var req AdminBatchSyncAPIKey7dWindowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	keys, err := h.adminService.AdminBatchSyncAPIKey7dWindow(c.Request.Context(), req.APIKeyIDs, *req.GroupID, req.AccountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, adminBatchAPIKeyResponse(keys))
}

// BatchReset7dUsage resets only the selected API keys' 7-day usage.
func (h *AdminAPIKeyHandler) BatchReset7dUsage(c *gin.Context) {
	var req AdminBatchAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	keys, err := h.adminService.AdminBatchResetAPIKey7dUsage(c.Request.Context(), req.APIKeyIDs, *req.GroupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, adminBatchAPIKeyResponse(keys))
}

func adminBatchAPIKeyResponse(keys []*service.APIKey) struct {
	Items        []*dto.APIKey `json:"items"`
	UpdatedCount int           `json:"updated_count"`
} {
	items := make([]*dto.APIKey, 0, len(keys))
	for _, key := range keys {
		items = append(items, dto.APIKeyFromService(key))
	}
	return struct {
		Items        []*dto.APIKey `json:"items"`
		UpdatedCount int           `json:"updated_count"`
	}{Items: items, UpdatedCount: len(items)}
}
