package match

import (
	"fmt"
	"sort"
)

type parsedCard struct {
	Rank int
	Suit byte
}

type handRank struct {
	Category int
	Values   []int
}

type sidePot struct {
	Amount        int
	EligibleSeats []int
}

func evaluateBestHand(cards []string) (handRank, string, error) {
	if len(cards) < 5 {
		return handRank{}, "", fmt.Errorf("need at least 5 cards")
	}
	parsed := make([]parsedCard, 0, len(cards))
	for _, card := range cards {
		item, err := parseCard(card)
		if err != nil {
			return handRank{}, "", err
		}
		parsed = append(parsed, item)
	}

	best := handRank{Category: -1}
	for a := 0; a < len(parsed)-4; a++ {
		for b := a + 1; b < len(parsed)-3; b++ {
			for c := b + 1; c < len(parsed)-2; c++ {
				for d := c + 1; d < len(parsed)-1; d++ {
					for e := d + 1; e < len(parsed); e++ {
						rank := evaluateFiveCards([]parsedCard{parsed[a], parsed[b], parsed[c], parsed[d], parsed[e]})
						if compareHandRank(rank, best) > 0 {
							best = rank
						}
					}
				}
			}
		}
	}

	return best, rankLabel(best.Category), nil
}

func evaluateFiveCards(cards []parsedCard) handRank {
	ranks := make([]int, 0, 5)
	counts := map[int]int{}
	flush := true
	for index, card := range cards {
		ranks = append(ranks, card.Rank)
		counts[card.Rank]++
		if index > 0 && card.Suit != cards[0].Suit {
			flush = false
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(ranks)))
	straight, high := straightHigh(ranks)
	if flush && straight {
		return handRank{Category: 8, Values: []int{high}}
	}

	type group struct{ Rank, Count int }
	groups := make([]group, 0, len(counts))
	for rank, count := range counts {
		groups = append(groups, group{Rank: rank, Count: count})
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Count != groups[j].Count {
			return groups[i].Count > groups[j].Count
		}
		return groups[i].Rank > groups[j].Rank
	})

	if groups[0].Count == 4 {
		return handRank{Category: 7, Values: []int{groups[0].Rank, groups[1].Rank}}
	}
	if groups[0].Count == 3 && groups[1].Count == 2 {
		return handRank{Category: 6, Values: []int{groups[0].Rank, groups[1].Rank}}
	}
	if flush {
		return handRank{Category: 5, Values: ranks}
	}
	if straight {
		return handRank{Category: 4, Values: []int{high}}
	}
	if groups[0].Count == 3 {
		kickers := collectRanksExcluding(ranks, groups[0].Rank)
		return handRank{Category: 3, Values: append([]int{groups[0].Rank}, kickers...)}
	}
	if groups[0].Count == 2 && groups[1].Count == 2 {
		kicker := groups[2].Rank
		highPair := max(groups[0].Rank, groups[1].Rank)
		lowPair := min(groups[0].Rank, groups[1].Rank)
		return handRank{Category: 2, Values: []int{highPair, lowPair, kicker}}
	}
	if groups[0].Count == 2 {
		kickers := collectRanksExcluding(ranks, groups[0].Rank)
		return handRank{Category: 1, Values: append([]int{groups[0].Rank}, kickers...)}
	}
	return handRank{Category: 0, Values: ranks}
}

func compareHandRank(left handRank, right handRank) int {
	if left.Category != right.Category {
		if left.Category > right.Category {
			return 1
		}
		return -1
	}
	maxLen := len(left.Values)
	if len(right.Values) > maxLen {
		maxLen = len(right.Values)
	}
	for i := 0; i < maxLen; i++ {
		lv, rv := 0, 0
		if i < len(left.Values) {
			lv = left.Values[i]
		}
		if i < len(right.Values) {
			rv = right.Values[i]
		}
		if lv == rv {
			continue
		}
		if lv > rv {
			return 1
		}
		return -1
	}
	return 0
}

func buildSidePots(totalContribution map[int]int, players []Player, folded map[int]bool) []sidePot {
	levels := []int{}
	seen := map[int]struct{}{}
	for seat, amount := range totalContribution {
		if amount <= 0 || players[seat].Eliminated {
			continue
		}
		if _, ok := seen[amount]; ok {
			continue
		}
		levels = append(levels, amount)
		seen[amount] = struct{}{}
	}
	sort.Ints(levels)
	previous := 0
	pots := []sidePot{}
	for _, level := range levels {
		participants := []int{}
		for seat, amount := range totalContribution {
			if amount >= level {
				participants = append(participants, seat)
			}
		}
		eligible := []int{}
		for _, seat := range participants {
			if !folded[seat] && !players[seat].Eliminated {
				eligible = append(eligible, seat)
			}
		}
		amount := (level - previous) * len(participants)
		if amount > 0 && len(eligible) > 0 {
			pots = append(pots, sidePot{Amount: amount, EligibleSeats: eligible})
		}
		previous = level
	}
	return pots
}

func resolvePotWinners(seats []int, results map[int]handRank) []int {
	best := []int{}
	var bestRank handRank
	for index, seat := range seats {
		if index == 0 || compareHandRank(results[seat], bestRank) > 0 {
			best = []int{seat}
			bestRank = results[seat]
			continue
		}
		if compareHandRank(results[seat], bestRank) == 0 {
			best = append(best, seat)
		}
	}
	return best
}

func orderSeatsFromDealer(seats []int, dealerSeat int, playerCount int) []int {
	ordered := append([]int(nil), seats...)
	sort.Slice(ordered, func(i, j int) bool {
		left := (ordered[i] - dealerSeat + playerCount) % playerCount
		right := (ordered[j] - dealerSeat + playerCount) % playerCount
		return left < right
	})
	return ordered
}

func parseCard(card string) (parsedCard, error) {
	if len(card) != 2 {
		return parsedCard{}, fmt.Errorf("invalid card %q", card)
	}
	rankMap := map[byte]int{'2': 2, '3': 3, '4': 4, '5': 5, '6': 6, '7': 7, '8': 8, '9': 9, 'T': 10, 'J': 11, 'Q': 12, 'K': 13, 'A': 14}
	rank, ok := rankMap[card[0]]
	if !ok {
		return parsedCard{}, fmt.Errorf("invalid card rank %q", card)
	}
	return parsedCard{Rank: rank, Suit: card[1]}, nil
}

func rankLabel(category int) string {
	labels := map[int]string{
		8: "同花顺",
		7: "四条",
		6: "葫芦",
		5: "同花",
		4: "顺子",
		3: "三条",
		2: "两对",
		1: "一对",
		0: "高牌",
	}
	return labels[category]
}

func straightHigh(ranks []int) (bool, int) {
	unique := []int{}
	seen := map[int]struct{}{}
	for _, rank := range ranks {
		if _, ok := seen[rank]; ok {
			continue
		}
		seen[rank] = struct{}{}
		unique = append(unique, rank)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(unique)))
	if len(unique) < 5 {
		return false, 0
	}
	for index := 0; index <= len(unique)-5; index++ {
		window := unique[index : index+5]
		if window[0]-window[4] == 4 {
			return true, window[0]
		}
	}
	if containsAll(unique, []int{14, 5, 4, 3, 2}) {
		return true, 5
	}
	return false, 0
}

func containsAll(values []int, target []int) bool {
	set := map[int]struct{}{}
	for _, value := range values {
		set[value] = struct{}{}
	}
	for _, value := range target {
		if _, ok := set[value]; !ok {
			return false
		}
	}
	return true
}

func collectRanksExcluding(ranks []int, excluded int) []int {
	values := []int{}
	for _, rank := range ranks {
		if rank == excluded {
			continue
		}
		values = append(values, rank)
	}
	return values
}
