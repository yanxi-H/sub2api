package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPerformanceDiagnosticsRouteRejectsInvalidTimeRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/ops/dashboard/performance-diagnostics", NewOpsHandler(&service.OpsService{}).GetDashboardPerformanceDiagnostics)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/ops/dashboard/performance-diagnostics?start_time=bad&end_time=2026-07-26T09:00:00Z", nil)

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
