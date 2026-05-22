package match

import (
	"context"
	"fmt"
	"time"

	backendai "holdem/backend/internal/ai"
)

type PlayerActionRequest struct {
	Action string `json:"action"`
	Amount int    `json:"amount,omitempty"`
}

func (s *Service) ApplyHeroAction(id string, req PlayerActionRequest) (Snapshot, error) {
	pending, snapshot, err := s.applyHumanAction(id, req)
	if err != nil {
		return Snapshot{}, err
	}
	s.publishPending(id, pending)
	return s.runUntilPause(id, snapshot)
}

func (s *Service) runUntilPause(id string, snapshot Snapshot) (Snapshot, error) {
	iterations := 0
	aiRequests := 0
	for {
		iterations++
		s.mu.RLock()
		current, ok := s.matches[id]
		hidden := s.hidden[id]
		var presetID string
		var input backendai.PromptInput
		consumeStep := false
		heroSeat := -1
		if ok && hidden != nil && hidden.hand != nil && current.Status != "finished" {
			if current.Control.Stopped || current.Status == "stopped" {
				s.mu.RUnlock()
				return current, nil
			}
			if shouldPauseAfterHand(current) {
				if !current.Control.CanStep {
					s.mu.RUnlock()
					return current, nil
				}
				consumeStep = true
			}
			heroSeat = humanSeat(current.Players)
			seat := hidden.hand.CurrentTurnSeat
			if seat >= 0 && seat < len(current.Players) && !current.Players[seat].IsHuman {
				if current.Control.Paused {
					s.mu.RUnlock()
					return current, nil
				}
				if current.Control.ManualMode {
					if !current.Control.CanStep {
						s.mu.RUnlock()
						return current, nil
					}
					consumeStep = true
				}
			}
			if seat >= 0 && seat < len(current.Players) && seat != heroSeat {
				presetID = current.Players[seat].PresetID
				input = s.buildPromptInput(current, hidden, seat)
			}
		}
		s.mu.RUnlock()
		if consumeStep {
			s.clearStepPermission(id)
		}

		if !ok {
			return Snapshot{}, fmt.Errorf("match not found")
		}
		// State-transition safety net. Each hand burns roughly a few dozen
		// iterations (player_acted / board_cards_dealt / private_reason /
		// hand_settled / ...), so 131072 leaves headroom for thousands of
		// hands of legitimate play before tripping; we keep the limit only
		// to catch real infinite loops, not to cap session length.
		if iterations > 131072 {
			return current, fmt.Errorf("safety stop: too many state transitions in one run")
		}
		if current.Status == "finished" || current.Status == "awaiting_human" {
			return current, nil
		}

		if presetID == "" {
			pending, nextSnapshot, err := s.advanceNonHumanState(id)
			if err != nil {
				return Snapshot{}, err
			}
			s.publishPending(id, pending)
			snapshot = nextSnapshot
			continue
		}

		preset := s.presets[presetID]
		aiRequests++
		// AI-decision safety net. A 6-handed table averages ~10 AI requests
		// per hand, so 8192 = ~800 hands of full-auto before we'd ever stop.
		// Same reasoning as the iterations cap: the limit is for runaway
		// loops, not for capping how long a benchmark can run.
		if aiRequests > 8192 {
			return current, fmt.Errorf("safety stop: too many ai decisions in one run")
		}
		decision, logEntry, err := s.ai.Decide(context.Background(), preset, input)
		if err != nil {
			decision = requestFailureDecision()
			logEntry.Error = err.Error()
		}

		pending, nextSnapshot, err := s.applyAIDecision(id, decision, logEntry)
		if err != nil {
			return Snapshot{}, err
		}
		s.publishPending(id, pending)
		snapshot = nextSnapshot
	}
}

func shouldPauseAfterHand(snapshot Snapshot) bool {
	if snapshot.Status != "hand_complete" {
		return false
	}
	// Use tableHasHumanSeat (which counts eliminated humans) instead of
	// hasHuman/humanSeat (which skip them). Without this, the moment the
	// hero busts out, finalizeHand flips Eliminated=true, hasHuman returns
	// false, runUntilPause blows straight past the hand_complete snapshot
	// into the next hand or match_finished, and the user never sees the
	// hand they were eliminated on. Once eliminated, the table behaves
	// like a semi-auto spectator: pause every hand, the user clicks
	// 继续下一手 to advance through the runout.
	if tableHasHumanSeat(snapshot.Players) {
		return true
	}
	return snapshot.Control.SemiAutoMode || snapshot.Control.ManualMode || snapshot.Control.Paused
}

// tableHasHumanSeat reports whether the *match was set up* with a human
// player, regardless of whether they currently have chips. Use this for
// "should the UX behave like human-vs-AI?" checks (pause cadence, hero
// footer visibility on the backend side). For "is there a live human
// whose turn it could be right now?", use humanSeat / hasHuman instead.
func tableHasHumanSeat(players []Player) bool {
	for _, player := range players {
		if player.IsHuman {
			return true
		}
	}
	return false
}

func (s *Service) clearStepPermission(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if snapshot, ok := s.matches[id]; ok {
		snapshot.Control.CanStep = false
		s.matches[id] = snapshot
	}
}

func (s *Service) publishPending(id string, pending []pendingEvent) {
	for _, item := range pending {
		var subscribers []chan StreamEvent
		s.mu.Lock()
		s.sequences[id]++
		event := StreamEvent{
			Type:       item.EventType,
			Sequence:   s.sequences[id],
			Timestamp:  time.Now().UTC(),
			Visibility: item.Visibility,
			Payload:    item.Payload,
		}

		if hidden := s.hidden[id]; hidden != nil && hidden.current != nil {
			hidden.current.Events = append(hidden.current.Events, ReplayEvent{
				Sequence:   event.Sequence,
				Type:       event.Type,
				Visibility: event.Visibility,
				Timestamp:  event.Timestamp,
				Payload:    event.Payload,
			})
		}

		if snapshot, ok := s.matches[id]; ok {
			snapshot.UpdatedAt = event.Timestamp
			if event.Visibility == "public" {
				snapshot.LastEvent = &event
			}
			s.matches[id] = snapshot
		}

		if event.Visibility == "public" {
			for ch := range s.subscribers[id] {
				subscribers = append(subscribers, ch)
			}
		}
		s.mu.Unlock()

		for _, ch := range subscribers {
			select {
			case ch <- event:
			default:
			}
		}
	}
}
