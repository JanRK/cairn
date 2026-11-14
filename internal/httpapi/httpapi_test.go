package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janrk/cairn/internal/httpapi"
	"github.com/janrk/cairn/internal/intake"
	"github.com/janrk/cairn/internal/report"
	"github.com/janrk/cairn/internal/store"
)

func newServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	log := slog.New(slog.DiscardHandler)
	mux := http.NewServeMux()
	httpapi.Register(mux, intake.New(st, log), st, log)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, st
}

func TestPostReportCreated(t *testing.T) {
	srv, st := newServer(t)
	body := `{"kind":"field-note","summary":"wrong flag","details":"use --x, key sk-ant-abcdefghijklmnopqrst","subject_type":"skill","subject":"pwsh"}`
	resp, err := http.Post(srv.URL+"/reports", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var rc struct {
		ID         string `json:"id"`
		ReceivedAt string `json:"received_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rc); err != nil || rc.ID == "" || rc.ReceivedAt == "" {
		t.Fatalf("bad body %+v %v", rc, err)
	}
	got, err := st.Get(context.Background(), rc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != report.SourceREST || got.Status != report.StatusNew || got.Subject != "pwsh" {
		t.Fatalf("stored %+v", got)
	}
	if strings.Contains(got.Details, "abcdefghijklmnopqrst") {
		t.Fatal("secret stored")
	}
}

func TestPostReportInvalid(t *testing.T) {
	srv, _ := newServer(t)
	for name, body := range map[string]string{
		"fields":   `{"kind":"nope","summary":""}`,
		"json":     `{`,
		"unknown":  `{"kind":"idea","summary":"s","details":"d","id":"x"}`,
		"trailing": `{"kind":"idea","summary":"s","details":"d"} {}`,
	} {
		resp, err := http.Post(srv.URL+"/reports", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		var eb struct {
			Errors []report.FieldError `json:"errors"`
		}
		err = json.NewDecoder(resp.Body).Decode(&eb)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest || err != nil || len(eb.Errors) == 0 {
			t.Errorf("%s: status %d errors %v (%v)", name, resp.StatusCode, eb.Errors, err)
		}
		if name == "fields" && len(eb.Errors) != 3 {
			t.Errorf("want kind, summary, details errors, got %v", eb.Errors)
		}
	}
}

func TestHealthz(t *testing.T) {
	srv, _ := newServer(t)
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

type badDB struct{}

func (badDB) Ping(context.Context) error { return errors.New("down") }

func TestHealthzDown(t *testing.T) {
	rec := httptest.NewRecorder()
	httpapi.Healthz(badDB{})(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
}
