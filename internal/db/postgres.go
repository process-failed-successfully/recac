package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// PostgresStore implements Store using PostgreSQL
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new Postgres store and applies migrations
func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &PostgresStore{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return store, nil
}

func (s *PostgresStore) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS observations (
			id SERIAL PRIMARY KEY,
			project_id TEXT NOT NULL DEFAULT 'default',
			agent_id TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS signals (
			key TEXT PRIMARY KEY,
			value TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS project_features (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			content TEXT NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS file_locks (
			path TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			lock_type TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP NOT NULL
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
func (s *PostgresStore) Close() error {
	return s.db.Close()
}

// SaveObservation saves a new observation
func (s *PostgresStore) SaveObservation(projectID, agentID, content string) error {
	query := `INSERT INTO observations (project_id, agent_id, content, created_at) VALUES ($1, $2, $3, NOW())`
	_, err := s.db.Exec(query, projectID, agentID, content)
	return err
}

// QueryHistory retrieves the most recent observations for a specific project
func (s *PostgresStore) QueryHistory(projectID string, limit int) ([]Observation, error) {
	query := `SELECT id, agent_id, content, created_at FROM observations WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2`
	rows, err := s.db.Query(query, projectID, limit)
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
func (s *PostgresStore) SetSignal(key, value string) error {
	query := `INSERT INTO signals (key, value, created_at) VALUES ($1, $2, NOW()) 
			  ON CONFLICT (key) DO UPDATE SET value = $2, created_at = NOW()`
	_, err := s.db.Exec(query, key, value)
	return err
}

// GetSignal retrieves a signal value by key
func (s *PostgresStore) GetSignal(key string) (string, error) {
	query := `SELECT value FROM signals WHERE key = $1`
	row := s.db.QueryRow(query, key)
	var value string
	err := row.Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil // Return empty string if not found
	}
	return value, err
}

// DeleteSignal deletes a signal by key
func (s *PostgresStore) DeleteSignal(key string) error {
	query := `DELETE FROM signals WHERE key = $1`
	_, err := s.db.Exec(query, key)
	return err
}

// SaveFeatures saves the feature list JSON blob
func (s *PostgresStore) SaveFeatures(features string) error {
	query := `INSERT INTO project_features (id, content, updated_at) VALUES (1, $1, NOW())
			  ON CONFLICT (id) DO UPDATE SET content = $1, updated_at = NOW()`
	_, err := s.db.Exec(query, features)
	return err
}

// GetFeatures retrieves the feature list JSON blob
func (s *PostgresStore) GetFeatures() (string, error) {
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
// Uses optimistic locking logic (read-modify-write) because managing JSONB updates for arrays is complex
// and we want compatibility with the interface logic.
func (s *PostgresStore) UpdateFeatureStatus(id string, status string, passes bool) error {
	// Transaction for safety
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Read existing (FOR UPDATE to lock row)
	var content string
	err = tx.QueryRow(`SELECT content FROM project_features WHERE id = 1 FOR UPDATE`).Scan(&content)
	if err != nil {
		return err
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

	_, err = tx.Exec(`UPDATE project_features SET content = $1, updated_at = NOW() WHERE id = 1`, string(updated))
	if err != nil {
		return err
	}

	return tx.Commit()
}

// AcquireLock attempts to acquire a lock on a path. It polls until timeout.
func (s *PostgresStore) AcquireLock(path, agentID string, timeout time.Duration) (bool, error) {
	start := time.Now()
	for {
		// 1. Check if lock exists and is valid
		var currentAgent string
		var expiresAt time.Time
		err := s.db.QueryRow(`SELECT agent_id, expires_at FROM file_locks WHERE path = $1`, path).Scan(&currentAgent, &expiresAt)

		if err == sql.ErrNoRows {
			// No lock, try to acquire
			_, err = s.db.Exec(`INSERT INTO file_locks (path, agent_id, expires_at) VALUES ($1, $2, $3)`,
				path, agentID, time.Now().Add(10*time.Minute))
			if err == nil {
				return true, nil
			}
			// If insertion failed, someone might have just taken it, retry
		} else if err == nil {
			// Lock exists
			if time.Now().After(expiresAt) {
				// Lock expired, "highjack" it
				_, err = s.db.Exec(`UPDATE file_locks SET agent_id = $1, expires_at = $2, created_at = NOW() WHERE path = $3`,
					agentID, time.Now().Add(10*time.Minute), path)
				if err == nil {
					return true, nil
				}
			} else if currentAgent == agentID {
				// Already held by us, renew
				_, err = s.db.Exec(`UPDATE file_locks SET expires_at = $1 WHERE path = $2`,
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
func (s *PostgresStore) ReleaseLock(path, agentID string) error {
	if agentID == "MANAGER" {
		_, err := s.db.Exec(`DELETE FROM file_locks WHERE path = $1`, path)
		return err
	}
	_, err := s.db.Exec(`DELETE FROM file_locks WHERE path = $1 AND agent_id = $2`, path, agentID)
	return err
}

// ReleaseAllLocks releases all locks held by an agent.
func (s *PostgresStore) ReleaseAllLocks(agentID string) error {
	_, err := s.db.Exec(`DELETE FROM file_locks WHERE agent_id = $1`, agentID)
	return err
}

// GetActiveLocks returns all current (not expired) locks.
func (s *PostgresStore) GetActiveLocks() ([]Lock, error) {
	rows, err := s.db.Query(`SELECT path, agent_id, expires_at FROM file_locks WHERE expires_at > NOW()`)
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

// Cleanup removes expired locks and old observations/signals.
func (s *PostgresStore) Cleanup() error {
	// 1. Remove expired locks
	_, err := s.db.Exec(`DELETE FROM file_locks WHERE expires_at < NOW()`)
	if err != nil {
		return fmt.Errorf("failed to clean expired locks: %w", err)
	}

	// 2. Remove old signals (older than 24h, keeping critical ones)
	criticalSignals := "'PROJECT_SIGNED_OFF', 'QA_PASSED', 'COMPLETED'"
	_, err = s.db.Exec(fmt.Sprintf(`DELETE FROM signals WHERE created_at < NOW() - INTERVAL '1 day' AND key NOT IN (%s)`, criticalSignals))
	if err != nil {
		return fmt.Errorf("failed to clean old signals: %w", err)
	}

	// 3. Trim observations (Keep last 10000)
	// Postgres supports DELETE ... USING ... or subquery
	_, err = s.db.Exec(`DELETE FROM observations WHERE id NOT IN (SELECT id FROM observations ORDER BY created_at DESC LIMIT 10000)`)
	if err != nil {
		return fmt.Errorf("failed to trim observations: %w", err)
	}

	return nil
}
