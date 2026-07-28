package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestRequestBodyAdmissionMaximumMatchesDecompressionMaximum(t *testing.T) {
	require.Equal(t, pkghttputil.MaxDecompressedBodySize, service.MaxRequestBodyAdmissionLimitBytes)
}

type requestBodyLaneTestCache struct {
	*concurrencyCacheMock
	acquireErr error
	wait       bool
}

func (c *requestBodyLaneTestCache) AcquireRequestBodyLane(context.Context, service.RequestBodyLane, int64, int64, int, int, string) (bool, error) {
	return false, c.acquireErr
}

func (c *requestBodyLaneTestCache) ReleaseRequestBodyLane(context.Context, service.RequestBodyLane, int64, int64, int, string) error {
	return nil
}

func (c *requestBodyLaneTestCache) IncrementRequestBodyLaneWaitCount(context.Context, int64, int, string) (bool, error) {
	return c.wait, nil
}

func (c *requestBodyLaneTestCache) DecrementRequestBodyLaneWaitCount(context.Context, int64, string) error {
	return nil
}

func TestRequestBodyLimitTooLarge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limit := int64(16)
	router := gin.New()
	router.Use(middleware.RequestBodyLimit(limit))
	router.POST("/test", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			if maxErr, ok := extractMaxBytesError(err); ok {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{
					"error": buildBodyTooLargeMessage(maxErr.Limit),
				})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "read_failed",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	payload := bytes.Repeat([]byte("a"), int(limit+1))
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(payload))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.Contains(t, recorder.Body.String(), buildBodyTooLargeMessage(limit))
}

func TestOpenAIResponsesReadLimitHonorsLowerTextLimit(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.MaxBodySize = 16
	cfg.Gateway.TextMaxBodySize = 8
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(bytes.Repeat([]byte("a"), 9)))

	_, err := readOpenAIResponsesRequestBodyWithPrealloc(req, cfg)

	var maxErr *http.MaxBytesError
	require.ErrorAs(t, err, &maxErr)
	require.Equal(t, int64(8), maxErr.Limit)
}

func TestRequestBodyAdmissionPolicyIgnoresLegacyHardLimit(t *testing.T) {
	account := &service.Account{
		Platform: service.PlatformOpenAI,
		Extra: map[string]any{
			service.LegacyRequestBodyLimitExtraKey:       int64(10),
			service.LegacyCompactBodyLimitBypassExtraKey: true,
		},
	}
	require.Equal(t, service.RequestBodyLaneDisabled, account.GetRequestBodyAdmissionPolicy().Classify(100, false))
}

func TestRequestBodyAdmissionPolicyClassifiesConfiguredLanes(t *testing.T) {
	account := &service.Account{
		Platform: service.PlatformOpenAI,
		Extra: map[string]any{
			service.RequestBodyAdmissionEnabledExtraKey: true,
			service.RequestBodyNormalLimitExtraKey:      int64(10),
			service.RequestBodyHeavyLimitExtraKey:       int64(20),
			service.RequestBodyRecoveryLimitExtraKey:    int64(30),
		},
	}
	policy := account.GetRequestBodyAdmissionPolicy()
	require.Equal(t, service.RequestBodyLaneNormal, policy.Classify(10, false))
	require.Equal(t, service.RequestBodyLaneHeavy, policy.Classify(11, false))
	require.Equal(t, service.RequestBodyLaneRejected, policy.Classify(21, false))
	require.Equal(t, service.RequestBodyLaneRecovery, policy.Classify(1, true))
	require.Equal(t, service.RequestBodyLaneRejected, policy.Classify(31, true))
}

func TestEnsureRecoveryExecutionDeadlineOnlyAppliesOnceToRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var deadline responsesRecoveryDeadline
	for _, lane := range []service.RequestBodyLane{service.RequestBodyLaneNormal, service.RequestBodyLaneHeavy} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", http.NoBody)
		cancel := deadline.apply(c, lane)
		_, hasDeadline := c.Request.Context().Deadline()
		require.False(t, hasDeadline)
		require.Nil(t, cancel)
	}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", http.NoBody)
	cancel := deadline.apply(c, service.RequestBodyLaneRecovery)
	require.NotNil(t, cancel)
	firstDeadline, hasDeadline := c.Request.Context().Deadline()
	require.True(t, hasDeadline)
	require.WithinDuration(t, time.Now().Add(requestBodyRecoveryExecutionLimit), firstDeadline, time.Second)
	cancel()

	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", http.NoBody)
	cancel = deadline.apply(c, service.RequestBodyLaneRecovery)
	t.Cleanup(cancel)
	secondDeadline, hasDeadline := c.Request.Context().Deadline()
	require.True(t, hasDeadline)
	require.Equal(t, firstDeadline, secondDeadline, "failover must reuse the original recovery deadline")
}

func TestReleaseSelectionForRequestBodyLaneWait(t *testing.T) {
	released := 0
	selection := &service.AccountSelectionResult{
		Account:  &service.Account{ID: 9, Concurrency: 10},
		Acquired: true,
		ReleaseFunc: func() {
			released++
		},
	}

	releaseSelectionForRequestBodyLaneWait(selection)
	require.Equal(t, 1, released)
	require.False(t, selection.Acquired)
	require.Nil(t, selection.ReleaseFunc)
	require.NotNil(t, selection.WaitPlan)
	require.Equal(t, 10, selection.WaitPlan.MaxConcurrency)
}

func TestLargeResponsesAccountSlotFollowsDetachedUpstreamLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, lane := range []service.RequestBodyLane{service.RequestBodyLaneHeavy, service.RequestBodyLaneRecovery} {
		t.Run(string(lane), func(t *testing.T) {
			cache := &concurrencyCacheMock{
				acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) {
					return true, nil
				},
			}
			h := &OpenAIGatewayHandler{
				gatewayService:    &service.OpenAIGatewayService{},
				concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, 0),
			}
			clientCtx, cancelClient := context.WithCancel(context.Background())
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", http.NoBody).WithContext(clientCtx)
			_, cancelAdmission := installRequestBodyAdmissionContexts(c)
			selection := &service.AccountSelectionResult{
				Account: &service.Account{ID: 9, Concurrency: 4},
				WaitPlan: &service.AccountWaitPlan{
					MaxConcurrency: 4,
					MaxWaiting:     1,
					Timeout:        time.Second,
				},
			}
			streamStarted := false

			release, acquired := h.acquireResponsesAccountSlotForLane(
				c, nil, "", selection, lane, false, &streamStarted, nil,
			)
			require.True(t, acquired)
			require.NotNil(t, release)

			cancelClient()
			select {
			case <-c.Request.Context().Done():
			case <-time.After(time.Second):
				t.Fatal("handler context did not observe client cancellation")
			}
			time.Sleep(25 * time.Millisecond)
			require.Zero(t, atomic.LoadInt32(&cache.releaseAccountCalled), "client disconnect must not release a slot while upstream draining continues")

			cancelAdmission()
			require.Eventually(t, func() bool {
				return atomic.LoadInt32(&cache.releaseAccountCalled) == 1
			}, time.Second, 10*time.Millisecond)
			release()
			require.Equal(t, int32(1), atomic.LoadInt32(&cache.releaseAccountCalled))
		})
	}
}

func TestOrdinaryRequestAboveHeavyLimitUsesStableCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", http.NoBody)
	selection := &service.AccountSelectionResult{Account: &service.Account{
		ID:       9,
		Platform: service.PlatformOpenAI,
		Extra: map[string]any{
			service.RequestBodyAdmissionEnabledExtraKey: true,
			service.RequestBodyNormalLimitExtraKey:      int64(10),
			service.RequestBodyHeavyLimitExtraKey:       int64(20),
			service.RequestBodyRecoveryLimitExtraKey:    int64(30),
		},
	}}
	streamStarted := false

	_, _, admitted, _ := (&OpenAIGatewayHandler{}).acquireResponsesRequestBodyLane(
		c, nil, selection, 1, 31, false, false, &streamStarted, nil, nil, false,
	)

	require.False(t, admitted)
	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.Equal(t, "invalid_request_error", gjson.GetBytes(recorder.Body.Bytes(), "error.type").String())
	require.Equal(t, requestBodyHeavyLimitExceededCode, gjson.GetBytes(recorder.Body.Bytes(), "error.code").String())
	require.Contains(t, gjson.GetBytes(recorder.Body.Bytes(), "error.message").String(), "compact the conversation")
}

func TestCompactRequestAboveRecoveryLimitUsesStableCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", http.NoBody)
	selection := &service.AccountSelectionResult{Account: &service.Account{
		ID:       9,
		Platform: service.PlatformOpenAI,
		Extra: map[string]any{
			service.RequestBodyAdmissionEnabledExtraKey: true,
			service.RequestBodyNormalLimitExtraKey:      int64(10),
			service.RequestBodyHeavyLimitExtraKey:       int64(20),
			service.RequestBodyRecoveryLimitExtraKey:    int64(30),
		},
	}}
	streamStarted := false

	_, _, admitted, _ := (&OpenAIGatewayHandler{}).acquireResponsesRequestBodyLane(
		c, nil, selection, 1, 31, true, false, &streamStarted, nil, nil, false,
	)

	require.False(t, admitted)
	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.Equal(t, requestBodyRecoveryLimitExceededCode, gjson.GetBytes(recorder.Body.Bytes(), "error.code").String())
}

func TestRequestBodyRejectedAcrossAccountsUsesLargestEligibleLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", http.NoBody)

	(&OpenAIGatewayHandler{}).rejectResponsesRequestBodyAcrossAccounts(c, 31, false, 30, false)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.Equal(t, requestBodyHeavyLimitExceededCode, gjson.GetBytes(recorder.Body.Bytes(), "error.code").String())
	require.Contains(t, gjson.GetBytes(recorder.Body.Bytes(), "error.message").String(), "largest configured limit is 30 bytes")
	require.Equal(t, string(service.RequestBodyLaneRejected), recorder.Header().Get("X-Sub2API-Request-Body-Lane"))
}

func TestRequestBodyAdmissionQueueFullUsesRateLimitTypeAndStableCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", http.NoBody)
	cache := &requestBodyLaneTestCache{concurrencyCacheMock: &concurrencyCacheMock{}}
	h := &OpenAIGatewayHandler{
		concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, 0),
	}
	selection := &service.AccountSelectionResult{Account: &service.Account{
		ID:          9,
		Platform:    service.PlatformOpenAI,
		Concurrency: 5,
		Extra: map[string]any{
			service.RequestBodyAdmissionEnabledExtraKey: true,
			service.RequestBodyNormalLimitExtraKey:      int64(10),
			service.RequestBodyHeavyLimitExtraKey:       int64(20),
			service.RequestBodyRecoveryLimitExtraKey:    int64(30),
		},
	}}
	streamStarted := false

	_, lane, admitted, retryOtherAccount := h.acquireResponsesRequestBodyLane(
		c, nil, selection, 1, 11, false, false, &streamStarted, nil, nil, true,
	)

	require.False(t, admitted)
	require.False(t, retryOtherAccount)
	require.Equal(t, service.RequestBodyLaneHeavy, lane)
	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
	require.Equal(t, "rate_limit_error", gjson.GetBytes(recorder.Body.Bytes(), "error.type").String())
	require.Equal(t, largeRequestQueueTimeoutCode, gjson.GetBytes(recorder.Body.Bytes(), "error.code").String())
}

func TestRequestBodyAdmissionUnavailableUsesAPIErrorTypeAndStableCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", http.NoBody)
	cache := &requestBodyLaneTestCache{
		concurrencyCacheMock: &concurrencyCacheMock{},
		acquireErr:           errors.New("redis unavailable"),
	}
	h := &OpenAIGatewayHandler{
		concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, 0),
	}
	selection := &service.AccountSelectionResult{Account: &service.Account{
		ID:          9,
		Platform:    service.PlatformOpenAI,
		Concurrency: 5,
		Extra: map[string]any{
			service.RequestBodyAdmissionEnabledExtraKey: true,
			service.RequestBodyNormalLimitExtraKey:      int64(10),
			service.RequestBodyHeavyLimitExtraKey:       int64(20),
			service.RequestBodyRecoveryLimitExtraKey:    int64(30),
		},
	}}
	streamStarted := false

	_, lane, admitted, _ := h.acquireResponsesRequestBodyLane(
		c, nil, selection, 1, 11, false, false, &streamStarted, nil, nil, false,
	)

	require.False(t, admitted)
	require.Equal(t, service.RequestBodyLaneHeavy, lane)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Equal(t, "api_error", gjson.GetBytes(recorder.Body.Bytes(), "error.type").String())
	require.Equal(t, requestBodyAdmissionUnavailableCode, gjson.GetBytes(recorder.Body.Bytes(), "error.code").String())
}
