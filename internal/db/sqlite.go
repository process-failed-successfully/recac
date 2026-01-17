package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// SQLiteStore implements Store using SQLite
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite store and applies migrations
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	// Enable WAL mode and 5s busy timeout for concurrency
	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
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
	// 1. Initial creation
	queries := []string{
		`CREATE TABLE IF NOT EXISTS observations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id TEXT NOT NULL DEFAULT 'default',
			agent_id TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS signals (
			project_id TEXT NOT NULL DEFAULT 'default',
			key TEXT NOT NULL,
			value TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (project_id, key)
		);`,
		`CREATE TABLE IF NOT EXISTS project_features (
			project_id TEXT PRIMARY KEY,
			content TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS project_specs (
			project_id TEXT PRIMARY KEY,
			content TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS file_locks (
			project_id TEXT NOT NULL DEFAULT 'default',
			path TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			lock_type TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL,
			PRIMARY KEY (project_id, path)
		);`,
		// Performance Indexes
		`CREATE INDEX IF NOT EXISTS idx_observations_project_created ON observations (project_id, created_at DESC);`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			// If it failed, we'll try fine-grained fixes
		}
	}

	// 2. Fine-grained column additions for older installations
	// Note: SQLite doesn't support ADD COLUMN IF NOT EXISTS, so we ignore errors if they exist.
	_, _ = s.db.Exec(`ALTER TABLE observations ADD COLUMN project_id TEXT NOT NULL DEFAULT 'default'`)
	_, _ = s.db.Exec(`ALTER TABLE signals ADD COLUMN project_id TEXT NOT NULL DEFAULT 'default'`)
	_, _ = s.db.Exec(`ALTER TABLE file_locks ADD COLUMN project_id TEXT NOT NULL DEFAULT 'default'`)

	// PK changes in SQLite require table recreation, which is risky to do here.
	// For now, adding the column is the most important part to avoid "no such column" errors.

	return nil
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// SaveObservation saves a new observation
func (s *SQLiteStore) SaveObservation(projectID, agentID, content string) error {
	query := `INSERT INTO observations (project_id, agent_id, content, created_at) VALUES (?, ?, ?, ?)`
	_, err := s.db.Exec(query, projectID, agentID, content, time.Now())
	return err
}

// QueryHistory retrieves the most recent observations for a specific project
func (s *SQLiteStore) QueryHistory(projectID string, limit int) ([]Observation, error) {
	query := `SELECT id, agent_id, content, created_at FROM observations WHERE project_id = ? ORDER BY created_at DESC LIMIT ?`
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
func (s *SQLiteStore) SetSignal(projectID, key, value string) error {
	query := `INSERT OR REPLACE INTO signals (project_id, key, value, created_at) VALUES (?, ?, ?, ?)`
	_, err := s.db.Exec(query, projectID, key, value, time.Now())
	return err
}

// GetSignal retrieves a signal value by key
func (s *SQLiteStore) GetSignal(projectID, key string) (string, error) {
	query := `SELECT value FROM signals WHERE project_id = ? AND key = ?`
	row := s.db.QueryRow(query, projectID, key)
	var value string
	err := row.Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil // Return empty string if not found
	}
	return value, err
}

// DeleteSignal deletes a signal by key
func (s *SQLiteStore) DeleteSignal(projectID, key string) error {
	query := `DELETE FROM signals WHERE project_id = ? AND key = ?`
	_, err := s.db.Exec(query, projectID, key)
	return err
}

// SaveFeatures saves the feature list JSON blob
func (s *SQLiteStore) SaveFeatures(projectID string, features string) error {
	query := `INSERT OR REPLACE INTO project_features (project_id, content, updated_at) VALUES (?, ?, ?)`
	_, err := s.db.Exec(query, projectID, features, time.Now())
	return err
}

// GetFeatures retrieves the feature list JSON blob
func (s *SQLiteStore) GetFeatures(projectID string) (string, error) {
	query := `SELECT content FROM project_features WHERE project_id = ?`
	row := s.db.QueryRow(query, projectID)
	var content string
	err := row.Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}

	return content, err
}

// SaveSpec saves the application specification content
func (s *SQLiteStore) SaveSpec(projectID string, spec string) error {
	query := `INSERT OR REPLACE INTO project_specs (project_id, content, updated_at) VALUES (?, ?, ?)`
	_, err := s.db.Exec(query, projectID, spec, time.Now())
	return err
}

// GetSpec retrieves the application specification content
func (s *SQLiteStore) GetSpec(projectID string) (string, error) {
	query := `SELECT content FROM project_specs WHERE project_id = ?`
	row := s.db.QueryRow(query, projectID)
	var content string
	err := row.Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}

	return content, err
}

// UpdateFeatureStatus updates a specific feature within the JSON blob
func (s *SQLiteStore) UpdateFeatureStatus(projectID string, id string, status string, passes bool) error {
	// 1. Read existing
	content, err := s.GetFeatures(projectID)
	if err != nil {
		return err
	}
	if content == "" {
		return fmt.Errorf("no features found in DB for project %s", projectID)
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

	return s.SaveFeatures(projectID, string(updated))
}

// AcquireLock attempts to acquire a lock on a path. It polls until timeout.
func (s *SQLiteStore) AcquireLock(projectID, path, agentID string, timeout time.Duration) (bool, error) {
	start := time.Now()
	for {
		// 1. Check if lock exists and is valid
		var currentAgent string
		var expiresAt time.Time
		err := s.db.QueryRow(`SELECT agent_id, expires_at FROM file_locks WHERE project_id = ? AND path = ?`, projectID, path).Scan(&currentAgent, &expiresAt)

		if err == sql.ErrNoRows {
			// No lock, try to acquire
			_, err = s.db.Exec(`INSERT INTO file_locks (project_id, path, agent_id, expires_at) VALUES (?, ?, ?, ?)`,
				projectID, path, agentID, time.Now().Add(10*time.Minute))
			if err == nil {
				return true, nil
			}
			// If insertion failed, someone might have just taken it, retry
		} else if err == nil {
			// Lock exists
			if time.Now().After(expiresAt) {
				// Lock expired, "highjack" it
				_, err = s.db.Exec(`UPDATE file_locks SET agent_id = ?, expires_at = ?, created_at = CURRENT_TIMESTAMP WHERE project_id = ? AND path = ?`,
					agentID, time.Now().Add(10*time.Minute), projectID, path)
				if err == nil {
					return true, nil
				}
			} else if currentAgent == agentID {
				// Already held by us, renew
				_, err = s.db.Exec(`UPDATE file_locks SET expires_at = ? WHERE project_id = ? AND path = ?`,
					time.Now().Add(10*time.Minute), projectID, path)
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
func (s *SQLiteStore) ReleaseLock(projectID, path, agentID string) error {
	if agentID == "MANAGER" {
		_, err := s.db.Exec(`DELETE FROM file_locks WHERE project_id = ? AND path = ?`, projectID, path)
		return err
	}
	_, err := s.db.Exec(`DELETE FROM file_locks WHERE project_id = ? AND path = ? AND agent_id = ?`, projectID, path, agentID)
	return err
}

// ReleaseAllLocks releases all locks held by an agent.
func (s *SQLiteStore) ReleaseAllLocks(projectID, agentID string) error {
	_, err := s.db.Exec(`DELETE FROM file_locks WHERE project_id = ? AND agent_id = ?`, projectID, agentID)
	return err
}

// Cleanup removes expired locks and old observations/signals.
func (s *SQLiteStore) Cleanup() error {
	// 1. Remove expired locks
	_, err := s.db.Exec(`DELETE FROM file_locks WHERE expires_at < CURRENT_TIMESTAMP`)
	if err != nil {
		return fmt.Errorf("failed to clean expired locks: %w", err)
	}

	// 2. Remove old signals (older than 24h, keeping critical ones)
	// Critical signals: PROJECT_SIGNED_OFF, QA_PASSED, COMPLETED
	criticalSignals := "'PROJECT_SIGNED_OFF', 'QA_PASSED', 'COMPLETED'"
	_, err = s.db.Exec(fmt.Sprintf(`DELETE FROM signals WHERE created_at < datetime('now', '-1 day') AND key NOT IN (%s)`, criticalSignals))
	if err != nil {
		return fmt.Errorf("failed to clean old signals: %w", err)
	}

	// 3. Trim observations (Keep last 10000)
	// SQLite doesn't support DELETE ... LIMIT directly in all versions, but we can use subquery
	_, err = s.db.Exec(`DELETE FROM observations WHERE id NOT IN (SELECT id FROM observations ORDER BY created_at DESC LIMIT 10000)`)
	if err != nil {
		return fmt.Errorf("failed to trim observations: %w", err)
	}

	return nil
}

// GetActiveLocks returns all current (not expired) locks.
func (s *SQLiteStore) GetActiveLocks(projectID string) ([]Lock, error) {
	rows, err := s.db.Query(`SELECT path, agent_id, expires_at FROM file_locks WHERE expires_at > ? AND project_id = ?`, time.Now(), projectID)
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
