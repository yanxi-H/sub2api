package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type passthroughFlushTestWriter struct {
	gin.ResponseWriter
	recorder         *httptest.ResponseRecorder
	failAfterWrites  int
	successfulWrites int
	failedWrites     int
	flushBodyLengths []int
}

func (w *passthroughFlushTestWriter) Write(data []byte) (int, error) {
	if w.failAfterWrites >= 0 && w.successfulWrites >= w.failAfterWrites {
		w.failedWrites++
		return 0, errors.New("client disconnected")
	}
	n, err := w.ResponseWriter.Write(data)
	if err == nil {
		w.successfulWrites++
	}
	return n, err
}

func (w *passthroughFlushTestWriter) WriteString(data string) (int, error) {
	return w.Write([]byte(data))
}

func (w *passthroughFlushTestWriter) Flush() {
	w.ResponseWriter.Flush()
	w.flushBodyLengths = append(w.flushBodyLengths, w.recorder.Body.Len())
}

type passthroughFlushTestErrorBody struct {
	payload []byte
	err     error
	sent    bool
}

func (r *passthroughFlushTestErrorBody) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		return copy(p, r.payload), nil
	}
	return 0, r.err
}

func (r *passthroughFlushTestErrorBody) Close() error { return nil }

type passthroughFlushStagedBody struct {
	chunks []string
	delays []time.Duration
	index  int
	offset int
}

func (r *passthroughFlushStagedBody) Read(p []byte) (int, error) {
	if r.index >= len(r.chunks) {
		return 0, io.EOF
	}
	if r.offset == 0 && r.index < len(r.delays) && r.delays[r.index] > 0 {
		time.Sleep(r.delays[r.index])
	}
	n := copy(p, r.chunks[r.index][r.offset:])
	r.offset += n
	if r.offset >= len(r.chunks[r.index]) {
		r.index++
		r.offset = 0
	}
	return n, nil
}

func (r *passthroughFlushStagedBody) Close() error { return nil }

func runPassthroughFlushTest(
	t *testing.T,
	body io.ReadCloser,
	failAfterWrites int,
	setups ...func(*gin.Context),
) (*openaiStreamingResultPassthrough, *httptest.ResponseRecorder, *passthroughFlushTestWriter, error) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	writer := &passthroughFlushTestWriter{
		ResponseWriter:  c.Writer,
		recorder:        recorder,
		failAfterWrites: failAfterWrites,
	}
	c.Writer = writer
	for _, setup := range setups {
		setup(c)
	}

	svc := &OpenAIGatewayService{cfg: &config.Config{
		Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize},
	}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       body,
	}
	result, err := svc.handleStreamingResponsePassthrough(
		context.Background(),
		resp,
		c,
		&Account{ID: 1, Platform: PlatformOpenAI, Name: "flush-test"},
		time.Now(),
		"",
		"",
	)
	return result, recorder, writer, err
}

func TestOpenAIStreamingPassthroughFlushesAtCompleteEventBoundaries(t *testing.T) {
	firstEvent := "event: response.output_text.delta\n" +
		"id: event-1\n" +
		`data: {"type":"response.output_text.delta","delta":"hello"}` + "\n\n"
	heartbeat := ": keepalive\n\n"
	terminalEvent := "event: response.completed\n" +
		`data: {"type":"response.completed","response":{"id":"resp_flush","usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}` + "\n\n"
	upstream := firstEvent + heartbeat + terminalEvent

	result, recorder, writer, err := runPassthroughFlushTest(t, io.NopCloser(strings.NewReader(upstream)), -1)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, upstream, recorder.Body.String())
	require.Equal(t, []int{
		len(firstEvent),
		len(firstEvent) + len(heartbeat),
		len(upstream),
	}, writer.flushBodyLengths)
	require.Equal(t, 3, result.usage.InputTokens)
	require.Equal(t, 2, result.usage.OutputTokens)
	require.NotNil(t, result.firstTokenMs)
}

func TestOpenAIStreamingPassthroughKeepsPreamblePendingUntilFirstOutputBoundary(t *testing.T) {
	preamble := "event: response.created\n" +
		`data: {"type":"response.created","response":{"id":"resp_pending"}}` + "\n\n" +
		": waiting\n\n"
	firstOutput := `data: {"type":"response.output_text.delta","delta":"ready"}` + "\n\n"
	terminalEvent := `data: {"type":"response.completed","response":{"id":"resp_pending","usage":{"input_tokens":4,"output_tokens":1,"total_tokens":5}}}` + "\n\n"
	upstream := preamble + firstOutput + terminalEvent

	_, recorder, writer, err := runPassthroughFlushTest(t, io.NopCloser(strings.NewReader(upstream)), -1)

	require.NoError(t, err)
	require.Equal(t, upstream, recorder.Body.String())
	require.Equal(t, []int{
		len(preamble) + len(firstOutput),
		len(upstream),
	}, writer.flushBodyLengths)
}

func TestOpenAIStreamingPassthroughFlushesTerminalEventAtEOFWithoutBlankLine(t *testing.T) {
	upstream := "event: response.completed\n" +
		`data: {"type":"response.completed","response":{"id":"resp_eof","usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}`
	wantBody := upstream + "\n"

	result, recorder, writer, err := runPassthroughFlushTest(t, io.NopCloser(strings.NewReader(upstream)), -1)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, wantBody, recorder.Body.String())
	require.Equal(t, []int{len(wantBody)}, writer.flushBodyLengths)
	require.Equal(t, 5, result.usage.InputTokens)
	require.Equal(t, 2, result.usage.OutputTokens)
	require.Nil(t, result.firstTokenMs, "a terminal lifecycle event is not a token")
}

func TestOpenAIStreamingPassthroughFailedBeforeOutputCanStillFailOverWithoutFlush(t *testing.T) {
	upstream := "event: response.created\n" +
		`data: {"type":"response.created","response":{"id":"resp_failover"}}` + "\n\n" +
		"event: response.failed\n" +
		`data: {"type":"response.failed","error":{"code":"server_error","message":"upstream processing failed"}}` + "\n\n"

	var requestContext *gin.Context
	_, recorder, writer, err := runPassthroughFlushTest(t, io.NopCloser(strings.NewReader(upstream)), -1, func(c *gin.Context) {
		requestContext = c
	})

	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Empty(t, recorder.Body.String())
	require.Empty(t, writer.flushBodyLengths)
	_, marked := GetOpsStreamError(requestContext)
	require.False(t, marked, "a retryable failed attempt must not poison a later successful failover")
}

func TestOpenAIStreamingPassthroughNonRetryableFailedBeforeOutputFlushesAtBoundary(t *testing.T) {
	upstream := "event: response.failed\n" +
		`data: {"type":"response.failed","error":{"code":"content_policy","message":"request blocked by policy"},"usage":{"input_tokens":6,"output_tokens":0,"total_tokens":6}}` + "\n\n"

	var requestContext *gin.Context
	result, recorder, writer, err := runPassthroughFlushTest(t, io.NopCloser(strings.NewReader(upstream)), -1, func(c *gin.Context) {
		requestContext = c
	})

	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))
	require.NotNil(t, result)
	require.Equal(t, upstream, recorder.Body.String())
	require.Equal(t, []int{len(upstream)}, writer.flushBodyLengths)
	require.Equal(t, 6, result.usage.InputTokens)
	require.Zero(t, result.usage.OutputTokens)
	streamErr, marked := GetOpsStreamError(requestContext)
	require.True(t, marked)
	require.True(t, streamErr.CountTowardsSLA)
	require.Equal(t, http.StatusForbidden, streamErr.IntendedStatus)
	require.Equal(t, "content_policy", streamErr.Code)
}

func TestOpenAIStreamingPassthroughFailedAfterOutputFlushesAtBoundaryAndKeepsUsage(t *testing.T) {
	firstOutput := `data: {"type":"response.output_text.delta","delta":"partial"}` + "\n\n"
	failedEvent := "event: response.failed\n" +
		`data: {"type":"response.failed","error":{"code":"server_error","message":"upstream processing failed"},"usage":{"input_tokens":7,"output_tokens":2,"total_tokens":9}}` + "\n\n"
	upstream := firstOutput + failedEvent

	var requestContext *gin.Context
	result, recorder, writer, err := runPassthroughFlushTest(t, io.NopCloser(strings.NewReader(upstream)), -1, func(c *gin.Context) {
		requestContext = c
	})

	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))
	require.NotNil(t, result)
	require.Equal(t, upstream, recorder.Body.String())
	require.Equal(t, []int{len(firstOutput), len(upstream)}, writer.flushBodyLengths)
	require.Equal(t, 7, result.usage.InputTokens)
	require.Equal(t, 2, result.usage.OutputTokens)
	streamErr, marked := GetOpsStreamError(requestContext)
	require.True(t, marked)
	require.Equal(t, http.StatusBadGateway, streamErr.IntendedStatus)
}

func TestOpenAIStreamingPassthroughGapIgnoresHeartbeatAndLifecycleEvents(t *testing.T) {
	firstOutput := `data: {"type":"response.output_text.delta","delta":"one"}` + "\n\n"
	heartbeat := ": keepalive\n\n"
	emptyFrame := "\n"
	lifecycle := `data: {"type":"response.in_progress","response":{"id":"resp_gap"}}` + "\n\n"
	secondOutput := `data: {"type":"response.output_text.delta","delta":"two"}` + "\n\n"
	completed := `data: {"type":"response.completed","response":{"id":"resp_gap","usage":{"input_tokens":2,"output_tokens":2}}}` + "\n\n"
	body := &passthroughFlushStagedBody{
		chunks: []string{firstOutput, heartbeat, emptyFrame, lifecycle, secondOutput, completed},
		delays: []time.Duration{0, 25 * time.Millisecond, 25 * time.Millisecond, 25 * time.Millisecond, 25 * time.Millisecond, 0},
	}

	result, _, _, err := runPassthroughFlushTest(t, body, -1)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.maxStreamGapMs)
	require.GreaterOrEqual(t, *result.maxStreamGapMs, 75, "non-semantic frames must not reset the token-to-token gap")
}

func TestOpenAIStreamingPassthroughTTFTIgnoresLifecycleEvents(t *testing.T) {
	lifecycle := `data: {"type":"response.output_item.added","item":{"type":"function_call"}}` + "\n\n"
	firstToken := `data: {"type":"response.function_call_arguments.delta","delta":"{}"}` + "\n\n"
	completed := `data: {"type":"response.completed","response":{"id":"resp_ttft","usage":{"input_tokens":2,"output_tokens":1}}}` + "\n\n"
	body := &passthroughFlushStagedBody{
		chunks: []string{lifecycle, firstToken, completed},
		delays: []time.Duration{0, 30 * time.Millisecond, 0},
	}

	result, _, _, err := runPassthroughFlushTest(t, body, -1)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.firstTokenMs)
	require.GreaterOrEqual(t, *result.firstTokenMs, 20, "TTFT must wait for a real token event")
}

func TestOpenAIStreamFailedEventSemanticStatus(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		message string
		want    int
	}{
		{name: "explicit status", payload: `{"type":"response.failed","response":{"error":{"status_code":529,"code":"server_error"}}}`, want: 529},
		{name: "context window", payload: `{"type":"response.failed","error":{"code":"context_length_exceeded"}}`, message: "input exceeds the context window", want: http.StatusBadRequest},
		{name: "rate limit", payload: `{"type":"response.failed","error":{"code":"rate_limit_exceeded"}}`, want: http.StatusTooManyRequests},
		{name: "content policy", payload: `{"type":"response.failed","error":{"code":"content_policy"}}`, want: http.StatusForbidden},
		{name: "unknown", payload: `{"type":"response.failed","error":{"code":"server_error"}}`, want: http.StatusBadGateway},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, openAIStreamFailedEventSemanticStatus([]byte(test.payload), test.message))
		})
	}
}

func TestObserveOpenAIStreamSemanticOutputKeepsMaximumGap(t *testing.T) {
	start := time.Date(2026, 7, 26, 8, 0, 0, 0, time.UTC)
	var last time.Time
	var maximum *int

	observeOpenAIStreamSemanticOutput(start, &last, &maximum)
	require.Nil(t, maximum)
	observeOpenAIStreamSemanticOutput(start.Add(20*time.Millisecond), &last, &maximum)
	require.NotNil(t, maximum)
	require.Equal(t, 20, *maximum)
	observeOpenAIStreamSemanticOutput(start.Add(25*time.Millisecond), &last, &maximum)
	require.Equal(t, 20, *maximum)
	observeOpenAIStreamSemanticOutput(start.Add(80*time.Millisecond), &last, &maximum)
	require.Equal(t, 55, *maximum)
}

func TestObserveOpenAIStreamTerminalGapIncludesTailStallWithoutCreatingTTFT(t *testing.T) {
	start := time.Date(2026, 7, 26, 8, 0, 0, 0, time.UTC)
	var last time.Time
	var maximum *int

	observeOpenAIStreamTerminalGap(start, &last, &maximum)
	require.True(t, last.IsZero())
	require.Nil(t, maximum)

	observeOpenAIStreamSemanticOutput(start, &last, &maximum)
	observeOpenAIStreamTerminalGap(start.Add(18*time.Second), &last, &maximum)
	require.NotNil(t, maximum)
	require.Equal(t, 18_000, *maximum)
}

func TestOpenAIStreamingPassthroughClientDisconnectStillDrainsTerminalUsage(t *testing.T) {
	firstOutput := `data: {"type":"response.output_text.delta","delta":"partial"}` + "\n\n"
	terminalEvent := `data: {"type":"response.completed","response":{"id":"resp_drain","usage":{"input_tokens":11,"output_tokens":4,"total_tokens":15}}}` + "\n\n"

	result, recorder, writer, err := runPassthroughFlushTest(
		t,
		io.NopCloser(strings.NewReader(firstOutput+terminalEvent)),
		2,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, firstOutput, recorder.Body.String())
	require.Equal(t, []int{len(firstOutput)}, writer.flushBodyLengths)
	require.Equal(t, 1, writer.failedWrites)
	require.Equal(t, 11, result.usage.InputTokens)
	require.Equal(t, 4, result.usage.OutputTokens)
}

func TestOpenAIStreamingPassthroughScannerErrorFlushesWrittenResidual(t *testing.T) {
	upstream := []byte(`data: {"type":"response.output_text.delta","delta":"partial"}`)
	readErr := errors.New("upstream read failed")

	_, recorder, writer, err := runPassthroughFlushTest(t, &passthroughFlushTestErrorBody{
		payload: upstream,
		err:     readErr,
	}, -1)

	require.ErrorIs(t, err, readErr)
	wantBody := string(upstream) + "\n"
	require.Equal(t, wantBody, recorder.Body.String())
	require.Equal(t, []int{len(wantBody)}, writer.flushBodyLengths)
}

func TestOpenAIStreamingPassthroughNamespaceRestoreErrorFlushesWrittenResidualOnce(t *testing.T) {
	writtenPrefix := `data: {"type":"response.output_text.delta","delta":"prefix"}` + "\n"
	overflowData := `data: {"type":"response.output_text.delta","delta":"not-written","overflow":1e1000}`

	_, recorder, writer, err := runPassthroughFlushTest(
		t,
		io.NopCloser(strings.NewReader(writtenPrefix+overflowData)),
		-1,
		func(c *gin.Context) {
			setOpenAIResponsesNamespaceNames(c, map[string]apicompat.ResponsesNamespaceName{
				"collaboration__spawn_agent": {Namespace: "collaboration", Name: "spawn_agent"},
			})
		},
	)

	require.ErrorContains(t, err, "restore OpenAI passthrough namespace response")
	require.Equal(t, writtenPrefix, recorder.Body.String())
	require.Equal(t, []int{len(writtenPrefix)}, writer.flushBodyLengths)
}

func TestOpenAIStreamingPassthroughBlankWriteFailureDoesNotFlushAndStillDrainsUsage(t *testing.T) {
	writtenDataLine := `data: {"type":"response.output_text.delta","delta":"partial"}` + "\n"
	terminalEvent := `data: {"type":"response.completed","response":{"id":"resp_blank_failure","usage":{"input_tokens":13,"output_tokens":5,"total_tokens":18}}}` + "\n\n"

	result, recorder, writer, err := runPassthroughFlushTest(
		t,
		io.NopCloser(strings.NewReader(writtenDataLine+"\n"+terminalEvent)),
		1,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, writtenDataLine, recorder.Body.String())
	require.Empty(t, writer.flushBodyLengths)
	require.Equal(t, 1, writer.successfulWrites)
	require.Equal(t, 1, writer.failedWrites)
	require.Equal(t, 13, result.usage.InputTokens)
	require.Equal(t, 5, result.usage.OutputTokens)
}
