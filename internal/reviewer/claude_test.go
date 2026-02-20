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

func TestBuildCodexArgs_DefaultSafeMode(t *testing.T) {
	exec := NewClaudeExecutor(".", &config.Config{
		Review: config.ReviewConfig{
			CLI:                        "codex",
			ClaudeSkipPermissionsCheck: false,
		},
	})

	args := exec.buildCodexArgs("test prompt", "/tmp/out.txt")

	requiredArgs := []string{
		"exec",
		"--skip-git-repo-check",
		"--color", "never",
		"--output-last-message", "/tmp/out.txt",
		"--full-auto",
		"test prompt",
	}

	for _, required := range requiredArgs {
		if !contains(args, required) {
			t.Fatalf("expected argument %q in codex args: %v", required, args)
		}
	}

	if contains(args, "--dangerously-bypass-approvals-and-sandbox") {
		t.Fatalf("unexpected dangerous bypass flag in default mode")
	}
}

func TestBuildCodexArgs_SkipPermissionsEnabled(t *testing.T) {
	exec := NewClaudeExecutor(".", &config.Config{
		Review: config.ReviewConfig{
			CLI:                        "codex",
			ClaudeSkipPermissionsCheck: true,
		},
	})

	args := exec.buildCodexArgs("test prompt", "/tmp/out.txt")

	if !contains(args, "--dangerously-bypass-approvals-and-sandbox") {
		t.Fatalf("expected dangerous bypass flag for codex when skip permissions enabled")
	}
	if contains(args, "--full-auto") {
		t.Fatalf("did not expect --full-auto when dangerous bypass is enabled")
	}
}

func TestReviewCLIFallbackToClaude(t *testing.T) {
	exec := NewClaudeExecutor(".", &config.Config{
		Review: config.ReviewConfig{},
	})

	if got := exec.reviewCLI(); got != "claude" {
		t.Fatalf("expected fallback review CLI to be claude, got %q", got)
	}
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
