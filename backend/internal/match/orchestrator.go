package match

import (
	"fmt"
	"sort"
	"time"
)

// orchestrator.go is the native-Go league runner. It reuses the existing
// full-auto spectator match engine: for every MatchPlan from CoreBuildSchedule
// it creates a {spectator, !semiAuto, !manual} match (which self-drives to
// completion on its own goroutine), polls it to terminal, enforces the
// per-match hand cap, then folds the finished replays into a leaderboard via
// CoreAggregateStandings. State lives in s.tournaments (guarded by s.mu) and
// is best-effort persisted through the optional tournamentStore.

const (
	tournamentPollInterval = 400 * time.Millisecond
	tournamentMatchTimeout = 12 * time.Minute
	tournamentStuckPolls   = 8 // consecutive non-running, non-terminal polls before we force-stop

	// Hard ceilings so a misconfigured league can't melt the token budget.
	maxTournamentMatches     = 500
	maxTournamentConcurrency = 6
	maxTournamentHands       = 1000

	defaultTournamentConcurrency = 2
	defaultTournamentHands       = 200
)

// tournamentState is one league's live state plus a stop latch the runner
// checks between/within matches.
type tournamentState struct {
	detail  TournamentDetail
	stopped bool
}

// TournamentMatch is one scheduled table inside a league.
type TournamentMatch struct {
	MatchID     string   `json:"matchId,omitempty"`
	Round       int      `json:"round"`
	PresetIDs   []string `json:"presetIds"`
	Status      string   `json:"status"` // pending|running|finished|stopped|failed
	WinnerName  string   `json:"winnerName,omitempty"`
	HandsPlayed int      `json:"handsPlayed"`
	Error       string   `json:"error,omitempty"`
}

// TournamentDetail is the full league record returned by the API and persisted
// to the store. Status is one of pending|running|stopped|finished|interrupted.
type TournamentDetail struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Status       string            `json:"status"`
	Config       TournamentConfig  `json:"config"`
	Matches      []TournamentMatch `json:"matches"`
	Standings    []Standing        `json:"standings"`
	MatchesTotal int               `json:"matchesTotal"`
	MatchesDone  int               `json:"matchesDone"`
	Decisions    int               `json:"decisions"`
	Error        string            `json:"error,omitempty"`
	CreatedAt    time.Time         `json:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
	FinishedAt   time.Time         `json:"finishedAt,omitempty"`
}

// tournamentStore is the optional persistence extension implemented by the
// SQLite store (matched structurally; test fakes simply don't implement it, so
// tournaments stay in-memory there). Kept separate from ReplayStore so adding
// it doesn't force every ReplayStore fake to grow methods.
type tournamentStore interface {
	SaveTournament(detail TournamentDetail) error
	ListTournaments() ([]TournamentDetail, error)
	DeleteTournament(id string) error
}

// CreateTournament validates + normalises the config, builds the schedule and
// launches the runner in the background, returning immediately (mirrors how
// CreateMatch kicks off autoplay and returns).
func (s *Service) CreateTournament(cfg TournamentConfig) (TournamentDetail, error) {
	cfg = s.normalizeTournamentConfig(cfg)
	if err := s.validateTournamentConfig(cfg); err != nil {
		return TournamentDetail{}, err
	}
	plans, err := CoreBuildSchedule(cfg)
	if err != nil {
		return TournamentDetail{}, err
	}
	if len(plans) == 0 {
		return TournamentDetail{}, fmt.Errorf("schedule is empty")
	}

	id, err := newID()
	if err != nil {
		return TournamentDetail{}, fmt.Errorf("generate tournament id: %w", err)
	}
	now := time.Now().UTC()
	matches := make([]TournamentMatch, len(plans))
	for i, plan := range plans {
		matches[i] = TournamentMatch{Round: plan.Round, PresetIDs: plan.PresetIDs, Status: "pending"}
	}
	detail := TournamentDetail{
		ID:           id,
		Name:         cfg.Name,
		Status:       "running",
		Config:       cfg,
		Matches:      matches,
		Standings:    []Standing{},
		MatchesTotal: len(plans),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	s.mu.Lock()
	s.tournaments[id] = &tournamentState{detail: detail}
	s.mu.Unlock()
	s.persistTournament(detail)

	go s.runTournament(id, plans, cfg)
	return cloneTournamentDetail(detail), nil
}

// normalizeTournamentConfig fills defaults and clamps every knob to a safe
// range so a bad request can't schedule an unbounded run.
func (s *Service) normalizeTournamentConfig(cfg TournamentConfig) TournamentConfig {
	if cfg.InitialChips <= 0 {
		cfg.InitialChips = 200
	}
	if cfg.SmallBlind <= 0 {
		cfg.SmallBlind = 10
	}
	if cfg.BigBlind <= 0 {
		cfg.BigBlind = 20
	}
	if cfg.Rounds < 1 {
		cfg.Rounds = 1
	}
	if cfg.MaxConcurrency < 1 {
		cfg.MaxConcurrency = defaultTournamentConcurrency
	}
	if cfg.MaxConcurrency > maxTournamentConcurrency {
		cfg.MaxConcurrency = maxTournamentConcurrency
	}
	if cfg.MaxHandsPerMatch <= 0 {
		cfg.MaxHandsPerMatch = defaultTournamentHands
	}
	if cfg.MaxHandsPerMatch > maxTournamentHands {
		cfg.MaxHandsPerMatch = maxTournamentHands
	}
	if cfg.MaxMatches <= 0 || cfg.MaxMatches > maxTournamentMatches {
		cfg.MaxMatches = maxTournamentMatches
	}
	if cfg.TableSize < 0 {
		cfg.TableSize = 0
	}
	if cfg.TableSize > 6 {
		cfg.TableSize = 6
	}
	if cfg.Seed == 0 {
		cfg.Seed = time.Now().UnixNano()
	}
	return cfg
}

func (s *Service) validateTournamentConfig(cfg TournamentConfig) error {
	if len(cfg.PresetIDs) < 2 {
		return fmt.Errorf("tournament needs at least 2 presets")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, pid := range cfg.PresetIDs {
		if _, ok := s.presets[pid]; !ok {
			return fmt.Errorf("unknown preset id %q", pid)
		}
	}
	return nil
}

// runTournament drives the whole schedule with a bounded worker pool. The
// unbuffered jobs channel provides natural backpressure: the producer blocks
// until a worker is free, so at most MaxConcurrency matches run at once.
func (s *Service) runTournament(id string, plans []MatchPlan, cfg TournamentConfig) {
	jobs := make(chan int)
	done := make(chan struct{})
	workers := cfg.MaxConcurrency
	if workers < 1 {
		workers = 1
	}
	for w := 0; w < workers; w++ {
		go func() {
			for idx := range jobs {
				if s.tournamentStopped(id) {
					s.updateTournament(id, func(d *TournamentDetail) {
						if d.Matches[idx].Status == "pending" {
							d.Matches[idx].Status = "stopped"
						}
					})
					continue
				}
				s.runOneTournamentMatch(id, idx, plans[idx], cfg)
			}
			done <- struct{}{}
		}()
	}
	for idx := range plans {
		jobs <- idx
	}
	close(jobs)
	for w := 0; w < workers; w++ {
		<-done
	}
	s.finalizeTournament(id)
}

// runOneTournamentMatch creates a full-auto spectator match for the plan and
// polls it to a terminal state, enforcing the per-match hand cap and a
// wall-clock + stuck-detection timeout, then records the result.
func (s *Service) runOneTournamentMatch(id string, idx int, plan MatchPlan, cfg TournamentConfig) {
	snapshot, err := s.CreateMatch(CreateRequest{
		InitialChips:  cfg.InitialChips,
		SmallBlind:    cfg.SmallBlind,
		BigBlind:      cfg.BigBlind,
		AIPresetIDs:   plan.PresetIDs,
		SpectatorMode: true,
	})
	if err != nil {
		s.updateTournament(id, func(d *TournamentDetail) {
			d.Matches[idx].Status = "failed"
			d.Matches[idx].Error = err.Error()
		})
		return
	}
	matchID := snapshot.ID
	s.updateTournament(id, func(d *TournamentDetail) {
		d.Matches[idx].MatchID = matchID
		d.Matches[idx].Status = "running"
	})

	deadline := time.Now().Add(tournamentMatchTimeout)
	stuck := 0
	for {
		snap, ok := s.GetMatch(matchID)
		if !ok {
			s.updateTournament(id, func(d *TournamentDetail) {
				d.Matches[idx].Status = "failed"
				d.Matches[idx].Error = "match disappeared"
			})
			return
		}
		if snap.Status == "finished" || snap.Status == "stopped" {
			break
		}
		switch {
		case s.tournamentStopped(id):
			_, _ = s.ControlMatch(matchID, ControlRequest{Action: "stop"})
		case cfg.MaxHandsPerMatch > 0 && snap.Table.CompletedHands >= cfg.MaxHandsPerMatch:
			_, _ = s.ControlMatch(matchID, ControlRequest{Action: "stop"})
		case time.Now().After(deadline):
			_, _ = s.ControlMatch(matchID, ControlRequest{Action: "stop"})
		case !snap.Control.Running:
			// Autoplay died without reaching a terminal state (e.g. the
			// per-match AI fuse tripped). Force-stop after it stays stuck.
			if stuck++; stuck >= tournamentStuckPolls {
				_, _ = s.ControlMatch(matchID, ControlRequest{Action: "stop"})
			}
		default:
			stuck = 0
		}
		time.Sleep(tournamentPollInterval)
	}
	s.completeTournamentMatch(id, idx, matchID)
}

func (s *Service) completeTournamentMatch(id string, idx int, matchID string) {
	snap, _ := s.GetMatch(matchID)
	status := snap.Status
	if status != "finished" && status != "stopped" {
		status = "stopped"
	}
	winner := snap.WinnerName
	hands := snap.Table.CompletedHands
	if replay, ok := s.GetReplay(matchID); ok {
		if replay.Summary.WinnerName != "" {
			winner = replay.Summary.WinnerName
		}
		if replay.Summary.HandsPlayed > 0 {
			hands = replay.Summary.HandsPlayed
		}
	}
	s.updateTournament(id, func(d *TournamentDetail) {
		d.Matches[idx].Status = status
		d.Matches[idx].WinnerName = winner
		d.Matches[idx].HandsPlayed = hands
	})
	s.recomputeStandings(id)
}

// recomputeStandings rebuilds the leaderboard from every terminal match's
// replay. Cheap to redo each completion for the league sizes we run, and keeps
// the path-dependent Elo deterministic (CoreAggregateStandings re-sorts by
// finish time internally).
func (s *Service) recomputeStandings(id string) {
	s.mu.RLock()
	st, ok := s.tournaments[id]
	if !ok {
		s.mu.RUnlock()
		return
	}
	matchIDs := make([]string, 0, len(st.detail.Matches))
	for _, m := range st.detail.Matches {
		if m.MatchID != "" && (m.Status == "finished" || m.Status == "stopped") {
			matchIDs = append(matchIDs, m.MatchID)
		}
	}
	names := make(map[string]string, len(st.detail.Config.PresetIDs))
	for _, pid := range st.detail.Config.PresetIDs {
		if preset, ok := s.presets[pid]; ok {
			names[pid] = preset.Name
		}
	}
	s.mu.RUnlock()

	replays := make([]ReplayDetail, 0, len(matchIDs))
	decisions := 0
	for _, mid := range matchIDs {
		if replay, ok := s.GetReplay(mid); ok {
			replays = append(replays, replay)
			decisions += len(replay.AILogs)
		}
	}
	standings := CoreAggregateStandings(StandingsInput{Replays: replays, Names: names})
	s.updateTournament(id, func(d *TournamentDetail) {
		d.Standings = standings
		d.Decisions = decisions
	})
}

func (s *Service) finalizeTournament(id string) {
	s.recomputeStandings(id)
	s.mu.RLock()
	st, ok := s.tournaments[id]
	stopped := ok && st.stopped
	s.mu.RUnlock()
	s.updateTournament(id, func(d *TournamentDetail) {
		d.FinishedAt = time.Now().UTC()
		if stopped {
			d.Status = "stopped"
		} else {
			d.Status = "finished"
		}
	})
}

// updateTournament applies fn under the lock, recomputes the done counter and
// UpdatedAt, then persists a clone outside the lock. Returns false if the
// tournament was deleted out from under the runner.
func (s *Service) updateTournament(id string, fn func(*TournamentDetail)) (TournamentDetail, bool) {
	s.mu.Lock()
	st, ok := s.tournaments[id]
	if !ok {
		s.mu.Unlock()
		return TournamentDetail{}, false
	}
	fn(&st.detail)
	done := 0
	for _, m := range st.detail.Matches {
		switch m.Status {
		case "finished", "stopped", "failed":
			done++
		}
	}
	st.detail.MatchesDone = done
	st.detail.UpdatedAt = time.Now().UTC()
	cp := cloneTournamentDetail(st.detail)
	s.mu.Unlock()
	s.persistTournament(cp)
	return cp, true
}

func (s *Service) tournamentStopped(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.tournaments[id]
	return ok && st.stopped
}

func (s *Service) persistTournament(detail TournamentDetail) {
	if ts, ok := s.store.(tournamentStore); ok {
		_ = ts.SaveTournament(detail)
	}
}

// ListTournaments returns league summaries, most-recently-updated first.
func (s *Service) ListTournaments() []TournamentDetail {
	s.mu.RLock()
	out := make([]TournamentDetail, 0, len(s.tournaments))
	for _, st := range s.tournaments {
		out = append(out, cloneTournamentDetail(st.detail))
	}
	s.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

func (s *Service) GetTournament(id string) (TournamentDetail, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.tournaments[id]
	if !ok {
		return TournamentDetail{}, false
	}
	return cloneTournamentDetail(st.detail), true
}

// ControlTournament currently supports only "stop": it latches the stop flag
// so the runner halts scheduling and force-stops in-flight matches.
func (s *Service) ControlTournament(id string, req ControlRequest) (TournamentDetail, error) {
	if req.Action != "stop" {
		return TournamentDetail{}, fmt.Errorf("unsupported tournament control action %q", req.Action)
	}
	s.mu.Lock()
	st, ok := s.tournaments[id]
	if !ok {
		s.mu.Unlock()
		return TournamentDetail{}, fmt.Errorf("tournament not found")
	}
	st.stopped = true
	if st.detail.Status == "running" || st.detail.Status == "pending" {
		st.detail.Status = "stopped"
	}
	cp := cloneTournamentDetail(st.detail)
	s.mu.Unlock()
	s.persistTournament(cp)
	return cp, nil
}

func (s *Service) DeleteTournament(id string) error {
	s.mu.Lock()
	st, ok := s.tournaments[id]
	if ok {
		st.stopped = true
		delete(s.tournaments, id)
	}
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("tournament not found")
	}
	if ts, sok := s.store.(tournamentStore); sok {
		if err := ts.DeleteTournament(id); err != nil {
			return fmt.Errorf("delete tournament from store: %w", err)
		}
	}
	return nil
}

func cloneTournamentDetail(d TournamentDetail) TournamentDetail {
	cp := d
	// Use make+copy (not append to a nil slice) so empty slices stay non-nil
	// and marshal to JSON `[]` rather than `null` — the frontend spreads these
	// arrays, and `[...null]` throws.
	cp.Config.PresetIDs = make([]string, len(d.Config.PresetIDs))
	copy(cp.Config.PresetIDs, d.Config.PresetIDs)
	cp.Matches = make([]TournamentMatch, len(d.Matches))
	for i, m := range d.Matches {
		ids := make([]string, len(m.PresetIDs))
		copy(ids, m.PresetIDs)
		m.PresetIDs = ids
		cp.Matches[i] = m
	}
	cp.Standings = make([]Standing, len(d.Standings))
	copy(cp.Standings, d.Standings)
	return cp
}
