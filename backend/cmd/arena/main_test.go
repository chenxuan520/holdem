package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"holdem/backend/internal/match"
)

func TestRunUsesAllPresetsAndCreatesWithoutWaiting(t *testing.T) {
	t.Setenv("HOLDEM_AUTH_PASSWORD", "secret")
	var received match.TournamentConfig
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Holdem-Password") != "secret" {
			t.Errorf("missing auth header")
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/presets":
			writeTestJSON(t, w, map[string]any{"presets": []map[string]string{{"id": "a"}, {"id": "b"}, {"id": "c"}}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/tournaments":
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Errorf("decode create request: %v", err)
				return
			}
			w.WriteHeader(http.StatusCreated)
			writeTestJSON(t, w, match.TournamentDetail{ID: "t1", Name: received.Name, Status: "running", MatchesTotal: 4})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"--api", server.URL, "--rounds", "4", "--wait=false"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if strings.Join(received.PresetIDs, ",") != "a,b,c" {
		t.Fatalf("preset IDs = %v, want all server presets", received.PresetIDs)
	}
	if received.Rounds != 4 {
		t.Fatalf("rounds = %d, want 4", received.Rounds)
	}
	if !strings.Contains(stdout.String(), "Tournament t1 started") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunWaitsAndPrintsStandings(t *testing.T) {
	t.Setenv("HOLDEM_AUTH_PASSWORD", "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/tournaments":
			w.WriteHeader(http.StatusCreated)
			writeTestJSON(t, w, match.TournamentDetail{ID: "t2", Name: "Rank", Status: "running", MatchesTotal: 1})
		case r.Method == http.MethodGet && r.URL.Path == "/api/tournaments/t2":
			writeTestJSON(t, w, match.TournamentDetail{
				ID:           "t2",
				Name:         "Rank",
				Status:       "finished",
				MatchesTotal: 1,
				MatchesDone:  1,
				Decisions:    12,
				Standings: []match.Standing{
					{Name: "Model A", WinRate: 1, Rating: 1516, AvgPlacement: 1, BB100: 25, ChipDelta: 100, Matches: 1},
					{Name: "Model B", WinRate: 0, Rating: 1484, AvgPlacement: 2, BB100: -25, ChipDelta: -100, ErrorRate: 0.5, Matches: 1},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"--api", server.URL, "--poll", "1ms", "a", "b"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, want := range []string{"Status: finished", "MODEL", "Model A", "100.0%", "1516"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout %q does not contain %q", stdout.String(), want)
		}
	}
	if !strings.Contains(stderr.String(), "[finished] 1/1 matches") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunReturnsAPIError(t *testing.T) {
	t.Setenv("HOLDEM_AUTH_PASSWORD", "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		writeTestJSON(t, w, map[string]string{"error": "unknown preset id"})
	}))
	defer server.Close()

	err := run(context.Background(), []string{"--api", server.URL, "--wait=false", "a", "missing"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "http 400: unknown preset id") || !strings.Contains(err.Error(), "creation outcome is unknown") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunHelpDoesNotCallAPI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"--help"}, &stdout, &stderr)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want flag.ErrHelp", err)
	}
	if !strings.Contains(stderr.String(), "Usage: arena") {
		t.Fatalf("help = %q", stderr.String())
	}
}

func TestParseOptionsRejectsUnsafeValues(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "too many rounds", args: []string{"--rounds", "501"}, want: "rounds must be"},
		{name: "invalid table size", args: []string{"--table-size", "1"}, want: "table size must be"},
		{name: "too many hands", args: []string{"--max-hands", "1001"}, want: "max hands must be"},
		{name: "too much concurrency", args: []string{"--concurrency", "7"}, want: "concurrency must be"},
		{name: "blind order", args: []string{"--small-blind", "20", "--big-blind", "10"}, want: "big blind must be"},
		{name: "short stack", args: []string{"--initial-chips", "20", "--big-blind", "20"}, want: "initial chips must be"},
		{name: "duplicate preset", args: []string{"a", "a"}, want: "duplicate preset ID"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseOptions(tt.args, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestAPIClientDoesNotForwardPasswordThroughRedirect(t *testing.T) {
	leaked := make(chan string, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked <- r.Header.Get("X-Holdem-Password")
		writeTestJSON(t, w, map[string]any{"presets": []any{}})
	}))
	defer target.Close()
	source := httptest.NewServer(http.RedirectHandler(target.URL, http.StatusTemporaryRedirect))
	defer source.Close()

	client := apiClient{baseURL: source.URL, password: "secret", httpClient: newHTTPClient()}
	err := client.doJSON(context.Background(), http.MethodGet, "/api/presets", nil, &map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "http 307") {
		t.Fatalf("error = %v, want redirect rejection", err)
	}
	select {
	case password := <-leaked:
		t.Fatalf("redirect target received password %q", password)
	default:
	}
}

func TestRunRejectsDuplicateAutoDiscoveredPresets(t *testing.T) {
	t.Setenv("HOLDEM_AUTH_PASSWORD", "")
	created := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/presets":
			writeTestJSON(t, w, map[string]any{"presets": []map[string]string{{"id": "a"}, {"id": "a"}}})
		case "/api/tournaments":
			created <- struct{}{}
			writeTestJSON(t, w, match.TournamentDetail{ID: "unexpected"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	err := run(context.Background(), []string{"--api", server.URL}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), `duplicate preset ID "a"`) {
		t.Fatalf("error = %v", err)
	}
	select {
	case <-created:
		t.Fatal("tournament was created with duplicate presets")
	default:
	}
}

func TestRunFinishesCreationThenStopsAfterCancellation(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
	}{
		{name: "wait", args: nil},
		{name: "no wait", args: []string{"--wait=false"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOLDEM_AUTH_PASSWORD", "")
			requestStarted := make(chan struct{})
			allowResponse := make(chan struct{})
			stopped := make(chan struct{}, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/api/tournaments":
					close(requestStarted)
					<-allowResponse
					w.WriteHeader(http.StatusCreated)
					writeTestJSON(t, w, match.TournamentDetail{ID: "t-cancel", Name: "Rank", Status: "running", MatchesTotal: 1})
				case r.Method == http.MethodPost && r.URL.Path == "/api/tournaments/t-cancel/control":
					stopped <- struct{}{}
					writeTestJSON(t, w, match.TournamentDetail{ID: "t-cancel", Status: "stopped"})
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			ctx, cancel := context.WithCancel(context.Background())
			result := make(chan error, 1)
			args := append([]string{"--api", server.URL}, tt.args...)
			args = append(args, "a", "b")
			go func() {
				result <- run(ctx, args, &bytes.Buffer{}, &bytes.Buffer{})
			}()
			<-requestStarted
			cancel()
			close(allowResponse)
			err := <-result
			if err == nil || !strings.Contains(err.Error(), "was stopped to avoid unattended AI usage") {
				t.Fatalf("error = %v", err)
			}
			select {
			case <-stopped:
			default:
				t.Fatal("stop endpoint was not called")
			}
		})
	}
}

func TestRunStopsTournamentWhenPollingFails(t *testing.T) {
	t.Setenv("HOLDEM_AUTH_PASSWORD", "")
	stopped := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/tournaments":
			w.WriteHeader(http.StatusCreated)
			writeTestJSON(t, w, match.TournamentDetail{ID: "t3", Name: "Rank", Status: "running", MatchesTotal: 1})
		case r.Method == http.MethodGet && r.URL.Path == "/api/tournaments/t3":
			w.WriteHeader(http.StatusServiceUnavailable)
			writeTestJSON(t, w, map[string]string{"error": "temporarily unavailable"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/tournaments/t3/control":
			stopped <- struct{}{}
			writeTestJSON(t, w, match.TournamentDetail{ID: "t3", Status: "stopped"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	err := run(context.Background(), []string{"--api", server.URL, "--poll", "1ms", "a", "b"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "was stopped to avoid unattended AI usage") {
		t.Fatalf("error = %v", err)
	}
	select {
	case <-stopped:
	default:
		t.Fatal("stop endpoint was not called")
	}
}

func TestTournamentResultErrorReportsFailedMatches(t *testing.T) {
	detail := match.TournamentDetail{
		ID:     "t4",
		Status: "finished",
		Matches: []match.TournamentMatch{
			{Status: "finished"},
			{Status: "failed", Error: "bad blinds"},
		},
	}
	err := tournamentResultError(detail)
	if err == nil || !strings.Contains(err.Error(), "1 failed matches: bad blinds") {
		t.Fatalf("error = %v", err)
	}
}

func writeTestJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("encode response: %v", err)
	}
}
