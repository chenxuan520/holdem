package match

import (
	"fmt"
	"sort"
	"time"
)

func advanceStreet(snapshot *Snapshot, hidden *hiddenState) ([]pendingEvent, error) {
	hand := hidden.hand
	pending := []pendingEvent{}

	for {
		if remainingInHand(snapshot.Players, hand) == 1 {
			more, err := awardSingleRemaining(snapshot, hidden)
			return append(pending, more...), err
		}

		if activeActors(snapshot.Players, hand) == 0 {
			if len(hand.Board) < 5 {
				more, err := dealNextBoard(snapshot, hidden)
				if err != nil {
					return nil, err
				}
				pending = append(pending, more...)
				continue
			}
			more, err := settleShowdown(snapshot, hidden)
			return append(pending, more...), err
		}

		switch hand.Stage {
		case "preflop", "flop", "turn":
			more, err := dealNextBoard(snapshot, hidden)
			if err != nil {
				return nil, err
			}
			pending = append(pending, more...)
			return pending, nil
		case "river":
			more, err := settleShowdown(snapshot, hidden)
			return append(pending, more...), err
		default:
			return nil, fmt.Errorf("unknown stage %q", hand.Stage)
		}
	}
}

func dealNextBoard(snapshot *Snapshot, hidden *hiddenState) ([]pendingEvent, error) {
	hand := hidden.hand
	dealCount := 0
	nextStage := ""
	switch hand.Stage {
	case "preflop":
		dealCount = 3
		nextStage = "flop"
	case "flop":
		dealCount = 1
		nextStage = "turn"
	case "turn":
		dealCount = 1
		nextStage = "river"
	default:
		return nil, fmt.Errorf("cannot deal board on stage %q", hand.Stage)
	}

	if len(hand.Deck) < dealCount {
		return nil, fmt.Errorf("deck exhausted")
	}
	hand.Board = append(hand.Board, hand.Deck[:dealCount]...)
	hand.Deck = hand.Deck[dealCount:]
	hand.Stage = nextStage
	hand.CurrentBet = 0
	hand.MinRaiseSize = snapshot.BigBlind
	hand.CurrentTurnSeat = firstPostflopTurn(snapshot.Players, hand.DealerSeat)
	for _, player := range snapshot.Players {
		hand.StreetContribution[player.Seat] = 0
		hand.Acted[player.Seat] = false
	}

	rebuildSnapshotTable(snapshot, hidden)
	snapshot.Status = statusForTurn(snapshot.Players, hand.CurrentTurnSeat)

	payload := map[string]any{
		"stage": nextStage,
		"board": append([]string(nil), hand.Board...),
	}
	return []pendingEvent{publicEvent("board_cards_dealt", payload)}, nil
}

func awardSingleRemaining(snapshot *Snapshot, hidden *hiddenState) ([]pendingEvent, error) {
	hand := hidden.hand
	winningSeat := -1
	for _, player := range snapshot.Players {
		if player.Eliminated || hand.Folded[player.Seat] {
			continue
		}
		winningSeat = player.Seat
		break
	}
	if winningSeat < 0 {
		return nil, fmt.Errorf("no remaining player to award")
	}

	snapshot.Players[winningSeat].Chips += hand.Pot
	hand.HandOver = true
	winners := []ReplayWinner{{
		Seat:       winningSeat,
		PlayerName: snapshot.Players[winningSeat].Name,
		Amount:     hand.Pot,
		HandLabel:  "无需摊牌",
	}}
	newlyEliminated := finalizeHand(snapshot, hidden, winners)

	pending := []pendingEvent{publicEvent("hand_settled", map[string]any{
		"handNumber": hand.Number,
		"winners":    winners,
		"board":      append([]string(nil), hand.Board...),
	})}
	pending = append(pending, eliminationEvents(newlyEliminated)...)
	return pending, nil
}

func settleShowdown(snapshot *Snapshot, hidden *hiddenState) ([]pendingEvent, error) {
	hand := hidden.hand
	results := make(map[int]handRank)
	labels := make(map[int]string)
	eligible := []int{}
	for _, player := range snapshot.Players {
		if player.Eliminated || hand.Folded[player.Seat] {
			continue
		}
		cards := append([]string(nil), hand.Board...)
		cards = append(cards, hand.HoleCards[player.Seat]...)
		rank, label, err := evaluateBestHand(cards)
		if err != nil {
			return nil, err
		}
		results[player.Seat] = rank
		labels[player.Seat] = label
		eligible = append(eligible, player.Seat)
	}

	pots := buildSidePots(hand.TotalContribution, snapshot.Players, hand.Folded)
	awards := map[int]int{}
	for _, pot := range pots {
		winners := resolvePotWinners(pot.EligibleSeats, results)
		share := pot.Amount / len(winners)
		remainder := pot.Amount % len(winners)
		ordered := orderSeatsFromDealer(winners, hand.DealerSeat, len(snapshot.Players))
		for index, seat := range ordered {
			awards[seat] += share
			if index < remainder {
				awards[seat]++
			}
		}
	}

	replayWinners := make([]ReplayWinner, 0, len(awards))
	for seat, amount := range awards {
		snapshot.Players[seat].Chips += amount
		replayWinners = append(replayWinners, ReplayWinner{
			Seat:       seat,
			PlayerName: snapshot.Players[seat].Name,
			Amount:     amount,
			HandLabel:  labels[seat],
		})
	}
	sort.Slice(replayWinners, func(i, j int) bool { return replayWinners[i].Seat < replayWinners[j].Seat })

	hand.HandOver = true
	newlyEliminated := finalizeHand(snapshot, hidden, replayWinners)
	pending := []pendingEvent{publicEvent("showdown_revealed", map[string]any{
		"handNumber": hand.Number,
		"board":      append([]string(nil), hand.Board...),
		"winners":    replayWinners,
	})}
	pending = append(pending, eliminationEvents(newlyEliminated)...)
	return pending, nil
}

func startNextHandOrFinish(snapshot *Snapshot, hidden *hiddenState) ([]pendingEvent, error) {
	activeSeats := seatsWithChips(snapshot.Players)
	if len(activeSeats) <= 1 {
		snapshot.Status = "finished"
		winnerName := ""
		if len(activeSeats) == 1 {
			winnerName = snapshot.Players[activeSeats[0]].Name
		}
		snapshot.WinnerName = winnerName
		hidden.replay.Summary.WinnerName = winnerName
		hidden.replay.Summary.HandsPlayed = len(hidden.replay.Hands)
		hidden.replay.Summary.FinishedAt = time.Now().UTC()
		return []pendingEvent{publicEvent("match_finished", map[string]any{
			"winnerName": winnerName,
			"handsPlayed": len(hidden.replay.Hands),
		})}, nil
	}

	dealerSeat := nextDealerSeat(snapshot.Players, hidden.hand.DealerSeat)
	players, hand, _, replayHand, err := prepareHand(snapshot.Players, hidden.hand.Number+1, dealerSeat, snapshot.SmallBlind, snapshot.BigBlind)
	if err != nil {
		return nil, err
	}
	snapshot.Players = players
	hidden.hand = hand
	hidden.current = replayHand
	rebuildSnapshotTable(snapshot, hidden)
	snapshot.Status = statusForTurn(snapshot.Players, hand.CurrentTurnSeat)
	return []pendingEvent{publicEvent("hand_started", map[string]any{
		"handNumber": hand.Number,
		"dealerSeat": hand.DealerSeat,
	})}, nil
}

func finalizeHand(snapshot *Snapshot, hidden *hiddenState, winners []ReplayWinner) []Player {
	newlyEliminated := make([]Player, 0)
	for seat := range snapshot.Players {
		wasEliminated := snapshot.Players[seat].Eliminated
		if snapshot.Players[seat].Chips == 0 {
			snapshot.Players[seat].Eliminated = true
		}
		if !wasEliminated && snapshot.Players[seat].Eliminated {
			newlyEliminated = append(newlyEliminated, snapshot.Players[seat])
		}
	}

	hidden.current.Board = append([]string(nil), hidden.hand.Board...)
	hidden.current.Pot = hidden.hand.Pot
	hidden.current.Winners = append([]ReplayWinner(nil), winners...)
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
	hidden.hand.CurrentTurnSeat = -1
	snapshot.Table.LastWinners = winnerNames(winners)
	snapshot.Table.CompletedHands = len(hidden.replay.Hands)
	snapshot.Table.ActionLog = append([]ActionLog(nil), hidden.hand.ActionLog...)
	snapshot.Table.DecisionLog = recentDecisionEntries(hidden.decisionTrail, 10)
	snapshot.Table.Board = append([]string(nil), hidden.hand.Board...)
	snapshot.Table.LegalActions = nil
	snapshot.Table.CurrentTurnSeat = -1
	snapshot.Status = "hand_complete"
	return newlyEliminated
}

func winnerNames(winners []ReplayWinner) []string {
	names := make([]string, 0, len(winners))
	for _, winner := range winners {
		names = append(names, winner.PlayerName)
	}
	return names
}

func eliminationEvents(players []Player) []pendingEvent {
	pending := []pendingEvent{}
	for _, player := range players {
		pending = append(pending, publicEvent("player_eliminated", map[string]any{
			"seat":       player.Seat,
			"playerName": player.Name,
		}))
	}
	return pending
}

func activeActors(players []Player, hand *handState) int {
	count := 0
	for _, player := range players {
		if playerCanAct(players, hand, player.Seat) {
			count++
		}
	}
	return count
}

func firstPostflopTurn(players []Player, dealerSeat int) int {
	seat := dealerSeat
	for step := 0; step < len(players); step++ {
		seat = nextLiveSeat(players, seat)
		if !players[seat].Eliminated && players[seat].Chips > 0 {
			return seat
		}
	}
	return -1
}

func nextDealerSeat(players []Player, previousDealer int) int {
	return nextLiveSeat(players, previousDealer)
}
