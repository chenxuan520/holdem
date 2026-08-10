package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type PresetFile struct {
	Presets []Preset `yaml:"presets"`
}

// Structured-output modes the preset can request from the OpenAI-compatible
// provider. The Preset.StructuredOutput field is normalized into one of these
// canonical values during LoadPresets, and the AI client switches behaviour
// based on that single field instead of guessing from the endpoint URL.
const (
	StructuredOutputToolCall   = "tool_call"
	StructuredOutputJSONObject = "json_object"
	StructuredOutputNone       = "none"
)

type Preset struct {
	ID               string `yaml:"id" json:"id"`
	Name             string `yaml:"name" json:"name"`
	Endpoint         string `yaml:"endpoint" json:"endpoint"`
	Token            string `yaml:"token" json:"-"`
	// ExtraHeaders are merged into every outbound HTTP request for this preset.
	// Intended for non-auth gateway metadata only, e.g. TTADK/llmbox's
	// `X-Source: ttadk` header that unlocks certain routed models.
	ExtraHeaders     map[string]string `yaml:"extra_headers,omitempty" json:"-"`
	Model            string `yaml:"model" json:"model"`
	SystemPrompt     string `yaml:"system_prompt" json:"systemPrompt"`
	StructuredOutput string `yaml:"structured_output,omitempty" json:"structuredOutput,omitempty"`
	// MaxTokens overrides the per-attempt output-token ceiling. Reasoning
	// models emit a long chain-of-thought before the answer, so the small
	// defaults get exhausted mid-reasoning and the answer content comes back
	// empty. 0 = use the built-in per-attempt defaults.
	MaxTokens int `yaml:"max_tokens,omitempty" json:"maxTokens,omitempty"`
	// ExtraBody is merged verbatim into the chat-completions request body — an
	// escape hatch for provider-specific knobs the common path doesn't model,
	// e.g. DeepSeek V4's `thinking: {type: disabled}` to turn off the
	// chain-of-thought, or `reasoning_effort`. Keys here override the defaults,
	// so use it for request params only — never messages / model.
	ExtraBody map[string]any `yaml:"extra_body,omitempty" json:"-"`
}

type PublicPreset struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Endpoint         string `json:"endpoint"`
	Model            string `json:"model"`
	SystemPrompt     string `json:"systemPrompt"`
	StructuredOutput string `json:"structuredOutput"`
}

func LoadPresets(path string) ([]Preset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read presets: %w", err)
	}

	var file PresetFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse presets yaml: %w", err)
	}

	if len(file.Presets) == 0 {
		return nil, fmt.Errorf("no presets configured")
	}

	seen := map[string]struct{}{}
	for i := range file.Presets {
		preset := &file.Presets[i]
		if strings.TrimSpace(preset.ID) == "" {
			preset.ID = defaultPresetID(preset.Name, i+1)
		}
		normalized, err := normalizeStructuredOutput(preset.StructuredOutput)
		if err != nil {
			return nil, fmt.Errorf("preset %d: %w", i+1, err)
		}
		preset.StructuredOutput = normalized
		if err := preset.validate(); err != nil {
			return nil, fmt.Errorf("preset %d: %w", i+1, err)
		}
		if _, ok := seen[preset.ID]; ok {
			return nil, fmt.Errorf("preset %d: duplicate id %q", i+1, preset.ID)
		}
		seen[preset.ID] = struct{}{}
	}

	return file.Presets, nil

}

func (p Preset) Public() PublicPreset {
	return PublicPreset{
		ID:               p.ID,
		Name:             p.Name,
		Endpoint:         p.Endpoint,
		Model:            p.Model,
		SystemPrompt:     p.SystemPrompt,
		StructuredOutput: p.StructuredOutputMode(),
	}
}

// StructuredOutputMode returns the canonical structured-output mode for this
// preset. An empty value (legacy yaml without the field) falls back to
// tool_call, which is the OpenAI-compatible default.
func (p Preset) StructuredOutputMode() string {
	if p.StructuredOutput == "" {
		return StructuredOutputToolCall
	}
	return p.StructuredOutput
}

// normalizeStructuredOutput maps user-facing aliases to canonical values, so
// the rest of the code only ever sees tool_call / json_object / none. An empty
// input is treated as the default (tool_call).
func normalizeStructuredOutput(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return StructuredOutputToolCall, nil
	case "tool_call", "tool_calls", "tools", "tool":
		return StructuredOutputToolCall, nil
	case "json_object", "json", "jsonobject":
		return StructuredOutputJSONObject, nil
	case "none", "off", "disabled", "false":
		return StructuredOutputNone, nil
	default:
		return "", fmt.Errorf("invalid structured_output %q (expected tool_call | json_object | none)", value)
	}
}

// InlinePresetInput is the JSON shape callers POST when supplying a
// preset inline (lobby's "自定义模型" form). It mirrors Preset but
// deliberately keeps Token in the JSON tag so the field actually arrives
// — Preset.Token is locked to `json:"-"` so a stray response with a
// Preset value can never leak credentials, and this input type is the
// only opt-in path for inbound tokens.
type InlinePresetInput struct {
	Name             string `json:"name"`
	Endpoint         string `json:"endpoint"`
	Token            string `json:"token"`
	Model            string `json:"model"`
	SystemPrompt     string `json:"systemPrompt"`
	StructuredOutput string         `json:"structuredOutput"`
	MaxTokens        int            `json:"maxTokens"`
	ExtraBody        map[string]any `json:"extraBody,omitempty"`
}

// ToPreset converts the inbound input into the internal Preset shape;
// callers should run PrepareInlinePreset right after to validate +
// canonicalise structured output before using it.
func (in InlinePresetInput) ToPreset() Preset {
	return Preset{
		Name:             in.Name,
		Endpoint:         in.Endpoint,
		Token:            in.Token,
		Model:            in.Model,
		SystemPrompt:     in.SystemPrompt,
		StructuredOutput: in.StructuredOutput,
		MaxTokens:        in.MaxTokens,
		ExtraBody:        in.ExtraBody,
	}
}

// PrepareInlinePreset normalises and validates a Preset that was supplied
// at request time (e.g. from the lobby's "自定义模型" form) instead of
// loaded from the YAML file. It returns the canonicalised preset on
// success; on error the caller should reject the request. ID is NOT set
// here — match.Service.CreateMatch generates an ephemeral inline-* id so
// it can't collide with built-in presets.
func PrepareInlinePreset(preset Preset) (Preset, error) {
	preset.Name = strings.TrimSpace(preset.Name)
	preset.Endpoint = strings.TrimSpace(preset.Endpoint)
	preset.Token = strings.TrimSpace(preset.Token)
	preset.Model = strings.TrimSpace(preset.Model)
	preset.SystemPrompt = strings.TrimSpace(preset.SystemPrompt)
	normalized, err := normalizeStructuredOutput(preset.StructuredOutput)
	if err != nil {
		return Preset{}, err
	}
	preset.StructuredOutput = normalized
	if preset.Name == "" {
		return Preset{}, fmt.Errorf("missing name")
	}
	if preset.Endpoint == "" {
		return Preset{}, fmt.Errorf("missing endpoint")
	}
	if preset.Token == "" {
		return Preset{}, fmt.Errorf("missing token")
	}
	if preset.Model == "" {
		return Preset{}, fmt.Errorf("missing model")
	}
	return preset, nil
}

func (p Preset) validate() error {
	// system_prompt is intentionally optional: the AI client ships a complete
	// NLHE decision framework + output contract by itself, and benchmark
	// presets are expected to keep the YAML system_prompt empty so every model
	// gets exactly the same prompt context. Only the connection fields are
	// strictly required.
	required := map[string]string{
		"id":       strings.TrimSpace(p.ID),
		"name":     strings.TrimSpace(p.Name),
		"endpoint": strings.TrimSpace(p.Endpoint),
		"token":    strings.TrimSpace(p.Token),
		"model":    strings.TrimSpace(p.Model),
	}
	for field, value := range required {
		if value == "" {
			return fmt.Errorf("missing %s", field)
		}
	}
	return nil
}

func defaultPresetID(name string, index int) string {
	base := strings.ToLower(strings.TrimSpace(name))
	base = strings.ReplaceAll(base, "_", "-")
	base = strings.ReplaceAll(base, " ", "-")

	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-':
			b.WriteRune(r)
		}
	}

	id := strings.Trim(b.String(), "-")
	if id == "" {
		id = fmt.Sprintf("preset-%d", index)
	}
	return id
}
