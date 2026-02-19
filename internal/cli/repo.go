package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gerrit-ai-review/gerrit-tools/internal/gerrit"
	"github.com/gerrit-ai-review/gerrit-tools/internal/git"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// repoCmd represents the repo command group
var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Repository operations",
	Long: `Manage local repository checkouts for code review.

These commands allow you to checkout patchsets to local directories
for detailed code inspection and analysis.`,
}

// repoCheckoutCmd checks out a specific patchset's code to a local directory
var repoCheckoutCmd = &cobra.Command{
	Use:   "checkout <change-id> [patchset-number]",
	Short: "Checkout a patchset to a local directory",
	Long: `Checkout a specific patchset's code to a local directory.

The command will:
  1. Fetch change details from Gerrit
  2. Clone or update the repository
  3. Fetch the specific patchset
  4. Create and checkout a review branch

The patchset-number is optional:
  - If omitted, checks out the current/latest patchset
  - If provided, checks out the specific patchset number

Examples:
  # Checkout current patchset
  gerrit-cli repo checkout 12345

  # Checkout specific patchset number
  gerrit-cli repo checkout 12345 3

Output:
  Returns JSON with repository path and metadata:
  {
    "success": true,
    "data": {
      "repo_path": "/tmp/ai-review-repos/myproject",
      "change_number": 12345,
      "patchset_number": 3,
      "project": "myproject",
      "branch": "review-12345-3",
      "ref": "refs/changes/45/12345/3"
    }
  }`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runRepoCheckout,
}

func init() {
	// Add subcommands to repoCmd
	repoCmd.AddCommand(repoCheckoutCmd)
}

// CheckoutResult represents the result of a checkout operation
type CheckoutResult struct {
	RepoPath       string `json:"repo_path"`
	ChangeNumber   int    `json:"change_number"`
	PatchsetNumber int    `json:"patchset_number"`
	Project        string `json:"project"`
	Branch         string `json:"branch"`
	Ref            string `json:"ref"`
}

// runRepoCheckout executes the repo checkout command
func runRepoCheckout(cmd *cobra.Command, args []string) error {
	changeID := args[0]
	var patchsetNum int
	var err error

	// Parse patchset number if provided
	if len(args) > 1 {
		patchsetNum, err = strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid patchset number: %s", args[1])
		}
	}

	format := viper.GetString("output.format")

	// Get Gerrit HTTP configuration
	httpURL := viper.GetString("gerrit.http_url")
	httpUser := viper.GetString("gerrit.http_user")
	httpPassword := viper.GetString("gerrit.http_password")

	if httpURL == "" || httpUser == "" || httpPassword == "" {
		fmt.Fprintln(os.Stderr, FormatErrorResponse(format, "Gerrit HTTP configuration not found. Set GERRIT_HTTP_URL, GERRIT_HTTP_USER, and GERRIT_HTTP_PASSWORD.", "CONFIG_ERROR"))
		return fmt.Errorf("configuration error")
	}

	// Get Git configuration
	sshAlias := viper.GetString("gerrit.ssh_alias")
	repoBasePath := viper.GetString("git.repo_base_path")

	if sshAlias == "" {
		fmt.Fprintln(os.Stderr, FormatErrorResponse(format, "Git SSH alias not found. Set GERRIT_SSH_ALIAS.", "CONFIG_ERROR"))
		return fmt.Errorf("configuration error")
	}

	if repoBasePath == "" {
		// Default to /tmp/ai-review-repos if not specified
		repoBasePath = "/tmp/ai-review-repos"
	}

	// Execute command with standard formatting
	return ExecuteCommand(format, "repo checkout", version, func() (interface{}, error) {
		ctx := context.Background()

		// Create Gerrit client
		client := gerrit.NewClient(httpURL, httpUser, httpPassword)

		// Fetch change details to get project and current revision
		change, err := client.GetChangeDetail(ctx, changeID, []string{"CURRENT_REVISION", "ALL_REVISIONS"})
		if err != nil {
			return nil, fmt.Errorf("failed to get change details: %w", err)
		}

		// Determine which patchset to checkout
		var revision *gerrit.RevisionInfo
		var targetPatchsetNum int

		if patchsetNum > 0 {
			// Find the specific patchset
			targetPatchsetNum = patchsetNum
			for _, rev := range change.Revisions {
				if rev.Number == patchsetNum {
					revision = rev
					break
				}
			}
			if revision == nil {
				return nil, fmt.Errorf("patchset %d not found in change %s", patchsetNum, changeID)
			}
		} else {
			// Use current revision
			if change.CurrentRevision == "" {
				return nil, fmt.Errorf("no current revision found for change %s", changeID)
			}
			revision = change.Revisions[change.CurrentRevision]
			if revision == nil {
				return nil, fmt.Errorf("current revision data not found for change %s", changeID)
			}
			targetPatchsetNum = revision.Number
		}

		// Construct git URL (format: ssh://{ssh_alias}/{project})
		gitURL := fmt.Sprintf("%s:%s", sshAlias, change.Project)

		// Construct local repo path
		// Replace / with - to create safe directory names
		safeName := filepath.Base(change.Project)
		repoPath := filepath.Join(repoBasePath, safeName)

		// Create repo manager
		repoManager := git.NewRepoManager(repoPath, gitURL)

		// Clone or update repository
		if err := repoManager.CloneOrUpdate(ctx); err != nil {
			return nil, fmt.Errorf("failed to clone/update repository: %w", err)
		}

		// Construct patchset ref from the revision info
		patchsetRef := revision.Ref

		// Fetch the specific patchset
		if err := repoManager.FetchPatchset(ctx, patchsetRef); err != nil {
			return nil, fmt.Errorf("failed to fetch patchset: %w", err)
		}

		// Checkout the patchset to a review branch
		branchName, err := repoManager.CheckoutPatchset(ctx, change.Number, targetPatchsetNum)
		if err != nil {
			return nil, fmt.Errorf("failed to checkout patchset: %w", err)
		}

		// Return result
		result := &CheckoutResult{
			RepoPath:       repoPath,
			ChangeNumber:   change.Number,
			PatchsetNumber: targetPatchsetNum,
			Project:        change.Project,
			Branch:         branchName,
			Ref:            patchsetRef,
		}

		return result, nil
	})
}
