package state

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type GeneratedFile struct {
	ID            int
	SourceRepo    string
	ContentType   string
	OutputPath    string
	SourceCommits []string
	CreatedAt     time.Time
}

func Open(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening state db: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating state db: %w", err)
	}

	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS processing_state (
			source_repo TEXT NOT NULL,
			content_type TEXT NOT NULL,
			last_commit_hash TEXT NOT NULL,
			last_commit_timestamp TEXT NOT NULL,
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (source_repo, content_type)
		);

		CREATE TABLE IF NOT EXISTS generated_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_repo TEXT NOT NULL,
			content_type TEXT NOT NULL,
			output_path TEXT NOT NULL,
			source_commits TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
	`)
	return err
}

func (s *Store) GetLastCommit(sourceRepo, contentType string) (string, error) {
	var hash string
	err := s.db.QueryRow(
		"SELECT last_commit_hash FROM processing_state WHERE source_repo = ? AND content_type = ?",
		sourceRepo, contentType,
	).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return hash, err
}

func (s *Store) SetLastCommit(sourceRepo, contentType, hash string, ts time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO processing_state (source_repo, content_type, last_commit_hash, last_commit_timestamp, updated_at)
		VALUES (?, ?, ?, ?, datetime('now'))
		ON CONFLICT (source_repo, content_type)
		DO UPDATE SET last_commit_hash = excluded.last_commit_hash,
		              last_commit_timestamp = excluded.last_commit_timestamp,
		              updated_at = datetime('now')
	`, sourceRepo, contentType, hash, ts.Format(time.RFC3339))
	return err
}

func (s *Store) RecordGeneration(sourceRepo, contentType, outputPath string, commitHashes []string) error {
	commitsJSON, err := json.Marshal(commitHashes)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		"INSERT INTO generated_files (source_repo, content_type, output_path, source_commits) VALUES (?, ?, ?, ?)",
		sourceRepo, contentType, outputPath, string(commitsJSON),
	)
	return err
}

func (s *Store) GetSourceRepo(contentType string) (string, error) {
	var repo string
	err := s.db.QueryRow(
		"SELECT source_repo FROM processing_state WHERE content_type = ? LIMIT 1",
		contentType,
	).Scan(&repo)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return repo, err
}

func (s *Store) GetAllState() ([]map[string]string, error) {
	rows, err := s.db.Query("SELECT source_repo, content_type, last_commit_hash, last_commit_timestamp, updated_at FROM processing_state ORDER BY updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]string
	for rows.Next() {
		var repo, ctype, hash, ts, updated string
		if err := rows.Scan(&repo, &ctype, &hash, &ts, &updated); err != nil {
			return nil, err
		}
		results = append(results, map[string]string{
			"source_repo":  repo,
			"content_type": ctype,
			"last_commit":  hash,
			"timestamp":    ts,
			"updated_at":   updated,
		})
	}
	return results, rows.Err()
}

func (s *Store) Close() error {
	return s.db.Close()
}
