package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/janrk/cairn/internal/report"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func seed(t *testing.T, s *Store, id, kind, subject string, at time.Time) {
	t.Helper()
	err := s.Create(context.Background(), report.Report{
		Input: report.Input{Kind: kind, Summary: "sum " + id, Details: "details " + id, Subject: subject},
		ID:    id, ReceivedAt: at, Status: report.StatusNew, Source: report.SourceREST,
	})
	if err != nil {
		t.Fatal(err)
	}
}

var t0 = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

func TestSetStatusHandledAt(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	seed(t, s, "A", "idea", "x", t0)
	seed(t, s, "B", "idea", "x", t0)

	t1 := t0.Add(time.Hour)
	if err := s.SetStatus(ctx, []string{"A"}, report.StatusAccepted, "", "", t1); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.Get(ctx, "A"); r.Status != report.StatusAccepted || r.HandledAt != nil {
		t.Fatalf("open move set handled_at: %+v", r)
	}

	if err := s.SetStatus(ctx, []string{"A", "B"}, report.StatusApplied, "done", "", t1); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"A", "B"} {
		r, _ := s.Get(ctx, id)
		if r.Status != report.StatusApplied || r.HandledAt == nil || !r.HandledAt.Equal(t1) || r.Note != "done" {
			t.Fatalf("%s: %+v", id, r)
		}
	}

	// handled -> handled keeps the original handled_at
	t2 := t1.Add(time.Hour)
	if err := s.SetStatus(ctx, []string{"A"}, report.StatusRejected, "", "", t2); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.Get(ctx, "A"); r.HandledAt == nil || !r.HandledAt.Equal(t1) || r.Note != "done" {
		t.Fatalf("handled->handled: %+v", r)
	}

	// back to open clears it
	if err := s.SetStatus(ctx, []string{"A"}, report.StatusNew, "", "", t2); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.Get(ctx, "A"); r.HandledAt != nil || r.Status != report.StatusNew {
		t.Fatalf("reopen: %+v", r)
	}

	// duplicate requires an existing duplicate_of
	if err := s.SetStatus(ctx, []string{"A"}, report.StatusDuplicate, "", "", t2); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing duplicate_of: %v", err)
	}
	if err := s.SetStatus(ctx, []string{"A"}, report.StatusDuplicate, "", "nope", t2); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown duplicate_of: %v", err)
	}
	if err := s.SetStatus(ctx, []string{"A"}, report.StatusDuplicate, "", "A", t2); !errors.Is(err, ErrInvalid) {
		t.Fatalf("self duplicate_of: %v", err)
	}
	if err := s.SetStatus(ctx, []string{"A"}, report.StatusDuplicate, "", "B", t2); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.Get(ctx, "A"); r.DuplicateOf != "B" || r.HandledAt == nil || !r.HandledAt.Equal(t2) {
		t.Fatalf("duplicate: %+v", r)
	}
	if err := s.SetStatus(ctx, []string{"A"}, report.StatusAccepted, "", "", t2); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.Get(ctx, "A"); r.DuplicateOf != "" {
		t.Fatalf("duplicate_of kept: %+v", r)
	}
	if err := s.SetStatus(ctx, []string{"A"}, report.StatusApplied, "", "B", t2); !errors.Is(err, ErrInvalid) {
		t.Fatalf("duplicate_of on non-duplicate: %v", err)
	}

	// bad status, unknown id (all-or-nothing), empty ids
	if err := s.SetStatus(ctx, []string{"A"}, "bogus", "", "", t2); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bogus: %v", err)
	}
	if err := s.SetStatus(ctx, []string{"A", "missing"}, report.StatusApplied, "", "", t2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if r, _ := s.Get(ctx, "A"); r.Status != report.StatusAccepted {
		t.Fatalf("partial update: %+v", r)
	}
	if err := s.SetStatus(ctx, nil, report.StatusApplied, "", "", t2); !errors.Is(err, ErrInvalid) {
		t.Fatalf("no ids: %v", err)
	}
}

func TestList(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	seed(t, s, "01A", "idea", "x", t0)
	seed(t, s, "01B", "outage", "x", t0.Add(time.Hour))
	seed(t, s, "01C", "idea", "y", t0.Add(2*time.Hour))
	if err := s.SetStatus(ctx, []string{"01C"}, report.StatusApplied, "", "", t0); err != nil {
		t.Fatal(err)
	}

	ids := func(f Filter) []string {
		t.Helper()
		rs, next, err := s.List(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, r := range rs {
			if r.Summary == "" {
				t.Fatalf("list must carry short fields only: %+v", r)
			}
			out = append(out, r.ID)
		}
		if next != "" {
			out = append(out, "next="+next)
		}
		return out
	}
	check := func(got []string, want ...string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("got %v want %v", got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("got %v want %v", got, want)
			}
		}
	}
	check(ids(Filter{}), "01A", "01B", "01C")
	check(ids(Filter{Status: report.StatusNew}), "01A", "01B")
	check(ids(Filter{Kind: "idea"}), "01A", "01C")
	check(ids(Filter{Subject: "y"}), "01C")
	check(ids(Filter{Since: t0.Add(time.Hour)}), "01B", "01C")
	check(ids(Filter{Limit: 2}), "01A", "01B", "next=01B")
	check(ids(Filter{Limit: 2, After: "01B"}), "01C")
}
