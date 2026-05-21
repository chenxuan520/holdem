package match

import (
	crand "crypto/rand"
	"fmt"
	"math/big"
)

type TableState struct {
	HandNumber     int            `json:"handNumber"`
	Stage          string         `json:"stage"`
	DealerSeat     int            `json:"dealerSeat"`
	SmallBlindSeat int            `json:"smallBlindSeat"`
	BigBlindSeat   int            `json:"bigBlindSeat"`
	CurrentTurnSeat int           `json:"currentTurnSeat"`
	Pot            int            `json:"pot"`
	Board          []string       `json:"board"`
	HeroCards      []string       `json:"heroCards"`
	VisibleHoleCards []VisibleHoleCards `json:"visibleHoleCards"`
	ToCall         int            `json:"toCall"`
	MinimumRaiseTo int            `json:"minimumRaiseTo"`
	LegalActions   []ActionOption `json:"legalActions"`
	ActionLog      []ActionLog    `json:"actionLog"`
	DecisionLog    []DecisionEntry `json:"decisionLog"`
	LastWinners    []string       `json:"lastWinners,omitempty"`
	CompletedHands int            `json:"completedHands"`
}

type ActionOption struct {
	Action string `json:"action"`
	Amount int    `json:"amount,omitempty"`
	Label  string `json:"label"`
}

type ActionLog struct {
	Seat       int    `json:"seat"`
	PlayerName string `json:"playerName"`
	Action     string `json:"action"`
	Amount     int    `json:"amount"`
	Street     string `json:"street"`
}

type VisibleHoleCards struct {
	Seat       int      `json:"seat"`
	PlayerName string   `json:"playerName"`
	Cards      []string `json:"cards"`
}

type DecisionEntry struct {
	Seat          int    `json:"seat"`
	PlayerName    string `json:"playerName"`
	Stage         string `json:"stage"`
	Action        string `json:"action"`
	Amount        int    `json:"amount"`
	PublicReason  string `json:"publicReason"`
	PrivateReason string `json:"privateReason,omitempty"`
	Model         string `json:"model,omitempty"`
	Endpoint      string `json:"endpoint,omitempty"`
}

func initTableState(players []Player, smallBlind int, bigBlind int) ([]Player, TableState, error) {
	updatedPlayers := append([]Player(nil), players...)
	playerCount := len(updatedPlayers)
	dealerSeat := initialDealerSeat(playerCount)
	smallBlindSeat, bigBlindSeat := blindSeats(dealerSeat, playerCount)

	contributions := make([]int, playerCount)
	updatedPlayers[smallBlindSeat].Chips -= smallBlind
	updatedPlayers[bigBlindSeat].Chips -= bigBlind
	contributions[smallBlindSeat] = smallBlind
	contributions[bigBlindSeat] = bigBlind

	deck, err := shuffledDeck()
	if err != nil {
		return nil, TableState{}, err
	}

	holeCards := make([][]string, playerCount)
	dealOrderStart := nextSeat(dealerSeat, playerCount)
	cardIndex := 0
	for round := 0; round < 2; round++ {
		for offset := 0; offset < playerCount; offset++ {
			seat := (dealOrderStart + offset) % playerCount
			holeCards[seat] = append(holeCards[seat], deck[cardIndex])
			cardIndex++
		}
	}

	currentTurnSeat := nextSeat(bigBlindSeat, playerCount)
	toCall := bigBlind - contributions[currentTurnSeat]
	minRaiseTo := toCall + bigBlind

	state := TableState{
		HandNumber:      1,
		Stage:           "preflop",
		DealerSeat:      dealerSeat,
		SmallBlindSeat:  smallBlindSeat,
		BigBlindSeat:    bigBlindSeat,
		CurrentTurnSeat: currentTurnSeat,
		Pot:             smallBlind + bigBlind,
		Board:           []string{},
		HeroCards:       holeCards[0],
		ToCall:          toCall,
		MinimumRaiseTo:  minRaiseTo,
		LegalActions:    buildLegalActions(updatedPlayers[currentTurnSeat], toCall, minRaiseTo),
		ActionLog: []ActionLog{
			{Seat: smallBlindSeat, PlayerName: updatedPlayers[smallBlindSeat].Name, Action: "post_small_blind", Amount: smallBlind, Street: "preflop"},
			{Seat: bigBlindSeat, PlayerName: updatedPlayers[bigBlindSeat].Name, Action: "post_big_blind", Amount: bigBlind, Street: "preflop"},
		},
	}

	return updatedPlayers, state, nil
}

func buildLegalActions(player Player, toCall int, minimumRaiseTo int) []ActionOption {
	actions := []ActionOption{}
	if toCall > 0 {
		actions = append(actions,
			ActionOption{Action: "fold", Label: "弃牌"},
			ActionOption{Action: "call", Amount: min(player.Chips, toCall), Label: fmt.Sprintf("跟注 %d", min(player.Chips, toCall))},
		)
	} else {
		actions = append(actions, ActionOption{Action: "check", Label: "过牌"})
	}

	if player.Chips > toCall {
		actions = append(actions, ActionOption{Action: "raise", Amount: min(player.Chips, minimumRaiseTo), Label: fmt.Sprintf("最小加注投入 %d", min(player.Chips, minimumRaiseTo))})
		actions = append(actions, ActionOption{Action: "all_in", Amount: player.Chips, Label: fmt.Sprintf("全下 %d", player.Chips)})
	}

	return actions
}

func legalActionsForSeat(players []Player, hand *handState, seat int) []ActionOption {
	if seat < 0 || seat >= len(players) {
		return nil
	}
	if hand.Folded[seat] || hand.AllIn[seat] || players[seat].Eliminated {
		return nil
	}
	toCall := max(0, hand.CurrentBet-hand.StreetContribution[seat])
	minimumRaiseTo := toCall + hand.MinRaiseSize
	return buildLegalActions(players[seat], toCall, minimumRaiseTo)
}

func blindSeats(dealerSeat int, playerCount int) (int, int) {
	if playerCount == 2 {
		return dealerSeat, nextSeat(dealerSeat, playerCount)
	}
	return nextSeat(dealerSeat, playerCount), nextSeat(dealerSeat+1, playerCount)
}

func initialDealerSeat(playerCount int) int {
	if playerCount <= 2 {
		return 0
	}
	return (playerCount - 3) % playerCount
}

func nextSeat(seat int, playerCount int) int {
	return (seat + 1) % playerCount
}

func shuffledDeck() ([]string, error) {
	ranks := []string{"A", "K", "Q", "J", "T", "9", "8", "7", "6", "5", "4", "3", "2"}
	suits := []string{"s", "h", "d", "c"}
	deck := make([]string, 0, len(ranks)*len(suits))
	for _, rank := range ranks {
		for _, suit := range suits {
			deck = append(deck, rank+suit)
		}
	}

	for i := len(deck) - 1; i > 0; i-- {
		pick, err := randomInt(i + 1)
		if err != nil {
			return nil, err
		}
		deck[i], deck[pick] = deck[pick], deck[i]
	}

	return deck, nil
}

func randomInt(max int) (int, error) {
	value, err := crand.Int(crand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(value.Int64()), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
