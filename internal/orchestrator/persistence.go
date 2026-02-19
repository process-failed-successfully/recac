package orchestrator

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "modernc.org/sqlite"
)

type Persistence interface {
	Init() error
	SaveJob(job JobInfo) error
	GetJob(id string) (*JobInfo, error)
	GetJobs(limit int) ([]JobInfo, error)
	Close() error
}

type SQLitePersistence struct {
	dbPath string
	db     *sql.DB
}

func NewSQLitePersistence(path string) *SQLitePersistence {
	return &SQLitePersistence{dbPath: path}
}

func (p *SQLitePersistence) Init() error {
	var err error
	p.db, err = sql.Open("sqlite", p.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Create table if not exists
	query := `
	CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		created_at TIMESTAMP,
		status TEXT,
		json_blob TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at DESC);
	`
	if _, err := p.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

func (p *SQLitePersistence) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

func (p *SQLitePersistence) SaveJob(job JobInfo) error {
	if p.db == nil {
		return fmt.Errorf("database not initialized")
	}

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	query := `
	INSERT INTO jobs (id, created_at, status, json_blob)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		created_at = excluded.created_at,
		status = excluded.status,
		json_blob = excluded.json_blob;
	`
	_, err = p.db.Exec(query, job.ID, job.StartTime, job.Status, string(data))
	if err != nil {
		return fmt.Errorf("failed to save job: %w", err)
	}
	return nil
}

func (p *SQLitePersistence) GetJob(id string) (*JobInfo, error) {
	if p.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var data string
	err := p.db.QueryRow("SELECT json_blob FROM jobs WHERE id = ?", id).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job %s not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query job: %w", err)
	}

	var job JobInfo
	if err := json.Unmarshal([]byte(data), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}
	return &job, nil
}

func (p *SQLitePersistence) GetJobs(limit int) ([]JobInfo, error) {
	if p.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := p.db.Query("SELECT json_blob FROM jobs ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query jobs: %w", err)
	}
	defer rows.Close()

	var jobs []JobInfo
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}
		var job JobInfo
		if err := json.Unmarshal([]byte(data), &job); err != nil {
			return nil, fmt.Errorf("failed to unmarshal job: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}
