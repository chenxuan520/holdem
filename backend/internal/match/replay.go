package match

import "time"

type ReplaySummary struct {
	ID           string    `json:"id"`
	Status       string    `json:"status,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	FinishedAt   time.Time `json:"finishedAt"`
	WinnerName   string    `json:"winnerName"`
	PlayerCount  int       `json:"playerCount"`
	HandsPlayed  int       `json:"handsPlayed"`
	InitialChips int       `json:"initialChips"`
	SmallBlind   int       `json:"smallBlind"`
	BigBlind     int       `json:"bigBlind"`
}

type ReplayDetail struct {
	Summary   ReplaySummary `json:"summary"`
	Players   []Player      `json:"players"`
	Hands     []ReplayHand  `json:"hands"`
	AILogs    []AILog       `json:"aiLogs"`
	CreatedAt time.Time     `json:"createdAt"`
}

type ActiveMatchRecord struct {
	Snapshot      Snapshot        `json:"snapshot"`
	Hand          *handState      `json:"hand"`
	Replay        ReplayDetail    `json:"replay"`
	Current       *ReplayHand     `json:"current"`
	DecisionTrail []DecisionEntry `json:"decisionTrail"`
}

type ReplayHand struct {
	HandNumber int                 `json:"handNumber"`
	DealerSeat int                 `json:"dealerSeat"`
	Board      []string            `json:"board"`
	Pot        int                 `json:"pot"`
	Winners    []ReplayWinner      `json:"winners"`
	Players    []ReplayPlayerState `json:"players"`
	Events     []ReplayEvent       `json:"events"`
	StartedAt  time.Time           `json:"startedAt"`
	FinishedAt time.Time           `json:"finishedAt"`
}

type ReplayWinner struct {
	Seat       int    `json:"seat"`
	PlayerName string `json:"playerName"`
	Amount     int    `json:"amount"`
	HandLabel  string `json:"handLabel"`
}

type ReplayPlayerState struct {
	Seat          int      `json:"seat"`
	Name          string   `json:"name"`
	IsHuman       bool     `json:"isHuman"`
	PresetID      string   `json:"presetId,omitempty"`
	StartingChips int      `json:"startingChips"`
	EndingChips   int      `json:"endingChips"`
	HoleCards     []string `json:"holeCards"`
	Folded        bool     `json:"folded"`
	AllIn         bool     `json:"allIn"`
	Eliminated    bool     `json:"eliminated"`
}

type ReplayEvent struct {
	Sequence   int       `json:"sequence"`
	Type       string    `json:"type"`
	Visibility string    `json:"visibility"`
	Timestamp  time.Time `json:"timestamp"`
	Payload    any       `json:"payload"`
}

type AILog struct {
	MatchID        string    `json:"matchId"`
	HandNumber     int       `json:"handNumber"`
	Seat           int       `json:"seat"`
	PlayerName     string    `json:"playerName"`
	Model          string    `json:"model"`
	Endpoint       string    `json:"endpoint"`
	RequestPayload any       `json:"requestPayload"`
	ResponseBody   string    `json:"responseBody"`
	Structured     any       `json:"structured"`
	CreatedAt      time.Time `json:"createdAt"`
	Error          string    `json:"error,omitempty"`
}

type hiddenState struct {
	hand          *handState
	replay        ReplayDetail
	current       *ReplayHand
	decisionTrail []DecisionEntry
}

type handState struct {
	Number             int
	DealerSeat         int
	SmallBlindSeat     int
	BigBlindSeat       int
	Stage              string
	Deck               []string
	Board              []string
	HoleCards          map[int][]string
	RevealedCards      map[int][]string
	Folded             map[int]bool
	AllIn              map[int]bool
	StreetContribution map[int]int
	TotalContribution  map[int]int
	CurrentBet         int
	MinRaiseSize       int
	CurrentTurnSeat    int
	Acted              map[int]bool
	Pot                int
	StartedAt          time.Time
	ActionLog          []ActionLog
	DecisionLog        []DecisionEntry
	HandOver           bool
}
