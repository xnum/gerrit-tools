package reviewer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gerrit-ai-review/gerrit-tools/internal/config"
	"github.com/gerrit-ai-review/gerrit-tools/internal/gerrit"
	"github.com/gerrit-ai-review/gerrit-tools/internal/git"
	"github.com/gerrit-ai-review/gerrit-tools/internal/logger"
	"github.com/gerrit-ai-review/gerrit-tools/pkg/types"
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
	executor := NewReviewExecutor(repoPath, r.cfg)
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
		if errors.Is(err, ErrRateLimited) {
			if postErr := r.postRateLimitFailure(ctx, req, reviewCLI, err); postErr != nil {
				r.log.Warnf("failed to post rate-limit failure notice for %s #%d/%d: %v",
					req.Project, req.ChangeNumber, req.PatchsetNumber, postErr)
			}
		}
		return fmt.Errorf("%s execution failed: %w", reviewCLI, err)
	}

	r.log.Debugf("%s output length: %d characters", reviewCLI, len(output))

	r.log.Infof("Review completed: %s/c/%s/+/%d/%d",
		r.cfg.Gerrit.HTTPUrl, req.Project, req.ChangeNumber, req.PatchsetNumber)

	elapsed := time.Since(startTime)
	r.log.Infof("Total time: %.1fs", elapsed.Seconds())

	return nil
}

func (r *Reviewer) postRateLimitFailure(ctx context.Context, req ReviewRequest, reviewCLI string, cause error) error {
	client := gerrit.NewClient(r.cfg.Gerrit.HTTPUrl, r.cfg.Gerrit.HTTPUser, r.cfg.Gerrit.HTTPPass)

	review := &types.ReviewResult{
		Summary: buildRateLimitFailureSummary(reviewCLI, cause),
		Vote:    0,
	}

	if err := client.PostReview(ctx, req.ChangeNumber, req.PatchsetNumber, review); err != nil {
		return err
	}

	r.log.Infof("Posted rate-limit failure notice: %s #%d/%d",
		req.Project, req.ChangeNumber, req.PatchsetNumber)
	return nil
}

func buildRateLimitFailureSummary(reviewCLI string, cause error) string {
	var sb strings.Builder
	errMsg := "rate limit"
	if cause != nil {
		errMsg = truncateForReviewMessage(cause.Error(), 220)
	}

	sb.WriteString("Automated review started but could not finish because the AI backend hit a rate limit.\n\n")
	sb.WriteString(fmt.Sprintf("Backend: %s\n", reviewCLI))
	sb.WriteString("Result: no review comments were produced.\n")
	sb.WriteString(fmt.Sprintf("Error: %s\n", errMsg))
	sb.WriteString("\nPlease retry this patchset later.")

	return sb.String()
}

func truncateForReviewMessage(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "...(truncated)"
}
