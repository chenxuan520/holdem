package match

import (
	"testing"
	"time"

	"holdem/backend/internal/config"
)

func TestSpectatorModeCanAutoplay(t *testing.T) {
	service := NewService([]config.Preset{
		{ID: "ai-1", Name: "AI 1", Endpoint: "https://api.deepseek.com", Token: "replace-with-your-token", Model: "deepseek-v4-flash", SystemPrompt: "benchmark"},
		{ID: "ai-2", Name: "AI 2", Endpoint: "https://api.deepseek.com", Token: "replace-with-your-token", Model: "deepseek-v4-flash", SystemPrompt: "benchmark"},
	}, nil)

	snapshot, err := service.CreateMatch(CreateRequest{
		InitialChips:  40,
		SmallBlind:    5,
		BigBlind:      10,
		AIPresetIDs:   []string{"ai-1", "ai-2"},
		SpectatorMode: true,
	})
	if err != nil {
		t.Fatalf("CreateMatch returned error: %v", err)
	}
	if hasHuman(snapshot.Players) {
		t.Fatalf("did not expect human player in spectator mode")
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		updated, ok := service.GetMatch(snapshot.ID)
		if !ok {
			t.Fatalf("expected match to remain available")
		}
		if updated.Table.CompletedHands > 0 || updated.Status == "finished" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	updated, _ := service.GetMatch(snapshot.ID)
	t.Fatalf("expected spectator autoplay to progress hands, got status=%s completed=%d", updated.Status, updated.Table.CompletedHands)
}

func TestSpectatorSemiAutoStopsAfterHandSettled(t *testing.T) {
	service := NewService([]config.Preset{
		{ID: "ai-1", Name: "AI 1", Endpoint: "https://api.deepseek.com", Token: "replace-with-your-token", Model: "deepseek-v4-flash", SystemPrompt: "benchmark"},
		{ID: "ai-2", Name: "AI 2", Endpoint: "https://api.deepseek.com", Token: "replace-with-your-token", Model: "deepseek-v4-flash", SystemPrompt: "benchmark"},
	}, nil)

	snapshot, err := service.CreateMatch(CreateRequest{
		InitialChips:  40,
		SmallBlind:    5,
		BigBlind:      10,
		AIPresetIDs:   []string{"ai-1", "ai-2"},
		SpectatorMode: true,
		SemiAutoMode:  true,
	})
	if err != nil {
		t.Fatalf("CreateMatch returned error: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		updated, ok := service.GetMatch(snapshot.ID)
		if !ok {
			t.Fatalf("expected match to remain available")
		}
		if updated.Status == "hand_complete" && updated.Table.CompletedHands == 1 && !updated.Control.Running {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	updated, _ := service.GetMatch(snapshot.ID)
	t.Fatalf("expected semi-auto spectator to stop after one hand, got status=%s completed=%d running=%v", updated.Status, updated.Table.CompletedHands, updated.Control.Running)
}
