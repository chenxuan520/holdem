package match

import "time"

func prepareHand(players []Player, handNumber int, dealerSeat int, smallBlind int, bigBlind int) ([]Player, *handState, TableState, *ReplayHand, error) {
	updatedPlayers := append([]Player(nil), players...)
	activeSeats := seatsWithChips(updatedPlayers)
	if len(activeSeats) < 2 {
		return nil, nil, TableState{}, nil, nil
	}

	deck, err := shuffledDeck()
	if err != nil {
		return nil, nil, TableState{}, nil, err
	}

	holeCards := make(map[int][]string, len(updatedPlayers))
	folded := make(map[int]bool, len(updatedPlayers))
	allIn := make(map[int]bool, len(updatedPlayers))
	streetContribution := make(map[int]int, len(updatedPlayers))
	totalContribution := make(map[int]int, len(updatedPlayers))
	acted := make(map[int]bool, len(updatedPlayers))

	dealerSeat = normalizeDealerSeat(updatedPlayers, dealerSeat)
	smallBlindSeat, bigBlindSeat := blindSeatsForPlayers(updatedPlayers, dealerSeat)
	postContribution(&updatedPlayers[smallBlindSeat], streetContribution, totalContribution, smallBlind, smallBlindSeat, allIn)
	postContribution(&updatedPlayers[bigBlindSeat], streetContribution, totalContribution, bigBlind, bigBlindSeat, allIn)

	cardIndex := 0
	dealStart := nextLiveSeat(updatedPlayers, dealerSeat)
	for round := 0; round < 2; round++ {
		seat := dealStart
		for dealt := 0; dealt < len(activeSeats); dealt++ {
			holeCards[seat] = append(holeCards[seat], deck[cardIndex])
			cardIndex++
			seat = nextLiveSeat(updatedPlayers, seat)
		}
	}

	hand := &handState{
		Number:             handNumber,
		DealerSeat:         dealerSeat,
		SmallBlindSeat:     smallBlindSeat,
		BigBlindSeat:       bigBlindSeat,
		Stage:              "preflop",
		Deck:               append([]string(nil), deck[cardIndex:]...),
		Board:              []string{},
		HoleCards:          holeCards,
		RevealedCards:      map[int][]string{},
		Folded:             folded,
		AllIn:              allIn,
		StreetContribution: streetContribution,
		TotalContribution:  totalContribution,
		CurrentBet:         totalContribution[bigBlindSeat],
		MinRaiseSize:       max(bigBlind, totalContribution[bigBlindSeat]),
		CurrentTurnSeat:    firstPreflopTurnForPlayers(updatedPlayers, dealerSeat, bigBlindSeat),
		Acted:              acted,
		Pot:                totalContribution[smallBlindSeat] + totalContribution[bigBlindSeat],
		StartedAt:          time.Now().UTC(),
		ActionLog: []ActionLog{
			{Seat: smallBlindSeat, PlayerName: updatedPlayers[smallBlindSeat].Name, Action: "post_small_blind", Amount: totalContribution[smallBlindSeat], Street: "preflop"},
			{Seat: bigBlindSeat, PlayerName: updatedPlayers[bigBlindSeat].Name, Action: "post_big_blind", Amount: totalContribution[bigBlindSeat], Street: "preflop"},
		},
	}

	replay := &ReplayHand{
		HandNumber: handNumber,
		DealerSeat: dealerSeat,
		Board:      []string{},
		Pot:        hand.Pot,
		StartedAt:  hand.StartedAt,
	}

	for _, player := range updatedPlayers {
		if player.Eliminated {
			continue
		}
		replay.Players = append(replay.Players, ReplayPlayerState{
			Seat:          player.Seat,
			Name:          player.Name,
			IsHuman:       player.IsHuman,
			PresetID:      player.PresetID,
			StartingChips: player.Chips + hand.TotalContribution[player.Seat],
			EndingChips:   player.Chips,
			HoleCards:     append([]string(nil), holeCards[player.Seat]...),
			AllIn:         allIn[player.Seat],
		})
	}

	table := buildTableState(updatedPlayers, hand, nil)
	return updatedPlayers, hand, table, replay, nil
}

func buildTableState(players []Player, hand *handState, lastWinners []string) TableState {
	heroCards := []string{}
	if seat := humanSeat(players); seat >= 0 {
		heroCards = cloneStrings(hand.HoleCards[seat])
	}
	visibleHoleCards := []VisibleHoleCards{}
	if !hasHuman(players) {
		for _, player := range players {
			if player.Eliminated {
				continue
			}
			visibleHoleCards = append(visibleHoleCards, VisibleHoleCards{
				Seat:       player.Seat,
				PlayerName: player.Name,
				Cards:      cloneStrings(hand.HoleCards[player.Seat]),
			})
		}
	} else {
		for _, player := range players {
			cards, ok := hand.RevealedCards[player.Seat]
			if !ok || len(cards) == 0 {
				continue
			}
			visibleHoleCards = append(visibleHoleCards, VisibleHoleCards{
				Seat:       player.Seat,
				PlayerName: player.Name,
				Cards:      cloneStrings(cards),
			})
		}
	}
	legalActions := legalActionsForSeat(players, hand, hand.CurrentTurnSeat)
	toCall := 0
	minRaiseTo := 0
	if hand.CurrentTurnSeat >= 0 {
		toCall = max(0, hand.CurrentBet-hand.StreetContribution[hand.CurrentTurnSeat])
		if toCall < players[hand.CurrentTurnSeat].Chips {
			minRaiseTo = toCall + hand.MinRaiseSize
		}
	}

	return TableState{
		HandNumber:       hand.Number,
		Stage:            hand.Stage,
		DealerSeat:       hand.DealerSeat,
		SmallBlindSeat:   hand.SmallBlindSeat,
		BigBlindSeat:     hand.BigBlindSeat,
		CurrentTurnSeat:  hand.CurrentTurnSeat,
		Pot:              hand.Pot,
		Board:            cloneStrings(hand.Board),
		HeroCards:        heroCards,
		VisibleHoleCards: visibleHoleCards,
		ToCall:           toCall,
		MinimumRaiseTo:   minRaiseTo,
		LegalActions:     cloneActions(legalActions),
		ActionLog:        cloneActionLog(hand.ActionLog),
		DecisionLog:      cloneDecisionLog(hand.DecisionLog),
		LastWinners:      cloneStrings(lastWinners),
	}
}

func rebuildSnapshotTable(snapshot *Snapshot, hidden *hiddenState) {
	snapshot.Table = buildTableState(snapshot.Players, hidden.hand, snapshot.Table.LastWinners)
	snapshot.Table.DecisionLog = recentDecisionEntries(hidden.decisionTrail, 10)
	snapshot.Table.CompletedHands = len(hidden.replay.Hands)
}

func postContribution(player *Player, streetContribution map[int]int, totalContribution map[int]int, requested int, seat int, allIn map[int]bool) {
	amount := requested
	if amount > player.Chips {
		amount = player.Chips
	}
	player.Chips -= amount
	streetContribution[seat] += amount
	totalContribution[seat] += amount
	if player.Chips == 0 {
		allIn[seat] = true
	}
}

func firstPreflopTurnForPlayers(players []Player, dealerSeat int, bigBlindSeat int) int {
	if len(seatsWithChips(players)) == 2 {
		return dealerSeat
	}
	return nextLiveSeat(players, bigBlindSeat)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func seatsWithChips(players []Player) []int {
	seats := make([]int, 0, len(players))
	for _, player := range players {
		if !player.Eliminated && player.Chips > 0 {
			seats = append(seats, player.Seat)
		}
	}
	return seats
}

func normalizeDealerSeat(players []Player, dealerSeat int) int {
	if dealerSeat >= 0 && dealerSeat < len(players) && !players[dealerSeat].Eliminated && players[dealerSeat].Chips > 0 {
		return dealerSeat
	}
	for _, seat := range seatsWithChips(players) {
		if seat > dealerSeat {
			return seat
		}
	}
	active := seatsWithChips(players)
	if len(active) == 0 {
		return 0
	}
	return active[0]
}

func nextLiveSeat(players []Player, seat int) int {
	if len(players) == 0 {
		return 0
	}
	for step := 1; step <= len(players); step++ {
		candidate := (seat + step) % len(players)
		if !players[candidate].Eliminated && players[candidate].Chips > 0 {
			return candidate
		}
	}
	return seat
}

func blindSeatsForPlayers(players []Player, dealerSeat int) (int, int) {
	if len(seatsWithChips(players)) == 2 {
		return dealerSeat, nextLiveSeat(players, dealerSeat)
	}
	smallBlindSeat := nextLiveSeat(players, dealerSeat)
	bigBlindSeat := nextLiveSeat(players, smallBlindSeat)
	return smallBlindSeat, bigBlindSeat
}

func hasHuman(players []Player) bool {
	return humanSeat(players) >= 0
}

func humanSeat(players []Player) int {
	for _, player := range players {
		if player.IsHuman && !player.Eliminated {
			return player.Seat
		}
	}
	return -1
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string{}, values...)
}

func cloneActions(values []ActionOption) []ActionOption {
	if len(values) == 0 {
		return []ActionOption{}
	}
	return append([]ActionOption{}, values...)
}

func cloneActionLog(values []ActionLog) []ActionLog {
	if len(values) == 0 {
		return []ActionLog{}
	}
	return append([]ActionLog{}, values...)
}

func cloneDecisionLog(values []DecisionEntry) []DecisionEntry {
	if len(values) == 0 {
		return []DecisionEntry{}
	}
	return append([]DecisionEntry{}, values...)
}

func recentDecisionEntries(values []DecisionEntry, limit int) []DecisionEntry {
	if len(values) <= limit {
		return cloneDecisionLog(values)
	}
	return cloneDecisionLog(values[len(values)-limit:])
}
