package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/gerrit-ai-review/gerrit-tools/internal/gerrit"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// patchsetCmd represents the patchset command group
var patchsetCmd = &cobra.Command{
	Use:   "patchset",
	Short: "Manage patchsets (revisions)",
	Long: `Query and retrieve information about patchsets.

A patchset (also called revision) represents a specific version of a change.
Each time a change is updated, a new patchset is created.`,
}

// patchsetDiffCmd gets the diff for a patchset
var patchsetDiffCmd = &cobra.Command{
	Use:   "diff <change-id> [revision-id]",
	Short: "Get diff for a patchset",
	Long: `Get the diff for a specific patchset.

The revision-id can be:
  - "current" (default) - the latest patchset
  - Numeric patchset number (e.g., 1, 2, 3)
  - Commit SHA

Examples:
  # Get diff for current patchset (all files)
  gerrit-cli patchset diff 12345

  # Get diff for specific patchset number
  gerrit-cli patchset diff 12345 2

  # Get diff for specific file only
  gerrit-cli patchset diff 12345 current --file src/main.go

  # List files only (no diff content)
  gerrit-cli patchset diff 12345 --list-files

  # Get incremental diff between patchset 1 and 3
  gerrit-cli patchset diff 12345 3 --base 1`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runPatchsetDiff,
}

func init() {
	// Add flags for patchsetDiffCmd
	patchsetDiffCmd.Flags().StringP("file", "f", "", "Get diff for specific file only")
	patchsetDiffCmd.Flags().BoolP("list-files", "l", false, "List files only (no diff content)")
	patchsetDiffCmd.Flags().StringP("base", "b", "", "Base patchset to compare against (for incremental diff)")

	// Add subcommands to patchsetCmd
	patchsetCmd.AddCommand(patchsetDiffCmd)
}

// runPatchsetDiff executes the patchset diff command
func runPatchsetDiff(cmd *cobra.Command, args []string) error {
	changeID := args[0]
	revisionID := "current"
	if len(args) > 1 {
		revisionID = args[1]
	}

	file, _ := cmd.Flags().GetString("file")
	listFiles, _ := cmd.Flags().GetBool("list-files")
	base, _ := cmd.Flags().GetString("base")
	format := viper.GetString("output.format")

	// Get Gerrit configuration
	httpURL := viper.GetString("gerrit.http_url")
	httpUser := viper.GetString("gerrit.http_user")
	httpPassword := viper.GetString("gerrit.http_password")

	if httpURL == "" || httpUser == "" || httpPassword == "" {
		fmt.Fprintln(os.Stderr, FormatErrorResponse(format, "Gerrit HTTP configuration not found. Set GERRIT_HTTP_URL, GERRIT_HTTP_USER, and GERRIT_HTTP_PASSWORD.", "CONFIG_ERROR"))
		return fmt.Errorf("configuration error")
	}

	// Create Gerrit client
	client := gerrit.NewClient(httpURL, httpUser, httpPassword)

	// Execute command with standard formatting
	return ExecuteCommand(format, "patchset diff", version, func() (interface{}, error) {
		ctx := context.Background()

		// If list-files flag is set, just return the file list
		if listFiles {
			files, err := client.GetRevisionFiles(ctx, changeID, revisionID, base)
			if err != nil {
				return nil, err
			}
			return files, nil
		}

		// If a specific file is requested, get diff for that file only
		if file != "" {
			diff, err := client.GetRevisionDiff(ctx, changeID, revisionID, file, base)
			if err != nil {
				return nil, err
			}
			return map[string]*gerrit.DiffInfo{file: diff}, nil
		}

		// Otherwise, get all files and their diffs
		files, err := client.GetRevisionFiles(ctx, changeID, revisionID, base)
		if err != nil {
			return nil, err
		}

		// Get diff for each file
		diffs := make(map[string]*gerrit.DiffInfo)
		for filePath := range files {
			// Skip /COMMIT_MSG and /MERGE_LIST special files from diff
			if filePath == "/COMMIT_MSG" || filePath == "/MERGE_LIST" {
				continue
			}

			diff, err := client.GetRevisionDiff(ctx, changeID, revisionID, filePath, base)
			if err != nil {
				// Log error but continue with other files
				fmt.Fprintf(os.Stderr, "Warning: failed to get diff for %s: %v\n", filePath, err)
				continue
			}
			diffs[filePath] = diff
		}

		return diffs, nil
	})
}
