package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

func newCompactBodySignalTestContext(t *testing.T, path string, body []byte) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestNormalizeOpenAIResponsesCompactRequest_RemoteV2StaysOnResponses(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	body := []byte(`{
		"model":"gpt-5.6-sol",
		"stream":true,
		"store":true,
		"prompt_cache_key":"pck-signal-1",
		"reasoning":{"effort":"max","context":"all_turns"},
		"input":[
			{"type":"message","role":"user","content":"hello"},
			{"type":"compaction_trigger"}
		]
	}`)
	c := newCompactBodySignalTestContext(t, "/v1/responses", body)
	c.Request.Header.Set("x-codex-beta-features", "responses_websockets_v2, remote_compaction_v2, another_feature")

	normalized, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)
	require.True(t, ok)

	require.Equal(t, "/v1/responses", c.Request.URL.Path)
	require.False(t, isOpenAIRemoteCompactPath(c))
	require.Equal(t, body, normalized)
	require.True(t, gjson.GetBytes(normalized, "stream").Bool())
	require.True(t, gjson.GetBytes(normalized, "store").Bool())
	require.Equal(t, "pck-signal-1", gjson.GetBytes(normalized, "prompt_cache_key").String())
	require.Equal(t, "max", gjson.GetBytes(normalized, "reasoning.effort").String())
	require.Equal(t, "all_turns", gjson.GetBytes(normalized, "reasoning.context").String())

	reqStream, streamOK := parseOpenAICompatibleStream(normalized)
	require.True(t, streamOK)
	require.True(t, reqStream)

	_, seedExists := c.Get(service.OpenAICompactSessionSeedKeyForTest())
	require.False(t, seedExists)
	_, streamMarkerExists := c.Get(service.OpenAICompactClientStreamKeyForTest())
	require.False(t, streamMarkerExists)
}

func TestNormalizeOpenAIResponsesCompactRequest_RemoteV2PathAliasesStayOnResponses(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	body := []byte(`{"model":"gpt-5.6-sol","stream":true,"input":[{"type":"compaction_trigger"}]}`)
	for _, path := range []string{"/v1/responses/", "/backend-api/codex/responses"} {
		t.Run(path, func(t *testing.T) {
			c := newCompactBodySignalTestContext(t, path, body)
			c.Request.Header.Set("x-codex-beta-features", "remote_compaction_v2")

			normalized, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)
			require.True(t, ok)
			require.Equal(t, path, c.Request.URL.Path)
			require.Equal(t, body, normalized)
		})
	}
}

func TestOpenAICompactRequestUsesRecoveryLane(t *testing.T) {
	compactBody := []byte(`{"model":"gpt-5.6-sol","stream":true,"input":[{"type":"compaction_trigger"}]}`)
	normalBody := []byte(`{"model":"gpt-5.6-sol","stream":true,"input":"hello"}`)
	account := &service.Account{
		Platform: service.PlatformOpenAI,
		Extra: map[string]any{
			service.RequestBodyAdmissionEnabledExtraKey: true,
			service.RequestBodyNormalLimitExtraKey:      int64(10),
			service.RequestBodyHeavyLimitExtraKey:       int64(20),
			service.RequestBodyRecoveryLimitExtraKey:    int64(30),
		},
	}

	tests := []struct {
		name      string
		path      string
		body      []byte
		bodyBytes int64
		want      service.RequestBodyLane
	}{
		{
			name:      "explicit compact path",
			path:      "/v1/responses/compact",
			body:      normalBody,
			bodyBytes: 5,
			want:      service.RequestBodyLaneRecovery,
		},
		{
			name:      "legacy compaction body signal",
			path:      "/v1/responses",
			body:      compactBody,
			bodyBytes: 11,
			want:      service.RequestBodyLaneRecovery,
		},
		{
			name:      "ordinary responses request",
			path:      "/v1/responses",
			body:      normalBody,
			bodyBytes: 11,
			want:      service.RequestBodyLaneHeavy,
		},
		{
			name:      "ordinary request above heavy limit",
			path:      "/v1/responses",
			body:      normalBody,
			bodyBytes: 21,
			want:      service.RequestBodyLaneRejected,
		},
		{
			name:      "request exceeds recovery limit",
			path:      "/v1/responses",
			body:      normalBody,
			bodyBytes: 31,
			want:      service.RequestBodyLaneRejected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCompactBodySignalTestContext(t, tt.path, tt.body)
			compactRequest := isOpenAICompactRequest(c, tt.body)
			require.Equal(t, tt.want, account.GetRequestBodyAdmissionPolicy().Classify(tt.bodyBytes, compactRequest))
		})
	}
}

func TestOpenAICompactRequestClassificationUsesOriginalBodySignal(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6-sol","stream":true,"input":[{"type":"compaction_trigger"}]}`)
	account := &service.Account{
		Platform: service.PlatformOpenAI,
		Extra: map[string]any{
			service.RequestBodyAdmissionEnabledExtraKey: true,
		},
	}
	c := newCompactBodySignalTestContext(t, "/v1/responses", body)

	// The decision must use the original request. Normalization below rewrites
	// legacy body-signals to /responses/compact for upstream compatibility.
	compactRequest := isOpenAICompactRequest(c, body)
	h := &OpenAIGatewayHandler{}
	_, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)
	require.True(t, ok)
	require.True(t, isOpenAIRemoteCompactPath(c))
	require.True(t, compactRequest)
	require.Equal(t, service.RequestBodyLaneRecovery, account.GetRequestBodyAdmissionPolicy().Classify(11, compactRequest))
}

func TestNormalizeOpenAIResponsesCompactRequest_BodySignalTrailingSlashPromoted(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	body := []byte(`{"model":"gpt-5.5","input":[{"type":"compaction_trigger"}]}`)
	c := newCompactBodySignalTestContext(t, "/v1/responses/", body)

	_, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)
	require.True(t, ok)
	require.Equal(t, "/v1/responses/compact", c.Request.URL.Path)
}

func TestNormalizeOpenAIResponsesCompactRequest_CodexDirectAliasPromoted(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	body := []byte(`{"model":"gpt-5.5","input":[{"type":"compaction_trigger"}]}`)
	c := newCompactBodySignalTestContext(t, "/backend-api/codex/responses", body)

	_, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)
	require.True(t, ok)
	require.Equal(t, "/backend-api/codex/responses/compact", c.Request.URL.Path)
}

func TestNormalizeOpenAIResponsesCompactRequest_NonRemoteV2BodySignalPromoted(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	tests := []struct {
		name       string
		body       []byte
		betaHeader string
		wantMarked bool
	}{
		{
			name:       "no_header",
			body:       []byte(`{"model":"gpt-5.5","stream":true,"input":[{"type":"compaction_trigger"}]}`),
			wantMarked: true,
		},
		{
			name:       "unrelated_header",
			body:       []byte(`{"model":"gpt-5.5","stream":true,"input":[{"type":"compaction_trigger"}]}`),
			betaHeader: "responses_websockets_v2",
			wantMarked: true,
		},
		{
			name:       "wrong_case_header",
			body:       []byte(`{"model":"gpt-5.5","stream":true,"input":[{"type":"compaction_trigger"}]}`),
			betaHeader: "REMOTE_COMPACTION_V2",
			wantMarked: true,
		},
		{
			name:       "stream_false",
			body:       []byte(`{"model":"gpt-5.5","stream":false,"input":[{"type":"compaction_trigger"}]}`),
			betaHeader: "remote_compaction_v2",
		},
		{
			name:       "stream_absent",
			body:       []byte(`{"model":"gpt-5.5","input":[{"type":"compaction_trigger"}]}`),
			betaHeader: "remote_compaction_v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCompactBodySignalTestContext(t, "/v1/responses", tt.body)
			if tt.betaHeader != "" {
				c.Request.Header.Set("x-codex-beta-features", tt.betaHeader)
			}

			normalized, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), tt.body)
			require.True(t, ok)
			require.Equal(t, "/v1/responses/compact", c.Request.URL.Path)
			require.False(t, gjson.GetBytes(normalized, "stream").Exists())

			marked, exists := c.Get(service.OpenAICompactClientStreamKeyForTest())
			require.Equal(t, tt.wantMarked, exists)
			if tt.wantMarked {
				require.Equal(t, true, marked)
			}
		})
	}
}

func TestNormalizeOpenAIResponsesCompactRequest_NoTriggerUntouched(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	body := []byte(`{"model":"gpt-5.5","stream":true,"input":[{"type":"message","role":"user","content":"hello"}]}`)
	c := newCompactBodySignalTestContext(t, "/v1/responses", body)

	normalized, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)
	require.True(t, ok)
	require.Equal(t, "/v1/responses", c.Request.URL.Path)
	require.False(t, isOpenAIRemoteCompactPath(c))
	require.Equal(t, body, normalized)
	require.True(t, gjson.GetBytes(normalized, "stream").Bool())
}

func TestNormalizeOpenAIResponsesCompactRequest_PathBasedNoDoubleSuffix(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	body := []byte(`{"model":"gpt-5.5","stream":true,"store":true,"input":[{"type":"message","role":"user","content":"hello"}]}`)
	c := newCompactBodySignalTestContext(t, "/v1/responses/compact", body)
	c.Request.Header.Set("x-codex-beta-features", "remote_compaction_v2")

	normalized, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)
	require.True(t, ok)
	require.Equal(t, "/v1/responses/compact", c.Request.URL.Path)
	require.False(t, gjson.GetBytes(normalized, "stream").Exists())
	require.False(t, gjson.GetBytes(normalized, "store").Exists())
}

func TestNormalizeOpenAIResponsesCompactRequest_SubpathNotPromoted(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	body := []byte(`{"model":"gpt-5.5","input":[{"type":"compaction_trigger"}]}`)
	c := newCompactBodySignalTestContext(t, "/v1/responses/resp_123/cancel", body)

	normalized, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)
	require.True(t, ok)
	require.Equal(t, "/v1/responses/resp_123/cancel", c.Request.URL.Path)
	require.Equal(t, body, normalized)
}

// path-based compact（Codex v1 unary 协议）即使 body 带 stream:true 也不标记，
// 保持 JSON 写回行为不变。
func TestNormalizeOpenAIResponsesCompactRequest_PathBasedStreamTrueNotMarked(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	body := []byte(`{"model":"gpt-5.5","stream":true,"input":[{"type":"message","role":"user","content":"hello"}]}`)
	c := newCompactBodySignalTestContext(t, "/v1/responses/compact", body)

	_, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)
	require.True(t, ok)
	_, exists := c.Get(service.OpenAICompactClientStreamKeyForTest())
	require.False(t, exists)
}
