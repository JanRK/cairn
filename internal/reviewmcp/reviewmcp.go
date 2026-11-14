// Package reviewmcp serves the reviewer-facing MCP endpoint (/mcp/review) on
// the review listener. It must never be mounted on the agent listener.
package reviewmcp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/janrk/cairn/internal/report"
	"github.com/janrk/cairn/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Path is where the review endpoint mounts.
const Path = "/mcp/review"

// Service backs the review tools. Now is the clock used for handled_at; Log may be nil.
type Service struct {
	Store *store.Store
	Now   func() time.Time
	Log   *slog.Logger
}

// Register mounts the review MCP endpoint on mux.
func Register(mux *http.ServeMux, svc *Service) {
	srv := NewServer(svc)
	mux.Handle(Path, mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, Logger: svc.Log}))
}

// ListInput filters list_reports.
type ListInput struct {
	Status  string `json:"status,omitempty" jsonschema:"filter by status: new, accepted, applied, rejected, duplicate"`
	Kind    string `json:"kind,omitempty" jsonschema:"filter by kind"`
	Subject string `json:"subject,omitempty" jsonschema:"filter by exact subject"`
	Since   string `json:"since,omitempty" jsonschema:"only reports received at or after this RFC 3339 time"`
	Cursor  string `json:"cursor,omitempty" jsonschema:"the next value from the previous page"`
	Limit   int    `json:"limit,omitempty" jsonschema:"page size, default 50, max 200"`
}

// ListOutput is one page of short reports.
type ListOutput struct {
	Reports []store.Summary `json:"reports"`
	Next    string          `json:"next,omitempty" jsonschema:"cursor for the next page; empty when done"`
}

// GetInput selects one report.
type GetInput struct {
	ID string `json:"id" jsonschema:"report id"`
}

// SetStatusInput moves reports to a status.
type SetStatusInput struct {
	IDs         []string `json:"ids" jsonschema:"one or more report ids"`
	Status      string   `json:"status" jsonschema:"target status: new, accepted (open) or applied, rejected, duplicate (handled)"`
	Note        string   `json:"note,omitempty" jsonschema:"optional reviewer note"`
	DuplicateOf string   `json:"duplicate_of,omitempty" jsonschema:"id of the original; required for status duplicate"`
}

// SetStatusOutput confirms the change.
type SetStatusOutput struct {
	Updated int    `json:"updated"`
	Status  string `json:"status"`
}

// NewServer builds the review MCP server.
func NewServer(svc *Service) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "cairn-review", Version: "1"}, nil)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_reports",
		Description: "List reports, oldest first, with short fields only. Filter by status, kind, subject and since; page with cursor.",
	}, svc.list)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_report",
		Description: "Get the full report with the given id.",
	}, svc.get)
	mcp.AddTool(s, &mcp.Tool{
		Name: "set_status",
		Description: "Set the status of one or more reports. Open: new, accepted. Handled: applied, rejected, duplicate. " +
			"Entering a handled status starts the retention clock; moving back to open stops it. duplicate needs duplicate_of.",
	}, svc.setStatus)
	return s
}

func (s *Service) list(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, ListOutput, error) {
	f := store.Filter{Status: in.Status, Kind: in.Kind, Subject: in.Subject, After: in.Cursor, Limit: in.Limit}
	if in.Status != "" && !report.ValidStatus(in.Status) {
		return nil, ListOutput{}, fmt.Errorf("unknown status %q", in.Status)
	}
	if in.Since != "" {
		t, err := time.Parse(time.RFC3339, in.Since)
		if err != nil {
			return nil, ListOutput{}, fmt.Errorf("since: %w", err)
		}
		f.Since = t
	}
	rs, next, err := s.Store.List(ctx, f)
	if err != nil {
		return nil, ListOutput{}, err
	}
	if rs == nil {
		rs = []store.Summary{}
	}
	return nil, ListOutput{Reports: rs, Next: next}, nil
}

func (s *Service) get(ctx context.Context, _ *mcp.CallToolRequest, in GetInput) (*mcp.CallToolResult, report.Report, error) {
	r, err := s.Store.Get(ctx, in.ID)
	return nil, r, err
}

func (s *Service) setStatus(ctx context.Context, _ *mcp.CallToolRequest, in SetStatusInput) (*mcp.CallToolResult, SetStatusOutput, error) {
	if err := s.Store.SetStatus(ctx, in.IDs, in.Status, in.Note, in.DuplicateOf, s.Now()); err != nil {
		return nil, SetStatusOutput{}, err
	}
	return nil, SetStatusOutput{Updated: len(in.IDs), Status: in.Status}, nil
}
