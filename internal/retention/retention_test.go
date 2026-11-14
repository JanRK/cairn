package retention

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janrk/cairn/internal/report"
	"github.com/janrk/cairn/internal/store"
)

var t0 = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

func setup(t *testing.T) *store.Store {
	t.Helper()
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	for _, id := range []string{"OLD", "RECENT", "REOPENED", "OPEN"} {
		if err := s.Create(ctx, report.Report{
			Input: report.Input{Kind: "idea", Summary: "s", Details: "d"},
			ID:    id, ReceivedAt: t0, Status: report.StatusNew, Source: report.SourceREST,
		}); err != nil {
			t.Fatal(err)
		}
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.SetStatus(ctx, []string{"OLD", "REOPENED"}, report.StatusApplied, "", "", t0))
	must(s.SetStatus(ctx, []string{"RECENT"}, report.StatusRejected, "", "", t0.Add(100*time.Hour)))
	must(s.SetStatus(ctx, []string{"REOPENED"}, report.StatusNew, "", "", t0.Add(time.Hour)))
	return s
}

func TestSweep(t *testing.T) {
	ctx := context.Background()
	s := setup(t)
	var buf bytes.Buffer
	sw := &Sweeper{Store: s, Retention: 168 * time.Hour, Log: slog.New(slog.NewTextHandler(&buf, nil)),
		Now: func() time.Time { return t0.Add(200 * time.Hour) }}
	n, err := sw.Sweep(ctx)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if _, err := s.Get(ctx, "OLD"); err == nil {
		t.Fatal("OLD not deleted")
	}
	for _, id := range []string{"RECENT", "REOPENED", "OPEN"} {
		if _, err := s.Get(ctx, id); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
	if c := strings.Count(buf.String(), "retention sweep"); c != 1 || !strings.Contains(buf.String(), "deleted=1") {
		t.Fatalf("log: %q", buf.String())
	}
	// Far future: open reports still never deleted.
	sw.Now = func() time.Time { return t0.Add(10000 * time.Hour) }
	if n, err := sw.Sweep(ctx); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	for _, id := range []string{"REOPENED", "OPEN"} {
		if _, err := s.Get(ctx, id); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	s := setup(t)
	sw := &Sweeper{Store: s, Retention: time.Hour, Log: slog.New(slog.DiscardHandler), Now: time.Now, Interval: time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { sw.Run(ctx); close(done) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop")
	}
}
