package admin

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *OpsHandler) GetMonitorCenterOpenAIStatus(c *gin.Context) {
	if h == nil || h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	result, err := h.opsService.GetMonitorCenterOpenAIStatus(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *OpsHandler) GetMonitorCenterOpenAIHistory(c *gin.Context) {
	if h == nil || h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	start, end, err := parseOpsTimeRange(c, "1h")
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.opsService.GetMonitorCenterOpenAIHistory(c.Request.Context(), start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *OpsHandler) GetMonitorCenterProbe(c *gin.Context) {
	if h == nil || h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	start, end, err := parseOpsTimeRange(c, "1h")
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.opsService.GetMonitorCenterProbe(c.Request.Context(), start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
