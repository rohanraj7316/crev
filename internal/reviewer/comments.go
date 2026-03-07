package reviewer

import (
	"regexp"
	"strings"
)

// ParseReviewBody splits a CodeGuardian-style review into a summary (tally + tests + finality)
// and zero or more issue blocks. Each issue block starts with [CG-XXX].
// If the body doesn't match the expected structure, summary is the full body and issues are nil.
func ParseReviewBody(body string) (summary string, issues []string) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", nil
	}

	// Find "## Tests analysis" so we can split intro+detailed from tests+finality
	idxTests := strings.Index(body, "## Tests analysis")
	if idxTests < 0 {
		// No structured format; treat whole thing as summary
		return body, nil
	}

	beforeTests := strings.TrimSpace(body[:idxTests])
	outro := strings.TrimSpace(body[idxTests:]) // "## Tests analysis" through "## Finality statement" and rest

	// Before "## Tests analysis" we have "## Review tally" ... "## Detailed review" ... [CG-001] ... [CG-002] ...
	idxDetailed := strings.Index(beforeTests, "## Detailed review")
	if idxDetailed < 0 {
		// No detailed section; summary is tally + outro
		return strings.TrimSpace(beforeTests + "\n\n" + outro), nil
	}

	intro := strings.TrimSpace(beforeTests[:idxDetailed]) // "## Review tally" and its content
	detailedSection := strings.TrimSpace(beforeTests[idxDetailed:])

	// Split detailed section into issue blocks by [CG-XXX] at line start
	issueBlocks := splitIssueBlocks(detailedSection)
	if len(issueBlocks) == 0 {
		// No issues parsed; summary is intro + outro
		return intro + "\n\n" + outro, nil
	}

	summary = intro + "\n\n" + outro
	return summary, issueBlocks
}

var issueStartRE = regexp.MustCompile(`(?m)^\[CG-\d+\]`)

// splitIssueBlocks returns each [CG-XXX] block as a separate string.
func splitIssueBlocks(detailedSection string) []string {
	indices := issueStartRE.FindAllStringIndex(detailedSection, -1)
	if len(indices) == 0 {
		return nil
	}
	out := make([]string, 0, len(indices))
	for i := 0; i < len(indices); i++ {
		start := indices[i][0]
		var end int
		if i+1 < len(indices) {
			end = indices[i+1][0]
			out = append(out, strings.TrimSpace(detailedSection[start:end]))
		} else {
			out = append(out, strings.TrimSpace(detailedSection[start:]))
		}
	}
	return out
}
