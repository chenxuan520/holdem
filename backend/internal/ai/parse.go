package ai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type ToolCall struct {
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func parseDecisionResponse(content string, toolCalls []ToolCall) (Decision, error) {
	for _, toolCall := range toolCalls {
		if strings.TrimSpace(toolCall.Function.Name) != "submit_action" {
			continue
		}
		decision, err := parseDecisionArguments(toolCall.Function.Arguments)
		if err == nil {
			return normalizeDecision(decision), nil
		}
	}

	decision, err := parseDecision(content)
	if err != nil {
		return Decision{}, err
	}
	return normalizeDecision(decision), nil
}

func parseDecision(content string) (Decision, error) {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
	}

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		trimmed = trimmed[start : end+1]
	}

	var decision Decision
	if err := json.Unmarshal([]byte(trimmed), &decision); err != nil {
		return Decision{}, fmt.Errorf("parse decision json: %w", err)
	}
	if decision.Action == "" {
		return Decision{}, fmt.Errorf("decision action is empty")
	}
	return normalizeDecision(decision), nil
}

func parseDecisionArguments(arguments string) (Decision, error) {
	trimmed := strings.TrimSpace(arguments)
	if trimmed == "" {
		return Decision{}, fmt.Errorf("tool arguments are empty")
	}

	var decision Decision
	if err := json.Unmarshal([]byte(trimmed), &decision); err == nil {
		if strings.TrimSpace(decision.Action) == "" {
			return Decision{}, fmt.Errorf("tool arguments missing action")
		}
		return decision, nil
	}

	decision = Decision{
		Action:        extractString(arguments, `"action"\s*:\s*"([^"]+)`),
		PublicReason:  extractString(arguments, `"public_reason"\s*:\s*"([^"]+)`),
		PrivateReason: extractString(arguments, `"private_reason"\s*:\s*"([^"]+)`),
	}
	if amount := extractString(arguments, `"amount"\s*:\s*(\d+)`); amount != "" {
		parsed, _ := strconv.Atoi(amount)
		decision.Amount = parsed
	}
	if strings.TrimSpace(decision.Action) == "" {
		return Decision{}, fmt.Errorf("parse tool arguments: action is missing")
	}
	return decision, nil
}

func normalizeDecision(decision Decision) Decision {
	decision.Action = strings.TrimSpace(strings.ToLower(decision.Action))
	decision.PublicReason = strings.TrimSpace(decision.PublicReason)
	decision.PrivateReason = strings.TrimSpace(decision.PrivateReason)
	if decision.PublicReason == "" {
		decision.PublicReason = "模型已返回动作，但未附带完整理由。"
	}
	if decision.PrivateReason == "" {
		decision.PrivateReason = decision.PublicReason
	}
	return decision
}

func extractString(value string, pattern string) string {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(value)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}
