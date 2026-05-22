package match

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildPromptInputSkipsEliminatedPlayers(t *testing.T) {
	service := &Service{}
	snapshot := Snapshot{
		ID:         "match-1",
		SmallBlind: 1,
		BigBlind:   2,
		Players: []Player{
			{Seat: 0, Name: "Hero", Chips: 90, IsHuman: true},
			{Seat: 1, Name: "Busted AI", Chips: 0, PresetID: "ai-bust", Eliminated: true},
			{Seat: 2, Name: "Live AI", Chips: 110, PresetID: "ai-live"},
		},
	}
	hidden := &hiddenState{hand: &handState{
		Number:             3,
		Stage:              "turn",
		DealerSeat:         0,
		SmallBlindSeat:     0,
		BigBlindSeat:       2,
		Board:              []string{"Ah", "Kd", "7c", "2s"},
		HoleCards:          map[int][]string{2: {"Qc", "Qd"}},
		Folded:             map[int]bool{0: false, 1: true, 2: false},
		AllIn:              map[int]bool{0: false, 1: false, 2: false},
		StreetContribution: map[int]int{0: 20, 2: 20},
		TotalContribution:  map[int]int{0: 22, 2: 22},
		CurrentBet:         20,
		MinRaiseSize:       20,
		Pot:                60,
	}}

	input := service.buildPromptInput(snapshot, hidden, 2)
	if got := len(input.Players); got != 2 {
		t.Fatalf("expected 2 active players in prompt, got %d", got)
	}

	seenSeats := map[int]bool{}
	for _, item := range input.Players {
		player, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("expected player payload to be map[string]any, got %T", item)
		}
		seatValue, ok := player["seat"].(int)
		if !ok {
			t.Fatalf("expected seat to be int, got %T", player["seat"])
		}
		if _, exists := player["eliminated"]; exists {
			t.Fatalf("did not expect eliminated field in active player payload: %+v", player)
		}
		seenSeats[seatValue] = true
	}

	if seenSeats[1] {
		t.Fatalf("expected eliminated seat 1 to be omitted from prompt players: %+v", seenSeats)
	}
	if !seenSeats[0] || !seenSeats[2] {
		t.Fatalf("expected active seats 0 and 2 to remain in prompt players: %+v", seenSeats)
	}
}

func TestPositionLabelByDistanceCoversCommonTableSizes(t *testing.T) {
	cases := []struct {
		live     int
		distance int
		want     string
	}{
		{2, 0, "BTN/SB"},
		{2, 1, "BB"},
		{3, 0, "BTN"}, {3, 1, "SB"}, {3, 2, "BB"},
		{4, 0, "BTN"}, {4, 1, "SB"}, {4, 2, "BB"}, {4, 3, "UTG"},
		{5, 0, "BTN"}, {5, 1, "SB"}, {5, 2, "BB"}, {5, 3, "UTG"}, {5, 4, "CO"},
		{6, 0, "BTN"}, {6, 1, "SB"}, {6, 2, "BB"}, {6, 3, "UTG"}, {6, 4, "HJ"}, {6, 5, "CO"},
	}
	for _, tc := range cases {
		got := positionLabelByDistance(tc.live, tc.distance)
		if got != tc.want {
			t.Fatalf("positionLabelByDistance(%d,%d) = %q, want %q", tc.live, tc.distance, got, tc.want)
		}
	}
}

func TestPositionLabelForSeatRespectsLiveSeats(t *testing.T) {
	players := []Player{
		{Seat: 0, Chips: 100},
		{Seat: 1, Chips: 0, Eliminated: true},
		{Seat: 2, Chips: 100},
		{Seat: 3, Chips: 100},
	}
	// dealer at seat 2 → among live seats [0,2,3] BTN is seat 2,
	// SB seat 3, BB seat 0 (3-handed since seat 1 is eliminated).
	if got := positionLabelForSeat(players, 2, 2); got != "BTN" {
		t.Fatalf("expected dealer to be BTN, got %q", got)
	}
	if got := positionLabelForSeat(players, 2, 3); got != "SB" {
		t.Fatalf("expected seat 3 to be SB, got %q", got)
	}
	if got := positionLabelForSeat(players, 2, 0); got != "BB" {
		t.Fatalf("expected seat 0 to be BB, got %q", got)
	}
}

func TestEffectiveStackForSeatPicksTighterSide(t *testing.T) {
	players := []Player{
		{Seat: 0, Chips: 50},
		{Seat: 1, Chips: 200},
		{Seat: 2, Chips: 80},
	}
	hand := &handState{
		Folded: map[int]bool{},
		AllIn:  map[int]bool{},
	}
	// Hero is seat 0 with 50; deepest live opp is seat 1 with 200.
	if got := effectiveStackForSeat(players, hand, 0); got != 50 {
		t.Fatalf("expected effective stack 50 when hero is shorter, got %d", got)
	}
	// Hero is seat 1 with 200; deepest opp is seat 2 with 80.
	if got := effectiveStackForSeat(players, hand, 1); got != 80 {
		t.Fatalf("expected effective stack 80 capped by deepest opp, got %d", got)
	}
}

func TestEffectiveStackForSeatSkipsFoldedAndAllIn(t *testing.T) {
	players := []Player{
		{Seat: 0, Chips: 100},
		{Seat: 1, Chips: 200},
		{Seat: 2, Chips: 1500},
	}
	hand := &handState{
		Folded: map[int]bool{1: true},
		AllIn:  map[int]bool{2: true},
	}
	// Only seat 0 is "active" so effective stack equals own chips.
	if got := effectiveStackForSeat(players, hand, 0); got != 100 {
		t.Fatalf("expected effective stack to fall back to own stack when no live opps, got %d", got)
	}
}

func TestCompactActionLogUsesDenseFormat(t *testing.T) {
	hand := &handState{ActionLog: []ActionLog{
		{Seat: 0, Action: "post_small_blind", Amount: 1, Street: "preflop"},
		{Seat: 1, Action: "post_big_blind", Amount: 2, Street: "preflop"},
		{Seat: 2, Action: "raise", Amount: 6, Street: "preflop"},
		{Seat: 0, Action: "fold", Street: "preflop"},
		{Seat: 1, Action: "call", Amount: 4, Street: "preflop"},
		{Seat: 1, Action: "check", Street: "flop"},
		{Seat: 2, Action: "all_in", Amount: 80, Street: "flop"},
	}}
	got := compactActionLog(hand, 8)
	want := []string{
		"pre:0.sb1",
		"pre:1.bb2",
		"pre:2.r6",
		"pre:0.f",
		"pre:1.c4",
		"flop:1.x",
		"flop:2.A80",
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d entries, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entry %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCompactActionLogTrimsToLimit(t *testing.T) {
	hand := &handState{ActionLog: []ActionLog{
		{Seat: 0, Action: "fold", Street: "preflop"},
		{Seat: 1, Action: "fold", Street: "preflop"},
		{Seat: 2, Action: "raise", Amount: 6, Street: "preflop"},
		{Seat: 3, Action: "call", Amount: 6, Street: "preflop"},
		{Seat: 4, Action: "fold", Street: "preflop"},
	}}
	got := compactActionLog(hand, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3 trimmed entries, got %d", len(got))
	}
	if got[0] != "pre:2.r6" {
		t.Fatalf("expected oldest kept entry to be raise, got %q", got[0])
	}
	if got[2] != "pre:4.f" {
		t.Fatalf("expected newest kept entry to be fold, got %q", got[2])
	}
}

func TestLastAggressorSeatPicksMostRecentRaiserOrAllIn(t *testing.T) {
	hand := &handState{ActionLog: []ActionLog{
		{Seat: 2, Action: "raise", Amount: 6, Street: "preflop"},
		{Seat: 0, Action: "fold", Street: "preflop"},
		{Seat: 1, Action: "call", Amount: 4, Street: "preflop"},
		{Seat: 1, Action: "check", Street: "flop"},
		{Seat: 2, Action: "all_in", Amount: 80, Street: "flop"},
	}}
	seat := lastAggressorSeat(hand)
	if seat == nil {
		t.Fatalf("expected last aggressor to be present")
	}
	if *seat != 2 {
		t.Fatalf("expected seat 2 (all-in), got %d", *seat)
	}
}

func TestLastAggressorSeatNilWhenNoAggression(t *testing.T) {
	hand := &handState{ActionLog: []ActionLog{
		{Seat: 0, Action: "post_small_blind", Amount: 1, Street: "preflop"},
		{Seat: 1, Action: "post_big_blind", Amount: 2, Street: "preflop"},
		{Seat: 2, Action: "call", Amount: 2, Street: "preflop"},
		{Seat: 0, Action: "call", Amount: 1, Street: "preflop"},
		{Seat: 1, Action: "check", Street: "preflop"},
	}}
	if got := lastAggressorSeat(hand); got != nil {
		t.Fatalf("expected nil aggressor in a limped pot, got seat %d", *got)
	}
}

func TestBuildPromptInputComputesPotOddsAndEffectiveStack(t *testing.T) {
	service := &Service{}
	snapshot := Snapshot{
		ID:         "match-2",
		SmallBlind: 1,
		BigBlind:   2,
		Players: []Player{
			{Seat: 0, Name: "Hero", Chips: 80, IsHuman: true},
			{Seat: 1, Name: "Villain", Chips: 200, PresetID: "ai-1"},
		},
	}
	hidden := &hiddenState{hand: &handState{
		Number:             5,
		Stage:              "flop",
		DealerSeat:         0,
		SmallBlindSeat:     0,
		BigBlindSeat:       1,
		Board:              []string{"Ah", "9d", "2c"},
		HoleCards:          map[int][]string{1: {"Kc", "Kd"}},
		Folded:             map[int]bool{},
		AllIn:              map[int]bool{},
		StreetContribution: map[int]int{0: 0, 1: 20},
		TotalContribution:  map[int]int{0: 0, 1: 22},
		CurrentBet:         20,
		MinRaiseSize:       20,
		Pot:                40,
		ActionLog: []ActionLog{
			{Seat: 1, Action: "raise", Amount: 20, Street: "flop"},
		},
	}}

	input := service.buildPromptInput(snapshot, hidden, 1)

	if input.Position != "BB" {
		// 2-handed live, dealer at seat 0 → seat 1 is BB.
		t.Fatalf("expected position BB for seat 1 in HU, got %q", input.Position)
	}
	if input.SB != 1 || input.BB != 2 {
		t.Fatalf("expected blinds 1/2 to be propagated, got sb=%d bb=%d", input.SB, input.BB)
	}
	if input.EffBB != 40 {
		t.Fatalf("expected effective stack of 80 chips = 40bb, got %v", input.EffBB)
	}
	// Seat 1 already matches the bet so toCall should be 0 and PotOdds should be omitted (zero).
	if input.ToCall != 0 || input.PotOdds != 0 {
		t.Fatalf("expected no toCall + zero pot odds, got toCall=%d potOdds=%v", input.ToCall, input.PotOdds)
	}
	if input.LastAggressor == nil || *input.LastAggressor != 1 {
		t.Fatalf("expected last aggressor seat 1, got %+v", input.LastAggressor)
	}
	if len(input.Log) != 1 || input.Log[0] != "flop:1.r20" {
		t.Fatalf("expected compact log [flop:1.r20], got %v", input.Log)
	}
}

func TestBuildPromptInputPotOddsForCallingSpot(t *testing.T) {
	service := &Service{}
	snapshot := Snapshot{
		SmallBlind: 1,
		BigBlind:   2,
		Players: []Player{
			{Seat: 0, Name: "Hero", Chips: 100, IsHuman: true},
			{Seat: 1, Name: "Villain", Chips: 100, PresetID: "ai-1"},
		},
	}
	hidden := &hiddenState{hand: &handState{
		Number:             1,
		Stage:              "river",
		DealerSeat:         0,
		SmallBlindSeat:     0,
		BigBlindSeat:       1,
		Board:              []string{"Ah", "9d", "2c", "5s", "7h"},
		HoleCards:          map[int][]string{0: {"Kc", "Qd"}},
		Folded:             map[int]bool{},
		AllIn:              map[int]bool{},
		StreetContribution: map[int]int{0: 0, 1: 30},
		TotalContribution:  map[int]int{0: 50, 1: 80},
		CurrentBet:         30,
		MinRaiseSize:       30,
		Pot:                90, // hero owes 30 into a 90 pot ⇒ pot odds 30/120 = 0.25
		ActionLog: []ActionLog{
			{Seat: 1, Action: "raise", Amount: 30, Street: "river"},
		},
	}}

	input := service.buildPromptInput(snapshot, hidden, 0)
	if input.ToCall != 30 {
		t.Fatalf("expected toCall=30, got %d", input.ToCall)
	}
	if input.PotOdds != 0.25 {
		t.Fatalf("expected pot odds = 0.25 (30/120), got %v", input.PotOdds)
	}
	if input.MinRaiseTo != 60 {
		t.Fatalf("expected minRaiseTo = toCall+minRaiseSize = 60, got %d", input.MinRaiseTo)
	}
}

func TestBuildPromptInputJSONIsCompact(t *testing.T) {
	service := &Service{}
	snapshot := Snapshot{
		ID:         "match-compact",
		SmallBlind: 1,
		BigBlind:   2,
		Players: []Player{
			{Seat: 0, Name: "Hero", Chips: 90, IsHuman: true},
			{Seat: 1, Name: "AI", Chips: 110, PresetID: "ai-1"},
		},
	}
	hidden := &hiddenState{hand: &handState{
		Number:             1,
		Stage:              "preflop",
		DealerSeat:         0,
		SmallBlindSeat:     0,
		BigBlindSeat:       1,
		HoleCards:          map[int][]string{1: {"Ah", "As"}},
		Folded:             map[int]bool{},
		AllIn:              map[int]bool{},
		StreetContribution: map[int]int{0: 1, 1: 2},
		TotalContribution:  map[int]int{0: 1, 1: 2},
		CurrentBet:         2,
		MinRaiseSize:       2,
		Pot:                3,
	}}
	input := service.buildPromptInput(snapshot, hidden, 1)
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal prompt input: %v", err)
	}
	body := string(data)

	// The compact shape should NOT include any of the legacy verbose keys.
	for _, banned := range []string{"\"matchId\"", "\"playerName\"", "\"recentActionLog\"", "\"legalActions\"", "\"yourTotalBetThisHand\"", "\"label\""} {
		if strings.Contains(body, banned) {
			t.Fatalf("expected compact input to drop %s, got: %s", banned, body)
		}
	}
	// And it SHOULD include the new short keys.
	for _, expected := range []string{"\"hand\"", "\"stage\"", "\"position\"", "\"hole\"", "\"yourChips\"", "\"effBB\"", "\"sb\"", "\"bb\"", "\"actions\""} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected compact input to include %s, got: %s", expected, body)
		}
	}
}
