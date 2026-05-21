package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"holdem/backend/internal/config"
)

type Client struct {
	httpClient *http.Client
}

type Decision struct {
	Action        string `json:"action"`
	Amount        int    `json:"amount,omitempty"`
	PublicReason  string `json:"public_reason"`
	PrivateReason string `json:"private_reason,omitempty"`
}

type RawLog struct {
	RequestPayload any
	ResponseBody   string
	Error          string
	Attempts       []AttemptLog `json:"attempts,omitempty"`
}

type AttemptLog struct {
	RequestPayload any    `json:"requestPayload"`
	ResponseBody   string `json:"responseBody"`
	Error          string `json:"error,omitempty"`
}

type PromptInput struct {
	MatchID         string      `json:"matchId"`
	HandNumber      int         `json:"handNumber"`
	Seat            int         `json:"seat"`
	PlayerName      string      `json:"playerName"`
	Stage           string      `json:"stage"`
	Board           []string    `json:"board"`
	HoleCards       []string    `json:"holeCards"`
	Pot             int         `json:"pot"`
	ToCall          int         `json:"toCall"`
	MinimumRaiseTo  int         `json:"minimumRaiseTo"`
	LegalActions    []any       `json:"legalActions"`
	Players         []any       `json:"players"`
	RecentActionLog []any       `json:"recentActionLog"`
}

func NewClient() *Client {
	return &Client{httpClient: &http.Client{Timeout: 45 * time.Second}}
}

func (c *Client) Decide(ctx context.Context, preset config.Preset, input PromptInput) (Decision, RawLog, error) {
	if strings.Contains(preset.Token, "replace-with-your-token") {
		payload := buildRequestPayload(preset, input, 1)
		return Decision{}, RawLog{RequestPayload: payload, Error: "placeholder token configured"}, fmt.Errorf("placeholder token configured")
	}

	var attempts []AttemptLog
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		requestPayload := buildRequestPayload(preset, input, attempt)
		decision, log, err := c.decideOnce(ctx, preset, requestPayload)
		attempts = append(attempts, AttemptLog{
			RequestPayload: requestPayload,
			ResponseBody:   log.ResponseBody,
			Error:          log.Error,
		})
		if err == nil {
			log.Attempts = attempts
			return decision, log, nil
		}
		lastErr = err
		if ctx.Err() != nil || attempt == 3 {
			return Decision{}, RawLog{
				RequestPayload: requestPayload,
				ResponseBody:   log.ResponseBody,
				Error:          summarizeAttemptErrors(attempts),
				Attempts:       attempts,
			}, lastErr
		}
		time.Sleep(time.Duration(attempt) * 250 * time.Millisecond)
	}

	return Decision{}, RawLog{Error: "unexpected retry flow", Attempts: attempts}, lastErr
}

func (c *Client) decideOnce(ctx context.Context, preset config.Preset, requestPayload map[string]any) (Decision, RawLog, error) {
	body, err := json.Marshal(requestPayload)
	if err != nil {
		return Decision{}, RawLog{RequestPayload: requestPayload, Error: err.Error()}, err
	}

	endpoint := strings.TrimRight(preset.Endpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Decision{}, RawLog{RequestPayload: requestPayload, Error: err.Error()}, err
	}
	req.Header.Set("Authorization", "Bearer "+preset.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Decision{}, RawLog{RequestPayload: requestPayload, Error: err.Error()}, err
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Decision{}, RawLog{RequestPayload: requestPayload, Error: err.Error()}, err
	}

	log := RawLog{RequestPayload: requestPayload, ResponseBody: string(rawBody)}
	if resp.StatusCode >= 400 {
		log.Error = fmt.Sprintf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(rawBody)))
		return Decision{}, log, fmt.Errorf("ai endpoint returned http %d", resp.StatusCode)
	}

	var completion struct {
		Choices []struct {
			Message struct {
				Content   string     `json:"content"`
				ToolCalls []ToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(rawBody, &completion); err != nil {
		log.Error = err.Error()
		return Decision{}, log, err
	}
	if len(completion.Choices) == 0 {
		log.Error = "empty choices"
		return Decision{}, log, fmt.Errorf("empty ai response")
	}

	decision, err := parseDecisionResponse(completion.Choices[0].Message.Content, completion.Choices[0].Message.ToolCalls)
	if err != nil {
		log.Error = err.Error()
		return Decision{}, log, err
	}

	return decision, log, nil
}

func buildRequestPayload(preset config.Preset, input PromptInput, attempt int) map[string]any {
	messages := []map[string]string{
		{"role": "system", "content": systemInstruction(preset.SystemPrompt)},
		{"role": "user", "content": mustJSON(input)},
	}
	if attempt > 1 {
		messages = append(messages, map[string]string{
			"role": "user",
			"content": fmt.Sprintf("上一轮响应未能形成完整可执行动作（第 %d 次重试）。请不要复述局面，不要输出自然语言段落，立即返回 submit_action 所需字段；如果使用 amount，必须是整数；如果当前动作不需要 amount，请填 0。", attempt),
		})
	}

	payload := map[string]any{
		"model":       preset.Model,
		"temperature": 0.1,
		"max_tokens":  maxTokensForAttempt(attempt),
		"messages":    messages,
	}
	if useToolCallingMode(preset) {
		payload["tools"] = decisionTools()
	} else if useJSONObjectMode(preset) {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
	return payload
}

func maxTokensForAttempt(attempt int) int {
	switch attempt {
	case 1:
		return 256
	case 2:
		return 384
	default:
		return 512
	}
}

func summarizeAttemptErrors(attempts []AttemptLog) string {
	parts := make([]string, 0, len(attempts))
	for index, attempt := range attempts {
		message := strings.TrimSpace(attempt.Error)
		if message == "" {
			message = "no explicit error"
		}
		parts = append(parts, fmt.Sprintf("attempt %d: %s", index+1, message))
	}
	return strings.Join(parts, " | ")
}

func systemInstruction(prompt string) string {
	return prompt + "\n\n如果存在 submit_action 工具，第一优先级就是直接调用它，不要先输出自然语言答案，不要解释规则，不要复述局面。无论采用工具调用还是普通 JSON，最终都必须给出 action / amount / public_reason / private_reason。只允许从 legalActions 里选择动作；若需要 amount，必须给整数；若动作不需要 amount，请填 0。public_reason 不能泄露你的底牌。两个 reason 都尽量压缩成一句短句。"
}

func mustJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func useJSONObjectMode(preset config.Preset) bool {
	return strings.Contains(strings.ToLower(preset.Endpoint), "deepseek") || strings.Contains(strings.ToLower(preset.Model), "deepseek")
}

func useToolCallingMode(preset config.Preset) bool {
	return strings.Contains(strings.ToLower(preset.Endpoint), "deepseek") || strings.Contains(strings.ToLower(preset.Model), "deepseek")
}

func decisionTools() []map[string]any {
	return []map[string]any{{
		"type": "function",
		"function": map[string]any{
			"name":        "submit_action",
			"description": "为当前德州扑克局面提交最终动作",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{
						"type": "string",
						"enum": []string{"fold", "check", "call", "raise", "all_in"},
					},
					"amount": map[string]any{"type": "integer"},
					"public_reason": map[string]any{"type": "string"},
					"private_reason": map[string]any{"type": "string"},
				},
				"required": []string{"action", "public_reason", "private_reason"},
			},
		},
	}}
}
