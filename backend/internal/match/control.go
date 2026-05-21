package match

import "fmt"

type ControlRequest struct {
	Action string `json:"action"`
}

func (s *Service) startAutoplayIfNeeded(id string) {
	s.mu.Lock()
	if s.autoplaying[id] {
		s.mu.Unlock()
		return
	}
	s.autoplaying[id] = true
	if snapshot, ok := s.matches[id]; ok {
		snapshot.Control.Running = true
		s.matches[id] = snapshot
	}
	s.mu.Unlock()

	go s.runAutoplay(id)
}

func (s *Service) finishAutoplay(id string) {
	s.mu.Lock()
	delete(s.autoplaying, id)
	if snapshot, ok := s.matches[id]; ok {
		snapshot.Control.Running = false
		s.matches[id] = snapshot
	}
	s.mu.Unlock()
}

func (s *Service) ControlMatch(id string, req ControlRequest) (Snapshot, error) {
	s.mu.Lock()
	snapshot, ok := s.matches[id]
	if !ok {
		s.mu.Unlock()
		return Snapshot{}, fmt.Errorf("match not found")
	}

	switch req.Action {
	case "pause":
		snapshot.Control.Paused = true
	case "resume":
		snapshot.Control.Paused = false
		if snapshot.Status == "hand_complete" {
			snapshot.Control.CanStep = true
		} else {
			snapshot.Control.CanStep = false
		}
		snapshot.Control.ManualMode = false
	case "auto_on":
		snapshot.Control.SemiAutoMode = false
		snapshot.Control.ManualMode = false
		snapshot.Control.Paused = false
		snapshot.Control.CanStep = false
	case "semi_auto_on":
		snapshot.Control.SemiAutoMode = true
		snapshot.Control.ManualMode = false
		snapshot.Control.Paused = false
		snapshot.Control.CanStep = false
	case "manual_on":
		snapshot.Control.ManualMode = true
		snapshot.Control.SemiAutoMode = false
		snapshot.Control.Paused = false
		snapshot.Control.CanStep = false
	case "manual_off":
		snapshot.Control.ManualMode = false
		snapshot.Control.SemiAutoMode = false
		snapshot.Control.CanStep = false
		snapshot.Control.Paused = false
	case "step":
		snapshot.Control.ManualMode = true
		snapshot.Control.SemiAutoMode = false
		snapshot.Control.Paused = false
		snapshot.Control.CanStep = true
	case "stop":
		snapshot.Control.Stopped = true
		snapshot.Status = "stopped"
		s.persistStoppedReplayLocked(id, &snapshot)
		if snapshot.Warning != "" {
			snapshot.Warning = "比赛已手动终止；" + snapshot.Warning
		} else {
			snapshot.Warning = "比赛已手动终止"
		}
	case "continue":
		snapshot.Control.Paused = false
		if snapshot.Status == "hand_complete" {
			snapshot.Control.CanStep = true
		} else {
			snapshot.Control.CanStep = false
		}
	default:
		s.mu.Unlock()
		return Snapshot{}, fmt.Errorf("unsupported control action %q", req.Action)
	}

	s.matches[id] = snapshot
	s.persistActiveMatchLocked(id, &snapshot)
	s.matches[id] = snapshot
	s.mu.Unlock()

	if req.Action == "resume" || req.Action == "manual_off" || req.Action == "continue" || req.Action == "step" || req.Action == "auto_on" || req.Action == "semi_auto_on" || req.Action == "manual_on" {
		s.startAutoplayIfNeeded(id)
	}

	updated, _ := s.GetMatch(id)
	return updated, nil
}

func hasPendingAIStep(snapshot Snapshot) bool {
	seat := snapshot.Table.CurrentTurnSeat
	if seat < 0 || seat >= len(snapshot.Players) {
		return false
	}
	return !snapshot.Players[seat].IsHuman
}
