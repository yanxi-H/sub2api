package admin

import (
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RequestArchiveHandler 请求文本存档的管理后台接口。
type RequestArchiveHandler struct {
	svc *service.RequestArchiveService
}

// NewRequestArchiveHandler 创建请求存档 handler。
func NewRequestArchiveHandler(svc *service.RequestArchiveService) *RequestArchiveHandler {
	return &RequestArchiveHandler{svc: svc}
}

// requestArchiveListResponse 列表项(含分页)。
type requestArchiveListResponse struct {
	Items []requestArchiveListItem `json:"items"`
	Total int64                    `json:"total"`
	Page  int                      `json:"page"`
	Size  int                      `json:"page_size"`
}

type requestArchiveListItem struct {
	ID          int64     `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	RequestID   string    `json:"request_id"`
	UserID      int64     `json:"user_id"`
	UserEmail   string    `json:"user_email"`
	APIKeyID    int64     `json:"api_key_id"`
	APIKeyName  string    `json:"api_key_name"`
	GroupID     *int64    `json:"group_id"`
	Endpoint    string    `json:"endpoint"`
	Protocol    string    `json:"protocol"`
	Model       string    `json:"model"`
	IPAddress   string    `json:"ip_address"`
	PromptPreview string  `json:"prompt_preview"`
	Truncated   bool      `json:"truncated"`
}

type requestArchiveDetailResponse struct {
	requestArchiveListItem
	PromptText string `json:"prompt_text"`
}

// List handles GET /api/v1/admin/request-archive
// 查询参数:search(关键词), user_id, api_key_id, start_date, end_date (YYYY-MM-DD), page, page_size
func (h *RequestArchiveHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	search := c.Query("search")
	var userID, apiKeyID *int64
	if uidStr := c.Query("user_id"); uidStr != "" {
		if uid, err := strconv.ParseInt(uidStr, 10, 64); err == nil {
			userID = &uid
		}
	}
	if kidStr := c.Query("api_key_id"); kidStr != "" {
		if kid, err := strconv.ParseInt(kidStr, 10, 64); err == nil {
			apiKeyID = &kid
		}
	}

	var startDate, endDate *time.Time
	if sd := c.Query("start_date"); sd != "" {
		if t, err := time.Parse("2006-01-02", sd); err == nil {
			startDate = &t
		}
	}
	if ed := c.Query("end_date"); ed != "" {
		if t, err := time.Parse("2006-01-02", ed); err == nil {
			// end_date 包含当天结束
			nextDay := t.AddDate(0, 0, 1)
			endDate = &nextDay
		}
	}

	repo := h.repo()
	if repo == nil {
		response.Success(c, requestArchiveListResponse{Items: []requestArchiveListItem{}, Total: 0, Page: page, Size: pageSize})
		return
	}

	entries, total, err := repo.List(c.Request.Context(), page, pageSize, search, userID, apiKeyID, startDate, endDate)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	items := make([]requestArchiveListItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, toRequestArchiveListItem(e))
	}
	response.Success(c, requestArchiveListResponse{Items: items, Total: total, Page: page, Size: pageSize})
}

// GetDetail handles GET /api/v1/admin/request-archive/:id
func (h *RequestArchiveHandler) GetDetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	repo := h.repo()
	if repo == nil {
		response.NotFound(c, "request archive not available")
		return
	}
	entry, err := repo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	resp := requestArchiveDetailResponse{
		requestArchiveListItem: toRequestArchiveListItem(*entry),
		PromptText:             entry.PromptText,
	}
	response.Success(c, resp)
}

// GetStatus handles GET /api/v1/admin/request-archive/status
func (h *RequestArchiveHandler) GetStatus(c *gin.Context) {
	response.Success(c, gin.H{
		"enabled": h.svc.IsEnabled(),
	})
}

func (h *RequestArchiveHandler) repo() service.RequestArchiveRepository {
	if h.svc == nil {
		return nil
	}
	return h.svc.Repository()
}

func toRequestArchiveListItem(e service.RequestArchiveEntry) requestArchiveListItem {
	return requestArchiveListItem{
		ID:            e.ID,
		CreatedAt:     e.CreatedAt,
		RequestID:     e.RequestID,
		UserID:        e.UserID,
		UserEmail:     e.UserEmail,
		APIKeyID:      e.APIKeyID,
		APIKeyName:    e.APIKeyName,
		GroupID:       e.GroupID,
		Endpoint:      e.Endpoint,
		Protocol:      e.Protocol,
		Model:         e.Model,
		IPAddress:     e.IPAddress,
		PromptPreview: e.PromptText,
		Truncated:     e.Truncated,
	}
}
