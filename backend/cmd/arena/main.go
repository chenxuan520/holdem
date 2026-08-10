package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"holdem/backend/internal/match"
)

const (
	defaultAPIURL  = "http://127.0.0.1:18130"
	maxRounds      = 500
	maxHands       = 1000
	maxMatches     = 500
	maxConcurrency = 6
)

type options struct {
	apiURL       string
	name         string
	tableSize    int
	rounds       int
	maxHands     int
	initialChips int
	smallBlind   int
	bigBlind     int
	concurrency  int
	maxMatches   int
	seed         int64
	wait         bool
	pollInterval time.Duration
	presetIDs    []string
}

type preset struct {
	ID string `json:"id"`
}

type apiClient struct {
	baseURL    string
	password   string
	httpClient *http.Client
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintf(os.Stderr, "arena: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	opts, err := parseOptions(args, stderr)
	if err != nil {
		return err
	}
	client := apiClient{
		baseURL:    strings.TrimRight(opts.apiURL, "/"),
		password:   os.Getenv("HOLDEM_AUTH_PASSWORD"),
		httpClient: newHTTPClient(),
	}

	if len(opts.presetIDs) == 0 {
		var payload struct {
			Presets []preset `json:"presets"`
		}
		if err := client.doJSON(ctx, http.MethodGet, "/api/presets", nil, &payload); err != nil {
			return fmt.Errorf("list presets: %w", err)
		}
		for _, item := range payload.Presets {
			if strings.TrimSpace(item.ID) != "" {
				opts.presetIDs = append(opts.presetIDs, item.ID)
			}
		}
	}
	if err := validatePresetIDs(opts.presetIDs); err != nil {
		return err
	}
	if len(opts.presetIDs) < 2 {
		return fmt.Errorf("at least 2 preset IDs are required")
	}

	cfg := match.TournamentConfig{
		Name:             opts.name,
		PresetIDs:        opts.presetIDs,
		TableSize:        opts.tableSize,
		Rounds:           opts.rounds,
		MaxHandsPerMatch: opts.maxHands,
		InitialChips:     opts.initialChips,
		SmallBlind:       opts.smallBlind,
		BigBlind:         opts.bigBlind,
		MaxConcurrency:   opts.concurrency,
		MaxMatches:       opts.maxMatches,
		Seed:             opts.seed,
	}
	var detail match.TournamentDetail
	if err := ctx.Err(); err != nil {
		return err
	}
	// Once creation starts, wait for the response even if the parent context is
	// cancelled. Recovering the tournament ID lets the wait path stop it instead
	// of leaving a paid run detached after Ctrl-C. The HTTP client still imposes
	// its own 30-second upper bound.
	if err := client.doJSON(context.WithoutCancel(ctx), http.MethodPost, "/api/tournaments", cfg, &detail); err != nil {
		return fmt.Errorf("create tournament: %w; creation outcome is unknown, check /api/tournaments before retrying", err)
	}
	fmt.Fprintf(stdout, "Tournament %s started: %s (%d matches)\n", detail.ID, detail.Name, detail.MatchesTotal)
	if !opts.wait {
		if err := ctx.Err(); err != nil {
			return stopAfterWaitFailure(client, detail.ID, fmt.Errorf("creation interrupted: %w", err))
		}
		return nil
	}

	lastProgress := ""
	for {
		progress := fmt.Sprintf("%s %d/%d", detail.Status, detail.MatchesDone, detail.MatchesTotal)
		if progress != lastProgress {
			fmt.Fprintf(stderr, "[%s] %d/%d matches, %d decisions\n", detail.Status, detail.MatchesDone, detail.MatchesTotal, detail.Decisions)
			lastProgress = progress
		}
		if tournamentDone(detail.Status) {
			printStandings(stdout, detail)
			return tournamentResultError(detail)
		}

		select {
		case <-ctx.Done():
			return stopAfterWaitFailure(client, detail.ID, fmt.Errorf("wait interrupted: %w", ctx.Err()))
		case <-time.After(opts.pollInterval):
		}
		if err := client.doJSON(ctx, http.MethodGet, "/api/tournaments/"+detail.ID, nil, &detail); err != nil {
			return stopAfterWaitFailure(client, detail.ID, fmt.Errorf("poll tournament: %w", err))
		}
	}
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	opts := options{}
	fs := flag.NewFlagSet("arena", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.apiURL, "api", envOrDefault("HOLDEM_API_URL", defaultAPIURL), "backend API base URL")
	fs.StringVar(&opts.name, "name", "CLI Arena", "tournament name")
	fs.IntVar(&opts.tableSize, "table-size", 0, "players per table; 0 puts everyone together up to 6")
	fs.IntVar(&opts.rounds, "rounds", 3, "number of schedule rounds")
	fs.IntVar(&opts.maxHands, "max-hands", 200, "maximum hands per match")
	fs.IntVar(&opts.initialChips, "initial-chips", 200, "initial chips per player")
	fs.IntVar(&opts.smallBlind, "small-blind", 10, "small blind")
	fs.IntVar(&opts.bigBlind, "big-blind", 20, "big blind")
	fs.IntVar(&opts.concurrency, "concurrency", 2, "maximum concurrent matches")
	fs.IntVar(&opts.maxMatches, "max-matches", 0, "schedule cap; 0 uses the server limit")
	fs.Int64Var(&opts.seed, "seed", 0, "schedule seed; 0 lets the server choose")
	fs.BoolVar(&opts.wait, "wait", true, "wait for completion and print the leaderboard")
	fs.DurationVar(&opts.pollInterval, "poll", 3*time.Second, "status polling interval")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: arena [flags] [preset-id ...]\n\nWith no preset IDs, every built-in preset returned by the server participates.\n\nFlags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if strings.TrimSpace(opts.apiURL) == "" {
		return options{}, fmt.Errorf("api URL cannot be empty")
	}
	if opts.wait && opts.pollInterval <= 0 {
		return options{}, fmt.Errorf("poll interval must be positive")
	}
	if opts.rounds < 1 || opts.rounds > maxRounds {
		return options{}, fmt.Errorf("rounds must be between 1 and %d", maxRounds)
	}
	if opts.tableSize != 0 && (opts.tableSize < 2 || opts.tableSize > 6) {
		return options{}, fmt.Errorf("table size must be 0 or between 2 and 6")
	}
	if opts.maxHands < 1 || opts.maxHands > maxHands {
		return options{}, fmt.Errorf("max hands must be between 1 and %d", maxHands)
	}
	if opts.concurrency < 1 || opts.concurrency > maxConcurrency {
		return options{}, fmt.Errorf("concurrency must be between 1 and %d", maxConcurrency)
	}
	if opts.maxMatches < 0 || opts.maxMatches > maxMatches {
		return options{}, fmt.Errorf("max matches must be between 0 and %d", maxMatches)
	}
	if opts.smallBlind <= 0 {
		return options{}, fmt.Errorf("small blind must be greater than 0")
	}
	if opts.bigBlind < opts.smallBlind {
		return options{}, fmt.Errorf("big blind must be greater than or equal to small blind")
	}
	if opts.initialChips <= opts.bigBlind {
		return options{}, fmt.Errorf("initial chips must be greater than big blind")
	}
	opts.presetIDs = fs.Args()
	if err := validatePresetIDs(opts.presetIDs); err != nil {
		return options{}, err
	}
	return opts, nil
}

func validatePresetIDs(ids []string) error {
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("preset ID cannot be empty")
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate preset ID %q", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (c apiClient) doJSON(ctx context.Context, method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.password != "" {
		req.Header.Set("X-Holdem-Password", c.password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		if readErr != nil {
			return fmt.Errorf("http %d", resp.StatusCode)
		}
		var payload struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &payload) == nil && payload.Error != "" {
			return fmt.Errorf("http %d: %s", resp.StatusCode, payload.Error)
		}
		message := strings.TrimSpace(string(data))
		if message == "" {
			return fmt.Errorf("http %d", resp.StatusCode)
		}
		return fmt.Errorf("http %d: %s", resp.StatusCode, message)
	}
	if output == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(output); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func tournamentDone(status string) bool {
	switch status {
	case "finished", "stopped", "interrupted", "failed":
		return true
	default:
		return false
	}
}

func stopAfterWaitFailure(client apiClient, tournamentID string, cause error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var detail match.TournamentDetail
	err := client.doJSON(ctx, http.MethodPost, "/api/tournaments/"+tournamentID+"/control", match.ControlRequest{Action: "stop"}, &detail)
	if err != nil {
		return fmt.Errorf("%w; tournament %s may still be running because stop failed: %v", cause, tournamentID, err)
	}
	return fmt.Errorf("%w; tournament %s was stopped to avoid unattended AI usage", cause, tournamentID)
}

func tournamentResultError(detail match.TournamentDetail) error {
	if detail.Status != "finished" {
		return fmt.Errorf("tournament %s ended with status %s", detail.ID, detail.Status)
	}
	if detail.Error != "" {
		return fmt.Errorf("tournament %s failed: %s", detail.ID, detail.Error)
	}
	failed := 0
	firstError := ""
	for _, item := range detail.Matches {
		if item.Status != "failed" {
			continue
		}
		failed++
		if firstError == "" {
			firstError = item.Error
		}
	}
	if failed > 0 {
		if firstError != "" {
			return fmt.Errorf("tournament %s finished with %d failed matches: %s", detail.ID, failed, firstError)
		}
		return fmt.Errorf("tournament %s finished with %d failed matches", detail.ID, failed)
	}
	return nil
}

func printStandings(output io.Writer, detail match.TournamentDetail) {
	fmt.Fprintf(output, "\nStatus: %s, %d/%d matches, %d decisions\n", detail.Status, detail.MatchesDone, detail.MatchesTotal, detail.Decisions)
	if len(detail.Standings) == 0 {
		fmt.Fprintln(output, "No standings available.")
		return
	}
	tw := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "#\tMODEL\tWIN\tELO\tAVG PLACE\tBB/100\tCHIPS\tERROR\tMATCHES")
	for i, standing := range detail.Standings {
		fmt.Fprintf(tw, "%d\t%s\t%.1f%%\t%d\t%.2f\t%.1f\t%+d\t%.1f%%\t%d\n",
			i+1,
			standing.Name,
			standing.WinRate*100,
			standing.Rating,
			standing.AvgPlacement,
			standing.BB100,
			standing.ChipDelta,
			standing.ErrorRate*100,
			standing.Matches,
		)
	}
	_ = tw.Flush()
}
