package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// Store interface defines the methods for persistent storage
type Store interface {
	Close() error
	SaveObservation(agentID, content string) error
	QueryHistory(limit int) ([]Observation, error)
	SetSignal(key, value string) error
	GetSignal(key string) (string, error)
	DeleteSignal(key string) error
	SaveFeatures(features string) error // JSON blob for flexibility
	GetFeatures() (string, error)
	UpdateFeatureStatus(id string, status string, passes bool) error

	// Locking methods
	AcquireLock(path, agentID string, timeout time.Duration) (bool, error)
	ReleaseLock(path, agentID string) error
	ReleaseAllLocks(agentID string) error
	GetActiveLocks() ([]Lock, error)
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
	queries := []string{
		`CREATE TABLE IF NOT EXISTS observations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			agent_id TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS signals (
			key TEXT PRIMARY KEY,
			value TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS project_features (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			content TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS file_locks (
			path TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			lock_type TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL
		);`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return err
		}
	}
	return nil
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

// SetSignal sets a signal key-value pair
func (s *SQLiteStore) SetSignal(key, value string) error {
	query := `INSERT OR REPLACE INTO signals (key, value, created_at) VALUES (?, ?, ?)`
	_, err := s.db.Exec(query, key, value, time.Now())
	return err
}

// GetSignal retrieves a signal value by key
func (s *SQLiteStore) GetSignal(key string) (string, error) {
	query := `SELECT value FROM signals WHERE key = ?`
	row := s.db.QueryRow(query, key)
	var value string
	err := row.Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil // Return empty string if not found
	}
	return value, err
}

// DeleteSignal deletes a signal by key
func (s *SQLiteStore) DeleteSignal(key string) error {
	query := `DELETE FROM signals WHERE key = ?`
	_, err := s.db.Exec(query, key)
	return err
}

// SaveFeatures saves the feature list JSON blob
func (s *SQLiteStore) SaveFeatures(features string) error {
	query := `INSERT OR REPLACE INTO project_features (id, content, updated_at) VALUES (1, ?, ?)`
	_, err := s.db.Exec(query, features, time.Now())
	return err
}

// GetFeatures retrieves the feature list JSON blob
func (s *SQLiteStore) GetFeatures() (string, error) {
	query := `SELECT content FROM project_features WHERE id = 1`
	row := s.db.QueryRow(query)
	var content string
	err := row.Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}

	return content, err
}

// UpdateFeatureStatus updates a specific feature within the JSON blob
func (s *SQLiteStore) UpdateFeatureStatus(id string, status string, passes bool) error {
	// 1. Read existing
	content, err := s.GetFeatures()
	if err != nil {
		return err
	}
	if content == "" {
		return fmt.Errorf("no features found in DB")
	}

	var fl FeatureList
	if err := json.Unmarshal([]byte(content), &fl); err != nil {
		return fmt.Errorf("failed to unmarshal features: %w", err)
	}

	// 2. Modify
	found := false
	for i := range fl.Features {
		if fl.Features[i].ID == id {
			fl.Features[i].Status = status
			fl.Features[i].Passes = passes
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("feature ID %s not found", id)
	}

	// 3. Save back
	updated, err := json.Marshal(fl)
	if err != nil {
		return err
	}

	return s.SaveFeatures(string(updated))
}

// AcquireLock attempts to acquire a lock on a path. It polls until timeout.
func (s *SQLiteStore) AcquireLock(path, agentID string, timeout time.Duration) (bool, error) {
	start := time.Now()
	for {
		// 1. Check if lock exists and is valid
		var currentAgent string
		var expiresAt time.Time
		err := s.db.QueryRow(`SELECT agent_id, expires_at FROM file_locks WHERE path = ?`, path).Scan(&currentAgent, &expiresAt)

		if err == sql.ErrNoRows {
			// No lock, try to acquire
			_, err = s.db.Exec(`INSERT INTO file_locks (path, agent_id, expires_at) VALUES (?, ?, ?)`,
				path, agentID, time.Now().Add(10*time.Minute))
			if err == nil {
				return true, nil
			}
			// If insertion failed, someone might have just taken it, retry
		} else if err == nil {
			// Lock exists
			if time.Now().After(expiresAt) {
				// Lock expired, "highjack" it
				_, err = s.db.Exec(`UPDATE file_locks SET agent_id = ?, expires_at = ?, created_at = CURRENT_TIMESTAMP WHERE path = ?`,
					agentID, time.Now().Add(10*time.Minute), path)
				if err == nil {
					return true, nil
				}
			} else if currentAgent == agentID {
				// Already held by us, renew
				_, err = s.db.Exec(`UPDATE file_locks SET expires_at = ? WHERE path = ?`,
					time.Now().Add(10*time.Minute), path)
				return err == nil, err
			}
		} else {
			return false, err
		}

		// 2. Check timeout
		if time.Since(start) >= timeout {
			return false, nil // Failed to acquire within timeout
		}

		// 3. Poll delay
		time.Sleep(500 * time.Millisecond)
	}
}

// ReleaseLock releases a lock. If agentID is "MANAGER", it can release any lock.
func (s *SQLiteStore) ReleaseLock(path, agentID string) error {
	if agentID == "MANAGER" {
		_, err := s.db.Exec(`DELETE FROM file_locks WHERE path = ?`, path)
		return err
	}
	_, err := s.db.Exec(`DELETE FROM file_locks WHERE path = ? AND agent_id = ?`, path, agentID)
	return err
}

// ReleaseAllLocks releases all locks held by an agent.
func (s *SQLiteStore) ReleaseAllLocks(agentID string) error {
	_, err := s.db.Exec(`DELETE FROM file_locks WHERE agent_id = ?`, agentID)
	return err
}

// GetActiveLocks returns all current (not expired) locks.
func (s *SQLiteStore) GetActiveLocks() ([]Lock, error) {
	rows, err := s.db.Query(`SELECT path, agent_id, expires_at FROM file_locks WHERE expires_at > ?`, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locks []Lock
	for rows.Next() {
		var l Lock
		if err := rows.Scan(&l.Path, &l.AgentID, &l.ExpiresAt); err != nil {
			return nil, err
		}
		locks = append(locks, l)
	}
	return locks, nil
}
