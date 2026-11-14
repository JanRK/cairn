package reviewmcp_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janrk/cairn/internal/httpapi"
	"github.com/janrk/cairn/internal/intake"
	"github.com/janrk/cairn/internal/reviewmcp"
	"github.com/janrk/cairn/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestReviewEndToEnd(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	agentMux := http.NewServeMux()
	httpapi.Register(agentMux, intake.New(st, log), st, log)
	agent := httptest.NewServer(agentMux)
	t.Cleanup(agent.Close)

	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	reviewMux := http.NewServeMux()
	reviewmcp.Register(reviewMux, &reviewmcp.Service{Store: st, Now: func() time.Time { return now }})
	review := httptest.NewServer(reviewMux)
	t.Cleanup(review.Close)

	var ids []string
	for _, body := range []string{
		`{"kind":"idea","summary":"one","details":"long details one","subject":"git"}`,
		`{"kind":"outage","summary":"two","details":"long details two","subject":"litellm"}`,
	} {
		resp, err := http.Post(agent.URL+"/reports", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		var rc intake.Receipt
		_ = json.NewDecoder(resp.Body).Decode(&rc)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("POST status %d", resp.StatusCode)
		}
		ids = append(ids, rc.ID)
	}

	sess := connect(t, review.URL+"/mcp/review")
	tools, err := sess.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tl := range tools.Tools {
		names[tl.Name] = true
	}
	if !names["list_reports"] || !names["get_report"] || !names["set_status"] || names["submit_report"] || len(names) != 3 {
		t.Fatalf("review tools: %v", names)
	}

	var list listPage
	call(t, sess, "list_reports", map[string]any{"kind": "outage"}, &list)
	if len(list.Reports) != 1 || list.Reports[0]["id"] != ids[1] || list.Reports[0]["details"] != nil {
		t.Fatalf("list: %+v", list)
	}
	call(t, sess, "list_reports", map[string]any{"limit": 1}, &list)
	if len(list.Reports) != 1 || list.Next != ids[0] {
		t.Fatalf("page 1: %+v", list)
	}
	call(t, sess, "list_reports", map[string]any{"limit": 1, "cursor": list.Next}, &list)
	if len(list.Reports) != 1 || list.Reports[0]["id"] != ids[1] || list.Next != "" {
		t.Fatalf("page 2: %+v", list)
	}

	var got map[string]any
	call(t, sess, "get_report", map[string]any{"id": ids[0]}, &got)
	if got["details"] != "long details one" || got["status"] != "new" {
		t.Fatalf("get: %+v", got)
	}

	call(t, sess, "set_status", map[string]any{"ids": ids, "status": "applied", "note": "fixed"}, nil)
	call(t, sess, "get_report", map[string]any{"id": ids[1]}, &got)
	if got["status"] != "applied" || got["note"] != "fixed" || got["handled_at"] != now.Format(time.RFC3339) {
		t.Fatalf("after set_status: %+v", got)
	}
	call(t, sess, "list_reports", map[string]any{"status": "new"}, &list)
	if len(list.Reports) != 0 {
		t.Fatalf("still new: %+v", list)
	}

	for _, bad := range []map[string]any{
		{"ids": ids[:1], "status": "duplicate"},
		{"ids": ids[:1], "status": "bogus"},
		{"ids": []string{"missing"}, "status": "accepted"},
	} {
		res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "set_status", Arguments: bad})
		if err == nil && !res.IsError {
			t.Fatalf("set_status %v: no error", bad)
		}
	}
	res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "list_reports", Arguments: map[string]any{"status": "bogus"}})
	if err == nil && !res.IsError {
		t.Fatal("bad status filter accepted")
	}

	// The agent listener never serves the review endpoint.
	resp, err := http.Post(agent.URL+"/mcp/review", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("agent /mcp/review: %d", resp.StatusCode)
	}
}

type listPage struct {
	Reports []map[string]any `json:"reports"`
	Next    string           `json:"next"`
}

func connect(t *testing.T, url string) *mcp.ClientSession {
	t.Helper()
	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	sess, err := c.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

func call(t *testing.T, sess *mcp.ClientSession, name string, args map[string]any, out any) {
	t.Helper()
	res, err := sess.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("%s: tool error %+v", name, res.Content)
	}
	if out == nil {
		return
	}
	b, _ := json.Marshal(res.StructuredContent)
	switch o := out.(type) {
	case *map[string]any:
		*o = nil
	case *listPage:
		*o = listPage{}
	}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatal(err)
	}
}
