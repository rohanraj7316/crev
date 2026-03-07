package reviewer

import (
	"regexp"
	"strings"
)

const (
	VerdictApprove   = "approve"
	VerdictNeedsWork = "needs_work"
)

var verdictRe = regexp.MustCompile(`(?mi)^\s*VERDICT:\s*(APPROVE|NEEDS_WORK)\s*$`)

// ParseVerdict extracts VERDICT: APPROVE or VERDICT: NEEDS_WORK from the end of the LLM response.
// Returns VerdictNeedsWork if no verdict is found (safe default).
func ParseVerdict(reviewText string) string {
	lines := strings.Split(reviewText, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if m := verdictRe.FindStringSubmatch(line); len(m) == 2 {
			return strings.ToLower(m[1])
		}
		// Only look at the last non-empty lines to avoid matching mid-review
		if len(lines)-i > 5 {
			break
		}
	}
	return VerdictNeedsWork
}

// StripVerdictFromComment removes the VERDICT: ... line from the review text before posting as a comment.
func StripVerdictFromComment(reviewText string) string {
	return verdictRe.ReplaceAllString(reviewText, "")
}
