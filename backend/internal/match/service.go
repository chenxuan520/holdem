package match

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"holdem/backend/internal/ai"
	"holdem/backend/internal/config"
)

type CreateRequest struct {
	InitialChips int `json:"initialChips"`
	SmallBlind   int `json:"smallBlind"`
	BigBlind     int `json:"bigBlind"`
	// AIPresetIDs lists AI seats in order. Each entry is either:
	//   - an id from `/api/presets` (built-in, loaded from
	//     config/ai-presets.yaml at startup), or
	//   - the marker "@inline:N" referencing AIInlinePresets[N], which
	//     is registered as an ephemeral per-match preset on CreateMatch.
	AIPresetIDs []string `json:"aiPresetIds"`
	// AIInlinePresets is the body of any `@inline:N` markers referenced
	// from AIPresetIDs. Tokens land here straight from the user's
	// browser; they do NOT get persisted to disk and aren't visible via
	// `/api/presets`. Each one gets a generated `inline-<id>` so AI flow
	// can look them up the same way as built-in presets during the
	// match's lifetime.
	//
	// We use config.InlinePresetInput here (not config.Preset) because
	// the latter has Token sealed under `json:"-"` to prevent any
	// outbound leak; the input variant is the explicit, audited entry
	// point for tokens coming in from the network.
	AIInlinePresets []config.InlinePresetInput `json:"aiInlinePresets,omitempty"`
	AIPlayerNames   []string        `json:"aiPlayerNames,omitempty"`
	HumanName       string          `json:"humanName,omitempty"`
	SpectatorMode   bool            `json:"spectatorMode"`
	SemiAutoMode    bool            `json:"semiAutoMode"`
	ManualMode      bool            `json:"manualMode"`
}

type Snapshot struct {
	ID           string       `json:"id"`
	Status       string       `json:"status"`
	InitialChips int          `json:"initialChips"`
	SmallBlind   int          `json:"smallBlind"`
	BigBlind     int          `json:"bigBlind"`
	Players      []Player     `json:"players"`
	Table        TableState   `json:"table"`
	Control      ControlState `json:"control"`
	CreatedAt    time.Time    `json:"createdAt"`
	UpdatedAt    time.Time    `json:"updatedAt"`
	WinnerName   string       `json:"winnerName,omitempty"`
	Warning      string       `json:"warning,omitempty"`
	LastEvent    *StreamEvent `json:"lastEvent,omitempty"`
}

type RecordSummary struct {
	ID                string    `json:"id"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
	FinishedAt        time.Time `json:"finishedAt,omitempty"`
	WinnerName        string    `json:"winnerName,omitempty"`
	PlayerCount       int       `json:"playerCount"`
	HandsPlayed       int       `json:"handsPlayed"`
	InitialChips      int       `json:"initialChips"`
	SmallBlind        int       `json:"smallBlind"`
	BigBlind          int       `json:"bigBlind"`
	SpectatorMode     bool      `json:"spectatorMode"`
	ContinueAvailable bool      `json:"continueAvailable"`
	ReplayAvailable   bool      `json:"replayAvailable"`
}

type ControlState struct {
	SpectatorMode bool `json:"spectatorMode"`
	SemiAutoMode  bool `json:"semiAutoMode"`
	Paused        bool `json:"paused"`
	ManualMode    bool `json:"manualMode"`
	CanStep       bool `json:"canStep"`
	Stopped       bool `json:"stopped"`
	Running       bool `json:"running"`
}

type Player struct {
	Seat       int    `json:"seat"`
	Name       string `json:"name"`
	Chips      int    `json:"chips"`
	IsHuman    bool   `json:"isHuman"`
	PresetID   string `json:"presetId,omitempty"`
	Eliminated bool   `json:"eliminated"`
}

type StreamEvent struct {
	Type       string    `json:"type"`
	Sequence   int       `json:"sequence"`
	Timestamp  time.Time `json:"timestamp"`
	Visibility string    `json:"visibility,omitempty"`
	Payload    any       `json:"payload"`
}

type Service struct {
	mu          sync.RWMutex
	presets     map[string]config.Preset
	matches     map[string]Snapshot
	hidden      map[string]*hiddenState
	replays     map[string]ReplayDetail
	subscribers map[string]map[chan StreamEvent]struct{}
	sequences   map[string]int
	ai          *ai.Client
	store       ReplayStore
	autoplaying map[string]bool
	tournaments map[string]*tournamentState
}

func NewService(presets []config.Preset, replayStore ReplayStore) *Service {
	presetMap := make(map[string]config.Preset, len(presets))
	for _, preset := range presets {
		presetMap[preset.ID] = preset
	}

	matches := map[string]Snapshot{}
	hidden := map[string]*hiddenState{}
	sequences := map[string]int{}
	replays := map[string]ReplayDetail{}
	tournaments := map[string]*tournamentState{}
	if replayStore != nil {
		if activeMatches, err := replayStore.ListActiveMatches(); err == nil {
			for _, record := range activeMatches {
				snapshot := record.Snapshot
				snapshot.Control.Running = false
				if snapshot.Control.SpectatorMode && snapshot.Status == "awaiting_ai" {
					snapshot.Control.Paused = true
				}
				hydrateLoadedMatch(&snapshot, &record)
				matches[snapshot.ID] = snapshot
				hidden[snapshot.ID] = &hiddenState{
					hand:          record.Hand,
					replay:        record.Replay,
					current:       record.Current,
					decisionTrail: record.DecisionTrail,
				}
				sequences[snapshot.ID] = maxSequenceForSnapshot(snapshot, record)
			}
		}
		if summaries, err := replayStore.ListReplays(); err == nil {
			for _, summary := range summaries {
				if replay, ok, err := replayStore.GetReplay(summary.ID); err == nil && ok {
					replays[summary.ID] = replay
				}
			}
		}
		// Tournaments are persisted via an optional store extension (the
		// SQLite store implements it; test fakes don't). In-flight leagues
		// don't auto-resume: their matches are loaded with Running=false and
		// nothing re-arms autoplay, so mark them interrupted.
		if ts, ok := replayStore.(tournamentStore); ok {
			if list, err := ts.ListTournaments(); err == nil {
				for _, detail := range list {
					if detail.Status == "running" || detail.Status == "pending" {
						detail.Status = "interrupted"
					}
					tournaments[detail.ID] = &tournamentState{detail: detail, stopped: true}
				}
			}
		}
	}

	return &Service{
		presets:     presetMap,
		matches:     matches,
		hidden:      hidden,
		replays:     replays,
		subscribers: map[string]map[chan StreamEvent]struct{}{},
		sequences:   sequences,
		ai:          ai.NewClient(),
		store:       replayStore,
		autoplaying: map[string]bool{},
		tournaments: tournaments,
	}
}

func hydrateLoadedMatch(snapshot *Snapshot, record *ActiveMatchRecord) {
	if snapshot == nil || record == nil || record.Hand == nil {
		return
	}
	if len(snapshot.Table.LastWinners) == 0 && record.Current != nil && len(record.Current.Winners) > 0 {
		snapshot.Table.LastWinners = winnerNames(record.Current.Winners)
	}
	if snapshot.Status == "hand_complete" {
		record.Hand.RevealedCards = rebuildRevealedCardsFromReplay(snapshot.Players, record.Current)
	}
	tempHidden := &hiddenState{
		hand:          record.Hand,
		replay:        record.Replay,
		current:       record.Current,
		decisionTrail: record.DecisionTrail,
	}
	rebuildSnapshotTable(snapshot, tempHidden)
}

func rebuildRevealedCardsFromReplay(players []Player, current *ReplayHand) map[int][]string {
	if current == nil || !hasShowdown(current) {
		return map[int][]string{}
	}
	revealed := map[int][]string{}
	for _, player := range current.Players {
		if player.Folded || len(player.HoleCards) == 0 {
			continue
		}
		revealed[player.Seat] = cloneStrings(player.HoleCards)
	}
	return revealed
}

func hasShowdown(current *ReplayHand) bool {
	if current == nil {
		return false
	}
	for _, winner := range current.Winners {
		if winner.HandLabel != "无需摊牌" {
			return true
		}
	}
	return false
}

func (s *Service) CreateMatch(req CreateRequest) (Snapshot, error) {
	if err := validateCreateRequest(req); err != nil {
		return Snapshot{}, err
	}

	// Resolve any "@inline:N" markers in AIPresetIDs into real preset
	// IDs registered in s.presets. Mutates req in-place; returns an
	// error if any inline preset is malformed.
	if err := s.registerInlinePresets(&req); err != nil {
		return Snapshot{}, err
	}

	players, err := s.buildPlayers(req)
	if err != nil {
		return Snapshot{}, err
	}
	players, hand, table, replayHand, err := prepareHand(players, 1, initialDealerSeat(len(players)), req.SmallBlind, req.BigBlind)
	if err != nil {
		return Snapshot{}, fmt.Errorf("prepare hand: %w", err)
	}

	id, err := newID()
	if err != nil {
		return Snapshot{}, fmt.Errorf("generate match id: %w", err)
	}

	now := time.Now().UTC()
	snapshot := Snapshot{
		ID:           id,
		Status:       statusForTurn(players, hand.CurrentTurnSeat),
		InitialChips: req.InitialChips,
		SmallBlind:   req.SmallBlind,
		BigBlind:     req.BigBlind,
		Players:      players,
		Table:        table,
		Control: ControlState{
			SpectatorMode: req.SpectatorMode,
			SemiAutoMode:  req.SemiAutoMode,
			ManualMode:    req.ManualMode,
			Running:       req.SpectatorMode && !req.ManualMode,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	hidden := &hiddenState{
		hand: hand,
		replay: ReplayDetail{
			Summary: ReplaySummary{
				ID:           id,
				Status:       "running",
				CreatedAt:    now,
				PlayerCount:  len(players),
				InitialChips: req.InitialChips,
				SmallBlind:   req.SmallBlind,
				BigBlind:     req.BigBlind,
			},
			Players:   append([]Player(nil), players...),
			CreatedAt: now,
		},
		current: replayHand,
	}

	s.mu.Lock()
	s.matches[id] = snapshot
	s.hidden[id] = hidden
	s.sequences[id] = 0
	s.persistActiveMatchLocked(id, &snapshot)
	s.matches[id] = snapshot
	s.mu.Unlock()

	s.publishPending(id, []pendingEvent{publicEvent("match_created", map[string]any{
		"match": snapshot,
	})})

	if !hasHuman(players) {
		if req.ManualMode {
			updated, _ := s.GetMatch(id)
			return updated, nil
		}
		s.startAutoplayIfNeeded(id)
		updated, _ := s.GetMatch(id)
		return updated, nil
	}

	return s.runUntilPause(id, snapshot)
}

func (s *Service) GetMatch(id string) (Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, ok := s.matches[id]
	return snapshot, ok
}

// ProbePreset is the entry point behind the lobby's "test AI" button. It
// returns a small structured result indicating whether the configured
// endpoint / token / model actually answers, plus a latency measurement and
// a short response snippet. The probe never creates a match and never
// touches replay state, so it is cheap to call repeatedly.
func (s *Service) ProbePreset(ctx context.Context, id string) (ai.ProbeResult, bool) {
	s.mu.RLock()
	preset, ok := s.presets[id]
	s.mu.RUnlock()
	if !ok {
		return ai.ProbeResult{OK: false, Error: "unknown preset id"}, false
	}
	return s.ai.Probe(ctx, preset), true
}

// ProbeInlinePreset checks an arbitrary endpoint/token/model combination
// the user typed into the lobby's "自定义模型" form, without registering
// the preset for matches. The token never lands in s.presets and never
// touches the SQLite store; this call is purely a connectivity test.
func (s *Service) ProbeInlinePreset(ctx context.Context, input config.InlinePresetInput) ai.ProbeResult {
	prepared, err := config.PrepareInlinePreset(input.ToPreset())
	if err != nil {
		return ai.ProbeResult{OK: false, Error: err.Error()}
	}
	if prepared.ID == "" {
		// Probe doesn't actually use the ID, but having a stable
		// non-empty value keeps logs readable if we ever start logging
		// probe attempts.
		prepared.ID = "inline-probe"
	}
	return s.ai.Probe(ctx, prepared)
}

// registerInlinePresets validates each preset supplied inline, generates
// an ephemeral `inline-<random>` id for it, registers it in s.presets so
// the rest of the match flow can look it up by id, and rewrites any
// `@inline:N` markers in req.AIPresetIDs to point at the generated ids.
// The lock is held for the duration so concurrent CreateMatch calls
// don't race on s.presets.
func (s *Service) registerInlinePresets(req *CreateRequest) error {
	if req == nil || len(req.AIInlinePresets) == 0 {
		// Even with no inline presets, refuse stray @inline markers so
		// the caller gets a clear error rather than "unknown preset".
		for _, ref := range req.AIPresetIDs {
			if strings.HasPrefix(ref, "@inline:") {
				return fmt.Errorf("aiPresetIds references %q but aiInlinePresets is empty", ref)
			}
		}
		return nil
	}

	prepared := make([]config.Preset, len(req.AIInlinePresets))
	for i, raw := range req.AIInlinePresets {
		ready, err := config.PrepareInlinePreset(raw.ToPreset())
		if err != nil {
			return fmt.Errorf("inline preset %d: %w", i+1, err)
		}
		// Always overwrite any client-supplied id so user-supplied
		// values can't collide with built-in preset ids or with each
		// other.
		suffix, err := newID()
		if err != nil {
			return fmt.Errorf("generate inline preset id: %w", err)
		}
		ready.ID = "inline-" + suffix
		prepared[i] = ready
	}

	s.mu.Lock()
	for _, preset := range prepared {
		s.presets[preset.ID] = preset
	}
	s.mu.Unlock()

	for i, ref := range req.AIPresetIDs {
		if !strings.HasPrefix(ref, "@inline:") {
			continue
		}
		idxStr := strings.TrimPrefix(ref, "@inline:")
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			return fmt.Errorf("invalid inline reference %q (expected @inline:N)", ref)
		}
		if idx < 0 || idx >= len(prepared) {
			return fmt.Errorf("inline reference %q out of range (have %d inline presets)", ref, len(prepared))
		}
		req.AIPresetIDs[i] = prepared[idx].ID
	}
	return nil
}

func (s *Service) ListRecords() []RecordSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records := make([]RecordSummary, 0, len(s.matches)+len(s.replays))
	replayIDs := make(map[string]struct{}, len(s.replays))
	for id, replay := range s.replays {
		replayIDs[id] = struct{}{}
		records = append(records, recordSummaryFromReplay(replay))
	}
	for id, snapshot := range s.matches {
		if _, ok := replayIDs[id]; ok && (snapshot.Status == "finished" || snapshot.Status == "stopped") {
			continue
		}
		records = append(records, recordSummaryFromSnapshot(snapshot))
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].UpdatedAt.Equal(records[j].UpdatedAt) {
			return records[i].CreatedAt.After(records[j].CreatedAt)
		}
		return records[i].UpdatedAt.After(records[j].UpdatedAt)
	})
	return records
}

func (s *Service) ListReplays() []ReplaySummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	summaries := make([]ReplaySummary, 0, len(s.replays))
	for _, replay := range s.replays {
		summaries = append(summaries, replay.Summary)
	}
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].FinishedAt.Equal(summaries[j].FinishedAt) {
			return summaries[i].CreatedAt.After(summaries[j].CreatedAt)
		}
		return summaries[i].FinishedAt.After(summaries[j].FinishedAt)
	})
	return summaries
}

func (s *Service) GetReplay(id string) (ReplayDetail, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	replay, ok := s.replays[id]
	if !ok {
		return ReplayDetail{}, false
	}
	return normalizeReplayDetail(replay), true
}

func (s *Service) DeleteReplay(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.replays[id]; !ok {
		return fmt.Errorf("replay not found")
	}
	if s.store != nil {
		if err := s.store.DeleteReplay(id); err != nil {
			return fmt.Errorf("delete replay from store: %w", err)
		}
	}
	delete(s.replays, id)
	return nil
}

func (s *Service) DeleteRecord(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := false
	if _, ok := s.replays[id]; ok {
		if s.store != nil {
			if err := s.store.DeleteReplay(id); err != nil {
				return fmt.Errorf("delete replay from store: %w", err)
			}
		}
		delete(s.replays, id)
		removed = true
	}
	if _, ok := s.matches[id]; ok {
		if s.store != nil {
			if err := s.store.DeleteActiveMatch(id); err != nil {
				return fmt.Errorf("delete active match from store: %w", err)
			}
		}
		s.removeMatchLocked(id)
		removed = true
	}
	if !removed {
		return fmt.Errorf("record not found")
	}
	return nil
}

func (s *Service) ClearReplays() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store != nil {
		if err := s.store.ClearReplays(); err != nil {
			return fmt.Errorf("clear replays from store: %w", err)
		}
	}
	s.replays = map[string]ReplayDetail{}
	return nil
}

func (s *Service) ClearRecords() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store != nil {
		if err := s.store.ClearReplays(); err != nil {
			return fmt.Errorf("clear replays from store: %w", err)
		}
		if err := s.store.ClearActiveMatches(); err != nil {
			return fmt.Errorf("clear active matches from store: %w", err)
		}
	}
	for id := range s.matches {
		s.removeMatchLocked(id)
	}
	s.replays = map[string]ReplayDetail{}
	return nil
}

func (s *Service) persistReplayLocked(id string, snapshot *Snapshot) {
	hidden := s.hidden[id]
	if hidden == nil {
		return
	}

	s.replays[id] = hidden.replay
	snapshot.Warning = ""
	if s.store == nil {
		return
	}

	if err := s.store.SaveReplay(hidden.replay); err != nil {
		snapshot.Warning = fmt.Sprintf("回放已保留在当前进程内，但写入 SQLite 失败：%v", err)
		return
	}
	_ = s.store.DeleteActiveMatch(id)
}

func (s *Service) persistActiveMatchLocked(id string, snapshot *Snapshot) {
	if s.store == nil {
		return
	}
	hidden := s.hidden[id]
	if hidden == nil {
		return
	}
	if snapshot.Status == "finished" || snapshot.Status == "stopped" {
		return
	}
	record := ActiveMatchRecord{
		Snapshot:      *snapshot,
		Hand:          hidden.hand,
		Replay:        hidden.replay,
		Current:       hidden.current,
		DecisionTrail: cloneDecisionLog(hidden.decisionTrail),
	}
	if err := s.store.SaveActiveMatch(record); err != nil {
		snapshot.Warning = fmt.Sprintf("活动牌桌已保留在当前进程内，但写入 SQLite 失败：%v", err)
	}
}

func (s *Service) persistStoppedReplayLocked(id string, snapshot *Snapshot) {
	hidden := s.hidden[id]
	if hidden == nil {
		return
	}
	appendCurrentReplayHandIfNeeded(snapshot, hidden)
	hidden.replay.Summary.Status = "stopped"
	hidden.replay.Summary.WinnerName = snapshot.WinnerName
	hidden.replay.Summary.HandsPlayed = len(hidden.replay.Hands)
	hidden.replay.Summary.FinishedAt = time.Now().UTC()
	s.persistReplayLocked(id, snapshot)
}

func appendCurrentReplayHandIfNeeded(snapshot *Snapshot, hidden *hiddenState) {
	if hidden.current == nil {
		return
	}
	if len(hidden.replay.Hands) > 0 && hidden.replay.Hands[len(hidden.replay.Hands)-1].HandNumber == hidden.current.HandNumber {
		return
	}
	hidden.current.Board = cloneStrings(hidden.hand.Board)
	hidden.current.Pot = hidden.hand.Pot
	hidden.current.FinishedAt = time.Now().UTC()
	for index := range hidden.current.Players {
		seat := hidden.current.Players[index].Seat
		hidden.current.Players[index].EndingChips = snapshot.Players[seat].Chips
		hidden.current.Players[index].Folded = hidden.hand.Folded[seat]
		hidden.current.Players[index].AllIn = hidden.hand.AllIn[seat]
		hidden.current.Players[index].Eliminated = snapshot.Players[seat].Eliminated
	}
	hidden.replay.Hands = append(hidden.replay.Hands, *hidden.current)
	hidden.replay.Summary.HandsPlayed = len(hidden.replay.Hands)
}

func (s *Service) removeMatchLocked(id string) {
	delete(s.matches, id)
	delete(s.hidden, id)
	delete(s.subscribers, id)
	delete(s.sequences, id)
	delete(s.autoplaying, id)
}

func maxSequenceForSnapshot(snapshot Snapshot, record ActiveMatchRecord) int {
	maxSeq := 0
	if snapshot.LastEvent != nil && snapshot.LastEvent.Sequence > maxSeq {
		maxSeq = snapshot.LastEvent.Sequence
	}
	for _, hand := range record.Replay.Hands {
		for _, event := range hand.Events {
			if event.Sequence > maxSeq {
				maxSeq = event.Sequence
			}
		}
	}
	if record.Current != nil {
		for _, event := range record.Current.Events {
			if event.Sequence > maxSeq {
				maxSeq = event.Sequence
			}
		}
	}
	return maxSeq
}

func recordSummaryFromSnapshot(snapshot Snapshot) RecordSummary {
	status := summarizeRecordStatus(snapshot)
	return RecordSummary{
		ID:                snapshot.ID,
		Status:            status,
		CreatedAt:         snapshot.CreatedAt,
		UpdatedAt:         snapshot.UpdatedAt,
		WinnerName:        snapshot.WinnerName,
		PlayerCount:       len(snapshot.Players),
		HandsPlayed:       snapshot.Table.CompletedHands,
		InitialChips:      snapshot.InitialChips,
		SmallBlind:        snapshot.SmallBlind,
		BigBlind:          snapshot.BigBlind,
		SpectatorMode:     snapshot.Control.SpectatorMode,
		ContinueAvailable: status == "running" || status == "paused",
		ReplayAvailable:   false,
	}
}

func recordSummaryFromReplay(replay ReplayDetail) RecordSummary {
	status := replay.Summary.Status
	if status == "" {
		status = "finished"
	}
	updatedAt := replay.Summary.FinishedAt
	if updatedAt.IsZero() {
		updatedAt = replay.Summary.CreatedAt
	}
	return RecordSummary{
		ID:                replay.Summary.ID,
		Status:            status,
		CreatedAt:         replay.Summary.CreatedAt,
		UpdatedAt:         updatedAt,
		FinishedAt:        replay.Summary.FinishedAt,
		WinnerName:        replay.Summary.WinnerName,
		PlayerCount:       replay.Summary.PlayerCount,
		HandsPlayed:       replay.Summary.HandsPlayed,
		InitialChips:      replay.Summary.InitialChips,
		SmallBlind:        replay.Summary.SmallBlind,
		BigBlind:          replay.Summary.BigBlind,
		SpectatorMode:     !containsHumanPlayer(replay.Players),
		ContinueAvailable: false,
		ReplayAvailable:   true,
	}
}

func containsHumanPlayer(players []Player) bool {
	for _, player := range players {
		if player.IsHuman {
			return true
		}
	}
	return false
}

func summarizeRecordStatus(snapshot Snapshot) string {
	if snapshot.Status == "stopped" || snapshot.Control.Stopped {
		return "stopped"
	}
	if snapshot.Status == "finished" {
		return "finished"
	}
	if snapshot.Control.Paused || snapshot.Control.ManualMode || (snapshot.Control.SemiAutoMode && snapshot.Status == "hand_complete") || snapshot.Status == "hand_complete" {
		return "paused"
	}
	return "running"
}

func (s *Service) Subscribe(id string) (<-chan StreamEvent, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.matches[id]; !ok {
		return nil, nil, fmt.Errorf("match not found")
	}

	ch := make(chan StreamEvent, 8)
	if _, ok := s.subscribers[id]; !ok {
		s.subscribers[id] = map[chan StreamEvent]struct{}{}
	}
	s.subscribers[id][ch] = struct{}{}

	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if subs, ok := s.subscribers[id]; ok {
			if _, exists := subs[ch]; exists {
				delete(subs, ch)
			}
		}
	}

	return ch, cancel, nil
}

func (s *Service) publish(id, eventType string, payload any) StreamEvent {
	s.mu.Lock()
	s.sequences[id]++
	event := StreamEvent{
		Type:      eventType,
		Sequence:  s.sequences[id],
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	}

	subscribers := make([]chan StreamEvent, 0, len(s.subscribers[id]))
	for ch := range s.subscribers[id] {
		subscribers = append(subscribers, ch)
	}
	s.mu.Unlock()

	for _, ch := range subscribers {
		select {
		case ch <- event:
		default:
		}
	}

	return event
}

func validateCreateRequest(req CreateRequest) error {
	if req.InitialChips <= 0 {
		return fmt.Errorf("initialChips must be greater than 0")
	}
	if req.SmallBlind <= 0 {
		return fmt.Errorf("smallBlind must be greater than 0")
	}
	if req.BigBlind < req.SmallBlind {
		return fmt.Errorf("bigBlind must be greater than or equal to smallBlind")
	}
	if req.InitialChips <= req.BigBlind {
		return fmt.Errorf("initialChips must be greater than bigBlind")
	}
	if req.SpectatorMode {
		if req.ManualMode && req.SemiAutoMode {
			return fmt.Errorf("manualMode and semiAutoMode cannot both be enabled")
		}
		if len(req.AIPresetIDs) < 2 || len(req.AIPresetIDs) > 6 {
			return fmt.Errorf("spectator mode requires 2 to 6 AI presets")
		}
		if len(req.AIPlayerNames) > len(req.AIPresetIDs) {
			return fmt.Errorf("aiPlayerNames cannot exceed aiPresetIds length")
		}
		return nil
	}
	if req.ManualMode || req.SemiAutoMode {
		return fmt.Errorf("manualMode and semiAutoMode are only available in spectator mode")
	}
	if len(req.AIPresetIDs) == 0 || len(req.AIPresetIDs) > 5 {
		return fmt.Errorf("aiPresetIds must contain 1 to 5 presets")
	}
	if len(req.AIPlayerNames) > len(req.AIPresetIDs) {
		return fmt.Errorf("aiPlayerNames cannot exceed aiPresetIds length")
	}
	return nil
}

func (s *Service) buildPlayers(req CreateRequest) ([]Player, error) {
	players := []Player{}
	seatOffset := 0
	if !req.SpectatorMode {
		humanName := req.HumanName
		if strings.TrimSpace(humanName) == "" {
			humanName = "你"
		}
		players = append(players, Player{
			Seat:    0,
			Name:    strings.TrimSpace(humanName),
			Chips:   req.InitialChips,
			IsHuman: true,
		})
		seatOffset = 1
	}

	totals := map[string]int{}
	for _, presetID := range req.AIPresetIDs {
		if _, ok := s.presets[presetID]; !ok {
			return nil, fmt.Errorf("unknown preset id %q", presetID)
		}
		totals[presetID]++
	}

	ordinals := map[string]int{}
	for index, presetID := range req.AIPresetIDs {
		preset := s.presets[presetID]
		ordinals[presetID]++
		name := strings.TrimSpace(aiDisplayName(req.AIPlayerNames, index))
		if name == "" {
			name = defaultAIPlayerName(preset.Name, ordinals[presetID], totals[presetID])
		}

		players = append(players, Player{
			Seat:     index + seatOffset,
			Name:     name,
			Chips:    req.InitialChips,
			PresetID: preset.ID,
		})
	}

	return players, nil
}

// defaultAIPlayerName produces the fallback display name for an AI seat when
// the request did not specify one. Rule: if the same preset shows up exactly
// once at the table, use the preset name as-is. If the same preset shows up
// multiple times, suffix every occurrence with `#1 / #2 / ...` so duplicates
// are never ambiguous and the first one is also clearly numbered (instead of
// "Foo" + "Foo 2", which makes the first one look like it lost its index).
func defaultAIPlayerName(presetName string, ordinal int, total int) string {
	if total <= 1 {
		return presetName
	}
	return fmt.Sprintf("%s #%d", presetName, ordinal)
}

func newID() (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func aiDisplayName(values []string, index int) string {
	if index < 0 || index >= len(values) {
		return ""
	}
	return values[index]
}
