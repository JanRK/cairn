package report

import (
	"strings"
	"testing"
)

func TestRedactString(t *testing.T) {
	cases := map[string]string{
		"bearer":     "Authorization: Bearer abc.DEF-123_xyz",
		"openai":     "key sk-proj-AbCdEfGhIjKlMnOpQrStUv12",
		"anthropic":  "sk-ant-api03-AbCdEfGhIjKlMnOpQrSt",
		"github":     "token ghp_AbCdEfGhIjKlMnOpQrStUvWxYz0123456789",
		"github_pat": "github_pat_11ABCDEFG0123456789_abcdefghijk",
		"aws":        "AKIAIOSFODNN7EXAMPLE",
		"slack":      "xoxb-1234567890-abcdefghij",
		"google":     "AIzaSyA-abcdefghijklmnopqrstuvwxyz12345",
		"pem":        "-----BEGIN RSA PRIVATE KEY-----\nMIIEow\nabc\n-----END RSA PRIVATE KEY-----",
	}
	secrets := map[string]string{
		"bearer":     "abc.DEF-123_xyz",
		"openai":     "AbCdEfGhIjKlMnOpQrStUv12",
		"anthropic":  "AbCdEfGhIjKlMnOpQrSt",
		"github":     "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789",
		"github_pat": "abcdefghijk",
		"aws":        "IOSFODNN7EXAMPLE",
		"slack":      "abcdefghij",
		"google":     "abcdefghijklmnop",
		"pem":        "MIIEow",
	}
	for name, in := range cases {
		out := RedactString(in)
		if strings.Contains(out, secrets[name]) {
			t.Errorf("%s: secret survived: %q", name, out)
		}
		if !strings.Contains(out, Redacted) {
			t.Errorf("%s: no marker: %q", name, out)
		}
	}
}

func TestRedactLeavesPlainText(t *testing.T) {
	in := "the skill-name tool failed with exit 1; ask the bearer of news"
	if out := RedactString(in); out != in {
		t.Fatalf("changed plain text: %q", out)
	}
}

func TestRedactAllTextFields(t *testing.T) {
	s := "Bearer secrettoken123"
	in := Input{Kind: s, Summary: s, Details: s, SubjectType: s, Subject: s, Evidence: s,
		SuggestedFix: s, Agent: s, Harness: s, Model: s, Workspace: s}
	out := Redact(in)
	for i, v := range []string{out.Kind, out.Summary, out.Details, out.SubjectType, out.Subject,
		out.Evidence, out.SuggestedFix, out.Agent, out.Harness, out.Model, out.Workspace} {
		if strings.Contains(v, "secrettoken123") {
			t.Errorf("field %d not redacted", i)
		}
	}
}
