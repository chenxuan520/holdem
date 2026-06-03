package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		  status TEXT NOT NULL DEFAULT 'finished',
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
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`ALTER TABLE replays ADD COLUMN status TEXT NOT NULL DEFAULT 'finished'`); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
		return err
	}
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS active_matches (
		  id TEXT PRIMARY KEY,
		  status TEXT NOT NULL,
		  created_at TEXT NOT NULL,
		  updated_at TEXT NOT NULL,
		  payload_json TEXT NOT NULL
		)
	`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS tournaments (
		  id TEXT PRIMARY KEY,
		  status TEXT NOT NULL,
		  created_at TEXT NOT NULL,
		  updated_at TEXT NOT NULL,
		  payload_json TEXT NOT NULL
		)
	`)
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLiteReplayStore) SaveReplay(replay match.ReplayDetail) error {
	payload, err := json.Marshal(replay)
	if err != nil {
		return err
	}
	status := replay.Summary.Status
	if status == "" {
		status = "finished"
	}
	_, err = s.db.Exec(`
		INSERT INTO replays (id, status, created_at, finished_at, winner_name, player_count, hands_played, initial_chips, small_blind, big_blind, payload_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		  status = excluded.status,
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
		status,
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
		SELECT id, status, created_at, finished_at, winner_name, player_count, hands_played, initial_chips, small_blind, big_blind
		FROM replays ORDER BY finished_at DESC, created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []match.ReplaySummary
	for rows.Next() {
		var summary match.ReplaySummary
		var status string
		var createdAt, finishedAt string
		if err := rows.Scan(&summary.ID, &status, &createdAt, &finishedAt, &summary.WinnerName, &summary.PlayerCount, &summary.HandsPlayed, &summary.InitialChips, &summary.SmallBlind, &summary.BigBlind); err != nil {
			return nil, err
		}
		summary.Status = status
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

func (s *SQLiteReplayStore) SaveActiveMatch(record match.ActiveMatchRecord) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	status := record.Snapshot.Status
	if status == "" {
		status = "running"
	}
	_, err = s.db.Exec(`
		INSERT INTO active_matches (id, status, created_at, updated_at, payload_json)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		  status = excluded.status,
		  created_at = excluded.created_at,
		  updated_at = excluded.updated_at,
		  payload_json = excluded.payload_json
	`,
		record.Snapshot.ID,
		status,
		record.Snapshot.CreatedAt.Format(timeLayout),
		record.Snapshot.UpdatedAt.Format(timeLayout),
		string(payload),
	)
	return err
}

func (s *SQLiteReplayStore) ListActiveMatches() ([]match.ActiveMatchRecord, error) {
	rows, err := s.db.Query(`SELECT payload_json FROM active_matches ORDER BY updated_at DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := []match.ActiveMatchRecord{}
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var record match.ActiveMatchRecord
		if err := json.Unmarshal([]byte(payload), &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *SQLiteReplayStore) DeleteActiveMatch(id string) error {
	_, err := s.db.Exec(`DELETE FROM active_matches WHERE id = ?`, id)
	return err
}

func (s *SQLiteReplayStore) ClearActiveMatches() error {
	_, err := s.db.Exec(`DELETE FROM active_matches`)
	return err
}

// SaveTournament / ListTournaments / DeleteTournament implement the optional
// tournament persistence extension consumed by match.Service (matched
// structurally). The whole TournamentDetail rides in payload_json so new
// fields round-trip without a schema migration.
func (s *SQLiteReplayStore) SaveTournament(detail match.TournamentDetail) error {
	payload, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	status := detail.Status
	if status == "" {
		status = "running"
	}
	_, err = s.db.Exec(`
		INSERT INTO tournaments (id, status, created_at, updated_at, payload_json)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		  status = excluded.status,
		  updated_at = excluded.updated_at,
		  payload_json = excluded.payload_json
	`,
		detail.ID,
		status,
		detail.CreatedAt.Format(timeLayout),
		detail.UpdatedAt.Format(timeLayout),
		string(payload),
	)
	return err
}

func (s *SQLiteReplayStore) ListTournaments() ([]match.TournamentDetail, error) {
	rows, err := s.db.Query(`SELECT payload_json FROM tournaments ORDER BY updated_at DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	details := []match.TournamentDetail{}
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var detail match.TournamentDetail
		if err := json.Unmarshal([]byte(payload), &detail); err != nil {
			return nil, err
		}
		details = append(details, detail)
	}
	return details, rows.Err()
}

func (s *SQLiteReplayStore) DeleteTournament(id string) error {
	_, err := s.db.Exec(`DELETE FROM tournaments WHERE id = ?`, id)
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
