package match

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"holdem/backend/internal/config"
)

// alwaysShoveServer answers every decision with a valid all_in tool call so
// league matches reach a terminal state in a handful of hands (fast, no retry
// backoff). Even if the engine coerces an invalid shove to fold, the per-match
// hand cap still terminates the table.
func alwaysShoveServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"tool_calls":[{"function":{"name":"submit_action","arguments":"{\"action\":\"all_in\",\"public_reason\":\"shove\",\"private_reason\":\"t\"}"}}]}}]}`)
	}))
}

func TestCreateTournamentValidation(t *testing.T) {
	service := NewService([]config.Preset{
		{ID: "a", Name: "Alpha", Endpoint: "https://example.com", Token: "t", Model: "m"},
	}, nil)
	if _, err := service.CreateTournament(TournamentConfig{PresetIDs: []string{"a"}}); err == nil {
		t.Fatal("expected error for fewer than 2 presets")
	}
	if _, err := service.CreateTournament(TournamentConfig{PresetIDs: []string{"a", "ghost"}}); err == nil {
		t.Fatal("expected error for unknown preset id")
	}
}

func TestCreateTournamentRunsToCompletion(t *testing.T) {
	server := alwaysShoveServer()
	defer server.Close()

	presets := []config.Preset{
		{ID: "a", Name: "Alpha", Endpoint: server.URL, Token: "t", Model: "m"},
		{ID: "b", Name: "Bravo", Endpoint: server.URL, Token: "t", Model: "m"},
		{ID: "c", Name: "Cara", Endpoint: server.URL, Token: "t", Model: "m"},
	}
	service := NewService(presets, nil)

	detail, err := service.CreateTournament(TournamentConfig{
		Name:             "smoke",
		PresetIDs:        []string{"a", "b", "c"},
		Rounds:           2,
		InitialChips:     60,
		SmallBlind:       5,
		BigBlind:         10,
		MaxConcurrency:   2,
		MaxHandsPerMatch: 50,
	})
	if err != nil {
		t.Fatalf("CreateTournament: %v", err)
	}
	if detail.Status != "running" {
		t.Fatalf("expected running, got %q", detail.Status)
	}
	// 3 presets <= table size -> one full table per round, 2 rounds.
	if detail.MatchesTotal != 2 {
		t.Fatalf("expected 2 scheduled matches, got %d", detail.MatchesTotal)
	}

	deadline := time.Now().Add(30 * time.Second)
	var final TournamentDetail
	for time.Now().Before(deadline) {
		final, _ = service.GetTournament(detail.ID)
		if final.Status == "finished" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if final.Status != "finished" {
		t.Fatalf("tournament did not finish in time: status=%q done=%d/%d", final.Status, final.MatchesDone, final.MatchesTotal)
	}
	if final.MatchesDone != final.MatchesTotal {
		t.Fatalf("expected all %d matches done, got %d", final.MatchesTotal, final.MatchesDone)
	}
	if len(final.Standings) == 0 {
		t.Fatal("expected standings to be populated")
	}
	totalWins := 0
	for _, s := range final.Standings {
		totalWins += s.Wins
	}
	if totalWins < 1 {
		t.Fatalf("expected at least one recorded win across standings, got %d", totalWins)
	}
}

func TestControlTournamentStop(t *testing.T) {
	server := alwaysShoveServer()
	defer server.Close()

	presets := []config.Preset{
		{ID: "a", Name: "Alpha", Endpoint: server.URL, Token: "t", Model: "m"},
		{ID: "b", Name: "Bravo", Endpoint: server.URL, Token: "t", Model: "m"},
	}
	service := NewService(presets, nil)
	detail, err := service.CreateTournament(TournamentConfig{
		PresetIDs:        []string{"a", "b"},
		Rounds:           50,
		InitialChips:     200,
		SmallBlind:       5,
		BigBlind:         10,
		MaxConcurrency:   1,
		MaxHandsPerMatch: 200,
	})
	if err != nil {
		t.Fatalf("CreateTournament: %v", err)
	}

	stopped, err := service.ControlTournament(detail.ID, ControlRequest{Action: "stop"})
	if err != nil {
		t.Fatalf("ControlTournament stop: %v", err)
	}
	if stopped.Status != "stopped" {
		t.Fatalf("expected stopped status, got %q", stopped.Status)
	}

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		cur, _ := service.GetTournament(detail.ID)
		// terminal once the runner drains: still "stopped" and no longer
		// scheduling. MatchesDone should not exceed total.
		if cur.MatchesDone <= cur.MatchesTotal && !cur.FinishedAt.IsZero() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("stopped tournament did not finalize in time")
}
