// Package report defines the report model, its validation and secret redaction.
// Validation and redaction are shared by every intake surface (REST, MCP).
package report

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxSummaryLen   = 200       // characters
	MaxDetailsBytes = 16 * 1024 // bytes
)

var (
	Kinds        = []string{"field-note", "tool-failure", "permission", "outage", "idea", "other"}
	SubjectTypes = []string{"skill", "mcp-server", "tool", "agent", "environment", "other"}
)

const (
	StatusNew = "new"

	SourceREST = "rest"
	SourceMCP  = "mcp"
)

// Input holds the agent-filled fields of a report.
type Input struct {
	Kind         string `json:"kind"`
	Summary      string `json:"summary"`
	Details      string `json:"details"`
	SubjectType  string `json:"subject_type,omitempty"`
	Subject      string `json:"subject,omitempty"`
	Evidence     string `json:"evidence,omitempty"`
	SuggestedFix string `json:"suggested_fix,omitempty"`
	Agent        string `json:"agent,omitempty"`
	Harness      string `json:"harness,omitempty"`
	Model        string `json:"model,omitempty"`
	Workspace    string `json:"workspace,omitempty"`
}

// Report is a stored report.
type Report struct {
	Input
	ID          string     `json:"id"`
	ReceivedAt  time.Time  `json:"received_at"`
	Status      string     `json:"status"`
	Source      string     `json:"source"`
	HandledAt   *time.Time `json:"handled_at,omitempty"`
	Note        string     `json:"note,omitempty"`
	DuplicateOf string     `json:"duplicate_of,omitempty"`
}

// FieldError describes one invalid field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e FieldError) Error() string { return e.Field + ": " + e.Message }

// Validate checks the agent-filled fields and returns one error per bad field.
func Validate(in Input) []FieldError {
	var errs []FieldError
	add := func(field, format string, a ...any) {
		errs = append(errs, FieldError{Field: field, Message: fmt.Sprintf(format, a...)})
	}

	switch {
	case in.Kind == "":
		add("kind", "required")
	case !slices.Contains(Kinds, in.Kind):
		add("kind", "must be one of %s", strings.Join(Kinds, ", "))
	}

	switch {
	case strings.TrimSpace(in.Summary) == "":
		add("summary", "required")
	case strings.ContainsAny(in.Summary, "\r\n"):
		add("summary", "must be a single line")
	case utf8.RuneCountInString(in.Summary) > MaxSummaryLen:
		add("summary", "must be at most %d characters", MaxSummaryLen)
	}

	switch {
	case strings.TrimSpace(in.Details) == "":
		add("details", "required")
	case len(in.Details) > MaxDetailsBytes:
		add("details", "must be at most %d bytes", MaxDetailsBytes)
	}

	if in.SubjectType != "" && !slices.Contains(SubjectTypes, in.SubjectType) {
		add("subject_type", "must be one of %s", strings.Join(SubjectTypes, ", "))
	}
	return errs
}

// Review statuses. Open statuses await triage; handled ones are done and
// subject to retention.
const (
	StatusAccepted  = "accepted"
	StatusApplied   = "applied"
	StatusRejected  = "rejected"
	StatusDuplicate = "duplicate"
)

var (
	OpenStatuses    = []string{StatusNew, StatusAccepted}
	HandledStatuses = []string{StatusApplied, StatusRejected, StatusDuplicate}
)

// ValidStatus reports whether s is a known status.
func ValidStatus(s string) bool {
	return slices.Contains(OpenStatuses, s) || slices.Contains(HandledStatuses, s)
}

// IsHandled reports whether s is a handled status.
func IsHandled(s string) bool { return slices.Contains(HandledStatuses, s) }
