package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"holdem/backend/internal/match"
)

type SQLiteReplayStore struct {
	db *sql.DB
}

func NewSQLiteReplayStore(path string) (*SQLiteReplayStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &SQLiteReplayStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteReplayStore) init() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS replays (
		  id TEXT PRIMARY KEY,
		  created_at TEXT NOT NULL,
		  finished_at TEXT NOT NULL,
		  winner_name TEXT NOT NULL,
		  player_count INTEGER NOT NULL,
		  hands_played INTEGER NOT NULL,
		  initial_chips INTEGER NOT NULL,
		  small_blind INTEGER NOT NULL,
		  big_blind INTEGER NOT NULL,
		  payload_json TEXT NOT NULL
		)
	`)
	return err
}

func (s *SQLiteReplayStore) SaveReplay(replay match.ReplayDetail) error {
	payload, err := json.Marshal(replay)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO replays (id, created_at, finished_at, winner_name, player_count, hands_played, initial_chips, small_blind, big_blind, payload_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		  created_at = excluded.created_at,
		  finished_at = excluded.finished_at,
		  winner_name = excluded.winner_name,
		  player_count = excluded.player_count,
		  hands_played = excluded.hands_played,
		  initial_chips = excluded.initial_chips,
		  small_blind = excluded.small_blind,
		  big_blind = excluded.big_blind,
		  payload_json = excluded.payload_json
	`,
		replay.Summary.ID,
		replay.Summary.CreatedAt.Format(timeLayout),
		replay.Summary.FinishedAt.Format(timeLayout),
		replay.Summary.WinnerName,
		replay.Summary.PlayerCount,
		replay.Summary.HandsPlayed,
		replay.Summary.InitialChips,
		replay.Summary.SmallBlind,
		replay.Summary.BigBlind,
		string(payload),
	)
	return err
}

func (s *SQLiteReplayStore) ListReplays() ([]match.ReplaySummary, error) {
	rows, err := s.db.Query(`
		SELECT id, created_at, finished_at, winner_name, player_count, hands_played, initial_chips, small_blind, big_blind
		FROM replays ORDER BY finished_at DESC, created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []match.ReplaySummary
	for rows.Next() {
		var summary match.ReplaySummary
		var createdAt, finishedAt string
		if err := rows.Scan(&summary.ID, &createdAt, &finishedAt, &summary.WinnerName, &summary.PlayerCount, &summary.HandsPlayed, &summary.InitialChips, &summary.SmallBlind, &summary.BigBlind); err != nil {
			return nil, err
		}
		summary.CreatedAt = mustParseTime(createdAt)
		summary.FinishedAt = mustParseTime(finishedAt)
		summaries = append(summaries, summary)
	}
	return summaries, rows.Err()
}

func (s *SQLiteReplayStore) GetReplay(id string) (match.ReplayDetail, bool, error) {
	row := s.db.QueryRow(`SELECT payload_json FROM replays WHERE id = ?`, id)
	var payload string
	if err := row.Scan(&payload); err != nil {
		if err == sql.ErrNoRows {
			return match.ReplayDetail{}, false, nil
		}
		return match.ReplayDetail{}, false, err
	}
	var replay match.ReplayDetail
	if err := json.Unmarshal([]byte(payload), &replay); err != nil {
		return match.ReplayDetail{}, false, err
	}
	return replay, true, nil
}

func (s *SQLiteReplayStore) DeleteReplay(id string) error {
	_, err := s.db.Exec(`DELETE FROM replays WHERE id = ?`, id)
	return err
}

func (s *SQLiteReplayStore) ClearReplays() error {
	_, err := s.db.Exec(`DELETE FROM replays`)
	return err
}

const timeLayout = "2006-01-02T15:04:05.999999999Z07:00"

func mustParseTime(value string) time.Time {
	parsed, err := time.Parse(timeLayout, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
