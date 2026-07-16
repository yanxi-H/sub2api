// Package handler provides HTTP request handlers for the application.
package handler

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// APIKeyHandler handles API key-related requests
type APIKeyHandler struct {
	apiKeyService *service.APIKeyService
}

// NewAPIKeyHandler creates a new APIKeyHandler
func NewAPIKeyHandler(apiKeyService *service.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{
		apiKeyService: apiKeyService,
	}
}

// CreateAPIKeyRequest represents the create API key request payload
type CreateAPIKeyRequest struct {
	Name          string   `json:"name" binding:"required"`
	GroupID       *int64   `json:"group_id"`        // nullable
	CustomKey     *string  `json:"custom_key"`      // 可选的自定义key
	IPWhitelist   []string `json:"ip_whitelist"`    // IP 白名单
	IPBlacklist   []string `json:"ip_blacklist"`    // IP 黑名单
	Quota         *float64 `json:"quota"`           // 配额限制 (USD)
	ExpiresInDays *int     `json:"expires_in_days"` // 过期天数

	// Rate limit fields (0 = unlimited)
	RateLimit5h *float64 `json:"rate_limit_5h"`
	RateLimit1d *float64 `json:"rate_limit_1d"`
	RateLimit7d *float64 `json:"rate_limit_7d"`

	// 管理员指定：Key 归属的目标用户。仅管理员可使用；普通用户传入会被忽略。
	UserID *int64 `json:"user_id"`
}

// UpdateAPIKeyRequest represents the update API key request payload
type UpdateAPIKeyRequest struct {
	Name        string   `json:"name"`
	GroupID     *int64   `json:"group_id"`
	Status      string   `json:"status" binding:"omitempty,oneof=active inactive"`
	IPWhitelist []string `json:"ip_whitelist"` // IP 白名单
	IPBlacklist []string `json:"ip_blacklist"` // IP 黑名单
	Quota       *float64 `json:"quota"`        // 配额限制 (USD), 0=无限制
	ExpiresAt   *string  `json:"expires_at"`   // 过期时间 (ISO 8601)
	ResetQuota  *bool    `json:"reset_quota"`  // 重置已用配额

	// Rate limit fields (nil = no change, 0 = unlimited)
	RateLimit5h         *float64 `json:"rate_limit_5h"`
	RateLimit1d         *float64 `json:"rate_limit_1d"`
	RateLimit7d         *float64 `json:"rate_limit_7d"`
	ResetRateLimitUsage *bool    `json:"reset_rate_limit_usage"` // 重置限速用量
	Sync7dWindowAccountID *int64 `json:"sync_7d_window_account_id"` // 同步上游账号7天窗口(管理员)

	// 管理员修改：Key 的归属用户。仅管理员可使用；普通用户传入会被忽略。
	UserID *int64 `json:"user_id"`
}

// List handles listing user's API keys with pagination
// GET /api/v1/api-keys
func (h *APIKeyHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}

	// Parse filter parameters
	var filters service.APIKeyListFilters
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		if len(search) > 100 {
			search = search[:100]
		}
		filters.Search = search
	}
	filters.Status = c.Query("status")
	if groupIDStr := c.Query("group_id"); groupIDStr != "" {
		gid, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err == nil {
			filters.GroupID = &gid
		}
	}

	// 管理员走全局视图：不按调用者 userID 过滤，列出全系统所有用户的 Key。
	// user_id 筛选参数仅对管理员生效，普通用户传入会被忽略以防越权。
	isAdmin := isAdminRole(c)
	if isAdmin {
		filters.AdminMode = true
		if userIDStr := c.Query("user_id"); userIDStr != "" {
			if uid, err := strconv.ParseInt(userIDStr, 10, 64); err == nil {
				filters.UserID = &uid
			}
		}
	}

	keys, result, err := h.apiKeyService.List(c.Request.Context(), subject.UserID, params, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.APIKey, 0, len(keys))
	for i := range keys {
		out = append(out, *dto.APIKeyFromService(&keys[i]))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

// isAdminRole 报告当前请求的登录用户是否为管理员。
func isAdminRole(c *gin.Context) bool {
	role, ok := middleware2.GetUserRoleFromContext(c)
	return ok && role == service.RoleAdmin
}

func requireAdminAPIKeyManagement(c *gin.Context) bool {
	if isAdminRole(c) {
		return true
	}
	response.Forbidden(c, "Only administrators can manage API keys")
	return false
}

// GetByID handles getting a single API key
// GET /api/v1/api-keys/:id
func (h *APIKeyHandler) GetByID(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid key ID")
		return
	}

	key, err := h.apiKeyService.GetByID(c.Request.Context(), keyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 验证所有权（管理员可查看任意用户的 Key）
	if !isAdminRole(c) && key.UserID != subject.UserID {
		response.NotFound(c, "API key not found")
		return
	}

	response.Success(c, dto.APIKeyFromService(key))
}

// Create handles creating a new API key
// POST /api/v1/api-keys
func (h *APIKeyHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if !requireAdminAPIKeyManagement(c) {
		return
	}

	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	svcReq := service.CreateAPIKeyRequest{
		Name:          req.Name,
		GroupID:       req.GroupID,
		CustomKey:     req.CustomKey,
		IPWhitelist:   req.IPWhitelist,
		IPBlacklist:   req.IPBlacklist,
		ExpiresInDays: req.ExpiresInDays,
	}
	if req.Quota != nil {
		svcReq.Quota = *req.Quota
	}
	if req.RateLimit5h != nil {
		svcReq.RateLimit5h = *req.RateLimit5h
	}
	if req.RateLimit1d != nil {
		svcReq.RateLimit1d = *req.RateLimit1d
	}
	if req.RateLimit7d != nil {
		svcReq.RateLimit7d = *req.RateLimit7d
	}

	// 解析目标归属用户：管理员指定了 user_id 则归属该用户，否则归属操作者本人。
	isAdmin := isAdminRole(c)
	targetUserID := subject.UserID
	if isAdmin && req.UserID != nil {
		targetUserID = *req.UserID
	}

	executeUserIdempotentJSON(c, "user.api_keys.create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		var key *service.APIKey
		var err error
		if isAdmin {
			key, err = h.apiKeyService.CreateAsAdmin(ctx, targetUserID, svcReq)
		} else {
			key, err = h.apiKeyService.Create(ctx, targetUserID, svcReq)
		}
		if err != nil {
			return nil, err
		}
		return dto.APIKeyFromService(key), nil
	})
}

// Update handles updating an API key
// PUT /api/v1/api-keys/:id
func (h *APIKeyHandler) Update(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if !requireAdminAPIKeyManagement(c) {
		return
	}

	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid key ID")
		return
	}

	var req UpdateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	svcReq := service.UpdateAPIKeyRequest{
		IPWhitelist:         req.IPWhitelist,
		IPBlacklist:         req.IPBlacklist,
		Quota:               req.Quota,
		ResetQuota:          req.ResetQuota,
		RateLimit5h:         req.RateLimit5h,
		RateLimit1d:         req.RateLimit1d,
		RateLimit7d:         req.RateLimit7d,
		ResetRateLimitUsage:   req.ResetRateLimitUsage,
		Sync7dWindowAccountID: req.Sync7dWindowAccountID,
	}
	if req.Name != "" {
		svcReq.Name = &req.Name
	}
	svcReq.GroupID = req.GroupID
	if req.Status != "" {
		svcReq.Status = &req.Status
	}
	// Parse expires_at if provided
	if req.ExpiresAt != nil {
		if *req.ExpiresAt == "" {
			// Empty string means clear expiration
			svcReq.ExpiresAt = nil
			svcReq.ClearExpiration = true
		} else {
			t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err != nil {
				response.BadRequest(c, "Invalid expires_at format: "+err.Error())
				return
			}
			svcReq.ExpiresAt = &t
		}
	}

	// 管理员修改归属用户：透传给 service，仅 UpdateAsAdmin 路径会使用。
	if isAdminRole(c) && req.UserID != nil {
		svcReq.UserID = req.UserID
	}

	if isAdminRole(c) {
		key, err := h.apiKeyService.UpdateAsAdmin(c.Request.Context(), keyID, subject.UserID, svcReq)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		response.Success(c, dto.APIKeyFromService(key))
		return
	}

	key, err := h.apiKeyService.Update(c.Request.Context(), keyID, subject.UserID, svcReq)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.APIKeyFromService(key))
}

// Delete handles deleting an API key
// DELETE /api/v1/api-keys/:id
func (h *APIKeyHandler) Delete(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if !requireAdminAPIKeyManagement(c) {
		return
	}

	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid key ID")
		return
	}

	if isAdminRole(c) {
		err = h.apiKeyService.DeleteAsAdmin(c.Request.Context(), keyID, subject.UserID)
	} else {
		err = h.apiKeyService.Delete(c.Request.Context(), keyID, subject.UserID)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "API key deleted successfully"})
}

// GetAvailableGroups 获取用户可以绑定的分组列表
// GET /api/v1/groups/available
func (h *APIKeyHandler) GetAvailableGroups(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	groups, err := h.apiKeyService.GetAvailableGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.Group, 0, len(groups))
	for i := range groups {
		out = append(out, *dto.GroupFromService(&groups[i]))
	}
	response.Success(c, out)
}

// GetUserGroupRates 获取当前用户的专属分组倍率配置
// GET /api/v1/groups/rates
func (h *APIKeyHandler) GetUserGroupRates(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	rates, err := h.apiKeyService.GetUserGroupRates(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, rates)
}
