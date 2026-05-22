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

// ProbeResult is what we hand back to the lobby when the user clicks
// "test AI". It's intentionally tiny: the goal is to confirm the endpoint /
// token / model triple actually answers, not to validate poker reasoning.
type ProbeResult struct {
	OK              bool   `json:"ok"`
	LatencyMs       int64  `json:"latencyMs"`
	Model           string `json:"model,omitempty"`
	ResponseSnippet string `json:"responseSnippet,omitempty"`
	Error           string `json:"error,omitempty"`
}

// PromptInput is the structured user content sent to the model. Field names are
// intentionally short and only include data the model actually needs to make a
// decision; pre-computed numerical hints (position, effBB, potOdds, ...) are
// folded in so the model spends fewer tokens deriving them.
type PromptInput struct {
	Hand  int    `json:"hand"`
	Stage string `json:"stage"`

	// Self
	Seat          int      `json:"seat"`
	Position      string   `json:"position,omitempty"`
	Hole          []string `json:"hole"`
	YourChips     int      `json:"yourChips"`
	YourStreetBet int      `json:"yourStreetBet,omitempty"`

	// Table
	Board []string `json:"board"`
	Pot   int      `json:"pot"`
	SB    int      `json:"sb"`
	BB    int      `json:"bb"`
	EffBB float64  `json:"effBB"`

	// Action context
	ToCall       int     `json:"toCall,omitempty"`
	MinRaiseTo   int     `json:"minRaiseTo,omitempty"`
	PotOdds      float64 `json:"potOdds,omitempty"`
	LegalActions []any   `json:"actions"`

	// Other players + recent action log (compact strings).
	Players       []any    `json:"players"`
	LastAggressor *int     `json:"lastAgg,omitempty"`
	Log           []string `json:"log,omitempty"`
}

func NewClient() *Client {
	return &Client{httpClient: &http.Client{Timeout: 45 * time.Second}}
}

func (c *Client) Decide(ctx context.Context, preset config.Preset, input PromptInput) (Decision, RawLog, error) {
	if strings.Contains(preset.Token, "replace-with-your-token") {
		payload := buildRequestPayload(preset, input, 1, "")
		return Decision{}, RawLog{RequestPayload: payload, Error: "placeholder token configured"}, fmt.Errorf("placeholder token configured")
	}

	var attempts []AttemptLog
	var lastErr error
	lastHint := ""
	for attempt := 1; attempt <= 3; attempt++ {
		requestPayload := buildRequestPayload(preset, input, attempt, lastHint)
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
		lastHint = retryHintFromError(log.Error)
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

func buildRequestPayload(preset config.Preset, input PromptInput, attempt int, lastHint string) map[string]any {
	mode := preset.StructuredOutputMode()
	messages := []map[string]string{
		{"role": "system", "content": systemInstruction(preset.SystemPrompt, mode)},
		{"role": "user", "content": mustJSON(input)},
	}
	if attempt > 1 {
		messages = append(messages, map[string]string{
			"role":    "user",
			"content": retryReminder(attempt, lastHint),
		})
	}

	payload := map[string]any{
		"model":       preset.Model,
		"temperature": 0.1,
		"max_tokens":  maxTokensForAttempt(attempt),
		"messages":    messages,
	}
	switch mode {
	case config.StructuredOutputToolCall:
		payload["tools"] = decisionTools()
	case config.StructuredOutputJSONObject:
		payload["response_format"] = map[string]string{"type": "json_object"}
	case config.StructuredOutputNone:
		// Rely on prompt + parser fallback only.
	}
	return payload
}

func maxTokensForAttempt(attempt int) int {
	// Reasoning is supposed to be internal; the visible output is just an
	// action plus two short reasons. 192 / 256 / 384 is plenty and keeps
	// per-call output cost bounded.
	switch attempt {
	case 1:
		return 192
	case 2:
		return 256
	default:
		return 384
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

// retryHintFromError compresses the last attempt's error into a one-line hint
// the next attempt's reminder can splice in. Empty string means "no hint".
func retryHintFromError(rawErr string) string {
	hint := strings.TrimSpace(rawErr)
	if hint == "" {
		return ""
	}
	if len(hint) > 160 {
		hint = hint[:160] + "..."
	}
	return hint
}

// retryReminder is appended only on retry attempts. We keep it terse: a single
// short line that reminds the model of the contract plus, when available, the
// last failure cause so the model can correct it specifically.
func retryReminder(attempt int, lastHint string) string {
	if lastHint == "" {
		return fmt.Sprintf("第%d次重试：直接给最终动作，不要复述局面，amount 必须是整数。", attempt)
	}
	return fmt.Sprintf("第%d次重试，上次失败原因：%s。请直接给出合法 action+amount。", attempt, lastHint)
}

// systemInstruction is the single source of truth for the model's poker
// thinking framework, action-validity rules, and output contract. It is shared
// by every preset so benchmark comparisons stay fair; differences in play come
// from the model itself, not from prompt tuning. Length is intentionally tight
// to keep input-token cost low.
func systemInstruction(prompt string, mode string) string {
	var b strings.Builder
	if trimmed := strings.TrimSpace(prompt); trimmed != "" {
		b.WriteString(trimmed)
		b.WriteString("\n\n")
	}
	b.WriteString("你在打无限注德州扑克，每次回合做单一动作。无 ICM。\n")
	b.WriteString("【思路】effBB→风险预算；potOdds→所需赢率；牌力 vs 对手范围+foldEquity→选 EV 最高的合法动作。不要复述局面，不要在输出里写思考过程。\n")
	b.WriteString("【动作】action 必须来自 actions[].action；raise/all_in 时 amount ∈ [minRaiseTo, yourChips]，其它动作 amount=0。preflop open 常用 2.5–3×bb；postflop bet 常用 50–75% pot。\n")
	switch mode {
	case config.StructuredOutputToolCall:
		b.WriteString("【输出】调用 submit_action 工具，参数 action/amount/public_reason/private_reason，禁止额外文本。")
	case config.StructuredOutputJSONObject:
		b.WriteString("【输出】只返回一个 JSON 对象：{\"action\",\"amount\",\"public_reason\",\"private_reason\"}，禁止 markdown 或其它文本。")
	case config.StructuredOutputNone:
		fallthrough
	default:
		b.WriteString("【输出】只返回一个 JSON 对象：{\"action\",\"amount\",\"public_reason\",\"private_reason\"}，禁止其它文本。")
	}
	b.WriteString("\n两个 reason 各 ≤25 字。public_reason 不暴露具体牌点；private_reason 可写底牌、范围、计算。\n")
	b.WriteString("【日志缩写】f=fold x=check c=call r=raise A=all_in，例如 'flop:2.r40' 表示座位2在flop加注到40。")
	return b.String()
}

func mustJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

// Probe sends an ultra-cheap chat completion (~5 input + ~4 output tokens)
// to verify the endpoint, token and model actually answer. It does NOT
// exercise the structured-output path; the real match flow already retries
// with fallbacks if structured output misbehaves, so the probe focuses on
// the cheaper "is the model reachable" check.
func (c *Client) Probe(ctx context.Context, preset config.Preset) ProbeResult {
	if strings.Contains(preset.Token, "replace-with-your-token") {
		return ProbeResult{OK: false, Error: "preset still uses placeholder token"}
	}

	payload := map[string]any{
		"model":       preset.Model,
		"temperature": 0,
		"max_tokens":  16,
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ProbeResult{OK: false, Error: err.Error()}
	}

	endpoint := strings.TrimRight(preset.Endpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ProbeResult{OK: false, Error: err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+preset.Token)
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return ProbeResult{OK: false, LatencyMs: latency, Error: err.Error()}
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ProbeResult{OK: false, LatencyMs: latency, Error: err.Error()}
	}

	if resp.StatusCode >= 400 {
		message := strings.TrimSpace(string(rawBody))
		if len(message) > 200 {
			message = message[:200] + "..."
		}
		return ProbeResult{
			OK:        false,
			LatencyMs: latency,
			Error:     fmt.Sprintf("http %d: %s", resp.StatusCode, message),
		}
	}

	var completion struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(rawBody, &completion); err != nil {
		return ProbeResult{OK: false, LatencyMs: latency, Error: "bad json from provider: " + err.Error()}
	}

	snippet := ""
	if len(completion.Choices) > 0 {
		snippet = strings.TrimSpace(completion.Choices[0].Message.Content)
	}
	if len(snippet) > 80 {
		snippet = snippet[:80] + "..."
	}
	model := completion.Model
	if model == "" {
		model = preset.Model
	}
	return ProbeResult{OK: true, LatencyMs: latency, Model: model, ResponseSnippet: snippet}
}

func decisionTools() []map[string]any {
	return []map[string]any{{
		"type": "function",
		"function": map[string]any{
			"name":        "submit_action",
			"description": "提交本回合最终动作",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{
						"type": "string",
						"enum": []string{"fold", "check", "call", "raise", "all_in"},
					},
					"amount":         map[string]any{"type": "integer"},
					"public_reason":  map[string]any{"type": "string"},
					"private_reason": map[string]any{"type": "string"},
				},
				"required": []string{"action", "public_reason", "private_reason"},
			},
		},
	}}
}
