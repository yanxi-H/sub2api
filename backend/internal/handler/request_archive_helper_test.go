package handler

import (
	"strings"
	"testing"
)

func TestExtractArchivePrompt_AnthropicMessages(t *testing.T) {
	body := []byte(`{
		"model": "claude-3-5-sonnet",
		"system": "你是一个助手",
		"messages": [
			{"role": "user", "content": "你好"},
			{"role": "assistant", "content": "你好，有什么可以帮你？"},
			{"role": "user", "content": "写一段代码"}
		]
	}`)
	text := extractArchivePrompt("anthropic_messages", body)
	if text == "" {
		t.Fatal("expected non-empty prompt text for anthropic_messages, got empty")
	}
	if !contains(text, "你好") {
		t.Errorf("expected text to contain '你好', got: %s", text)
	}
	if !contains(text, "写一段代码") {
		t.Errorf("expected text to contain '写一段代码', got: %s", text)
	}
	t.Logf("extracted: %s", text)
}

func TestExtractArchivePrompt_OpenAIResponses_Codex(t *testing.T) {
	// 模拟 Codex 请求:input 数组,最后一条是 function_call_output
	body := []byte(`{
		"model": "gpt-5",
		"input": [
			{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "帮我查看文件"}]},
			{"type": "function_call", "name": "read_file"},
			{"type": "function_call_output", "output": "file contents here"}
		]
	}`)
	text := extractArchivePrompt("openai_responses", body)
	if text == "" {
		t.Fatal("expected non-empty prompt text for codex openai_responses, got empty")
	}
	if !contains(text, "帮我查看文件") {
		t.Errorf("expected text to contain user message, got: %s", text)
	}
	t.Logf("extracted: %s", text)
}

func TestExtractArchivePrompt_OpenAIChat(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "hello world"},
			{"role": "assistant", "content": "hi there"},
			{"role": "user", "content": "write code"}
		]
	}`)
	text := extractArchivePrompt("openai_chat_completions", body)
	if text == "" {
		t.Fatal("expected non-empty prompt text for openai_chat, got empty")
	}
	if !contains(text, "hello world") {
		t.Errorf("expected 'hello world', got: %s", text)
	}
	if !contains(text, "write code") {
		t.Errorf("expected 'write code', got: %s", text)
	}
	t.Logf("extracted: %s", text)
}

func TestExtractArchivePrompt_AnthropicContentArray(t *testing.T) {
	// content 是数组形式
	body := []byte(`{
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "图片里的内容是什么"}]}
		]
	}`)
	text := extractArchivePrompt("anthropic_messages", body)
	if text == "" {
		t.Fatal("expected non-empty for content array, got empty")
	}
	if !contains(text, "图片里的内容是什么") {
		t.Errorf("expected content array text, got: %s", text)
	}
	t.Logf("extracted: %s", text)
}

func TestExtractArchivePrompt_EmptyBody(t *testing.T) {
	text := extractArchivePrompt("anthropic_messages", []byte(`{}`))
	if text != "" {
		t.Errorf("expected empty for empty body, got: %s", text)
	}
}

func TestExtractArchivePrompt_InvalidJSON(t *testing.T) {
	text := extractArchivePrompt("anthropic_messages", []byte(`not json`))
	if text != "" {
		t.Errorf("expected empty for invalid json, got: %s", text)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
