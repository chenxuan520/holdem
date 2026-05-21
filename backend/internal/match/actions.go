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
		if iterations > 2048 {
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
		if aiRequests > 256 {
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
	if hasHuman(snapshot.Players) {
		return true
	}
	return snapshot.Control.SemiAutoMode || snapshot.Control.ManualMode || snapshot.Control.Paused
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
