package ai

import (
	"strings"
	"testing"

	"holdem/backend/internal/config"
)

func TestBuildRequestPayloadDefaultsToToolCalling(t *testing.T) {
	payload := buildRequestPayload(config.Preset{Model: "gpt-5.4"}, PromptInput{}, 1, "")
	if _, ok := payload["tools"]; !ok {
		t.Fatalf("expected default mode to attach tools, got %+v", payload)
	}
	if _, ok := payload["response_format"]; ok {
		t.Fatalf("did not expect response_format with default mode, got %+v", payload)
	}
}

func TestBuildRequestPayloadHonorsJSONObjectMode(t *testing.T) {
	preset := config.Preset{Model: "gpt-5.4", StructuredOutput: config.StructuredOutputJSONObject}
	payload := buildRequestPayload(preset, PromptInput{}, 1, "")
	format, ok := payload["response_format"].(map[string]string)
	if !ok || format["type"] != "json_object" {
		t.Fatalf("expected json_object response_format, got %+v", payload["response_format"])
	}
	if _, ok := payload["tools"]; ok {
		t.Fatalf("did not expect tools when explicitly using json_object, got %+v", payload)
	}
}

func TestBuildRequestPayloadHonorsNoneMode(t *testing.T) {
	preset := config.Preset{Model: "gpt-5.4", StructuredOutput: config.StructuredOutputNone}
	payload := buildRequestPayload(preset, PromptInput{}, 1, "")
	if _, ok := payload["tools"]; ok {
		t.Fatalf("did not expect tools when mode is none, got %+v", payload)
	}
	if _, ok := payload["response_format"]; ok {
		t.Fatalf("did not expect response_format when mode is none, got %+v", payload)
	}
}

func TestBuildRequestPayloadStillIncludesRetryHint(t *testing.T) {
	payload := buildRequestPayload(config.Preset{Model: "gpt-5.4", StructuredOutput: config.StructuredOutputToolCall}, PromptInput{}, 2, "")
	messages, ok := payload["messages"].([]map[string]string)
	if !ok {
		t.Fatalf("expected messages slice, got %T", payload["messages"])
	}
	if len(messages) < 3 {
		t.Fatalf("expected retry hint message to be appended on attempt > 1, got %d", len(messages))
	}
	if messages[len(messages)-1]["role"] != "user" {
		t.Fatalf("expected retry hint to be a user message, got %+v", messages[len(messages)-1])
	}
}

func TestBuildRequestPayloadIncludesLastErrorHintOnRetry(t *testing.T) {
	preset := config.Preset{Model: "gpt-5.4", StructuredOutput: config.StructuredOutputToolCall}
	payload := buildRequestPayload(preset, PromptInput{}, 2, "tool arguments missing action")
	messages, ok := payload["messages"].([]map[string]string)
	if !ok {
		t.Fatalf("expected messages slice, got %T", payload["messages"])
	}
	last := messages[len(messages)-1]
	if last["role"] != "user" {
		t.Fatalf("expected retry hint to be user message, got %+v", last)
	}
	if !strings.Contains(last["content"], "tool arguments missing action") {
		t.Fatalf("expected retry hint to mention last error, got %q", last["content"])
	}
}

func TestSystemInstructionEnumeratesPokerThinkingFramework(t *testing.T) {
	out := systemInstruction("", config.StructuredOutputToolCall)
	for _, marker := range []string{"effBB", "potOdds", "minRaiseTo", "submit_action", "f=fold"} {
		if !strings.Contains(out, marker) {
			t.Fatalf("expected systemInstruction to mention %q, got: %s", marker, out)
		}
	}

	jsonOut := systemInstruction("", config.StructuredOutputJSONObject)
	if !strings.Contains(jsonOut, "JSON 对象") {
		t.Fatalf("expected json_object branch to mention JSON 对象, got: %s", jsonOut)
	}
	if strings.Contains(jsonOut, "submit_action") {
		t.Fatalf("did not expect submit_action mention in json_object branch, got: %s", jsonOut)
	}
}

func TestSystemInstructionPrependsPresetPromptWhenSet(t *testing.T) {
	out := systemInstruction("你是一个保守的玩家", config.StructuredOutputToolCall)
	if !strings.Contains(out, "你是一个保守的玩家") {
		t.Fatalf("expected preset system_prompt to be prepended, got: %s", out)
	}
}

func TestBuildRequestPayloadHonorsPresetMaxTokens(t *testing.T) {
	// A reasoning preset with an explicit ceiling overrides the small default
	// on every attempt so its chain-of-thought has room before the answer.
	preset := config.Preset{Model: "deepseek-v4-pro", StructuredOutput: config.StructuredOutputJSONObject, MaxTokens: 4096}
	for _, attempt := range []int{1, 2, 3} {
		payload := buildRequestPayload(preset, PromptInput{}, attempt, "")
		if payload["max_tokens"] != 4096 {
			t.Fatalf("attempt %d: expected preset max_tokens 4096, got %v", attempt, payload["max_tokens"])
		}
	}

	// Presets without an override keep the cheap per-attempt defaults.
	def := buildRequestPayload(config.Preset{Model: "gpt-5.4"}, PromptInput{}, 1, "")
	if def["max_tokens"] != 192 {
		t.Fatalf("expected default attempt-1 max_tokens 192, got %v", def["max_tokens"])
	}
}

func TestBuildRequestPayloadMergesExtraBody(t *testing.T) {
	preset := config.Preset{
		Model:            "deepseek-v4-pro",
		StructuredOutput: config.StructuredOutputToolCall,
		ExtraBody:        map[string]any{"thinking": map[string]any{"type": "disabled"}},
	}
	payload := buildRequestPayload(preset, PromptInput{}, 1, "")
	thinking, ok := payload["thinking"].(map[string]any)
	if !ok || thinking["type"] != "disabled" {
		t.Fatalf("expected extra_body thinking passthrough, got %v", payload["thinking"])
	}
	// The merge must not clobber the core request fields.
	if payload["model"] != "deepseek-v4-pro" {
		t.Fatalf("extra_body merge clobbered model: %v", payload["model"])
	}
	if _, ok := payload["messages"]; !ok {
		t.Fatalf("extra_body merge dropped messages")
	}
}

func TestBuildRequestPayloadAllowsExtraBodyToDeleteDefaults(t *testing.T) {
	preset := config.Preset{
		Model:            "gpt-5.2-codex",
		StructuredOutput: config.StructuredOutputToolCall,
		ExtraBody:        map[string]any{"temperature": nil, "top_p": 0.2},
	}
	payload := buildRequestPayload(preset, PromptInput{}, 1, "")
	if _, ok := payload["temperature"]; ok {
		t.Fatalf("expected extra_body nil to delete temperature, got %+v", payload)
	}
	if payload["top_p"] != 0.2 {
		t.Fatalf("expected extra_body to add top_p, got %+v", payload)
	}
}

func TestBuildProbePayloadUsesCompactPromptAndMergesExtraBody(t *testing.T) {
	preset := config.Preset{
		Model:            "kimi-k2.5",
		StructuredOutput: config.StructuredOutputJSONObject,
		ExtraBody:        map[string]any{"temperature": nil, "thinking": map[string]any{"type": "disabled"}},
	}
	payload := buildProbePayload(preset)
	if _, ok := payload["temperature"]; ok {
		t.Fatalf("expected probe payload to omit deleted temperature, got %+v", payload)
	}
	if payload["max_tokens"] != 16 {
		t.Fatalf("expected default probe max_tokens 16, got %+v", payload["max_tokens"])
	}
	messages, ok := payload["messages"].([]map[string]string)
	if !ok || len(messages) != 1 || messages[0]["content"] != "Reply with exactly pong." {
		t.Fatalf("expected compact deterministic probe prompt, got %+v", payload["messages"])
	}
	if _, ok := payload["response_format"]; ok {
		t.Fatalf("expected probe payload to skip structured-output forcing, got %+v", payload["response_format"])
	}
	thinking, ok := payload["thinking"].(map[string]any)
	if !ok || thinking["type"] != "disabled" {
		t.Fatalf("expected probe payload to merge extra_body, got %+v", payload["thinking"])
	}
}

func TestBuildProbePayloadCapsHighPresetMaxTokens(t *testing.T) {
	payload := buildProbePayload(config.Preset{Model: "glm-5.1", MaxTokens: 512})
	if payload["max_tokens"] != 128 {
		t.Fatalf("expected high preset max_tokens to be capped at 128 for probe, got %+v", payload["max_tokens"])
	}
}
