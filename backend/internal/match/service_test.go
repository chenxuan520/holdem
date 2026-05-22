package match

import (
	"errors"
	"fmt"
	backendai "holdem/backend/internal/ai"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

	if next.Status != "hand_complete" {
		t.Fatalf("expected hand_complete after hero fold, got %s", next.Status)
	}
	if next.Table.HandNumber != 1 {
		t.Fatalf("expected to stay on settled hand 1 before continue, got %d", next.Table.HandNumber)
	}
	if next.Table.CompletedHands != 1 {
		t.Fatalf("expected completedHands 1, got %d", next.Table.CompletedHands)
	}
	if len(next.Table.LastWinners) == 0 {
		t.Fatalf("expected lastWinners to be populated")
	}

	continued, err := service.ControlMatch(snapshot.ID, ControlRequest{Action: "continue"})
	if err != nil {
		t.Fatalf("ControlMatch continue returned error: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		continued, _ = service.GetMatch(snapshot.ID)
		if continued.Table.CompletedHands >= 2 || continued.Table.HandNumber >= 2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected match to advance after continue, got hand=%d completed=%d status=%s", continued.Table.HandNumber, continued.Table.CompletedHands, continued.Status)
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
		InitialChips:  60,
		SmallBlind:    5,
		BigBlind:      10,
		HumanName:     "主播",
		AIPresetIDs:   []string{"ai-1", "ai-2"},
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

func TestRepeatedAIFailuresFallbackToFold(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"{"}}]}`)
	}))
	defer server.Close()

	service := NewService([]config.Preset{
		{ID: "ai-1", Name: "AI 1", Endpoint: server.URL, Token: "test-token", Model: "deepseek-v4-flash", SystemPrompt: "test"},
		{ID: "ai-2", Name: "AI 2", Endpoint: "https://api.openai.com/v1", Token: "replace-with-your-token", Model: "gpt-4.1-mini", SystemPrompt: "test"},
	}, nil)

	snapshot, err := service.CreateMatch(CreateRequest{
		InitialChips:  40,
		SmallBlind:    5,
		BigBlind:      10,
		AIPresetIDs:   []string{"ai-1", "ai-2"},
		SpectatorMode: true,
		ManualMode:    true,
	})
	if err != nil {
		t.Fatalf("CreateMatch returned error: %v", err)
	}

	service.mu.Lock()
	current := service.matches[snapshot.ID]
	current.Control.CanStep = true
	service.matches[snapshot.ID] = current
	service.mu.Unlock()

	_, err = service.runUntilPause(snapshot.ID, current)
	if err != nil {
		t.Fatalf("runUntilPause returned error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 AI attempts before fallback, got %d", attempts)
	}
	hidden := service.hidden[snapshot.ID]
	if hidden == nil || len(hidden.replay.AILogs) == 0 {
		t.Fatalf("expected AI log to be recorded")
	}
	decision, ok := hidden.replay.AILogs[0].Structured.(backendai.Decision)
	if !ok {
		t.Fatalf("expected structured AI log to keep decision, got %T", hidden.replay.AILogs[0].Structured)
	}
	if decision.Action != "fold" {
		t.Fatalf("expected repeated request failure to fallback to fold, got %+v", decision)
	}
	if hidden.replay.AILogs[0].AttemptCount != 3 {
		t.Fatalf("expected attemptCount 3, got %d", hidden.replay.AILogs[0].AttemptCount)
	}
	if len(hidden.replay.Hands) == 0 {
		t.Fatalf("expected completed hand replay after AI fold fallback")
	}
	aiFolded := false
	for _, player := range hidden.replay.Hands[0].Players {
		if player.Name == "AI 1" {
			aiFolded = player.Folded
		}
	}
	if !aiFolded {
		t.Fatalf("expected AI player to be marked folded in replay")
	}
}

func TestListRecordsIncludesActiveMatch(t *testing.T) {
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

	records := service.ListRecords()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].ID != snapshot.ID || !records[0].ContinueAvailable || records[0].ReplayAvailable {
		t.Fatalf("unexpected active record summary: %+v", records[0])
	}
}

func TestStopMatchCreatesReplayRecord(t *testing.T) {
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

	stopped, err := service.ControlMatch(snapshot.ID, ControlRequest{Action: "stop"})
	if err != nil {
		t.Fatalf("ControlMatch returned error: %v", err)
	}
	if stopped.Status != "stopped" {
		t.Fatalf("expected stopped status, got %s", stopped.Status)
	}
	replay, ok := service.GetReplay(snapshot.ID)
	if !ok {
		t.Fatalf("expected stopped match replay to be available")
	}
	if replay.Summary.Status != "stopped" {
		t.Fatalf("expected replay status stopped, got %s", replay.Summary.Status)
	}
	records := service.ListRecords()
	if len(records) != 1 || !records[0].ReplayAvailable || records[0].ContinueAvailable || records[0].Status != "stopped" {
		t.Fatalf("unexpected record summary after stop: %+v", records)
	}
	if len(replay.Hands) == 0 {
		t.Fatalf("expected stopped replay to include current hand snapshot")
	}
}

func TestDeleteRecordRemovesActiveMatch(t *testing.T) {
	service := NewService([]config.Preset{{ID: "ai-1", Name: "AI 1", Endpoint: "https://api.openai.com/v1", Token: "replace-with-your-token", Model: "gpt-4.1-mini", SystemPrompt: "test"}}, nil)
	snapshot, err := service.CreateMatch(CreateRequest{InitialChips: 60, SmallBlind: 5, BigBlind: 10, AIPresetIDs: []string{"ai-1"}})
	if err != nil {
		t.Fatalf("CreateMatch returned error: %v", err)
	}
	if err := service.DeleteRecord(snapshot.ID); err != nil {
		t.Fatalf("DeleteRecord returned error: %v", err)
	}
	if _, ok := service.GetMatch(snapshot.ID); ok {
		t.Fatalf("expected active match to be removed")
	}
}

func TestBuildTableStateRevealsShowdownCardsToHuman(t *testing.T) {
	players := []Player{
		{Seat: 0, Name: "你", Chips: 100, IsHuman: true},
		{Seat: 1, Name: "AI A", Chips: 80, PresetID: "ai-1"},
		{Seat: 2, Name: "AI B", Chips: 60, PresetID: "ai-2"},
	}
	hand := &handState{
		Number:             1,
		Stage:              "river",
		DealerSeat:         0,
		SmallBlindSeat:     1,
		BigBlindSeat:       2,
		CurrentTurnSeat:    -1,
		Pot:                120,
		Board:              []string{"As", "Kd", "7c", "2s", "9h"},
		HoleCards:          map[int][]string{0: {"Qc", "Qd"}, 1: {"Jh", "Jd"}, 2: {"Ts", "Td"}},
		RevealedCards:      map[int][]string{1: {"Jh", "Jd"}, 2: {"Ts", "Td"}},
		Folded:             map[int]bool{0: false, 1: false, 2: false},
		AllIn:              map[int]bool{},
		StreetContribution: map[int]int{},
	}

	table := buildTableState(players, hand, []string{"你"})
	if len(table.VisibleHoleCards) != 2 {
		t.Fatalf("expected 2 revealed AI hole card sets, got %d", len(table.VisibleHoleCards))
	}
	if table.VisibleHoleCards[0].PlayerName != "AI A" || len(table.VisibleHoleCards[0].Cards) != 2 {
		t.Fatalf("unexpected revealed cards payload: %+v", table.VisibleHoleCards)
	}
}

func TestFirstPostflopTurnSkipsFoldedPlayers(t *testing.T) {
	players := []Player{
		{Seat: 0, Name: "你", Chips: 180, IsHuman: true},
		{Seat: 1, Name: "AI A", Chips: 190, PresetID: "ai-1"},
		{Seat: 2, Name: "AI B", Chips: 180, PresetID: "ai-2"},
	}
	hand := &handState{
		Folded: map[int]bool{1: true},
		AllIn:  map[int]bool{},
	}

	seat := firstPostflopTurn(players, hand, 0)
	if seat != 2 {
		t.Fatalf("expected folded seat 1 to be skipped, got seat %d", seat)
	}
}

func TestHydrateLoadedMatchRevealsShowdownCards(t *testing.T) {
	snapshot := Snapshot{
		ID: "match-1",
		Players: []Player{
			{Seat: 0, Name: "你", Chips: 230, IsHuman: true},
			{Seat: 1, Name: "AI A", Chips: 190, PresetID: "ai-1"},
			{Seat: 2, Name: "AI B", Chips: 180, PresetID: "ai-2"},
		},
		Status: "hand_complete",
		Table:  TableState{LastWinners: []string{"你"}},
	}
	record := ActiveMatchRecord{
		Hand: &handState{
			Number:             1,
			Stage:              "river",
			DealerSeat:         0,
			SmallBlindSeat:     1,
			BigBlindSeat:       2,
			CurrentTurnSeat:    -1,
			Pot:                50,
			Board:              []string{"Qs", "5h", "2h", "Tc", "9s"},
			HoleCards:          map[int][]string{0: {"Kd", "9c"}, 1: {"7d", "7c"}, 2: {"Jh", "3d"}},
			Folded:             map[int]bool{1: true, 2: false},
			AllIn:              map[int]bool{},
			StreetContribution: map[int]int{},
		},
		Replay: ReplayDetail{},
		Current: &ReplayHand{
			Winners: []ReplayWinner{{Seat: 0, PlayerName: "你", Amount: 50, HandLabel: "一对"}},
			Players: []ReplayPlayerState{
				{Seat: 0, Name: "你", IsHuman: true, HoleCards: []string{"Kd", "9c"}},
				{Seat: 1, Name: "AI A", HoleCards: []string{"7d", "7c"}, Folded: true},
				{Seat: 2, Name: "AI B", HoleCards: []string{"Jh", "3d"}, Folded: false},
			},
		},
	}

	hydrateLoadedMatch(&snapshot, &record)
	if len(snapshot.Table.VisibleHoleCards) != 2 {
		t.Fatalf("expected hero and AI showdown hands after hydration, got %+v", snapshot.Table.VisibleHoleCards)
	}
	if snapshot.Table.VisibleHoleCards[0].PlayerName != "你" || snapshot.Table.VisibleHoleCards[1].PlayerName != "AI B" {
		t.Fatalf("expected hero and AI B showdown hands to be visible, got %+v", snapshot.Table.VisibleHoleCards)
	}
}

func TestHydrateLoadedMatchRevealsAllShowdownParticipants(t *testing.T) {
	snapshot := Snapshot{
		ID: "match-2",
		Players: []Player{
			{Seat: 0, Name: "你", Chips: 100, IsHuman: true},
			{Seat: 1, Name: "AI A", Chips: 140, PresetID: "ai-1"},
			{Seat: 2, Name: "AI B", Chips: 160, PresetID: "ai-2"},
		},
		Status: "hand_complete",
	}
	record := ActiveMatchRecord{
		Hand: &handState{
			Number:          2,
			Stage:           "river",
			DealerSeat:      0,
			SmallBlindSeat:  1,
			BigBlindSeat:    2,
			CurrentTurnSeat: -1,
			Pot:             100,
			Board:           []string{"Qs", "5h", "2h", "Tc", "9s"},
			HoleCards: map[int][]string{
				0: {"5s", "6c"},
				1: {"Qh", "Kd"},
				2: {"9d", "Jc"},
			},
			Folded:             map[int]bool{0: true, 1: false, 2: false},
			AllIn:              map[int]bool{},
			StreetContribution: map[int]int{},
		},
		Replay: ReplayDetail{},
		Current: &ReplayHand{
			Winners: []ReplayWinner{{Seat: 2, PlayerName: "AI B", Amount: 100, HandLabel: "两对"}},
			Players: []ReplayPlayerState{
				{Seat: 0, Name: "你", IsHuman: true, HoleCards: []string{"5s", "6c"}, Folded: true},
				{Seat: 1, Name: "AI A", HoleCards: []string{"Qh", "Kd"}, Folded: false},
				{Seat: 2, Name: "AI B", HoleCards: []string{"9d", "Jc"}, Folded: false},
			},
		},
	}

	hydrateLoadedMatch(&snapshot, &record)
	if len(snapshot.Table.VisibleHoleCards) != 2 {
		t.Fatalf("expected 2 showdown AI hands after hydration, got %+v", snapshot.Table.VisibleHoleCards)
	}
	if snapshot.Table.VisibleHoleCards[0].PlayerName != "AI A" || snapshot.Table.VisibleHoleCards[1].PlayerName != "AI B" {
		t.Fatalf("expected AI A and AI B showdown hands to be visible, got %+v", snapshot.Table.VisibleHoleCards)
	}
}

func TestGetReplayNormalizesNilSlices(t *testing.T) {
	service := NewService(nil, nil)
	service.replays["replay-1"] = ReplayDetail{
		Summary: ReplaySummary{ID: "replay-1"},
		Hands: []ReplayHand{{
			HandNumber: 1,
			Board:      nil,
			Players: []ReplayPlayerState{{
				Seat:      0,
				Name:      "你",
				HoleCards: nil,
			}},
			Events:  nil,
			Winners: nil,
		}},
		Players: nil,
		AILogs:  nil,
	}

	replay, ok := service.GetReplay("replay-1")
	if !ok {
		t.Fatalf("expected replay to exist")
	}
	if replay.Players == nil || replay.AILogs == nil {
		t.Fatalf("expected top-level slices to be normalized, got %+v", replay)
	}
	if replay.Hands[0].Board == nil || replay.Hands[0].Events == nil || replay.Hands[0].Winners == nil {
		t.Fatalf("expected hand slices to be normalized, got %+v", replay.Hands[0])
	}
	if replay.Hands[0].Players[0].HoleCards == nil {
		t.Fatalf("expected player hole cards slice to be normalized")
	}
}

type failingReplayStore struct{}

func (failingReplayStore) SaveReplay(ReplayDetail) error         { return errors.New("boom") }
func (failingReplayStore) ListReplays() ([]ReplaySummary, error) { return nil, nil }
func (failingReplayStore) GetReplay(string) (ReplayDetail, bool, error) {
	return ReplayDetail{}, false, nil
}
func (failingReplayStore) DeleteReplay(string) error                       { return nil }
func (failingReplayStore) ClearReplays() error                             { return nil }
func (failingReplayStore) SaveActiveMatch(ActiveMatchRecord) error         { return nil }
func (failingReplayStore) ListActiveMatches() ([]ActiveMatchRecord, error) { return nil, nil }
func (failingReplayStore) DeleteActiveMatch(string) error                  { return nil }
func (failingReplayStore) ClearActiveMatches() error                       { return nil }
