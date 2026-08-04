package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type codexRadarDashboardServiceStub struct {
	result *service.CodexRadarDashboardRecommendations
	err    error
	calls  int
}

func (s *codexRadarDashboardServiceStub) GetDashboardRecommendations(context.Context) (*service.CodexRadarDashboardRecommendations, error) {
	s.calls++
	return s.result, s.err
}

func TestUsageHandlerDashboardRecommendations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &codexRadarDashboardServiceStub{
		result: &service.CodexRadarDashboardRecommendations{
			StationAvailable: true,
			StationRecommendations: []service.CodexRadarStationRecommendationSet{{
				Key:   "daily_development",
				Items: []service.CodexRadarStationRecommendation{{Model: "model-a", Effort: "high"}},
			}},
		},
	}
	h := NewUsageHandler(nil, nil, nil, nil, stub)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/usage/dashboard/recommendations", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 1})

	h.DashboardRecommendations(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	require.Equal(t, 1, stub.calls)

	var got response.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &got))
	require.Equal(t, 0, got.Code)
}

func TestUsageHandlerDashboardRecommendationsRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &codexRadarDashboardServiceStub{}
	h := NewUsageHandler(nil, nil, nil, nil, stub)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/usage/dashboard/recommendations", nil)

	h.DashboardRecommendations(c)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Equal(t, 0, stub.calls)
}
