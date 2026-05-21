package match

import "testing"

func TestBuildPromptInputSkipsEliminatedPlayers(t *testing.T) {
	service := &Service{}
	snapshot := Snapshot{
		ID: "match-1",
		Players: []Player{
			{Seat: 0, Name: "Hero", Chips: 90, IsHuman: true},
			{Seat: 1, Name: "Busted AI", Chips: 0, PresetID: "ai-bust", Eliminated: true},
			{Seat: 2, Name: "Live AI", Chips: 110, PresetID: "ai-live"},
		},
	}
	hidden := &hiddenState{hand: &handState{
		Number:             3,
		Stage:              "turn",
		Board:              []string{"Ah", "Kd", "7c", "2s"},
		HoleCards:          map[int][]string{2: {"Qc", "Qd"}},
		Folded:             map[int]bool{0: false, 1: true, 2: false},
		AllIn:              map[int]bool{0: false, 1: false, 2: false},
		StreetContribution: map[int]int{0: 20, 2: 20},
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
