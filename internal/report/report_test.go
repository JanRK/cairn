package report

import (
	"strings"
	"testing"
)

func validInput() Input {
	return Input{Kind: "field-note", Summary: "skill says X", Details: "it is Y"}
}

func fields(errs []FieldError) map[string]bool {
	m := map[string]bool{}
	for _, e := range errs {
		m[e.Field] = true
	}
	return m
}

func TestValidateAcceptsMinimalInput(t *testing.T) {
	if errs := Validate(validInput()); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestValidateRequiredFields(t *testing.T) {
	got := fields(Validate(Input{}))
	for _, f := range []string{"kind", "summary", "details"} {
		if !got[f] {
			t.Errorf("missing error for %s", f)
		}
	}
}

func TestValidateEnumsAndLimits(t *testing.T) {
	in := validInput()
	in.Kind = "bogus"
	in.SubjectType = "planet"
	in.Summary = strings.Repeat("a", MaxSummaryLen+1)
	in.Details = strings.Repeat("a", MaxDetailsBytes+1)
	got := fields(Validate(in))
	for _, f := range []string{"kind", "subject_type", "summary", "details"} {
		if !got[f] {
			t.Errorf("missing error for %s", f)
		}
	}
}

func TestValidateSummaryOneLine(t *testing.T) {
	in := validInput()
	in.Summary = "a\nb"
	if !fields(Validate(in))["summary"] {
		t.Fatal("multi-line summary accepted")
	}
}

func TestValidateLimitBoundaries(t *testing.T) {
	in := validInput()
	in.Summary = strings.Repeat("é", MaxSummaryLen) // 200 chars, 400 bytes
	in.Details = strings.Repeat("a", MaxDetailsBytes)
	in.SubjectType = "mcp-server"
	if errs := Validate(in); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestValidateAllKinds(t *testing.T) {
	for _, k := range Kinds {
		in := validInput()
		in.Kind = k
		if errs := Validate(in); len(errs) != 0 {
			t.Errorf("kind %s rejected: %v", k, errs)
		}
	}
}
