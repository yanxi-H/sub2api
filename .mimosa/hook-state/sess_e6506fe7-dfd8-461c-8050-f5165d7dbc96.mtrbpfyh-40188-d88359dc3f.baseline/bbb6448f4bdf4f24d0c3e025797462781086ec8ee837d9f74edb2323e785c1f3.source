//go:build unit

package handler

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

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

func TestGrokRealtimeBillingResultRequiresObservedAudio(t *testing.T) {
	if grokRealtimeBillingResult("grok-voice-latest", time.Second, false) != nil {
		t.Fatal("a session without observed audio must not be billed")
	}
	if grokRealtimeBillingResult("grok-voice-latest", 0, true) != nil {
		t.Fatal("zero-duration sessions must not be billed")
	}
}

func TestGrokRealtimeBillingResultUsesForcedUniqueID(t *testing.T) {
	first := grokRealtimeBillingResult("grok-voice-latest", 90*time.Second, true)
	second := grokRealtimeBillingResult("grok-voice-latest", 90*time.Second, true)
	if first == nil || second == nil {
		t.Fatal("observed audio sessions should be billable")
	}
	if first.RequestID == "" {
		t.Fatalf("unexpected billing request ID %q", first.RequestID)
	}
	if first.RequestID == second.RequestID {
		t.Fatal("independent realtime connections must not share a billing request ID")
	}
	if first.AudioUsage == nil || first.AudioUsage.Mode != "realtime" || first.AudioUsage.DurationOrUnits != 1.5 {
		t.Fatalf("unexpected audio usage: %#v", first.AudioUsage)
	}
}
