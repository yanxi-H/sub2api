package handler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type promptAuditOrderCase struct {
	file       string
	function   string
	auditToken string
}

func TestPromptAuditGatePrecedesAccountBillingAndUpstreamSideEffects(t *testing.T) {
	tests := []promptAuditOrderCase{
		{file: "gateway_handler.go", function: "Messages", auditToken: "checkSecurityAudit"},
		{file: "gateway_handler_chat_completions.go", function: "ChatCompletions", auditToken: "checkSecurityAudit"},
		{file: "gateway_handler_responses.go", function: "Responses", auditToken: "checkSecurityAudit"},
		{file: "gemini_v1beta_handler.go", function: "GeminiV1BetaModels", auditToken: "checkSecurityAudit"},
		{file: "openai_gateway_handler.go", function: "Responses", auditToken: "checkSecurityAudit"},
		{file: "openai_gateway_handler.go", function: "Messages", auditToken: "checkSecurityAudit"},
		{file: "openai_chat_completions.go", function: "ChatCompletions", auditToken: "checkSecurityAudit"},
		{file: "openai_images.go", function: "Images", auditToken: "checkSecurityAudit"},
		{file: "grok_media.go", function: "handleGrokMedia", auditToken: "checkSecurityAudit"},
		{file: "openai_embeddings.go", function: "Embeddings", auditToken: "checkSecurityAudit"},
		{file: "openai_alpha_search.go", function: "AlphaSearch", auditToken: "checkSecurityAudit"},
		{file: "image_task_handler.go", function: "Submit", auditToken: "checkSecurityAuditBeforeSubmit"},
		{file: "batch_image_handler.go", function: "Submit", auditToken: "checkSecurityAuditBeforeSubmit"},
		{file: "grok_audio.go", function: "GrokVoice", auditToken: "checkSecurityAudit"},
		{file: "gateway_web_search.go", function: "WebSearch", auditToken: "checkSecurityAudit"},
	}
	sideEffectTokens := []string{
		"CheckBillingEligibility(", "SelectAccount", ".Forward", "acquireResponsesUserSlot(",
		"AcquireUserSlot", "TryAcquireUserSlot", "acquireImageGenerationSlot(",
		"h.tasks.Create(", "h.service.Submit(",
	}
	for _, tt := range tests {
		t.Run(tt.file+"/"+tt.function, func(t *testing.T) {
			functionSource := stripGoComments(goFunctionSource(t, tt.file, tt.function))
			auditIndex := strings.Index(functionSource, tt.auditToken)
			require.NotEqual(t, -1, auditIndex, "missing Prompt Audit gate")
			foundSideEffect := false
			for _, sideEffect := range sideEffectTokens {
				index := strings.Index(functionSource, sideEffect)
				if index < 0 {
					continue
				}
				foundSideEffect = true
				require.Lessf(t, auditIndex, index, "%s must run before %s", tt.auditToken, sideEffect)
			}
			require.True(t, foundSideEffect, "coverage case must contain a downstream side effect")
		})
	}
}

func TestGrokRealtimeBlockingGuardPrecedesAccountBillingAndUpstreamSideEffects(t *testing.T) {
	functionSource := stripGoComments(goFunctionSource(t, "grok_audio.go", "GrokRealtime"))
	guardIndex := strings.Index(functionSource, "rejectGrokRealtimeWithoutPreRoutingAudit")
	require.NotEqual(t, -1, guardIndex, "missing pre-routing blocking-audit guard")
	for _, sideEffect := range []string{"TryAcquireUserSlotForAPIKey", "CheckBillingEligibility(", "SelectAccount", "GetRequestCredential("} {
		index := strings.Index(functionSource, sideEffect)
		require.NotEqualf(t, -1, index, "missing expected downstream side effect %s", sideEffect)
		require.Lessf(t, guardIndex, index, "blocking-audit guard must run before %s", sideEffect)
	}
}

func TestGrokRealtimeUpgradeFollowsHTTPHandshakeChecks(t *testing.T) {
	functionSource := stripGoComments(goFunctionSource(t, "grok_audio.go", "GrokRealtime"))
	upgradeIndex := strings.Index(functionSource, "coderws.Accept")
	require.NotEqual(t, -1, upgradeIndex, "missing WebSocket upgrade")
	for _, handshakeCheck := range []string{"TryAcquireUserSlotForAPIKey", "CheckBillingEligibility(", "SelectAccount", "acquireResponsesAccountSlot", "GetRequestCredential("} {
		index := strings.Index(functionSource, handshakeCheck)
		require.NotEqualf(t, -1, index, "missing expected handshake check %s", handshakeCheck)
		require.Lessf(t, index, upgradeIndex, "%s must complete before WebSocket upgrade", handshakeCheck)
	}
	require.Contains(t, functionSource, `acquireResponsesAccountSlot(c, apiKey.GroupID, "", selection, false, &streamStarted`, "pre-upgrade account waits must not emit streaming keepalives")
}

func TestGrokEntrypointsAcquireUserAndAPIKeyConcurrency(t *testing.T) {
	tests := []struct {
		file, function, acquireToken string
	}{
		{file: "grok_audio.go", function: "GrokRealtime", acquireToken: "TryAcquireUserSlotForAPIKey"},
		{file: "grok_audio.go", function: "GrokVoice", acquireToken: "acquireResponsesUserSlot"},
		{file: "gateway_web_search.go", function: "WebSearch", acquireToken: "AcquireUserSlotWithWait"},
	}
	for _, tt := range tests {
		t.Run(tt.function, func(t *testing.T) {
			functionSource := stripGoComments(goFunctionSource(t, tt.file, tt.function))
			require.Contains(t, functionSource, tt.acquireToken)
		})
	}
}

func stripGoComments(source string) string {
	source = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(source, "")
	return regexp.MustCompile(`(?m)//.*$`).ReplaceAllString(source, "")
}

func goFunctionSource(t *testing.T, filename, functionName string) string {
	t.Helper()
	raw, err := os.ReadFile(filename)
	require.NoError(t, err)
	files := token.NewFileSet()
	parsed, err := parser.ParseFile(files, filename, raw, 0)
	require.NoError(t, err)
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != functionName || function.Body == nil {
			continue
		}
		start := files.Position(function.Pos()).Offset
		end := files.Position(function.End()).Offset
		require.Greater(t, end, start)
		return string(raw[start:end])
	}
	t.Fatalf("function %s not found in %s", functionName, filename)
	return ""
}
