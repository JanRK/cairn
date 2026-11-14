package agentmcp_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janrk/cairn/internal/agentmcp"
	"github.com/janrk/cairn/internal/intake"
	"github.com/janrk/cairn/internal/report"
	"github.com/janrk/cairn/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func connect(t *testing.T) (*mcp.ClientSession, *store.Store) {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	log := slog.New(slog.DiscardHandler)
	mux := http.NewServeMux()
	agentmcp.Register(mux, intake.New(st, log), log)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs, st
}

func TestListToolsOnlySubmitReport(t *testing.T) {
	cs, _ := connect(t)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Tools) != 1 || res.Tools[0].Name != "submit_report" {
		t.Fatalf("tools %+v", res.Tools)
	}
	d := res.Tools[0].Description
	for _, want := range []string{"wrong or missing skill facts", "denied permissions", "outages", "secrets", "log dumps", "self-fixed"} {
		if !strings.Contains(d, want) {
			t.Errorf("description lacks %q", want)
		}
	}
}

func TestSubmitValid(t *testing.T) {
	cs, st := connect(t)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "submit_report",
		Arguments: map[string]any{
			"kind": "field-note", "summary": "wrong flag", "subject": "pwsh",
			"details": "use --x, key sk-ant-abcdefghijklmnopqrst",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool error %+v", res.Content[0])
	}
	var rc intake.Receipt
	b, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(b, &rc); err != nil || rc.ID == "" || rc.ReceivedAt.IsZero() {
		t.Fatalf("receipt %s %v", b, err)
	}
	got, err := st.Get(context.Background(), rc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != report.SourceMCP || got.Subject != "pwsh" {
		t.Fatalf("stored %+v", got)
	}
	if strings.Contains(got.Details, "abcdefghijklmnopqrst") {
		t.Fatal("secret stored")
	}
}

func TestSubmitInvalidListsEachField(t *testing.T) {
	cs, _ := connect(t)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "submit_report",
		Arguments: map[string]any{"kind": "nope", "summary": "a\nb", "subject_type": "x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("want tool error")
	}
	text := res.Content[0].(*mcp.TextContent).Text
	var eb struct {
		Errors []report.FieldError `json:"errors"`
	}
	if err := json.Unmarshal([]byte(text), &eb); err != nil {
		t.Fatalf("error text %q: %v", text, err)
	}
	fields := map[string]bool{}
	for _, e := range eb.Errors {
		fields[e.Field] = true
	}
	for _, f := range []string{"kind", "summary", "details", "subject_type"} {
		if !fields[f] {
			t.Errorf("missing error for %s in %s", f, text)
		}
	}
}
