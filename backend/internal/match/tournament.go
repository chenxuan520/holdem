package match

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
)

// tournament.go holds the pure, IO-free league math shared by both backends.
// CoreBuildSchedule turns a config into concrete per-table plans; the
// orchestrator (Go Service goroutine / CF TournamentDO alarm loop) creates a
// full-auto spectator match per plan and feeds the finished replays back into
// CoreAggregateStandings. Keeping both here (and exposing them through the
// wasm core) means the leaderboard math is byte-identical across the native Go
// and Cloudflare Workers backends, exactly like the Core* reducer funcs.

const (
	eloInitial = 1500.0
	eloK       = 32.0
)

// TournamentConfig is the user-supplied league setup consumed by
// CoreBuildSchedule. TableSize<=0 defaults to "everyone at one table"
// (min(pool, 6)); see CoreBuildSchedule for the scheduling rules.
type TournamentConfig struct {
	Name             string   `json:"name"`
	PresetIDs        []string `json:"presetIds"`
	TableSize        int      `json:"tableSize"`
	Rounds           int      `json:"rounds"`
	MaxHandsPerMatch int      `json:"maxHandsPerMatch"`
	InitialChips     int      `json:"initialChips"`
	SmallBlind       int      `json:"smallBlind"`
	BigBlind         int      `json:"bigBlind"`
	MaxConcurrency   int      `json:"maxConcurrency"`
	MaxMatches       int      `json:"maxMatches"`
	// Seed makes the partition shuffle (TableSize < pool only) reproducible.
	// 0 falls back to a fixed seed so a given config is deterministic.
	Seed int64 `json:"seed,omitempty"`
}

// MatchPlan is one scheduled table: the preset ids that will sit at it.
type MatchPlan struct {
	Round     int      `json:"round"`
	PresetIDs []string `json:"presetIds"`
}

// Standing is one row of the leaderboard. WinRate (夺冠率) is the headline;
// Rating is a composition-robust multiplayer Elo; the rest are supporting
// columns. WinRate / ErrorRate are fractions in [0,1] (the UI formats as %).
type Standing struct {
	PresetID     string  `json:"presetId"`
	Name         string  `json:"name"`
	Matches      int     `json:"matches"`
	Wins         int     `json:"wins"`
	WinRate      float64 `json:"winRate"`
	AvgPlacement float64 `json:"avgPlacement"`
	Rating       int     `json:"rating"`
	ChipDelta    int     `json:"chipDelta"`
	BB100        float64 `json:"bb100"`
	ErrorRate    float64 `json:"errorRate"`
	AvgAttempts  float64 `json:"avgAttempts"`
}

// StandingsInput is the aggregation payload. Names maps preset id -> display
// name (the orchestrator passes the canonical preset names); a missing entry
// falls back to the per-seat name found in the replay, then the id itself.
type StandingsInput struct {
	Replays []ReplayDetail    `json:"replays"`
	Names   map[string]string `json:"names,omitempty"`
}

// CoreBuildSchedule expands a config into per-table match plans.
//
//   - pool <= tableSize  -> "everyone at one table": one full table per round,
//     repeated Rounds times. Zero pairing bias, real multiway play.
//   - pool >  tableSize  -> each round, seed-shuffle the pool and split it into
//     evenly-sized tables (each 2..6), repeated Rounds times. Pairings are
//     approximately balanced across rounds.
//
// MaxMatches (>0) truncates the schedule as a hard cap.
func CoreBuildSchedule(cfg TournamentConfig) ([]MatchPlan, error) {
	pool := make([]string, 0, len(cfg.PresetIDs))
	for _, id := range cfg.PresetIDs {
		if strings.TrimSpace(id) != "" {
			pool = append(pool, id)
		}
	}
	if len(pool) < 2 {
		return nil, fmt.Errorf("tournament needs at least 2 presets")
	}

	size := cfg.TableSize
	if size <= 0 {
		size = len(pool)
	}
	if size < 2 {
		size = 2
	}
	if size > 6 {
		size = 6
	}
	rounds := cfg.Rounds
	if rounds < 1 {
		rounds = 1
	}

	plans := make([]MatchPlan, 0, rounds)
	if len(pool) <= size {
		for r := 1; r <= rounds; r++ {
			plans = append(plans, MatchPlan{Round: r, PresetIDs: append([]string(nil), pool...)})
		}
		return capPlans(plans, cfg.MaxMatches), nil
	}

	seed := cfg.Seed
	if seed == 0 {
		seed = 1
	}
	rng := rand.New(rand.NewSource(seed))
	for r := 1; r <= rounds; r++ {
		shuffled := append([]string(nil), pool...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		for _, table := range partitionPool(shuffled, size) {
			plans = append(plans, MatchPlan{Round: r, PresetIDs: table})
		}
	}
	return capPlans(plans, cfg.MaxMatches), nil
}

// partitionPool splits pool into evenly-sized tables of at most `size` seats,
// keeping every table within 2..6. It picks ceil(n/size) tables, then reduces
// the table count if that would leave a table with <2 seats, and distributes
// the remainder so sizes differ by at most one.
func partitionPool(pool []string, size int) [][]string {
	n := len(pool)
	tables := (n + size - 1) / size
	for tables > 1 && n/tables < 2 {
		tables--
	}
	if tables < 1 {
		tables = 1
	}
	base := n / tables
	extra := n % tables
	out := make([][]string, 0, tables)
	idx := 0
	for t := 0; t < tables; t++ {
		sz := base
		if t < extra {
			sz++
		}
		out = append(out, append([]string(nil), pool[idx:idx+sz]...))
		idx += sz
	}
	return out
}

func capPlans(plans []MatchPlan, maxMatches int) []MatchPlan {
	if maxMatches > 0 && len(plans) > maxMatches {
		return plans[:maxMatches]
	}
	return plans
}

// seatResult is one preset's outcome in a single match.
type seatResult struct {
	placement   int
	finalChips  int
	handsPlayed int
}

// CoreAggregateStandings folds finished/stopped replays into a leaderboard.
// Per match it derives a placement for every (non-human) seat from the final
// chip stacks + elimination order, then aggregates per preset id. Ratings are
// a multiplayer Elo folded over the matches in finish-time order, so the
// result is deterministic and can be recomputed from scratch any time.
func CoreAggregateStandings(in StandingsInput) []Standing {
	type acc struct {
		name       string
		matches    int
		wins       int
		placeSum   int
		chipDelta  int
		deltaBB    float64
		hands      int
		aiLogs     int
		aiErrors   int
		attemptSum int
	}
	stats := map[string]*acc{}
	order := make([]string, 0)
	get := func(pid string) *acc {
		a, ok := stats[pid]
		if !ok {
			a = &acc{}
			stats[pid] = a
			order = append(order, pid)
		}
		return a
	}

	// Sort replays chronologically so the path-dependent Elo fold is stable.
	replays := append([]ReplayDetail(nil), in.Replays...)
	sort.SliceStable(replays, func(i, j int) bool {
		ti, tj := replays[i].Summary.FinishedAt, replays[j].Summary.FinishedAt
		if ti.Equal(tj) {
			return replays[i].Summary.ID < replays[j].Summary.ID
		}
		return ti.Before(tj)
	})

	ratings := map[string]float64{}
	ratingOf := func(pid string) float64 {
		if v, ok := ratings[pid]; ok {
			return v
		}
		return eloInitial
	}

	for ri := range replays {
		replay := replays[ri]
		results := matchResults(replay)
		if len(results) == 0 {
			continue
		}
		bb := replay.Summary.BigBlind

		for pid, res := range results {
			a := get(pid)
			if a.name == "" {
				a.name = in.Names[pid]
			}
			a.matches++
			if res.placement == 1 {
				a.wins++
			}
			a.placeSum += res.placement
			delta := res.finalChips - replay.Summary.InitialChips
			a.chipDelta += delta
			if bb > 0 {
				a.deltaBB += float64(delta) / float64(bb)
			}
			a.hands += res.handsPlayed
		}

		seatPreset := map[int]string{}
		for _, p := range replay.Players {
			if p.IsHuman {
				continue
			}
			seatPreset[p.Seat] = p.PresetID
		}
		for _, log := range replay.AILogs {
			pid := seatPreset[log.Seat]
			if pid == "" {
				continue
			}
			a := get(pid)
			a.aiLogs++
			if strings.TrimSpace(log.Error) != "" {
				a.aiErrors++
			}
			att := log.AttemptCount
			if att < 1 {
				att = 1
			}
			a.attemptSum += att
		}

		applyEloMatch(ratings, ratingOf, results)
	}

	out := make([]Standing, 0, len(order))
	for _, pid := range order {
		a := stats[pid]
		name := a.name
		if name == "" {
			name = pid
		}
		s := Standing{
			PresetID:  pid,
			Name:      name,
			Matches:   a.matches,
			Wins:      a.wins,
			ChipDelta: a.chipDelta,
			Rating:    int(math.Round(ratingOf(pid))),
		}
		if a.matches > 0 {
			s.WinRate = float64(a.wins) / float64(a.matches)
			s.AvgPlacement = float64(a.placeSum) / float64(a.matches)
		}
		if a.hands > 0 {
			s.BB100 = a.deltaBB / float64(a.hands) * 100
		}
		if a.aiLogs > 0 {
			s.ErrorRate = float64(a.aiErrors) / float64(a.aiLogs)
			s.AvgAttempts = float64(a.attemptSum) / float64(a.aiLogs)
		}
		out = append(out, s)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].WinRate != out[j].WinRate {
			return out[i].WinRate > out[j].WinRate
		}
		if out[i].AvgPlacement != out[j].AvgPlacement {
			return out[i].AvgPlacement < out[j].AvgPlacement
		}
		if out[i].Rating != out[j].Rating {
			return out[i].Rating > out[j].Rating
		}
		if out[i].BB100 != out[j].BB100 {
			return out[i].BB100 > out[j].BB100
		}
		return out[i].PresetID < out[j].PresetID
	})
	return out
}

// matchResults derives each non-human preset's placement, final chips and
// hands played in a single match. Placement ranks by final chips desc, with
// later elimination (or never eliminated) breaking ties; equal keys share a
// placement (standard competition ranking). A preset appearing twice keeps its
// best (lowest) placement and the duplicate seat is ignored.
func matchResults(replay ReplayDetail) map[string]seatResult {
	if len(replay.Hands) == 0 {
		return nil
	}
	isHuman := map[int]bool{}
	seatPreset := map[int]string{}
	for _, p := range replay.Players {
		isHuman[p.Seat] = p.IsHuman
		seatPreset[p.Seat] = p.PresetID
	}

	finalChips := map[int]int{}
	lastHand := map[int]int{}
	handsPlayed := map[int]int{}
	elimAt := map[int]int{}

	for hi := range replay.Hands {
		hand := replay.Hands[hi]
		for _, ps := range hand.Players {
			seat := ps.Seat
			if isHuman[seat] {
				continue
			}
			handsPlayed[seat]++
			if hand.HandNumber >= lastHand[seat] {
				lastHand[seat] = hand.HandNumber
				finalChips[seat] = ps.EndingChips
			}
			if ps.Eliminated && elimAt[seat] == 0 {
				elimAt[seat] = hand.HandNumber
			}
		}
	}

	type entry struct {
		seat  int
		chips int
		elim  int
	}
	entries := make([]entry, 0, len(handsPlayed))
	for seat := range handsPlayed {
		elim := elimAt[seat]
		if elim == 0 {
			elim = math.MaxInt32
		}
		entries = append(entries, entry{seat: seat, chips: finalChips[seat], elim: elim})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].chips != entries[j].chips {
			return entries[i].chips > entries[j].chips
		}
		if entries[i].elim != entries[j].elim {
			return entries[i].elim > entries[j].elim
		}
		return entries[i].seat < entries[j].seat
	})

	results := map[string]seatResult{}
	place := 0
	for i, e := range entries {
		if i == 0 || e.chips != entries[i-1].chips || e.elim != entries[i-1].elim {
			place = i + 1
		}
		pid := seatPreset[e.seat]
		if pid == "" {
			continue
		}
		if _, ok := results[pid]; ok {
			continue
		}
		results[pid] = seatResult{placement: place, finalChips: e.chips, handsPlayed: handsPlayed[e.seat]}
	}
	return results
}

// applyEloMatch updates ratings from one match's placements. Each pair of
// presets contributes a standard Elo result (win/draw/loss by placement);
// per-preset deltas are computed against pre-match ratings and scaled by
// 1/(n-1) so a single table swings a rating by at most ~eloK regardless of
// table size. Deterministic: pids are processed in sorted order.
func applyEloMatch(ratings map[string]float64, ratingOf func(string) float64, results map[string]seatResult) {
	pids := make([]string, 0, len(results))
	for pid := range results {
		pids = append(pids, pid)
	}
	if len(pids) < 2 {
		return
	}
	sort.Strings(pids)

	pre := make(map[string]float64, len(pids))
	for _, pid := range pids {
		pre[pid] = ratingOf(pid)
	}
	n := len(pids)
	for _, a := range pids {
		var expected, actual float64
		for _, b := range pids {
			if a == b {
				continue
			}
			expected += 1.0 / (1.0 + math.Pow(10, (pre[b]-pre[a])/400.0))
			switch {
			case results[a].placement < results[b].placement:
				actual += 1.0
			case results[a].placement == results[b].placement:
				actual += 0.5
			}
		}
		ratings[a] = pre[a] + eloK*(actual-expected)/float64(n-1)
	}
}
