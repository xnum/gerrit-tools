package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gerrit-ai-review/gerrit-tools/internal/gerrit"
	"github.com/gerrit-ai-review/gerrit-tools/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// reviewCmd represents the review command group
var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Post reviews on changes",
	Long: `Post code review feedback with votes and inline comments.

Use this command to submit review feedback on a change, including:
  - Overall review message
  - Vote/score (e.g., Code-Review +1/-1/+2/-2)
  - Inline comments on specific files and lines`,
}

// reviewPostCmd posts a review
var reviewPostCmd = &cobra.Command{
	Use:   "post <change-id> [revision-id]",
	Short: "Post a review on a change",
	Long: `Post a code review with message, vote, and inline comments.

The revision-id can be:
  - "current" (default) - the latest patchset
  - Numeric patchset number (e.g., 1, 2, 3)
  - Commit SHA

Examples:
  # Post a simple review message with +1 vote
  gerrit-cli review post 12345 --message "Looks good!" --vote 1

  # Post review with -1 vote
  gerrit-cli review post 12345 --message "Needs work" --vote -1

  # Post review with inline comments
  gerrit-cli review post 12345 --message "Review feedback" --vote 0 \
    --comment "src/main.go:10:Consider using a constant here" \
    --comment "src/main.go:25:This function is too complex"

  # Post review on specific patchset
  gerrit-cli review post 12345 2 --message "Review of patchset 2" --vote 1

Inline Comment Format:
  file:line:message

  Example: "src/main.go:42:This should be refactored"`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runReviewPost,
}

func init() {
	// Add flags for reviewPostCmd
	reviewPostCmd.Flags().StringP("message", "m", "", "Review message (required)")
	reviewPostCmd.Flags().IntP("vote", "v", 0, "Code-Review vote (-2, -1, 0, +1, +2)")
	reviewPostCmd.Flags().StringSliceP("comment", "c", []string{}, "Inline comments in format 'file:line:message'")

	reviewPostCmd.MarkFlagRequired("message")

	// Add subcommands to reviewCmd
	reviewCmd.AddCommand(reviewPostCmd)
}

// parseInlineComment parses an inline comment string in format "file:line:message"
func parseInlineComment(commentStr string) (*types.Comment, error) {
	parts := strings.SplitN(commentStr, ":", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid comment format: %s (expected file:line:message)", commentStr)
	}

	line, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid line number in comment: %s", parts[1])
	}

	return &types.Comment{
		File:    parts[0],
		Line:    line,
		Message: parts[2],
	}, nil
}

// runReviewPost executes the review post command
func runReviewPost(cmd *cobra.Command, args []string) error {
	changeID := args[0]
	revisionID := "current"
	if len(args) > 1 {
		revisionID = args[1]
	}

	message, _ := cmd.Flags().GetString("message")
	vote, _ := cmd.Flags().GetInt("vote")
	commentStrs, _ := cmd.Flags().GetStringSlice("comment")
	format := viper.GetString("output.format")

	// Validate vote
	if vote < -2 || vote > 2 {
		fmt.Fprintln(os.Stderr, FormatErrorResponse(format, "Vote must be between -2 and +2", "INVALID_VOTE"))
		return fmt.Errorf("invalid vote")
	}

	// Parse inline comments
	var comments []types.Comment
	for _, commentStr := range commentStrs {
		comment, err := parseInlineComment(commentStr)
		if err != nil {
			fmt.Fprintln(os.Stderr, FormatErrorResponse(format, err.Error(), "INVALID_COMMENT"))
			return err
		}
		comments = append(comments, *comment)
	}

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
	return ExecuteCommand(format, "review post", version, func() (interface{}, error) {
		ctx := context.Background()

		// First, get the change to extract numeric IDs if needed
		change, err := client.GetChangeDetail(ctx, changeID, []string{})
		if err != nil {
			return nil, fmt.Errorf("failed to get change details: %w", err)
		}

		// Determine patchset number
		patchsetNum := 0
		if revisionID == "current" {
			// Find the current revision number
			if change.CurrentRevision != "" && change.Revisions != nil {
				if rev, ok := change.Revisions[change.CurrentRevision]; ok {
					patchsetNum = rev.Number
				}
			}
		} else {
			// Try to parse as number
			num, err := strconv.Atoi(revisionID)
			if err != nil {
				return nil, fmt.Errorf("invalid revision ID: %s", revisionID)
			}
			patchsetNum = num
		}

		if patchsetNum == 0 {
			return nil, fmt.Errorf("could not determine patchset number")
		}

		// Build review result
		reviewResult := &types.ReviewResult{
			Summary:  message,
			Vote:     vote,
			Comments: comments,
		}

		// Post the review
		err = client.PostReview(ctx, change.Number, patchsetNum, reviewResult)
		if err != nil {
			return nil, fmt.Errorf("failed to post review: %w", err)
		}

		// Return success response
		return map[string]interface{}{
			"change":   change.Number,
			"patchset": patchsetNum,
			"vote":     vote,
			"message":  message,
			"comments": len(comments),
		}, nil
	})
}
