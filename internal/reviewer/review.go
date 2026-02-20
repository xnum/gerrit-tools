package reviewer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gerrit-ai-review/gerrit-tools/internal/config"
	"github.com/gerrit-ai-review/gerrit-tools/internal/git"
	"github.com/gerrit-ai-review/gerrit-tools/internal/logger"
)

func configuredReviewCLI(cfg *config.Config) string {
	cli := strings.ToLower(strings.TrimSpace(cfg.Review.CLI))
	if cli == "" {
		return "claude"
	}
	return cli
}

// Reviewer handles the complete code review workflow
type Reviewer struct {
	cfg *config.Config
	log *logger.Logger
}

// ReviewRequest represents a request to review a patchset
type ReviewRequest struct {
	Project        string
	ChangeNumber   int
	PatchsetNumber int
}

// NewReviewer creates a new Reviewer instance
func NewReviewer(cfg *config.Config) *Reviewer {
	return &Reviewer{
		cfg: cfg,
		log: logger.Get(),
	}
}

// ReviewChange performs a complete review workflow
// This prepares the git environment and executes the configured review CLI with gerrit-cli.
func (r *Reviewer) ReviewChange(ctx context.Context, req ReviewRequest) error {
	startTime := time.Now()

	// Setup git repository
	gitURL := r.cfg.GetGitURL(req.Project)
	repoPath := r.cfg.GetRepoPath(req.Project)

	r.log.Debugf("Git URL: %s", gitURL)
	r.log.Debugf("Repo path: %s", repoPath)

	repoMgr := git.NewRepoManager(repoPath, gitURL)

	// Clone or update
	r.log.Debugf("Cloning/updating repository...")
	if err := repoMgr.CloneOrUpdate(ctx); err != nil {
		return fmt.Errorf("failed to clone/update: %w", err)
	}

	// Fetch patchset
	ref := git.GetPatchsetRef(req.ChangeNumber, req.PatchsetNumber)
	r.log.Debugf("Fetching patchset: %s", ref)
	if err := repoMgr.FetchPatchset(ctx, ref); err != nil {
		return fmt.Errorf("failed to fetch patchset: %w", err)
	}

	// Checkout
	r.log.Debugf("Checking out patchset...")
	branchName, err := repoMgr.CheckoutPatchset(ctx, req.ChangeNumber, req.PatchsetNumber)
	if err != nil {
		return fmt.Errorf("failed to checkout: %w", err)
	}

	defer func() {
		if err := repoMgr.Cleanup(ctx, branchName); err != nil {
			r.log.Warnf("Cleanup failed: %v", err)
		}
	}()

	// Check if there are changes
	r.log.Debugf("Checking for changes...")
	changedFiles, _, err := repoMgr.GetDiffStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get diff stats: %w", err)
	}

	if changedFiles == 0 {
		r.log.Info("No changes found, skipping review")
		return nil
	}

	r.log.Debugf("Changed files: %d", changedFiles)

	// Build prompt and execute configured review CLI
	r.log.Debugf("Building review prompt...")
	executor := NewClaudeExecutor(repoPath, r.cfg)
	changeInfo := ChangeInfo{
		Project:        req.Project,
		ChangeNumber:   req.ChangeNumber,
		PatchsetNumber: req.PatchsetNumber,
	}

	prompt, err := executor.BuildPrompt(changeInfo)
	if err != nil {
		return fmt.Errorf("failed to build prompt: %w", err)
	}

	reviewCLI := configuredReviewCLI(r.cfg)
	r.log.Debugf("Prompt length: %d characters", len(prompt))
	r.log.Infof("Executing %s for review (timeout: %ds)...", reviewCLI, r.cfg.Review.ClaudeTimeout)

	output, err := executor.ExecuteReview(ctx, prompt)
	if err != nil {
		return fmt.Errorf("%s execution failed: %w", reviewCLI, err)
	}

	r.log.Debugf("%s output length: %d characters", reviewCLI, len(output))

	r.log.Infof("Review completed: %s/c/%s/+/%d/%d",
		r.cfg.Gerrit.HTTPUrl, req.Project, req.ChangeNumber, req.PatchsetNumber)

	elapsed := time.Since(startTime)
	r.log.Infof("Total time: %.1fs", elapsed.Seconds())

	return nil
}
