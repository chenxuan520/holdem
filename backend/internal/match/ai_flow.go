package match

import (
	"fmt"

	backendai "holdem/backend/internal/ai"
)

// buildPromptInput shapes the per-decision payload sent to the model. We make
// the prompt as small and information-dense as possible while still letting the
// model think like a competent NLHE player:
//
//   - drop matchId / playerName / stage labels the model never needs;
//   - pre-compute position, effective stack (in BB), pot odds and last
//     aggressor so the model spends fewer tokens deriving them;
//   - replace the verbose action log structs with a compact "stage:seat.code"
//     string format; the system prompt teaches the legend.
//
// The exact same prompt is sent for every preset; only endpoint / token /
// model differ across benchmark seats so comparisons stay fair.
func (s *Service) buildPromptInput(snapshot Snapshot, hidden *hiddenState, seat int) backendai.PromptInput {
	hand := hidden.hand

	dealerSeat := -1
	if hand != nil {
		dealerSeat = hand.DealerSeat
	}

	you := snapshot.Players[seat]
	yourStreetBet := 0
	if hand != nil {
		yourStreetBet = hand.StreetContribution[seat]
	}

	myPosition := positionLabelForSeat(snapshot.Players, dealerSeat, seat)
	effective := effectiveStackForSeat(snapshot.Players, hand, seat)
	effectiveBB := 0.0
	if snapshot.BigBlind > 0 {
		effectiveBB = roundOneDecimal(float64(effective) / float64(snapshot.BigBlind))
	}

	toCall := 0
	minRaiseTo := 0
	potOdds := 0.0
	if hand != nil {
		toCall = max(0, hand.CurrentBet-hand.StreetContribution[seat])
		if toCall > 0 && (hand.Pot+toCall) > 0 {
			potOdds = roundTwoDecimal(float64(toCall) / float64(hand.Pot+toCall))
		}
		minRaiseTo = toCall + hand.MinRaiseSize
	}

	legalActions := compactLegalActions(legalActionsForSeat(snapshot.Players, hand, seat))
	players := compactPlayersPayload(snapshot.Players, hand, seat, dealerSeat)
	lastAggressor := lastAggressorSeat(hand)
	logEntries := compactActionLog(hand, 8)

	hole := []string{}
	if hand != nil {
		hole = append(hole, hand.HoleCards[seat]...)
	}
	board := []string{}
	if hand != nil {
		board = append(board, hand.Board...)
	}

	stage := ""
	if hand != nil {
		stage = hand.Stage
	}

	handNumber := 0
	pot := 0
	if hand != nil {
		handNumber = hand.Number
		pot = hand.Pot
	}

	return backendai.PromptInput{
		Hand:          handNumber,
		Stage:         stage,
		Seat:          seat,
		Position:      myPosition,
		Hole:          hole,
		YourChips:     you.Chips,
		YourStreetBet: yourStreetBet,
		Board:         board,
		Pot:           pot,
		SB:            snapshot.SmallBlind,
		BB:            snapshot.BigBlind,
		EffBB:         effectiveBB,
		ToCall:        toCall,
		MinRaiseTo:    minRaiseTo,
		PotOdds:       potOdds,
		LegalActions:  legalActions,
		Players:       players,
		LastAggressor: lastAggressor,
		Log:           logEntries,
	}
}

// positionLabelForSeat returns a standard NLHE seat label (BTN / SB / BB /
// UTG / HJ / CO) for `seat`, derived from where it sits relative to the
// dealer button along the *live* (non-eliminated, non-empty) seats.
func positionLabelForSeat(players []Player, dealerSeat int, seat int) string {
	live := liveSeats(players)
	if len(live) < 2 || dealerSeat < 0 {
		return ""
	}

	dealerIdx := -1
	playerIdx := -1
	for i, s := range live {
		if s == dealerSeat {
			dealerIdx = i
		}
		if s == seat {
			playerIdx = i
		}
	}
	if dealerIdx == -1 || playerIdx == -1 {
		return ""
	}
	distance := (playerIdx - dealerIdx + len(live)) % len(live)
	return positionLabelByDistance(len(live), distance)
}

func positionLabelByDistance(liveCount int, distance int) string {
	switch liveCount {
	case 2:
		if distance == 0 {
			return "BTN/SB"
		}
		return "BB"
	case 3:
		switch distance {
		case 0:
			return "BTN"
		case 1:
			return "SB"
		case 2:
			return "BB"
		}
	case 4:
		switch distance {
		case 0:
			return "BTN"
		case 1:
			return "SB"
		case 2:
			return "BB"
		case 3:
			return "UTG"
		}
	case 5:
		switch distance {
		case 0:
			return "BTN"
		case 1:
			return "SB"
		case 2:
			return "BB"
		case 3:
			return "UTG"
		case 4:
			return "CO"
		}
	case 6:
		switch distance {
		case 0:
			return "BTN"
		case 1:
			return "SB"
		case 2:
			return "BB"
		case 3:
			return "UTG"
		case 4:
			return "HJ"
		case 5:
			return "CO"
		}
	}
	return ""
}

// effectiveStackForSeat is the most chips that can actually go in given the
// seat versus the deepest still-live opponent. Folded / all-in / eliminated
// seats are excluded. The result is in chips; convert to BB at the call site.
func effectiveStackForSeat(players []Player, hand *handState, seat int) int {
	if seat < 0 || seat >= len(players) {
		return 0
	}
	you := players[seat].Chips
	maxOpp := 0
	for _, player := range players {
		if player.Seat == seat || player.Eliminated {
			continue
		}
		if hand != nil && (hand.Folded[player.Seat] || hand.AllIn[player.Seat]) {
			continue
		}
		if player.Chips > maxOpp {
			maxOpp = player.Chips
		}
	}
	if maxOpp == 0 {
		return you
	}
	if you < maxOpp {
		return you
	}
	return maxOpp
}

// liveSeats returns seat numbers (in clockwise order, 0..N-1) that haven't
// been eliminated. We use this as the canonical "around the table" ordering
// for position math.
func liveSeats(players []Player) []int {
	seats := make([]int, 0, len(players))
	for _, player := range players {
		if !player.Eliminated {
			seats = append(seats, player.Seat)
		}
	}
	return seats
}

// compactPlayersPayload returns one entry per still-relevant seat (we omit
// eliminated players entirely; they aren't useful context). Each entry only
// includes fields that vary, so default-zero / default-false fields are left
// out to save tokens.
func compactPlayersPayload(players []Player, hand *handState, mySeat int, dealerSeat int) []any {
	out := make([]any, 0, len(players))
	for _, player := range players {
		if player.Eliminated {
			continue
		}
		entry := map[string]any{
			"seat":  player.Seat,
			"name":  player.Name,
			"chips": player.Chips,
		}
		if pos := positionLabelForSeat(players, dealerSeat, player.Seat); pos != "" {
			entry["position"] = pos
		}
		if hand != nil {
			if v := hand.StreetContribution[player.Seat]; v > 0 {
				entry["streetBet"] = v
			}
			if hand.Folded[player.Seat] {
				entry["folded"] = true
			}
			if hand.AllIn[player.Seat] {
				entry["allIn"] = true
			}
		}
		if player.Seat == mySeat {
			entry["self"] = true
		}
		if player.IsHuman {
			entry["human"] = true
		}
		out = append(out, entry)
	}
	return out
}

// compactLegalActions strips the中文 label from each action option and omits
// zero amounts. The model only needs `action` + (when present) `amount`.
func compactLegalActions(options []ActionOption) []any {
	out := make([]any, 0, len(options))
	for _, option := range options {
		entry := map[string]any{"action": option.Action}
		if option.Amount > 0 {
			entry["amount"] = option.Amount
		}
		out = append(out, entry)
	}
	return out
}

// compactActionLog renders the most recent `limit` ActionLog entries into the
// dense "stage:seat.code[amount]" string format described in the system
// prompt. Saving roughly 200 tokens per request vs the previous full-struct
// JSON.
func compactActionLog(hand *handState, limit int) []string {
	if hand == nil {
		return nil
	}
	entries := hand.ActionLog
	if len(entries) == 0 {
		return nil
	}
	start := 0
	if len(entries) > limit {
		start = len(entries) - limit
	}
	out := make([]string, 0, len(entries)-start)
	for _, entry := range entries[start:] {
		out = append(out, formatLogEntry(entry))
	}
	return out
}

func formatLogEntry(entry ActionLog) string {
	stage := stageCode(entry.Street)
	code, withAmount := actionCode(entry.Action)
	if withAmount {
		return fmt.Sprintf("%s:%d.%s%d", stage, entry.Seat, code, entry.Amount)
	}
	return fmt.Sprintf("%s:%d.%s", stage, entry.Seat, code)
}

func stageCode(stage string) string {
	switch stage {
	case "preflop":
		return "pre"
	case "flop":
		return "flop"
	case "turn":
		return "turn"
	case "river":
		return "river"
	default:
		if stage == "" {
			return "?"
		}
		return stage
	}
}

func actionCode(action string) (string, bool) {
	switch action {
	case "fold":
		return "f", false
	case "check":
		return "x", false
	case "call":
		return "c", true
	case "raise":
		return "r", true
	case "all_in":
		return "A", true
	case "post_small_blind":
		return "sb", true
	case "post_big_blind":
		return "bb", true
	default:
		return action, true
	}
}

// lastAggressorSeat returns the seat (as a *int so it can be omitted entirely
// in JSON) that performed the most recent aggressive action — bet / raise /
// all_in — across the current hand. If nobody has been aggressive yet, the
// pointer is nil.
func lastAggressorSeat(hand *handState) *int {
	if hand == nil {
		return nil
	}
	for i := len(hand.ActionLog) - 1; i >= 0; i-- {
		entry := hand.ActionLog[i]
		switch entry.Action {
		case "raise", "all_in":
			seat := entry.Seat
			return &seat
		}
	}
	return nil
}

func roundOneDecimal(value float64) float64 {
	return float64(int(value*10+0.5)) / 10
}

func roundTwoDecimal(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

func fallbackDecision(snapshot Snapshot, hand *handState, seat int) backendai.Decision {
	actions := legalActionsForSeat(snapshot.Players, hand, seat)
	for _, action := range actions {
		if action.Action == "check" {
			return backendai.Decision{Action: "check", PublicReason: "模型响应异常，系统回退为安全过牌。", PrivateReason: "模型响应异常，系统自动回退为过牌。"}
		}
	}
	for _, action := range actions {
		if action.Action == "call" {
			return backendai.Decision{Action: "call", Amount: action.Amount, PublicReason: "模型响应异常，系统回退为安全跟注。", PrivateReason: "模型响应异常，系统自动回退为跟注。"}
		}
	}
	return backendai.Decision{Action: "fold", PublicReason: "模型响应异常，系统回退为安全弃牌。", PrivateReason: "模型响应异常，系统自动回退为弃牌。"}
}

func requestFailureDecision() backendai.Decision {
	return backendai.Decision{
		Action:        "fold",
		PublicReason:  "因请求出错，系统直接弃牌止损。",
		PrivateReason: "模型连续 3 次请求出错，系统直接弃牌止损。",
	}
}

func privateReasonForView(players []Player, value string) string {
	if hasHuman(players) {
		return ""
	}
	return value
}
