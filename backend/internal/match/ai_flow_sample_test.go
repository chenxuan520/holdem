package match

import (
	"encoding/json"
	"fmt"
	"testing"
)

// TestBuildPromptInputDumpsSampleSize is a non-failing smoke test that prints
// the actual JSON we send to the model for a representative 6-handed turn
// situation. It's gated behind `-v` so day-to-day runs stay quiet, but lets
// us spot-check the prompt length whenever we touch the prompt code.
func TestBuildPromptInputDumpsSampleSize(t *testing.T) {
	if !testing.Verbose() {
		t.Skip("only meaningful with -v")
	}

	service := &Service{}
	snapshot := Snapshot{
		ID:         "sample",
		SmallBlind: 1,
		BigBlind:   2,
		Players: []Player{
			{Seat: 0, Name: "GPT-5.4 A", Chips: 86, PresetID: "ai-1"},
			{Seat: 1, Name: "GPT-5.4 B", Chips: 152, PresetID: "ai-2"},
			{Seat: 2, Name: "GPT-5.4 C", Chips: 90, PresetID: "ai-3"},
			{Seat: 3, Name: "GPT-5.4 D", Chips: 198, PresetID: "ai-4"},
			{Seat: 4, Name: "GPT-5.4 E", Chips: 76, PresetID: "ai-5"},
			{Seat: 5, Name: "GPT-5.4 F", Chips: 198, PresetID: "ai-6"},
		},
	}
	hidden := &hiddenState{hand: &handState{
		Number:             7,
		Stage:              "turn",
		DealerSeat:         0,
		SmallBlindSeat:     1,
		BigBlindSeat:       2,
		Board:              []string{"As", "Kd", "7c", "5d"},
		HoleCards:          map[int][]string{3: {"Ah", "Qd"}},
		Folded:             map[int]bool{0: true, 4: true, 5: true},
		AllIn:              map[int]bool{},
		StreetContribution: map[int]int{1: 0, 2: 30, 3: 0},
		TotalContribution:  map[int]int{0: 0, 1: 1, 2: 32, 3: 2, 4: 0, 5: 0},
		CurrentBet:         30,
		MinRaiseSize:       30,
		Pot:                65,
		ActionLog: []ActionLog{
			{Seat: 1, Action: "post_small_blind", Amount: 1, Street: "preflop"},
			{Seat: 2, Action: "post_big_blind", Amount: 2, Street: "preflop"},
			{Seat: 3, Action: "raise", Amount: 6, Street: "preflop"},
			{Seat: 4, Action: "fold", Street: "preflop"},
			{Seat: 5, Action: "fold", Street: "preflop"},
			{Seat: 0, Action: "fold", Street: "preflop"},
			{Seat: 1, Action: "call", Amount: 5, Street: "preflop"},
			{Seat: 2, Action: "call", Amount: 4, Street: "preflop"},
			{Seat: 1, Action: "check", Street: "flop"},
			{Seat: 2, Action: "check", Street: "flop"},
			{Seat: 3, Action: "raise", Amount: 12, Street: "flop"},
			{Seat: 1, Action: "fold", Street: "flop"},
			{Seat: 2, Action: "call", Amount: 12, Street: "flop"},
			{Seat: 2, Action: "raise", Amount: 30, Street: "turn"},
		},
	}}

	input := service.buildPromptInput(snapshot, hidden, 3)
	data, _ := json.Marshal(input)
	fmt.Printf("[prompt-sample] user-content bytes=%d\n%s\n", len(data), string(data))
}
