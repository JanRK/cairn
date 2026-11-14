package intake

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/janrk/cairn/internal/report"
)

type memStore struct{ got []report.Report }

func (m *memStore) Create(_ context.Context, r report.Report) error {
	m.got = append(m.got, r)
	return nil
}

func TestSubmitStampsRedactsAndLogs(t *testing.T) {
	var logs bytes.Buffer
	st := &memStore{}
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	svc := &Service{Store: st, Log: slog.New(slog.NewJSONHandler(&logs, nil)), Now: func() time.Time { return now }}

	rc, err := svc.Submit(context.Background(), report.Input{
		Kind: "tool-failure", Summary: "x", Details: "SECRETDETAILS Bearer abcdefghijkl", Subject: "mcp-x",
	}, report.SourceREST)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.got) != 1 {
		t.Fatal("not stored")
	}
	r := st.got[0]
	if r.ID != rc.ID || len(r.ID) != 26 || !rc.ReceivedAt.Equal(now) || r.Status != "new" || r.Source != "rest" {
		t.Fatalf("bad stamp: %+v", r)
	}
	if strings.Contains(r.Details, "abcdefghijkl") {
		t.Fatal("not redacted")
	}
	l := logs.String()
	if strings.Count(l, "\n") != 1 || !strings.Contains(l, rc.ID) || !strings.Contains(l, `"kind":"tool-failure"`) || !strings.Contains(l, `"subject":"mcp-x"`) {
		t.Fatalf("bad log: %s", l)
	}
	if strings.Contains(l, "SECRETDETAILS") {
		t.Fatal("details logged")
	}
}

func TestSubmitInvalid(t *testing.T) {
	st := &memStore{}
	svc := New(st, slog.New(slog.DiscardHandler))
	_, err := svc.Submit(context.Background(), report.Input{}, report.SourceMCP)
	var ve *ValidationError
	if !errors.As(err, &ve) || len(ve.Fields) == 0 || len(st.got) != 0 {
		t.Fatalf("want validation error, got %v", err)
	}
}
