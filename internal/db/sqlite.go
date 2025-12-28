package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// Store interface defines the methods for persistent storage
type Store interface {
	Close() error
	SaveObservation(agentID, content string) error
	QueryHistory(limit int) ([]Observation, error)
}

// Observation represents a recorded event or fact
type Observation struct {
	ID        int64     `json:"id"`
	AgentID   string    `json:"agent_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// SQLiteStore implements Store using SQLite
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite store and applies migrations
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return store, nil
}

func (s *SQLiteStore) migrate() error {
	query := `
	CREATE TABLE IF NOT EXISTS observations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := s.db.Exec(query)
	return err
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// SaveObservation saves a new observation
func (s *SQLiteStore) SaveObservation(agentID, content string) error {
	query := `INSERT INTO observations (agent_id, content, created_at) VALUES (?, ?, ?)`
	_, err := s.db.Exec(query, agentID, content, time.Now())
	return err
}

// QueryHistory retrieves the most recent observations
func (s *SQLiteStore) QueryHistory(limit int) ([]Observation, error) {
	query := `SELECT id, agent_id, content, created_at FROM observations ORDER BY created_at DESC LIMIT ?`
	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Observation
	for rows.Next() {
		var obs Observation
		if err := rows.Scan(&obs.ID, &obs.AgentID, &obs.Content, &obs.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, obs)
	}
	return results, nil
}
