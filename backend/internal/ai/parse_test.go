package ai

import (
	"encoding/json"
	"testing"
)

func TestParseDecisionResponseFromToolCalls(t *testing.T) {
	decision, err := parseDecisionResponse("", []ToolCall{{Function: ToolFunction{Name: "submit_action", Arguments: `{"action":"call","amount":30,"public_reason":"跟注。","private_reason":"牌力够用。"}`}}})
	if err != nil {
		t.Fatalf("parseDecisionResponse returned error: %v", err)
	}
	if decision.Action != "call" || decision.Amount != 30 {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestParseDecisionResponseCanSalvagePartialToolArguments(t *testing.T) {
	decision, err := parseDecisionResponse("", []ToolCall{{Function: ToolFunction{Name: "submit_action", Arguments: `{"action":"raise"`}}})
	if err != nil {
		t.Fatalf("parseDecisionResponse returned error: %v", err)
	}
	if decision.Action != "raise" {
		t.Fatalf("expected raise, got %+v", decision)
	}
	if decision.PublicReason == "" {
		t.Fatalf("expected default public reason to be filled")
	}
}

func TestCompletionContentUnmarshalIgnoresThinkingBlocks(t *testing.T) {
	var content completionContent
	if err := json.Unmarshal([]byte(`[{"type":"thinking","text":"internal"},{"type":"text","text":"final json"}]`), &content); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}
	if content.String() != "final json" {
		t.Fatalf("expected only text blocks to survive, got %q", content.String())
	}
}
