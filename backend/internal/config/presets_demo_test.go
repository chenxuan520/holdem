package config

import (
	"path/filepath"
	"testing"
)

func TestRepositoryDemoPresetsLoad(t *testing.T) {
	presets, err := LoadPresets(filepath.Clean("../../../config/ai-presets.demo.yaml"))
	if err != nil {
		t.Fatalf("LoadPresets returned error: %v", err)
	}
	if len(presets) != 3 {
		t.Fatalf("expected 3 demo presets, got %d", len(presets))
	}
	for _, preset := range presets {
		if preset.Token != "replace-with-your-token" {
			t.Fatalf("expected demo preset %q to keep placeholder token, got %q", preset.ID, preset.Token)
		}
	}
}
