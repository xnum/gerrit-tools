package reviewer

import (
	"errors"
	"strings"
	"testing"
)

func TestBuildRateLimitFailureSummary(t *testing.T) {
	summary := buildRateLimitFailureSummary("codex", errors.New("rate limit exceeded"))

	if !strings.Contains(summary, "Backend: codex") {
		t.Fatalf("expected backend in summary, got %q", summary)
	}
	if !strings.Contains(summary, "rate limit exceeded") {
		t.Fatalf("expected cause in summary, got %q", summary)
	}
	if !strings.Contains(summary, "Please retry this patchset later.") {
		t.Fatalf("expected retry guidance in summary, got %q", summary)
	}
}

func TestTruncateForReviewMessage(t *testing.T) {
	got := truncateForReviewMessage("abcdef", 4)
	if got != "abcd...(truncated)" {
		t.Fatalf("unexpected truncate output: %q", got)
	}
}
