package config

import "testing"

func TestNormalizeStructuredOutput(t *testing.T) {
	cases := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"", StructuredOutputToolCall, false},
		{"tool_call", StructuredOutputToolCall, false},
		{"Tool_Call", StructuredOutputToolCall, false},
		{"tools", StructuredOutputToolCall, false},
		{"tool_calls", StructuredOutputToolCall, false},
		{"json_object", StructuredOutputJSONObject, false},
		{"JSON", StructuredOutputJSONObject, false},
		{"none", StructuredOutputNone, false},
		{"off", StructuredOutputNone, false},
		{"disabled", StructuredOutputNone, false},
		{"weird-mode", "", true},
	}

	for _, tc := range cases {
		got, err := normalizeStructuredOutput(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("normalizeStructuredOutput(%q) expected error, got nil", tc.input)
			}
			continue
		}
		if err != nil {
			t.Fatalf("normalizeStructuredOutput(%q) unexpected error: %v", tc.input, err)
		}
		if got != tc.expected {
			t.Fatalf("normalizeStructuredOutput(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestPresetStructuredOutputModeFallsBackToToolCall(t *testing.T) {
	empty := Preset{}
	if got := empty.StructuredOutputMode(); got != StructuredOutputToolCall {
		t.Fatalf("empty preset expected default %q, got %q", StructuredOutputToolCall, got)
	}

	explicit := Preset{StructuredOutput: StructuredOutputJSONObject}
	if got := explicit.StructuredOutputMode(); got != StructuredOutputJSONObject {
		t.Fatalf("explicit json_object preset expected %q, got %q", StructuredOutputJSONObject, got)
	}

	disabled := Preset{StructuredOutput: StructuredOutputNone}
	if got := disabled.StructuredOutputMode(); got != StructuredOutputNone {
		t.Fatalf("explicit none preset expected %q, got %q", StructuredOutputNone, got)
	}
}

func TestPublicPresetExposesStructuredOutput(t *testing.T) {
	p := Preset{
		ID:           "abc",
		Name:         "Tester",
		Endpoint:     "https://example.com/v1",
		Token:        "secret-token",
		Model:        "gpt-test",
		SystemPrompt: "你好",
	}
	pub := p.Public()
	if pub.StructuredOutput != StructuredOutputToolCall {
		t.Fatalf("expected default StructuredOutput in public view, got %q", pub.StructuredOutput)
	}
	if pub.SystemPrompt != "你好" {
		t.Fatalf("expected system prompt to be preserved, got %q", pub.SystemPrompt)
	}
}
