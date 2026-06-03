package match

import (
	"math"
	"testing"
	"time"
)

func TestCoreBuildScheduleAllOnOneTable(t *testing.T) {
	cfg := TournamentConfig{PresetIDs: []string{"a", "b", "c", "d"}, TableSize: 6, Rounds: 3}
	plans, err := CoreBuildSchedule(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plans) != 3 {
		t.Fatalf("expected 3 full-table rounds, got %d", len(plans))
	}
	for _, p := range plans {
		if len(p.PresetIDs) != 4 {
			t.Fatalf("expected all 4 presets at one table, got %v", p.PresetIDs)
		}
	}
}

func TestCoreBuildScheduleDefaultsTableSizeToPool(t *testing.T) {
	// TableSize 0 -> min(pool, 6); 3 presets means one full table per round.
	plans, err := CoreBuildSchedule(TournamentConfig{PresetIDs: []string{"a", "b", "c"}, Rounds: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(plans))
	}
	if len(plans[0].PresetIDs) != 3 {
		t.Fatalf("expected full table of 3, got %v", plans[0].PresetIDs)
	}
}

func TestCoreBuildSchedulePartitionsLargePool(t *testing.T) {
	pool := []string{"a", "b", "c", "d", "e"}
	cfg := TournamentConfig{PresetIDs: pool, TableSize: 2, Rounds: 2, Seed: 7}
	plans, err := CoreBuildSchedule(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 5 presets / size 2 -> 2 tables per round (sizes 3,2); 2 rounds -> 4.
	if len(plans) != 4 {
		t.Fatalf("expected 4 tables, got %d", len(plans))
	}
	byRound := map[int][]string{}
	for _, p := range plans {
		if len(p.PresetIDs) < 2 || len(p.PresetIDs) > 6 {
			t.Fatalf("table size out of range: %v", p.PresetIDs)
		}
		byRound[p.Round] = append(byRound[p.Round], p.PresetIDs...)
	}
	for round, seats := range byRound {
		if len(seats) != len(pool) {
			t.Fatalf("round %d should cover all %d presets once, got %v", round, len(pool), seats)
		}
		seen := map[string]bool{}
		for _, id := range seats {
			if seen[id] {
				t.Fatalf("round %d seated %q twice", round, id)
			}
			seen[id] = true
		}
	}
}

func TestCoreBuildScheduleMaxMatchesCap(t *testing.T) {
	plans, err := CoreBuildSchedule(TournamentConfig{PresetIDs: []string{"a", "b"}, TableSize: 2, Rounds: 10, MaxMatches: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plans) != 3 {
		t.Fatalf("expected MaxMatches cap to 3, got %d", len(plans))
	}
}

func TestCoreBuildScheduleRejectsTinyPool(t *testing.T) {
	if _, err := CoreBuildSchedule(TournamentConfig{PresetIDs: []string{"a"}}); err == nil {
		t.Fatal("expected error for <2 presets")
	}
}

func psState(seat, chips int, eliminated bool) ReplayPlayerState {
	return ReplayPlayerState{Seat: seat, EndingChips: chips, Eliminated: eliminated}
}

// headsUp builds a 2-player replay where seat 0 busts seat 1 by hand 2.
func headsUp(id string, finishedAt time.Time, loserErrored bool) ReplayDetail {
	loserLog := AILog{Seat: 1, AttemptCount: 1}
	if loserErrored {
		loserLog.Error = "boom"
	}
	return ReplayDetail{
		Summary: ReplaySummary{ID: id, FinishedAt: finishedAt, BigBlind: 20, InitialChips: 200, HandsPlayed: 2},
		Players: []Player{{Seat: 0, Name: "Alpha", PresetID: "A"}, {Seat: 1, Name: "Bravo", PresetID: "B"}},
		Hands: []ReplayHand{
			{HandNumber: 1, Players: []ReplayPlayerState{psState(0, 210, false), psState(1, 190, false)}},
			{HandNumber: 2, Players: []ReplayPlayerState{psState(0, 400, false), psState(1, 0, true)}},
		},
		AILogs: []AILog{loserLog, {Seat: 0, AttemptCount: 1}},
	}
}

func findStanding(t *testing.T, standings []Standing, pid string) Standing {
	t.Helper()
	for _, s := range standings {
		if s.PresetID == pid {
			return s
		}
	}
	t.Fatalf("standing %q not found", pid)
	return Standing{}
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestCoreAggregateStandingsHeadsUp(t *testing.T) {
	base := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	in := StandingsInput{
		Replays: []ReplayDetail{
			headsUp("m1", base, true),             // B errors this match
			headsUp("m2", base.Add(time.Hour), false),
		},
		Names: map[string]string{"A": "Alpha", "B": "Bravo"},
	}
	standings := CoreAggregateStandings(in)
	if len(standings) != 2 {
		t.Fatalf("expected 2 standings, got %d", len(standings))
	}
	if standings[0].PresetID != "A" {
		t.Fatalf("expected winner A on top, got %q", standings[0].PresetID)
	}

	a := findStanding(t, standings, "A")
	if a.Name != "Alpha" || a.Matches != 2 || a.Wins != 2 {
		t.Fatalf("A aggregate wrong: %+v", a)
	}
	if !approx(a.WinRate, 1.0) || !approx(a.AvgPlacement, 1.0) {
		t.Fatalf("A winRate/place wrong: %+v", a)
	}
	if a.ChipDelta != 400 || !approx(a.BB100, 500) {
		t.Fatalf("A chips wrong: chipDelta=%d bb100=%v", a.ChipDelta, a.BB100)
	}
	if a.Rating <= 1500 {
		t.Fatalf("winner rating should rise above 1500, got %d", a.Rating)
	}
	if !approx(a.ErrorRate, 0) {
		t.Fatalf("A errorRate should be 0, got %v", a.ErrorRate)
	}

	b := findStanding(t, standings, "B")
	if !approx(b.WinRate, 0) || !approx(b.AvgPlacement, 2.0) {
		t.Fatalf("B winRate/place wrong: %+v", b)
	}
	if b.ChipDelta != -400 || !approx(b.BB100, -500) {
		t.Fatalf("B chips wrong: chipDelta=%d bb100=%v", b.ChipDelta, b.BB100)
	}
	if b.Rating >= 1500 || b.Rating >= a.Rating {
		t.Fatalf("loser rating should fall below winner/1500: A=%d B=%d", a.Rating, b.Rating)
	}
	if !approx(b.ErrorRate, 0.5) {
		t.Fatalf("B errorRate should be 0.5 (1 of 2 logs), got %v", b.ErrorRate)
	}
}

func TestCoreAggregateStandingsMultiwayPlacement(t *testing.T) {
	// 3-handed: seat0 wins, seat1 busts hand2, seat2 busts hand3 ->
	// placements A=1, C=2 (busted later), B=3 (busted earlier).
	replay := ReplayDetail{
		Summary: ReplaySummary{ID: "t1", FinishedAt: time.Now().UTC(), BigBlind: 20, InitialChips: 200, HandsPlayed: 3},
		Players: []Player{
			{Seat: 0, Name: "A", PresetID: "A"},
			{Seat: 1, Name: "B", PresetID: "B"},
			{Seat: 2, Name: "C", PresetID: "C"},
		},
		Hands: []ReplayHand{
			{HandNumber: 1, Players: []ReplayPlayerState{psState(0, 300, false), psState(1, 150, false), psState(2, 150, false)}},
			{HandNumber: 2, Players: []ReplayPlayerState{psState(0, 450, false), psState(1, 0, true), psState(2, 150, false)}},
			{HandNumber: 3, Players: []ReplayPlayerState{psState(0, 600, false), psState(2, 0, true)}},
		},
	}
	standings := CoreAggregateStandings(StandingsInput{Replays: []ReplayDetail{replay}})
	a := findStanding(t, standings, "A")
	b := findStanding(t, standings, "B")
	c := findStanding(t, standings, "C")
	if !approx(a.AvgPlacement, 1) || !approx(c.AvgPlacement, 2) || !approx(b.AvgPlacement, 3) {
		t.Fatalf("placements wrong: A=%v B=%v C=%v", a.AvgPlacement, b.AvgPlacement, c.AvgPlacement)
	}
	if !(a.Rating > c.Rating && c.Rating > b.Rating) {
		t.Fatalf("ratings should order A>C>B: A=%d C=%d B=%d", a.Rating, c.Rating, b.Rating)
	}
}
