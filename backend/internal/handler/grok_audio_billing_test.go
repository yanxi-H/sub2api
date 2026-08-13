//go:build unit

package handler

import (
	"encoding/json"
	"strings"
	"testing"

	coderws "github.com/coder/websocket"
)

func TestBuildGrokRealtimeAuditBodyExtractsTextAndDropsAudio(t *testing.T) {
	body, hasText, err := buildGrokRealtimeAuditBody([]byte(`{
		"type":"session.update",
		"session":{"instructions":"speak clearly","input_audio_format":"pcm16"},
		"audio":"AUDIO_BASE64_CANARY",
		"item":{"content":[{"type":"input_text","text":"hello realtime"},{"type":"input_audio","audio":"AUDIO_BASE64_CANARY"}]}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if !hasText {
		t.Fatal("expected text-bearing realtime event")
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	encoded := string(body)
	if !strings.Contains(encoded, "speak clearly") || !strings.Contains(encoded, "hello realtime") {
		t.Fatalf("audit body missed text fields: %s", encoded)
	}
	if strings.Contains(encoded, "AUDIO_BASE64_CANARY") {
		t.Fatalf("audit body leaked audio payload: %s", encoded)
	}
}

func TestBuildGrokRealtimeAuditBodySkipsAudioOnlyEvent(t *testing.T) {
	body, hasText, err := buildGrokRealtimeAuditBody([]byte(`{"type":"input_audio_buffer.append","audio":"AUDIO_BASE64_CANARY"}`))
	if err != nil {
		t.Fatal(err)
	}
	if hasText || body != nil {
		t.Fatalf("audio-only frame must not produce prompt audit body: %s", string(body))
	}
}

func TestBuildGrokRealtimeAuditBodyRejectsInvalidJSON(t *testing.T) {
	if _, _, err := buildGrokRealtimeAuditBody([]byte(`{"type":`)); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestIsExpectedGrokRealtimeClose(t *testing.T) {
	for _, status := range []coderws.StatusCode{
		coderws.StatusNormalClosure,
		coderws.StatusGoingAway,
		coderws.StatusNoStatusRcvd,
		coderws.StatusAbnormalClosure,
	} {
		if !isExpectedGrokRealtimeClose(coderws.CloseError{Code: status}) {
			t.Fatalf("status %v should be treated as an expected session close", status)
		}
	}
	if isExpectedGrokRealtimeClose(coderws.CloseError{Code: coderws.StatusPolicyViolation}) {
		t.Fatal("policy violations must not be treated as billable normal closes")
	}
}
