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
