package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/janrk/cairn/internal/report"
)

func TestCreateAndGet(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if err := s.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	r := report.Report{
		Input:      report.Input{Kind: "idea", Summary: "s", Details: "d", Subject: "x", Model: "m"},
		ID:         "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		ReceivedAt: time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC),
		Status:     report.StatusNew,
		Source:     report.SourceREST,
	}
	if err := s.Create(ctx, r); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Input != r.Input || got.Status != r.Status || got.Source != r.Source || !got.ReceivedAt.Equal(r.ReceivedAt) || got.HandledAt != nil {
		t.Fatalf("got %+v want %+v", got, r)
	}
	if _, err := s.Get(ctx, "missing"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
