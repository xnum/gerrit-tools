package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetPatchsetRef(t *testing.T) {
	tests := []struct {
		changeNum   int
		patchsetNum int
		expectedRef string
	}{
		{12345, 3, "refs/changes/45/12345/3"},
		{100, 1, "refs/changes/00/100/1"},
		{99, 2, "refs/changes/99/99/2"},
		{1, 1, "refs/changes/01/1/1"},
	}

	for _, tt := range tests {
		t.Run(tt.expectedRef, func(t *testing.T) {
			ref := GetPatchsetRef(tt.changeNum, tt.patchsetNum)
			if ref != tt.expectedRef {
				t.Errorf("GetPatchsetRef(%d, %d) = %s, want %s",
					tt.changeNum, tt.patchsetNum, ref, tt.expectedRef)
			}
		})
	}
}

func TestParseDiffStats(t *testing.T) {
	input := ` main.go           | 10 +++++-----
 config.go         |  5 +++++
 README.md         | 20 --------------------
 3 files changed, 15 insertions(+), 25 deletions(-)`

	stats := ParseDiffStats(input)

	// Check main.go
	if stat, ok := stats["main.go"]; !ok {
		t.Error("Expected main.go in stats")
	} else {
		if stat.Changes != 10 {
			t.Errorf("main.go changes = %d, want 10", stat.Changes)
		}
		if stat.Additions != 5 {
			t.Errorf("main.go additions = %d, want 5", stat.Additions)
		}
		if stat.Deletions != 5 {
			t.Errorf("main.go deletions = %d, want 5", stat.Deletions)
		}
	}

	// Check config.go
	if stat, ok := stats["config.go"]; !ok {
		t.Error("Expected config.go in stats")
	} else {
		if stat.Changes != 5 {
			t.Errorf("config.go changes = %d, want 5", stat.Changes)
		}
		if stat.Additions != 5 {
			t.Errorf("config.go additions = %d, want 5", stat.Additions)
		}
	}

	// Check README.md
	if stat, ok := stats["README.md"]; !ok {
		t.Error("Expected README.md in stats")
	} else {
		if stat.Changes != 20 {
			t.Errorf("README.md changes = %d, want 20", stat.Changes)
		}
		if stat.Deletions != 20 {
			t.Errorf("README.md deletions = %d, want 20", stat.Deletions)
		}
	}
}

func TestRepoManager_Clone(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")

	// Create a temporary git repository to clone from
	sourceRepo := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(sourceRepo, 0755); err != nil {
		t.Fatal(err)
	}

	// Initialize source repo
	commands := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test User"},
		{"git", "commit", "--allow-empty", "-m", "Initial commit"},
	}

	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = sourceRepo
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to setup source repo: %v", err)
		}
	}

	// Test cloning
	rm := NewRepoManager(repoPath, sourceRepo)
	ctx := context.Background()

	if err := rm.CloneOrUpdate(ctx); err != nil {
		t.Errorf("CloneOrUpdate() failed: %v", err)
	}

	// Verify .git directory exists
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); os.IsNotExist(err) {
		t.Error("Expected .git directory to exist after clone")
	}
}

func TestRepoManager_CloneOrUpdate_ExistingRepo(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	sourceRepo := filepath.Join(tmpDir, "source")

	// Create and initialize source repo
	if err := os.MkdirAll(sourceRepo, 0755); err != nil {
		t.Fatal(err)
	}

	commands := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test User"},
		{"git", "commit", "--allow-empty", "-m", "Initial commit"},
	}

	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = sourceRepo
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to setup source repo: %v", err)
		}
	}

	rm := NewRepoManager(repoPath, sourceRepo)
	ctx := context.Background()

	// First clone
	if err := rm.CloneOrUpdate(ctx); err != nil {
		t.Fatalf("First CloneOrUpdate() failed: %v", err)
	}

	// Second call should update (fetch) instead of clone
	if err := rm.CloneOrUpdate(ctx); err != nil {
		t.Errorf("Second CloneOrUpdate() failed: %v", err)
	}
}

func TestDiffStat(t *testing.T) {
	stat := DiffStat{
		File:      "test.go",
		Changes:   10,
		Additions: 7,
		Deletions: 3,
	}

	if stat.File != "test.go" {
		t.Errorf("Expected File 'test.go', got '%s'", stat.File)
	}
	if stat.Changes != 10 {
		t.Errorf("Expected Changes 10, got %d", stat.Changes)
	}
	if stat.Additions != 7 {
		t.Errorf("Expected Additions 7, got %d", stat.Additions)
	}
	if stat.Deletions != 3 {
		t.Errorf("Expected Deletions 3, got %d", stat.Deletions)
	}
}

func TestCheckoutPatchset_ReusesExistingBranch(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	ctx := context.Background()
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	sourceRepo := filepath.Join(tmpDir, "source")

	if err := os.MkdirAll(sourceRepo, 0755); err != nil {
		t.Fatal(err)
	}

	setup := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test User"},
		{"git", "commit", "--allow-empty", "-m", "Initial commit"},
	}
	for _, args := range setup {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = sourceRepo
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to setup source repo: %v", err)
		}
	}

	branchCmd := exec.Command("git", "branch", "--show-current")
	branchCmd.Dir = sourceRepo
	branchOut, err := branchCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to detect source branch: %v, output=%s", err, string(branchOut))
	}
	sourceBranch := strings.TrimSpace(string(branchOut))
	if sourceBranch == "" {
		t.Fatalf("detected empty source branch")
	}

	rm := NewRepoManager(repoPath, sourceRepo)
	if err := rm.CloneOrUpdate(ctx); err != nil {
		t.Fatalf("CloneOrUpdate() failed: %v", err)
	}

	if err := rm.FetchPatchset(ctx, fmt.Sprintf("refs/heads/%s", sourceBranch)); err != nil {
		t.Fatalf("FetchPatchset() failed: %v", err)
	}

	branchName, err := rm.CheckoutPatchset(ctx, 10722, 4)
	if err != nil {
		t.Fatalf("first CheckoutPatchset() failed: %v", err)
	}
	if branchName != "review-10722-4" {
		t.Fatalf("unexpected branch name %q", branchName)
	}

	// Simulate next run with same branch still present/current.
	if err := rm.FetchPatchset(ctx, fmt.Sprintf("refs/heads/%s", sourceBranch)); err != nil {
		t.Fatalf("second FetchPatchset() failed: %v", err)
	}
	if _, err := rm.CheckoutPatchset(ctx, 10722, 4); err != nil {
		t.Fatalf("second CheckoutPatchset() should succeed with existing branch: %v", err)
	}

	currentBranchCmd := exec.Command("git", "branch", "--show-current")
	currentBranchCmd.Dir = repoPath
	currentOut, err := currentBranchCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get current branch: %v, output=%s", err, string(currentOut))
	}
	if got := strings.TrimSpace(string(currentOut)); got != "review-10722-4" {
		t.Fatalf("expected current branch review-10722-4, got %q", got)
	}
}
