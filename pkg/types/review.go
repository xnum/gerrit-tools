package types

import (
	"fmt"
	"strings"
)

// ReviewResult represents the parsed output from Claude's code review
type ReviewResult struct {
	Summary  string    // Overall summary of the review
	Vote     int       // Code-Review vote: -1, 0, or 1
	Comments []Comment // Inline comments for specific files/lines
}

// Comment represents a single inline comment on a specific file and line
type Comment struct {
	File    string // File path relative to repo root
	Line    int    // Line number (1-indexed)
	Message string // Comment text
}

// String returns a human-readable representation of the review result
func (r *ReviewResult) String() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Vote: %+d\n", r.Vote))
	sb.WriteString(fmt.Sprintf("Summary: %s\n", r.Summary))

	if len(r.Comments) > 0 {
		sb.WriteString(fmt.Sprintf("\nComments (%d):\n", len(r.Comments)))
		for _, c := range r.Comments {
			sb.WriteString(fmt.Sprintf("  - %s:%d: %s\n", c.File, c.Line, c.Message))
		}
	} else {
		sb.WriteString("\nNo inline comments.\n")
	}

	return sb.String()
}

// VoteLabel returns a human-readable label for the vote
func (r *ReviewResult) VoteLabel() string {
	switch r.Vote {
	case -1:
		return "I would prefer this is not merged as is"
	case 0:
		return "No score"
	case 1:
		return "Looks good to me, but someone else must approve"
	default:
		return fmt.Sprintf("Unknown vote: %d", r.Vote)
	}
}

// HasComments returns true if there are any inline comments
func (r *ReviewResult) HasComments() bool {
	return len(r.Comments) > 0
}
