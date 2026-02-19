package reviewer

import (
	"testing"

	"github.com/gerrit-ai-review/gerrit-tools/internal/config"
)

func TestBuildClaudeArgs_DefaultSecureMode(t *testing.T) {
	exec := NewClaudeExecutor(".", &config.Config{
		Review: config.ReviewConfig{
			ClaudeSkipPermissionsCheck: false,
		},
	})

	args := exec.buildClaudeArgs("test prompt")

	for _, arg := range args {
		if arg == "--dangerously-skip-permissions" {
			t.Fatalf("unexpected --dangerously-skip-permissions in default mode")
		}
	}
}

func TestBuildClaudeArgs_SkipPermissionsEnabled(t *testing.T) {
	exec := NewClaudeExecutor(".", &config.Config{
		Review: config.ReviewConfig{
			ClaudeSkipPermissionsCheck: true,
		},
	})

	args := exec.buildClaudeArgs("test prompt")

	found := false
	for _, arg := range args {
		if arg == "--dangerously-skip-permissions" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected --dangerously-skip-permissions when enabled")
	}
}
