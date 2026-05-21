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

type Preset struct {
	ID           string `yaml:"id" json:"id"`
	Name         string `yaml:"name" json:"name"`
	Endpoint     string `yaml:"endpoint" json:"endpoint"`
	Token        string `yaml:"token" json:"-"`
	Model        string `yaml:"model" json:"model"`
	SystemPrompt string `yaml:"system_prompt" json:"systemPrompt"`
}

type PublicPreset struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Endpoint     string `json:"endpoint"`
	Model        string `json:"model"`
	SystemPrompt string `json:"systemPrompt"`
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
		ID:           p.ID,
		Name:         p.Name,
		Endpoint:     p.Endpoint,
		Model:        p.Model,
		SystemPrompt: p.SystemPrompt,
	}
}

func (p Preset) validate() error {
	required := map[string]string{
		"id":            strings.TrimSpace(p.ID),
		"name":          strings.TrimSpace(p.Name),
		"endpoint":      strings.TrimSpace(p.Endpoint),
		"token":         strings.TrimSpace(p.Token),
		"model":         strings.TrimSpace(p.Model),
		"system_prompt": strings.TrimSpace(p.SystemPrompt),
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
