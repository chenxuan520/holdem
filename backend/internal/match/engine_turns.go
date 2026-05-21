package match

import "fmt"

func advanceState(snapshot *Snapshot, hidden *hiddenState) ([]pendingEvent, error) {
	if hidden.hand == nil {
		return nil, fmt.Errorf("hand state missing")
	}
	if hidden.hand.HandOver {
		return startNextHandOrFinish(snapshot, hidden)
	}
	return nil, nil
}

func resolveActionAmount(players []Player, hand *handState, seat int, action string, requestedAmount int) (int, error) {
	toCall := max(0, hand.CurrentBet-hand.StreetContribution[seat])
	player := players[seat]
	switch action {
	case "fold":
		return 0, nil
	case "check":
		if toCall != 0 {
			return 0, fmt.Errorf("check is not legal")
		}
		return 0, nil
	case "call":
		if toCall == 0 {
			return 0, fmt.Errorf("call is not legal")
		}
		return min(player.Chips, toCall), nil
	case "raise":
		minimum := toCall + hand.MinRaiseSize
		amount := requestedAmount
		if amount == 0 {
			amount = minimum
		}
		if amount < minimum {
			return 0, fmt.Errorf("raise amount must be at least %d", minimum)
		}
		if amount > player.Chips {
			return 0, fmt.Errorf("raise amount exceeds chips")
		}
		return amount, nil
	case "all_in":
		if player.Chips <= 0 {
			return 0, fmt.Errorf("player has no chips left")
		}
		return player.Chips, nil
	default:
		return 0, fmt.Errorf("unknown action %q", action)
	}
}

func resetActedState(players []Player, hand *handState, actorSeat int) {
	for _, player := range players {
		if player.Eliminated || hand.Folded[player.Seat] || hand.AllIn[player.Seat] {
			continue
		}
		hand.Acted[player.Seat] = false
	}
	hand.Acted[actorSeat] = true
}

func advanceAfterAction(snapshot *Snapshot, hidden *hiddenState) ([]pendingEvent, error) {
	hand := hidden.hand
	if remainingInHand(snapshot.Players, hand) == 1 {
		return awardSingleRemaining(snapshot, hidden)
	}

	if bettingRoundComplete(snapshot.Players, hand) {
		return advanceStreet(snapshot, hidden)
	}

	hand.CurrentTurnSeat = nextActionSeat(snapshot.Players, hand, hand.CurrentTurnSeat)
	snapshot.Status = statusForTurn(snapshot.Players, hand.CurrentTurnSeat)
	rebuildSnapshotTable(snapshot, hidden)
	return nil, nil
}

func bettingRoundComplete(players []Player, hand *handState) bool {
	for _, player := range players {
		if player.Eliminated || hand.Folded[player.Seat] || hand.AllIn[player.Seat] {
			continue
		}
		if player.Chips == 0 {
			continue
		}
		if !hand.Acted[player.Seat] {
			return false
		}
		if hand.StreetContribution[player.Seat] != hand.CurrentBet {
			return false
		}
	}
	return true
}

func nextActionSeat(players []Player, hand *handState, currentSeat int) int {
	for step := 1; step <= len(players); step++ {
		seat := (currentSeat + step) % len(players)
		if playerCanAct(players, hand, seat) {
			return seat
		}
	}
	return -1
}

func playerCanAct(players []Player, hand *handState, seat int) bool {
	if seat < 0 || seat >= len(players) {
		return false
	}
	player := players[seat]
	if player.Eliminated || hand.Folded[seat] || hand.AllIn[seat] {
		return false
	}
	return player.Chips > 0
}

func remainingInHand(players []Player, hand *handState) int {
	count := 0
	for _, player := range players {
		if player.Eliminated || hand.Folded[player.Seat] {
			continue
		}
		count++
	}
	return count
}

func statusForTurn(players []Player, seat int) string {
	if seat < 0 || seat >= len(players) {
		return "progressing"
	}
	if players[seat].IsHuman {
		return "awaiting_human"
	}
	if seat >= 0 && seat < len(players) {
		return "awaiting_ai"
	}
	return "progressing"
}
