// Package agentmcp serves the agent MCP endpoint (/mcp) with its single tool,
// submit_report.
package agentmcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/janrk/cairn/internal/intake"
	"github.com/janrk/cairn/internal/report"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolName is the only tool on /mcp.
const ToolName = "submit_report"

// Description is the reporting policy shown to agents.
const Description = `File a field note with cairn so a human can improve skills, tools and setup later.

Report:
- wrong or missing skill facts (a skill told you something false, outdated or incomplete)
- tool failures (an MCP server or tool errored, hung or returned garbage)
- denied permissions (you were blocked from something you needed)
- outages (a service, endpoint or dependency was unreachable)
- ideas worth keeping (a better approach, a missing tool, a useful skill)

Do not report:
- self-fixed routine errors (typos, a retry that worked, ordinary test failures)
- secrets (tokens, keys, passwords); they are redacted anyway, but leave them out
- log dumps; quote only the few lines that matter in evidence

One report per finding. summary is one line; details says what happened, what you expected and why it matters.`

// Register mounts the streamable HTTP MCP handler on mux at /mcp.
func Register(mux *http.ServeMux, svc *intake.Service, log *slog.Logger) {
	srv := NewServer(svc, log)
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, Logger: log})
	mux.Handle("/mcp", h)
}

// NewServer returns an MCP server exposing only submit_report.
func NewServer(svc *intake.Service, log *slog.Logger) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "cairn", Version: "v1"}, &mcp.ServerOptions{Logger: log})
	s.AddTool(&mcp.Tool{Name: ToolName, Description: Description, InputSchema: inputSchema()}, submit(svc, log))
	return s
}

func submit(svc *intake.Service, log *slog.Logger) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in report.Input
		dec := json.NewDecoder(bytes.NewReader(req.Params.Arguments))
		dec.DisallowUnknownFields()
		if len(req.Params.Arguments) > 0 {
			if err := dec.Decode(&in); err != nil {
				return fieldErrors([]report.FieldError{{Field: "arguments", Message: err.Error()}}), nil
			}
		}
		rc, err := svc.Submit(ctx, in, report.SourceMCP)
		var ve *intake.ValidationError
		switch {
		case errors.As(err, &ve):
			return fieldErrors(ve.Fields), nil
		case err != nil:
			log.ErrorContext(ctx, "store report", "err", err)
			res := &mcp.CallToolResult{}
			res.SetError(errors.New("internal error: report not stored"))
			return res, nil
		}
		b, err := json.Marshal(rc)
		if err != nil {
			return nil, err
		}
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: string(b)}},
			StructuredContent: json.RawMessage(b),
		}, nil
	}
}

func fieldErrors(fs []report.FieldError) *mcp.CallToolResult {
	b, _ := json.Marshal(struct {
		Errors []report.FieldError `json:"errors"`
	}{fs})
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}
}

// inputSchema describes the fields without constraints so that every check
// runs in report.Validate and the agent gets one error per bad field.
func inputSchema() *jsonschema.Schema {
	str := func(desc string) *jsonschema.Schema { return &jsonschema.Schema{Type: "string", Description: desc} }
	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"kind":          str("Required. One of: " + strings.Join(report.Kinds, ", ") + "."),
			"summary":       str("Required. One line, at most 200 characters."),
			"details":       str("Required. What happened, what you expected, why it matters. At most 16 KiB."),
			"subject_type":  str("Optional. One of: " + strings.Join(report.SubjectTypes, ", ") + "."),
			"subject":       str("Optional. Name of the skill, server, tool or service concerned."),
			"evidence":      str("Optional. The few lines of output or the command that show the problem."),
			"suggested_fix": str("Optional. What should change."),
			"agent":         str("Optional. Your agent name."),
			"harness":       str("Optional. The harness you run in, e.g. opencode."),
			"model":         str("Optional. Your model id."),
			"workspace":     str("Optional. Workspace or repository you were working in."),
		},
	}
}
