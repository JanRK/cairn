package report

import "regexp"

// Redacted replaces every secret match.
const Redacted = "[REDACTED]"

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----[\s\S]*?(?:-----END [A-Z0-9 ]*PRIVATE KEY-----|$)`),
	regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9\-._~+/]{8,}=*`),
	regexp.MustCompile(`\bsk-(?:ant-|proj-)?[A-Za-z0-9\-_]{16,}`),
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`\b(?:AKIA|ASIA)[A-Z0-9]{16}\b`),
	regexp.MustCompile(`\bxox[abposr]-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`\bAIza[0-9A-Za-z\-_]{35}`),
	regexp.MustCompile(`\bglpat-[A-Za-z0-9\-_]{20,}`),
}

// RedactString replaces simple secret patterns in s.
func RedactString(s string) string {
	for _, re := range secretPatterns {
		s = re.ReplaceAllString(s, Redacted)
	}
	return s
}

// Redact returns in with every text field redacted.
func Redact(in Input) Input {
	for _, p := range []*string{&in.Kind, &in.Summary, &in.Details, &in.SubjectType, &in.Subject,
		&in.Evidence, &in.SuggestedFix, &in.Agent, &in.Harness, &in.Model, &in.Workspace} {
		*p = RedactString(*p)
	}
	return in
}
