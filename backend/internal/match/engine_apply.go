package match

import (
	"fmt"
	"strings"

	backendai "holdem/backend/internal/ai"
)

func (s *Service) applyHumanAction(id string, req PlayerActionRequest) ([]pendingEvent, Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, ok := s.matches[id]
	if !ok {
		return nil, Snapshot{}, fmt.Errorf("match not found")
	}
	hidden := s.hidden[id]
	if hidden == nil || hidden.hand == nil {
		return nil, Snapshot{}, fmt.Errorf("match hand not ready")
	}
	heroSeat := humanSeat(snapshot.Players)
	if heroSeat < 0 {
		return nil, Snapshot{}, fmt.Errorf("this match has no human player")
	}
	if hidden.hand.CurrentTurnSeat != heroSeat {
		return nil, Snapshot{}, fmt.Errorf("it is not the hero turn")
	}

	pending, err := applyActionToState(&snapshot, hidden, heroSeat, req.Action, req.Amount, "", "")
	if err != nil {
		return nil, Snapshot{}, err
	}
	s.matches[id] = snapshot
	s.persistActiveMatchLocked(id, &snapshot)
	s.matches[id] = snapshot
	if snapshot.Status == "finished" {
		s.persistReplayLocked(id, &snapshot)
		s.matches[id] = snapshot
	}
	return pending, snapshot, nil
}

func (s *Service) applyAIDecision(id string, decision backendai.Decision, logEntry backendai.RawLog) ([]pendingEvent, Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, ok := s.matches[id]
	if !ok {
		return nil, Snapshot{}, fmt.Errorf("match not found")
	}
	hidden := s.hidden[id]
	if hidden == nil || hidden.hand == nil {
		return nil, Snapshot{}, fmt.Errorf("match hand not ready")
	}
	seat := hidden.hand.CurrentTurnSeat
	if seat < 0 || seat >= len(snapshot.Players) {
		return nil, Snapshot{}, fmt.Errorf("ai turn is unavailable")
	}
	if snapshot.Players[seat].IsHuman {
		return nil, Snapshot{}, fmt.Errorf("current turn belongs to human")
	}

	preset := s.presets[snapshot.Players[seat].PresetID]
	hidden.replay.AILogs = append(hidden.replay.AILogs, AILog{
		MatchID:        id,
		HandNumber:     hidden.hand.Number,
		Seat:           seat,
		PlayerName:     snapshot.Players[seat].Name,
		Model:          preset.Model,
		Endpoint:       preset.Endpoint,
		RequestPayload: logEntry.RequestPayload,
		ResponseBody:   logEntry.ResponseBody,
		Structured:     decision,
		CreatedAt:      hidden.hand.StartedAt,
		Error:          logEntry.Error,
	})

	pending := []pendingEvent{replayOnlyEvent("ai_decision_recorded", map[string]any{
		"seat":            seat,
		"playerName":      snapshot.Players[seat].Name,
		"action":          decision.Action,
		"amount":          decision.Amount,
		"publicReason":    decision.PublicReason,
		"privateReason":   decision.PrivateReason,
		"rawResponseBody": logEntry.ResponseBody,
	})}

	appliedDecision := decision
	more, err := applyActionToState(&snapshot, hidden, seat, decision.Action, decision.Amount, decision.PublicReason, decision.PrivateReason)
	if err != nil {
		fallback := fallbackDecision(snapshot, hidden.hand, seat)
		appliedDecision = fallback
		more, err = applyActionToState(&snapshot, hidden, seat, fallback.Action, fallback.Amount, fallback.PublicReason, fallback.PrivateReason)
		if err != nil {
			return nil, Snapshot{}, err
		}
	}
	entry := DecisionEntry{
		Seat:          seat,
		PlayerName:    snapshot.Players[seat].Name,
		Stage:         hidden.hand.Stage,
		Action:        appliedDecision.Action,
		Amount:        appliedDecision.Amount,
		PublicReason:  appliedDecision.PublicReason,
		PrivateReason: privateReasonForView(snapshot.Players, appliedDecision.PrivateReason),
		Model:         preset.Model,
		Endpoint:      preset.Endpoint,
	}
	hidden.hand.DecisionLog = append(hidden.hand.DecisionLog, entry)
	hidden.decisionTrail = append(hidden.decisionTrail, entry)
	rebuildSnapshotTable(&snapshot, hidden)
	pending = append(pending, more...)
	s.matches[id] = snapshot
	s.persistActiveMatchLocked(id, &snapshot)
	s.matches[id] = snapshot
	if snapshot.Status == "finished" {
		s.persistReplayLocked(id, &snapshot)
		s.matches[id] = snapshot
	}
	return pending, snapshot, nil
}

func (s *Service) advanceNonHumanState(id string) ([]pendingEvent, Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, ok := s.matches[id]
	if !ok {
		return nil, Snapshot{}, fmt.Errorf("match not found")
	}
	hidden := s.hidden[id]
	if hidden == nil {
		return nil, Snapshot{}, fmt.Errorf("match state missing")
	}

	pending, err := advanceState(&snapshot, hidden)
	if err != nil {
		return nil, Snapshot{}, err
	}
	s.matches[id] = snapshot
	s.persistActiveMatchLocked(id, &snapshot)
	s.matches[id] = snapshot
	if snapshot.Status == "finished" {
		s.persistReplayLocked(id, &snapshot)
		s.matches[id] = snapshot
	}
	return pending, snapshot, nil
}

func applyActionToState(snapshot *Snapshot, hidden *hiddenState, seat int, action string, requestedAmount int, publicReason string, privateReason string) ([]pendingEvent, error) {
	hand := hidden.hand
	action = strings.ToLower(strings.TrimSpace(action))
	amount, err := resolveActionAmount(snapshot.Players, hand, seat, action, requestedAmount)
	if err != nil {
		return nil, err
	}

	player := snapshot.Players[seat]
	pending := []pendingEvent{}
	summary := map[string]any{
		"seat":         seat,
		"playerName":   player.Name,
		"action":       action,
		"amount":       amount,
		"stage":        hand.Stage,
		"publicReason": publicReason,
	}

	switch action {
	case "fold":
		hand.Folded[seat] = true
		hand.Acted[seat] = true
	case "check":
		hand.Acted[seat] = true
	case "call":
		postContribution(&snapshot.Players[seat], hand.StreetContribution, hand.TotalContribution, amount, seat, hand.AllIn)
		hand.Pot += amount
		hand.Acted[seat] = true
	case "raise", "all_in":
		previous := hand.StreetContribution[seat]
		postContribution(&snapshot.Players[seat], hand.StreetContribution, hand.TotalContribution, amount, seat, hand.AllIn)
		hand.Pot += amount
		newBet := hand.StreetContribution[seat]
		raiseSize := newBet - hand.CurrentBet
		hand.CurrentBet = max(hand.CurrentBet, newBet)
		if raiseSize > 0 {
			if raiseSize >= hand.MinRaiseSize {
				hand.MinRaiseSize = raiseSize
			}
			resetActedState(snapshot.Players, hand, seat)
		} else {
			hand.Acted[seat] = true
		}
		_ = previous
	default:
		return nil, fmt.Errorf("unsupported action %q", action)
	}

	hand.ActionLog = append(hand.ActionLog, ActionLog{
		Seat:       seat,
		PlayerName: player.Name,
		Action:     action,
		Amount:     amount,
		Street:     hand.Stage,
	})

	if player.IsHuman {
		pending = append(pending, publicEvent("player_acted", summary))
	} else {
		pending = append(pending, publicEvent("ai_acted", summary))
	}

	more, err := advanceAfterAction(snapshot, hidden)
	if err != nil {
		return nil, err
	}
	pending = append(pending, more...)
	if privateReason != "" {
		pending = append(pending, replayOnlyEvent("private_reason_recorded", map[string]any{
			"seat":          seat,
			"playerName":    player.Name,
			"privateReason": privateReason,
		}))
	}
	return pending, nil
}
