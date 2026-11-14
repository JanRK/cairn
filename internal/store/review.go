package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/janrk/cairn/internal/report"
)

// ErrInvalid is returned when a status change is malformed.
var ErrInvalid = errors.New("invalid request")

// Default and maximum page sizes for List.
const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// Filter selects reports for List. Zero fields do not filter.
type Filter struct {
	Status  string
	Kind    string
	Subject string
	Since   time.Time // received_at >= Since
	After   string    // paging cursor: return ids greater than this
	Limit   int
}

// Summary is the short form of a report returned by List.
type Summary struct {
	ID          string     `json:"id"`
	Kind        string     `json:"kind"`
	Summary     string     `json:"summary"`
	SubjectType string     `json:"subject_type,omitempty"`
	Subject     string     `json:"subject,omitempty"`
	Status      string     `json:"status"`
	ReceivedAt  time.Time  `json:"received_at"`
	HandledAt   *time.Time `json:"handled_at,omitempty"`
}

// List returns reports matching f in id (arrival) order, and the cursor for
// the next page or "" when there is none.
func (s *Store) List(ctx context.Context, f Filter) ([]Summary, string, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	limit = min(limit, MaxLimit)

	var (
		where []string
		args  []any
	)
	add := func(cond string, v any) { where = append(where, cond); args = append(args, v) }
	if f.Status != "" {
		add("status = ?", f.Status)
	}
	if f.Kind != "" {
		add("kind = ?", f.Kind)
	}
	if f.Subject != "" {
		add("subject = ?", f.Subject)
	}
	if !f.Since.IsZero() {
		add("received_at >= ?", f.Since.UTC().Format(timeLayout))
	}
	if f.After != "" {
		add("id > ?", f.After)
	}
	q := `SELECT id, kind, summary, subject_type, subject, status, received_at, handled_at FROM reports`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY id LIMIT ?"
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list reports: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Summary
	for rows.Next() {
		var (
			r        Summary
			received string
			handled  *string
		)
		if err := rows.Scan(&r.ID, &r.Kind, &r.Summary, &r.SubjectType, &r.Subject, &r.Status, &received, &handled); err != nil {
			return nil, "", fmt.Errorf("scan report: %w", err)
		}
		if r.ReceivedAt, err = time.Parse(timeLayout, received); err != nil {
			return nil, "", fmt.Errorf("parse received_at: %w", err)
		}
		if handled != nil {
			t, err := time.Parse(timeLayout, *handled)
			if err != nil {
				return nil, "", fmt.Errorf("parse handled_at: %w", err)
			}
			r.HandledAt = &t
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("list reports: %w", err)
	}
	var next string
	if len(out) > limit {
		out = out[:limit]
		next = out[limit-1].ID
	}
	return out, next, nil
}

// SetStatus moves every report in ids to status, all or nothing. Entering a
// handled status from an open one sets handled_at to now; moving to an open
// status clears it. A non-empty note replaces the stored note. duplicateOf is
// required for, and only allowed with, the duplicate status.
func (s *Store) SetStatus(ctx context.Context, ids []string, status, note, duplicateOf string, now time.Time) error {
	switch {
	case len(ids) == 0:
		return fmt.Errorf("%w: ids required", ErrInvalid)
	case !report.ValidStatus(status):
		return fmt.Errorf("%w: unknown status %q", ErrInvalid, status)
	case status == report.StatusDuplicate && duplicateOf == "":
		return fmt.Errorf("%w: duplicate_of required for status duplicate", ErrInvalid)
	case status != report.StatusDuplicate && duplicateOf != "":
		return fmt.Errorf("%w: duplicate_of only allowed with status duplicate", ErrInvalid)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if duplicateOf != "" {
		for _, id := range ids {
			if id == duplicateOf {
				return fmt.Errorf("%w: report %s cannot duplicate itself", ErrInvalid, id)
			}
		}
		var one int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM reports WHERE id = ?`, duplicateOf).Scan(&one); err != nil {
			return fmt.Errorf("%w: duplicate_of %s does not exist", ErrInvalid, duplicateOf)
		}
	}

	var handled any
	if report.IsHandled(status) {
		handled = now.UTC().Format(timeLayout)
	}
	for _, id := range ids {
		res, err := tx.ExecContext(ctx, `UPDATE reports SET
			status = ?,
			handled_at = CASE WHEN ? IS NULL THEN NULL ELSE COALESCE(handled_at, ?) END,
			note = CASE WHEN ? = '' THEN note ELSE ? END,
			duplicate_of = ?
			WHERE id = ?`, status, handled, handled, note, note, duplicateOf, id)
		if err != nil {
			return fmt.Errorf("update report: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("%w: %s", ErrNotFound, id)
		}
	}
	return tx.Commit()
}

// DeleteHandledBefore deletes reports in a handled status whose handled_at is
// before cutoff, and returns how many were deleted.
func (s *Store) DeleteHandledBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM reports WHERE handled_at IS NOT NULL AND handled_at < ?`,
		cutoff.UTC().Format(timeLayout))
	if err != nil {
		return 0, fmt.Errorf("delete handled reports: %w", err)
	}
	return res.RowsAffected()
}
