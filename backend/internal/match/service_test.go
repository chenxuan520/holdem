package match

import (
	"errors"
	"testing"

	"holdem/backend/internal/config"
)

func TestCreateMatchStartsPreflopState(t *testing.T) {
	service := NewService([]config.Preset{
		{ID: "ai-1", Name: "AI 1", Endpoint: "https://api.openai.com/v1", Token: "replace-with-your-token", Model: "gpt-4.1-mini", SystemPrompt: "test"},
	}, nil)

	snapshot, err := service.CreateMatch(CreateRequest{
		InitialChips: 60,
		SmallBlind:   5,
		BigBlind:     10,
		AIPresetIDs:  []string{"ai-1"},
	})
	if err != nil {
		t.Fatalf("CreateMatch returned error: %v", err)
	}

	if snapshot.Status != "awaiting_human" {
		t.Fatalf("expected awaiting_human after create, got %s", snapshot.Status)
	}
	if snapshot.Table.Stage != "preflop" {
		t.Fatalf("expected preflop stage, got %s", snapshot.Table.Stage)
	}
	if len(snapshot.Table.HeroCards) != 2 {
		t.Fatalf("expected 2 hero cards, got %d", len(snapshot.Table.HeroCards))
	}
	if len(snapshot.Table.LegalActions) == 0 {
		t.Fatalf("expected legal actions to be populated")
	}
	if len(service.ListReplays()) != 0 {
		t.Fatalf("did not expect replay before match finished")
	}
}

func TestCompletedHandsAdvanceAcrossHands(t *testing.T) {
	service := NewService([]config.Preset{
		{ID: "ai-1", Name: "AI 1", Endpoint: "https://api.openai.com/v1", Token: "replace-with-your-token", Model: "gpt-4.1-mini", SystemPrompt: "test"},
	}, nil)

	snapshot, err := service.CreateMatch(CreateRequest{
		InitialChips: 40,
		SmallBlind:   5,
		BigBlind:     10,
		AIPresetIDs:  []string{"ai-1"},
	})
	if err != nil {
		t.Fatalf("CreateMatch returned error: %v", err)
	}

	next, err := service.ApplyHeroAction(snapshot.ID, PlayerActionRequest{Action: "fold"})
	if err != nil {
		t.Fatalf("ApplyHeroAction returned error: %v", err)
	}

	if next.Table.HandNumber != 2 {
		t.Fatalf("expected next hand number 2, got %d", next.Table.HandNumber)
	}
	if next.Table.CompletedHands != 1 {
		t.Fatalf("expected completedHands 1, got %d", next.Table.CompletedHands)
	}
	if len(next.Table.LastWinners) == 0 {
		t.Fatalf("expected lastWinners to be populated")
	}
}

func TestPersistReplayLockedSetsWarningOnStoreFailure(t *testing.T) {
	service := NewService(nil, failingReplayStore{})
	snapshot := Snapshot{ID: "match-1"}
	service.hidden[snapshot.ID] = &hiddenState{
		replay: ReplayDetail{Summary: ReplaySummary{ID: snapshot.ID}},
	}

	service.persistReplayLocked(snapshot.ID, &snapshot)

	if snapshot.Warning == "" {
		t.Fatalf("expected warning when replay store fails")
	}
	if _, ok := service.replays[snapshot.ID]; !ok {
		t.Fatalf("expected replay to remain available in memory")
	}
}

func TestCreateMatchUsesCustomPlayerNames(t *testing.T) {
	service := NewService([]config.Preset{
		{ID: "ai-1", Name: "Benchmark A", Endpoint: "https://api.openai.com/v1", Token: "replace-with-your-token", Model: "gpt-4.1-mini", SystemPrompt: "test"},
		{ID: "ai-2", Name: "Benchmark B", Endpoint: "https://api.openai.com/v1", Token: "replace-with-your-token", Model: "gpt-4.1-mini", SystemPrompt: "test"},
	}, nil)

	snapshot, err := service.CreateMatch(CreateRequest{
		InitialChips: 60,
		SmallBlind:   5,
		BigBlind:     10,
		HumanName:    "主播",
		AIPresetIDs:  []string{"ai-1", "ai-2"},
		AIPlayerNames: []string{"老鲨鱼", "小狐狸"},
	})
	if err != nil {
		t.Fatalf("CreateMatch returned error: %v", err)
	}

	if snapshot.Players[0].Name != "主播" {
		t.Fatalf("expected custom human name, got %q", snapshot.Players[0].Name)
	}
	if snapshot.Players[1].Name != "老鲨鱼" || snapshot.Players[2].Name != "小狐狸" {
		t.Fatalf("expected custom ai names, got %+v", snapshot.Players)
	}
}

func TestDeleteReplayRemovesReplayFromMemory(t *testing.T) {
	service := NewService(nil, nil)
	service.replays["replay-1"] = ReplayDetail{Summary: ReplaySummary{ID: "replay-1"}}

	if err := service.DeleteReplay("replay-1"); err != nil {
		t.Fatalf("DeleteReplay returned error: %v", err)
	}
	if _, ok := service.replays["replay-1"]; ok {
		t.Fatalf("expected replay to be removed from memory")
	}
}

func TestClearReplaysRemovesAllFromMemory(t *testing.T) {
	service := NewService(nil, nil)
	service.replays["replay-1"] = ReplayDetail{Summary: ReplaySummary{ID: "replay-1"}}
	service.replays["replay-2"] = ReplayDetail{Summary: ReplaySummary{ID: "replay-2"}}

	if err := service.ClearReplays(); err != nil {
		t.Fatalf("ClearReplays returned error: %v", err)
	}
	if len(service.replays) != 0 {
		t.Fatalf("expected all replays to be removed, got %d", len(service.replays))
	}
}

type failingReplayStore struct{}

func (failingReplayStore) SaveReplay(ReplayDetail) error { return errors.New("boom") }
func (failingReplayStore) ListReplays() ([]ReplaySummary, error) { return nil, nil }
func (failingReplayStore) GetReplay(string) (ReplayDetail, bool, error) {
	return ReplayDetail{}, false, nil
}
func (failingReplayStore) DeleteReplay(string) error { return nil }
func (failingReplayStore) ClearReplays() error     { return nil }
