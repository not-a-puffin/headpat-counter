package store

type ScoreEntry struct {
	Rank  int64   `json:"rank"`
	Score float64 `json:"score"`
	User  string  `json:"user"`
}

type ScoreboardStore interface {
	ScoreboardIncr(boardName, userName string, points float64) error
	GetLeaderboard(boardName string, count int) ([]ScoreEntry, error)
	GetScoreByUser(boardName, userName string) (ScoreEntry, error)
	ResetScoreboard(boardName string) error
}
