package match

type ReplayStore interface {
	SaveReplay(replay ReplayDetail) error
	ListReplays() ([]ReplaySummary, error)
	GetReplay(id string) (ReplayDetail, bool, error)
	DeleteReplay(id string) error
	ClearReplays() error
	SaveActiveMatch(record ActiveMatchRecord) error
	ListActiveMatches() ([]ActiveMatchRecord, error)
	DeleteActiveMatch(id string) error
	ClearActiveMatches() error
}
