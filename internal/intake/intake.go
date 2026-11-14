// Package intake turns agent input into stored reports. Every intake surface
// (REST, MCP) goes through Service.Submit.
package intake

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"time"

	"github.com/janrk/cairn/internal/report"
	"github.com/oklog/ulid/v2"
)

// Store is the persistence the service needs.
type Store interface {
	Create(ctx context.Context, r report.Report) error
}

// ValidationError carries every invalid field.
type ValidationError struct {
	Fields []report.FieldError
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid report: %v", e.Fields)
}

// Receipt is returned to the agent.
type Receipt struct {
	ID         string    `json:"id"`
	ReceivedAt time.Time `json:"received_at"`
}

// Service validates, redacts, stamps and stores reports.
type Service struct {
	Store Store
	Log   *slog.Logger
	Now   func() time.Time
}

// New returns a service using the real clock.
func New(st Store, log *slog.Logger) *Service {
	return &Service{Store: st, Log: log, Now: time.Now}
}

// Submit stores a report from source. Invalid input returns *ValidationError.
func (s *Service) Submit(ctx context.Context, in report.Input, source string) (Receipt, error) {
	if errs := report.Validate(in); len(errs) > 0 {
		return Receipt{}, &ValidationError{Fields: errs}
	}
	now := s.Now().UTC()
	r := report.Report{
		Input:      report.Redact(in),
		ID:         ulid.MustNew(ulid.Timestamp(now), rand.Reader).String(),
		ReceivedAt: now,
		Status:     report.StatusNew,
		Source:     source,
	}
	if err := s.Store.Create(ctx, r); err != nil {
		return Receipt{}, err
	}
	s.Log.InfoContext(ctx, "report stored",
		"id", r.ID, "kind", r.Kind, "subject", r.Subject, "source", r.Source)
	return Receipt{ID: r.ID, ReceivedAt: r.ReceivedAt}, nil
}
