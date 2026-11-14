// Package store persists reports in SQLite.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/janrk/cairn/internal/report"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// ErrNotFound is returned when a report does not exist.
var ErrNotFound = errors.New("report not found")

const schema = `
CREATE TABLE IF NOT EXISTS reports (
	id            TEXT PRIMARY KEY,
	kind          TEXT NOT NULL,
	summary       TEXT NOT NULL,
	details       TEXT NOT NULL,
	subject_type  TEXT NOT NULL DEFAULT '',
	subject       TEXT NOT NULL DEFAULT '',
	evidence      TEXT NOT NULL DEFAULT '',
	suggested_fix TEXT NOT NULL DEFAULT '',
	agent         TEXT NOT NULL DEFAULT '',
	harness       TEXT NOT NULL DEFAULT '',
	model         TEXT NOT NULL DEFAULT '',
	workspace     TEXT NOT NULL DEFAULT '',
	received_at   TEXT NOT NULL,
	status        TEXT NOT NULL,
	source        TEXT NOT NULL,
	handled_at    TEXT,
	note          TEXT NOT NULL DEFAULT '',
	duplicate_of  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS reports_status ON reports(status);
CREATE INDEX IF NOT EXISTS reports_handled_at ON reports(handled_at);
`

// Fixed width so stored timestamps sort lexically.
const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

// Store is a SQLite-backed report store.
type Store struct {
	db *sql.DB
}

// Open opens (and migrates) the database at path.
func Open(ctx context.Context, path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// One process, one writer: a single connection avoids SQLITE_BUSY.
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate db: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Ping checks that the database answers.
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// Create inserts a report.
func (s *Store) Create(ctx context.Context, r report.Report) error {
	var handled any
	if r.HandledAt != nil {
		handled = r.HandledAt.UTC().Format(timeLayout)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO reports
		(id, kind, summary, details, subject_type, subject, evidence, suggested_fix,
		 agent, harness, model, workspace, received_at, status, source, handled_at, note, duplicate_of)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.Kind, r.Summary, r.Details, r.SubjectType, r.Subject, r.Evidence, r.SuggestedFix,
		r.Agent, r.Harness, r.Model, r.Workspace, r.ReceivedAt.UTC().Format(timeLayout),
		r.Status, r.Source, handled, r.Note, r.DuplicateOf)
	if err != nil {
		return fmt.Errorf("insert report: %w", err)
	}
	return nil
}

// Get returns the full report with the given id.
func (s *Store) Get(ctx context.Context, id string) (report.Report, error) {
	var (
		r        report.Report
		received string
		handled  sql.NullString
	)
	err := s.db.QueryRowContext(ctx, `SELECT id, kind, summary, details, subject_type, subject,
		evidence, suggested_fix, agent, harness, model, workspace, received_at, status, source,
		handled_at, note, duplicate_of FROM reports WHERE id = ?`, id).Scan(
		&r.ID, &r.Kind, &r.Summary, &r.Details, &r.SubjectType, &r.Subject, &r.Evidence,
		&r.SuggestedFix, &r.Agent, &r.Harness, &r.Model, &r.Workspace, &received, &r.Status,
		&r.Source, &handled, &r.Note, &r.DuplicateOf)
	if errors.Is(err, sql.ErrNoRows) {
		return report.Report{}, ErrNotFound
	}
	if err != nil {
		return report.Report{}, fmt.Errorf("get report: %w", err)
	}
	if r.ReceivedAt, err = time.Parse(timeLayout, received); err != nil {
		return report.Report{}, fmt.Errorf("parse received_at: %w", err)
	}
	if handled.Valid {
		t, err := time.Parse(timeLayout, handled.String)
		if err != nil {
			return report.Report{}, fmt.Errorf("parse handled_at: %w", err)
		}
		r.HandledAt = &t
	}
	return r, nil
}
