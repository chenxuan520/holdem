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

func TestDefaultAINamesOmitOrdinalForUniquePresets(t *testing.T) {
	service := NewService([]config.Preset{
		{ID: "ai-1", Name: "Benchmark A", Endpoint: "https://api.openai.com/v1", Token: "replace-with-your-token", Model: "gpt-4.1-mini", SystemPrompt: "test"},
		{ID: "ai-2", Name: "Benchmark B", Endpoint: "https://api.openai.com/v1", Token: "replace-with-your-token", Model: "gpt-4.1-mini", SystemPrompt: "test"},
	}, nil)

	snapshot, err := service.CreateMatch(CreateRequest{
		InitialChips: 100,
		SmallBlind:   5,
		BigBlind:     10,
		AIPresetIDs:  []string{"ai-1", "ai-2"},
	})
	if err != nil {
		t.Fatalf("CreateMatch returned error: %v", err)
	}

	aiSeats := snapshot.Players[1:]
	if len(aiSeats) != 2 {
		t.Fatalf("expected 2 ai seats, got %d", len(aiSeats))
	}
	if aiSeats[0].Name != "Benchmark A" || aiSeats[1].Name != "Benchmark B" {
		t.Fatalf("expected unique-preset names without ordinals, got %+v", aiSeats)
	}
}

func TestDefaultAINamesAddOrdinalsForRepeatedPresets(t *testing.T) {
	service := NewService([]config.Preset{
		{ID: "ai-1", Name: "Benchmark A", Endpoint: "https://api.openai.com/v1", Token: "replace-with-your-token", Model: "gpt-4.1-mini", SystemPrompt: "test"},
	}, nil)

	snapshot, err := service.CreateMatch(CreateRequest{
		InitialChips:  100,
		SmallBlind:    5,
		BigBlind:      10,
		AIPresetIDs:   []string{"ai-1", "ai-1", "ai-1"},
		SpectatorMode: true,
	})
	if err != nil {
		t.Fatalf("CreateMatch returned error: %v", err)
	}

	if len(snapshot.Players) != 3 {
		t.Fatalf("expected 3 ai seats, got %d", len(snapshot.Players))
	}
	expected := []string{"Benchmark A #1", "Benchmark A #2", "Benchmark A #3"}
	for i, want := range expected {
		if snapshot.Players[i].Name != want {
			t.Fatalf("expected seat %d to be %q, got %q", i, want, snapshot.Players[i].Name)
		}
	}
}

func TestCustomAINameOverridesDefault(t *testing.T) {
	service := NewService([]config.Preset{
		{ID: "ai-1", Name: "Benchmark A", Endpoint: "https://api.openai.com/v1", Token: "replace-with-your-token", Model: "gpt-4.1-mini", SystemPrompt: "test"},
	}, nil)

	snapshot, err := service.CreateMatch(CreateRequest{
		InitialChips:  100,
		SmallBlind:    5,
		BigBlind:      10,
		AIPresetIDs:   []string{"ai-1", "ai-1"},
		AIPlayerNames: []string{"老鲨鱼", ""},
		SpectatorMode: true,
	})
	if err != nil {
		t.Fatalf("CreateMatch returned error: %v", err)
	}

	if snapshot.Players[0].Name != "老鲨鱼" {
		t.Fatalf("expected first ai seat to keep custom name, got %q", snapshot.Players[0].Name)
	}
	if snapshot.Players[1].Name != "Benchmark A #2" {
		t.Fatalf("expected second ai seat to fall back to numbered default, got %q", snapshot.Players[1].Name)
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
	// The hand is in hand_complete status; finalizeHand has populated
	// hidden.current with the fold result, but hidden.replay.Hands stays
	// empty until the next-hand transition (so settle-time events have a
	// chance to land in hidden.current.Events first). Inspect hidden.current
	// for the fold marker instead.
	if hidden.current == nil {
		t.Fatalf("expected current replay hand to exist after AI fold fallback")
	}
	aiFolded := false
	for _, player := range hidden.current.Players {
		if player.Name == "AI 1" {
			aiFolded = player.Folded
		}
	}
	if !aiFolded {
		t.Fatalf("expected AI player to be marked folded in current hand")
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

// TestPrepareHandShortBigBlindAvoidsCheckDeadlock reproduces the production
// bug where a short-stacked BB seat ended up as the only contributor to the
// CurrentBet reference, leaving the SB seat with contribution > CurrentBet
// forever and dead-locking betting into an infinite check loop. The fix is
// to set CurrentBet = max(SB contribution, BB contribution).
func TestPrepareHandShortBigBlindAvoidsCheckDeadlock(t *testing.T) {
	players := []Player{
		{Seat: 0, Name: "Hero", Chips: 200, IsHuman: true},
		{Seat: 1, Name: "GPT", Chips: 200},
		// Seat 2 is intentionally left at chip == 5 so it gets picked as BB
		// and then forced to post only 5 of the nominal 20.
		{Seat: 2, Name: "ShortBB", Chips: 5},
	}
	updated, hand, _, _, err := prepareHand(players, 1, 0, 10, 20)
	if err != nil {
		t.Fatalf("prepareHand returned error: %v", err)
	}
	// Sanity: dealer=0, SB=1, BB=2. SB posts 10 fully; BB posts 5 (all-in).
	if hand.SmallBlindSeat != 1 || hand.BigBlindSeat != 2 {
		t.Fatalf("expected SB=1 BB=2, got SB=%d BB=%d", hand.SmallBlindSeat, hand.BigBlindSeat)
	}
	if hand.TotalContribution[1] != 10 {
		t.Fatalf("expected SB to post full 10, got %d", hand.TotalContribution[1])
	}
	if hand.TotalContribution[2] != 5 {
		t.Fatalf("expected BB to post partial 5, got %d", hand.TotalContribution[2])
	}
	if !hand.AllIn[2] {
		t.Fatalf("expected short BB seat to be marked all-in")
	}
	if hand.CurrentBet != 10 {
		t.Fatalf("expected CurrentBet to track SB amount (10) since it exceeds the partial BB (5), got %d", hand.CurrentBet)
	}

	// Now drive the betting round forward; if CurrentBet were the buggy 5,
	// every subsequent action loops back into a check deadlock. With the
	// fix, hero must call 10 to match SB, then SB checks, then the round
	// completes and we advance past preflop.
	snapshot := &Snapshot{Players: updated, SmallBlind: 10, BigBlind: 20}
	hidden := &hiddenState{hand: hand, current: &ReplayHand{HandNumber: 1, Players: []ReplayPlayerState{
		{Seat: 0, Name: "Hero", IsHuman: true, StartingChips: 200, EndingChips: 200, HoleCards: hand.HoleCards[0]},
		{Seat: 1, Name: "GPT", StartingChips: 200, EndingChips: 190, HoleCards: hand.HoleCards[1]},
		{Seat: 2, Name: "ShortBB", StartingChips: 5, EndingChips: 0, HoleCards: hand.HoleCards[2], AllIn: true},
	}}}

	// Hero (UTG, first to act preflop in 3-handed live) faces toCall = 10.
	heroSeat := hand.CurrentTurnSeat
	if heroSeat != 0 {
		t.Fatalf("expected first preflop actor to be hero seat 0, got %d", heroSeat)
	}
	toCall := hand.CurrentBet - hand.StreetContribution[heroSeat]
	if toCall != 10 {
		t.Fatalf("expected hero toCall=10 (full SB amount), got %d", toCall)
	}
	if _, err := applyActionToState(snapshot, hidden, heroSeat, "call", 10, "", ""); err != nil {
		t.Fatalf("hero call failed: %v", err)
	}
	// After hero calls, control should land on the SB seat with toCall=0
	// (already at 10) and a check available.
	if hand.CurrentTurnSeat != 1 {
		t.Fatalf("expected SB to act next, got seat %d", hand.CurrentTurnSeat)
	}
	if got := hand.CurrentBet - hand.StreetContribution[1]; got != 0 {
		t.Fatalf("expected SB toCall=0 after hero match, got %d", got)
	}
	if _, err := applyActionToState(snapshot, hidden, 1, "check", 0, "", ""); err != nil {
		t.Fatalf("SB check failed: %v", err)
	}
	// Round must be complete now and stage must move past preflop.
	if hand.Stage == "preflop" && !hand.HandOver {
		t.Fatalf("expected betting round to advance off preflop, still on preflop with HandOver=%v", hand.HandOver)
	}
}

// TestRunoutShowdownEventsLandInReplayHand reproduces the bug where a hand
// that ends via auto-runout (e.g. turn all-in -> river dealt + showdown) used
// to lose its tail events (river board_cards_dealt / showdown_revealed) in the
// stored replay. The fix is to defer pushing hidden.current onto
// hidden.replay.Hands until after publishPending has appended every settle-
// time event.
func TestRunoutShowdownEventsLandInReplayHand(t *testing.T) {
	snapshot := &Snapshot{
		ID:         "match-runout",
		SmallBlind: 10,
		BigBlind:   20,
		Players: []Player{
			{Seat: 0, Name: "你", Chips: 0, IsHuman: true},
			{Seat: 1, Name: "AI A", Chips: 200, PresetID: "ai-1"},
		},
	}
	hand := &handState{
		Number:             1,
		Stage:              "turn",
		DealerSeat:         0,
		SmallBlindSeat:     0,
		BigBlindSeat:       1,
		CurrentTurnSeat:    -1,
		Board:              []string{"5h", "Jd", "7c", "Jh"},
		Deck:               []string{"6d", "Qc", "Kh", "2s"},
		HoleCards:          map[int][]string{0: {"Ad", "Th"}, 1: {"As", "Ac"}},
		RevealedCards:      map[int][]string{},
		Folded:             map[int]bool{},
		AllIn:              map[int]bool{0: true, 1: true},
		StreetContribution: map[int]int{0: 0, 1: 0},
		TotalContribution:  map[int]int{0: 200, 1: 200},
		Acted:              map[int]bool{0: true, 1: true},
		CurrentBet:         0,
		MinRaiseSize:       20,
		Pot:                400,
	}
	current := &ReplayHand{
		HandNumber: 1,
		DealerSeat: 0,
		Players: []ReplayPlayerState{
			{Seat: 0, Name: "你", IsHuman: true, StartingChips: 200, EndingChips: 0, HoleCards: []string{"Ad", "Th"}},
			{Seat: 1, Name: "AI A", PresetID: "ai-1", StartingChips: 200, EndingChips: 200, HoleCards: []string{"As", "Ac"}},
		},
	}
	hidden := &hiddenState{hand: hand, current: current}

	// Simulate publishPending in-place: every event we collect gets appended
	// to hidden.current.Events the same way the real publishPending does.
	publish := func(pending []pendingEvent) {
		for i, ev := range pending {
			hidden.current.Events = append(hidden.current.Events, ReplayEvent{
				Sequence:   i + 1,
				Type:       ev.EventType,
				Visibility: ev.Visibility,
				Timestamp:  time.Now().UTC(),
				Payload:    ev.Payload,
			})
		}
	}

	// 1) Run the runout: turn -> river -> showdown.
	pending, err := advanceStreet(snapshot, hidden)
	if err != nil {
		t.Fatalf("advanceStreet returned error: %v", err)
	}
	publish(pending)

	// 2) Hand is now over; advancing state pushes it into replay.Hands.
	more, err := advanceState(snapshot, hidden)
	if err != nil {
		t.Fatalf("advanceState returned error: %v", err)
	}
	publish(more)

	if len(hidden.replay.Hands) != 1 {
		t.Fatalf("expected 1 hand pushed to replay, got %d", len(hidden.replay.Hands))
	}
	storedEvents := hidden.replay.Hands[0].Events
	have := map[string]bool{}
	for _, ev := range storedEvents {
		have[ev.Type] = true
	}
	for _, want := range []string{"board_cards_dealt", "showdown_revealed"} {
		if !have[want] {
			t.Fatalf("expected stored hand to contain %q event, got types %v", want, eventTypeList(storedEvents))
		}
	}

	// And the stored board really is 5 cards including the river.
	if len(hidden.replay.Hands[0].Board) != 5 {
		t.Fatalf("expected stored board of 5 cards, got %v", hidden.replay.Hands[0].Board)
	}
}

func eventTypeList(events []ReplayEvent) []string {
	out := make([]string, 0, len(events))
	for _, ev := range events {
		out = append(out, ev.Type)
	}
	return out
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
