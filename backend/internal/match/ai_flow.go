package match

import (
	backendai "holdem/backend/internal/ai"
)

func (s *Service) buildPromptInput(snapshot Snapshot, hidden *hiddenState, seat int) backendai.PromptInput {
	players := make([]any, 0, len(snapshot.Players))
	for _, player := range snapshot.Players {
		if player.Eliminated {
			continue
		}
		players = append(players, map[string]any{
			"seat":   player.Seat,
			"name":   player.Name,
			"chips":  player.Chips,
			"folded": hidden.hand.Folded[player.Seat],
			"allIn":  hidden.hand.AllIn[player.Seat],
		})
	}

	legalActions := make([]any, 0, len(legalActionsForSeat(snapshot.Players, hidden.hand, seat)))
	for _, action := range legalActionsForSeat(snapshot.Players, hidden.hand, seat) {
		legalActions = append(legalActions, action)
	}

	recent := make([]any, 0, len(hidden.hand.ActionLog))
	start := 0
	if len(hidden.hand.ActionLog) > 8 {
		start = len(hidden.hand.ActionLog) - 8
	}
	for _, entry := range hidden.hand.ActionLog[start:] {
		recent = append(recent, entry)
	}

	return backendai.PromptInput{
		MatchID:         snapshot.ID,
		HandNumber:      hidden.hand.Number,
		Seat:            seat,
		PlayerName:      snapshot.Players[seat].Name,
		Stage:           hidden.hand.Stage,
		Board:           append([]string(nil), hidden.hand.Board...),
		HoleCards:       append([]string(nil), hidden.hand.HoleCards[seat]...),
		Pot:             hidden.hand.Pot,
		ToCall:          max(0, hidden.hand.CurrentBet-hidden.hand.StreetContribution[seat]),
		MinimumRaiseTo:  max(0, hidden.hand.CurrentBet-hidden.hand.StreetContribution[seat]) + hidden.hand.MinRaiseSize,
		LegalActions:    legalActions,
		Players:         players,
		RecentActionLog: recent,
	}
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
