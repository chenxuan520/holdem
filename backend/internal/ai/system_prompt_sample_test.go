package ai

import (
	"fmt"
	"testing"

	"holdem/backend/internal/config"
)

// TestSystemInstructionSize is a non-failing diagnostic that prints how big
// the assembled system prompt is. Useful when tuning prompt density.
func TestSystemInstructionSize(t *testing.T) {
	if !testing.Verbose() {
		t.Skip("only meaningful with -v")
	}

	out := systemInstruction("", config.StructuredOutputToolCall)
	fmt.Printf("[system-prompt] tool_call bytes=%d\n%s\n---\n", len(out), out)

	out = systemInstruction("", config.StructuredOutputJSONObject)
	fmt.Printf("[system-prompt] json_object bytes=%d\n", len(out))

	out = systemInstruction("", config.StructuredOutputNone)
	fmt.Printf("[system-prompt] none bytes=%d\n", len(out))
}
