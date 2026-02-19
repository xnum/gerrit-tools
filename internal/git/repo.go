package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// RepoManager handles git repository operations
type RepoManager struct {
	repoPath string
	gitURL   string
}

// NewRepoManager creates a new repository manager
func NewRepoManager(repoPath, gitURL string) *RepoManager {
	return &RepoManager{
		repoPath: repoPath,
		gitURL:   gitURL,
	}
}

// CloneOrUpdate clones the repository if it doesn't exist, or updates it if it does
func (r *RepoManager) CloneOrUpdate(ctx context.Context) error {
	// Check if repo already exists
	if _, err := os.Stat(filepath.Join(r.repoPath, ".git")); err == nil {
		// Repo exists, fetch latest
		return r.fetch(ctx)
	}

	// Repo doesn't exist, clone it
	return r.clone(ctx)
}

// clone performs initial clone of the repository
func (r *RepoManager) clone(ctx context.Context) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(r.repoPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "clone", r.gitURL, r.repoPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// fetch updates the repository with latest refs
func (r *RepoManager) fetch(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "fetch", "origin")
	cmd.Dir = r.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// FetchPatchset fetches a specific patchset ref from Gerrit
// ref format: refs/changes/45/12345/3
func (r *RepoManager) FetchPatchset(ctx context.Context, ref string) error {
	cmd := exec.CommandContext(ctx, "git", "fetch", "origin", ref)
	cmd.Dir = r.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch patchset failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// CheckoutPatchset creates a branch and checks out the patchset
// Returns the branch name that was created
func (r *RepoManager) CheckoutPatchset(ctx context.Context, changeNum, patchsetNum int) (string, error) {
	branchName := fmt.Sprintf("review-%d-%d", changeNum, patchsetNum)

	// Delete branch if it already exists
	cmd := exec.CommandContext(ctx, "git", "branch", "-D", branchName)
	cmd.Dir = r.repoPath
	_ = cmd.Run() // Ignore error if branch doesn't exist

	// Create and checkout new branch from FETCH_HEAD
	cmd = exec.CommandContext(ctx, "git", "checkout", "-b", branchName, "FETCH_HEAD")
	cmd.Dir = r.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git checkout failed: %w\nOutput: %s", err, string(output))
	}

	return branchName, nil
}

// GetDiffStats returns statistics about changed files
// Returns: number of changed files, diff stats output, error
func (r *RepoManager) GetDiffStats(ctx context.Context) (int, string, error) {
	// Get list of changed files
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "HEAD^")
	cmd.Dir = r.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, "", fmt.Errorf("git diff failed: %w\nOutput: %s", err, string(output))
	}

	files := strings.TrimSpace(string(output))
	if files == "" {
		return 0, "", nil
	}

	fileList := strings.Split(files, "\n")
	changedFiles := len(fileList)

	// Get detailed stats
	cmd = exec.CommandContext(ctx, "git", "diff", "--stat", "HEAD^")
	cmd.Dir = r.repoPath
	statsOutput, err := cmd.CombinedOutput()
	if err != nil {
		return 0, "", fmt.Errorf("git diff --stat failed: %w\nOutput: %s", err, string(statsOutput))
	}

	return changedFiles, string(statsOutput), nil
}

// Cleanup removes the temporary review branch and returns to master/main
func (r *RepoManager) Cleanup(ctx context.Context, branchName string) error {
	// Try to checkout main branch (try both 'main' and 'master')
	for _, mainBranch := range []string{"main", "master"} {
		cmd := exec.CommandContext(ctx, "git", "checkout", mainBranch)
		cmd.Dir = r.repoPath
		if err := cmd.Run(); err == nil {
			// Successfully checked out main branch
			break
		}
	}

	// Delete the review branch
	cmd := exec.CommandContext(ctx, "git", "branch", "-D", branchName)
	cmd.Dir = r.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git branch -D failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// GetPatchsetRef constructs the Gerrit ref path for a patchset
// Format: refs/changes/<last-two-digits>/<change-number>/<patchset-number>
func GetPatchsetRef(changeNum, patchsetNum int) string {
	lastTwoDigits := changeNum % 100
	return fmt.Sprintf("refs/changes/%02d/%d/%d", lastTwoDigits, changeNum, patchsetNum)
}

// GetCommitMessage returns the commit message of the current HEAD
func (r *RepoManager) GetCommitMessage(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "log", "-1", "--pretty=format:%B")
	cmd.Dir = r.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git log failed: %w\nOutput: %s", err, string(output))
	}

	return string(output), nil
}

// GetChangedFiles returns a list of files changed in the current commit
func (r *RepoManager) GetChangedFiles(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "HEAD^")
	cmd.Dir = r.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w\nOutput: %s", err, string(output))
	}

	files := strings.TrimSpace(string(output))
	if files == "" {
		return []string{}, nil
	}

	return strings.Split(files, "\n"), nil
}

// GetFileDiff returns the diff for a specific file
func (r *RepoManager) GetFileDiff(ctx context.Context, filePath string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD^", "--", filePath)
	cmd.Dir = r.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w\nOutput: %s", err, string(output))
	}

	return string(output), nil
}

// ParseDiffStats parses the git diff --stat output
// Returns a map of filename -> changes info
func ParseDiffStats(statsOutput string) map[string]DiffStat {
	result := make(map[string]DiffStat)

	// Regex to parse lines like: "path/to/file.go | 10 +++++-----"
	re := regexp.MustCompile(`^\s*(.+?)\s+\|\s+(\d+)\s+([+\-]+)`)

	lines := strings.Split(statsOutput, "\n")
	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if len(matches) >= 3 {
			fileName := strings.TrimSpace(matches[1])
			changes, _ := strconv.Atoi(matches[2])

			additions := 0
			deletions := 0
			if len(matches) >= 4 {
				plusMinus := matches[3]
				additions = strings.Count(plusMinus, "+")
				deletions = strings.Count(plusMinus, "-")
			}

			result[fileName] = DiffStat{
				File:      fileName,
				Changes:   changes,
				Additions: additions,
				Deletions: deletions,
			}
		}
	}

	return result
}

// DiffStat represents statistics for a single file's changes
type DiffStat struct {
	File      string
	Changes   int // Total number of changed lines
	Additions int // Number of added lines
	Deletions int // Number of deleted lines
}
