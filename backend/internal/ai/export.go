package ai

import (
	"encoding/json"
	"fmt"

	"holdem/backend/internal/config"
)

// This file exposes the byte-exact, pure pieces of the AI client to callers
// outside the package (notably the WebAssembly referee core in
// backend/cmd/wasmcore, used by the Cloudflare Workers backend). The Worker
// performs the LLM fetch itself, but builds the request body and parses the
// response here so the wire bytes — and therefore the prompt-cache prefix —
// stay identical to the native Go backend.

// BuildRequestBody returns the exact JSON request body decideOnce would POST
// to the model for the given attempt (model + structured-output mode +
// system prompt + the compact PromptInput + any retry reminder). The auth
// token is NOT part of the body; the caller adds the Authorization header.
func BuildRequestBody(preset config.Preset, input PromptInput, attempt int, lastHint string) ([]byte, error) {
	return json.Marshal(buildRequestPayload(preset, input, attempt, lastHint))
}

// ParseResponseBody mirrors decideOnce's parsing of a successful (2xx) chat
// completion body into a Decision, including the tool_calls / content
// fallback handled by parseDecisionResponse.
func ParseResponseBody(rawBody string) (Decision, error) {
	var completion struct {
		Choices []struct {
			Message struct {
				Content   string     `json:"content"`
				ToolCalls []ToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(rawBody), &completion); err != nil {
		return Decision{}, err
	}
	if len(completion.Choices) == 0 {
		return Decision{}, fmt.Errorf("empty ai response")
	}
	return parseDecisionResponse(completion.Choices[0].Message.Content, completion.Choices[0].Message.ToolCalls)
}

// RetryHint exposes retryHintFromError so the CF retry loop can compose the
// same follow-up reminder the native Decide loop would on the next attempt.
func RetryHint(rawErr string) string {
	return retryHintFromError(rawErr)
}

// SummarizeAttempts exposes summarizeAttemptErrors so the CF side can build
// the RawLog.Error string it hands back to the reducer after a retry run.
func SummarizeAttempts(attempts []AttemptLog) string {
	return summarizeAttemptErrors(attempts)
}
