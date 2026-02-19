package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gerrit-ai-review/gerrit-tools/internal/gerrit"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// draftCmd represents the draft command group
var draftCmd = &cobra.Command{
	Use:   "draft",
	Short: "Manage draft comments",
	Long: `Create, list, update, and delete draft comments on changes.

Draft comments are not visible to others until published. Use this
command group to iteratively build up your code review feedback before
publishing it all at once.`,
}

// draftCreateCmd creates a new draft comment
var draftCreateCmd = &cobra.Command{
	Use:   "create <change-id> <file> <line> <message> [revision-id]",
	Short: "Create a new draft comment",
	Long: `Create a new draft comment on a specific file and line.

The revision-id can be:
  - "current" (default) - the latest patchset
  - Numeric patchset number (e.g., 1, 2, 3)
  - Commit SHA

Priority and Type:
  Use [P0-P3] prefix and 👍/👎 emoji in message for categorization:
  - [P0] 👎 Critical issue (auto-unresolved)
  - [P1] 👎 High priority (auto-unresolved)
  - [P2] 👎 Medium priority (auto-resolved)
  - [P3] 👍 Low priority / positive feedback (auto-resolved)

Examples:
  # Create critical issue comment
  gerrit-cli draft create 10661 src/main.go 42 "[P0] 👎 SQL injection vulnerability"

  # Create positive feedback
  gerrit-cli draft create 10661 src/utils.go 88 "[P3] 👍 Good use of defer"

  # Create on specific patchset
  gerrit-cli draft create 10661 src/main.go 42 "[P1] 👎 Missing error handling" 3

  # Override auto-resolved (mark P3 as unresolved)
  gerrit-cli draft create 10661 src/main.go 50 "[P3] 👎 Minor issue" --unresolved`,
	Args: cobra.RangeArgs(4, 5),
	RunE: runDraftCreate,
}

// draftListCmd lists draft comments
var draftListCmd = &cobra.Command{
	Use:   "list <change-id> [revision-id]",
	Short: "List draft comments",
	Long: `List all draft comments for the current user.

The revision-id can be:
  - "current" (default) - the latest patchset
  - Numeric patchset number (e.g., 1, 2, 3)
  - Commit SHA

Examples:
  # List all drafts on current patchset
  gerrit-cli draft list 10661

  # List drafts on specific patchset
  gerrit-cli draft list 10661 3

  # Filter by file
  gerrit-cli draft list 10661 --file src/main.go

  # Only unresolved drafts
  gerrit-cli draft list 10661 --unresolved`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runDraftList,
}

// draftUpdateCmd updates a draft comment
var draftUpdateCmd = &cobra.Command{
	Use:   "update <change-id> <draft-id> <message> [revision-id]",
	Short: "Update an existing draft comment",
	Long: `Update the message or unresolved status of a draft comment.

The revision-id can be:
  - "current" (default) - the latest patchset
  - Numeric patchset number (e.g., 1, 2, 3)
  - Commit SHA

Examples:
  # Update message
  gerrit-cli draft update 10661 abc123 "[P1] 👎 Updated message"

  # Mark as resolved
  gerrit-cli draft update 10661 abc123 --resolved

  # Mark as unresolved
  gerrit-cli draft update 10661 abc123 --unresolved

  # Update message on specific patchset
  gerrit-cli draft update 10661 abc123 "[P2] 👎 New message" 3`,
	Args: cobra.RangeArgs(3, 4),
	RunE: runDraftUpdate,
}

// draftDeleteCmd deletes a draft comment
var draftDeleteCmd = &cobra.Command{
	Use:   "delete <change-id> <draft-id> [revision-id]",
	Short: "Delete a draft comment",
	Long: `Delete a draft comment. This cannot be undone.

The revision-id can be:
  - "current" (default) - the latest patchset
  - Numeric patchset number (e.g., 1, 2, 3)
  - Commit SHA

Examples:
  # Delete a draft
  gerrit-cli draft delete 10661 abc123

  # Delete draft on specific patchset
  gerrit-cli draft delete 10661 abc123 3`,
	Args: cobra.RangeArgs(2, 3),
	RunE: runDraftDelete,
}

func init() {
	// Flags for draftCreateCmd
	draftCreateCmd.Flags().Bool("resolved", false, "Mark as resolved (override auto-detection)")
	draftCreateCmd.Flags().Bool("unresolved", false, "Mark as unresolved (override auto-detection)")
	draftCreateCmd.Flags().String("in-reply-to", "", "Reply to another comment ID")

	// Flags for draftListCmd
	draftListCmd.Flags().StringP("file", "f", "", "Filter drafts for specific file")
	draftListCmd.Flags().BoolP("unresolved", "u", false, "Show only unresolved drafts")

	// Flags for draftUpdateCmd
	draftUpdateCmd.Flags().Bool("resolved", false, "Mark as resolved")
	draftUpdateCmd.Flags().Bool("unresolved", false, "Mark as unresolved")

	// Add subcommands to draftCmd
	draftCmd.AddCommand(draftCreateCmd)
	draftCmd.AddCommand(draftListCmd)
	draftCmd.AddCommand(draftUpdateCmd)
	draftCmd.AddCommand(draftDeleteCmd)
}

// determineUnresolved determines if a comment should be unresolved based on priority prefix
func determineUnresolved(message string) *bool {
	// Check for P0 or P1 prefix (unresolved)
	if strings.HasPrefix(message, "[P0]") || strings.HasPrefix(message, "[P1]") {
		unresolved := true
		return &unresolved
	}

	// Check for P2 or P3 prefix (resolved)
	if strings.HasPrefix(message, "[P2]") || strings.HasPrefix(message, "[P3]") {
		unresolved := false
		return &unresolved
	}

	// No priority prefix, don't set unresolved field (let Gerrit use default)
	return nil
}

// runDraftCreate executes the draft create command
func runDraftCreate(cmd *cobra.Command, args []string) error {
	changeID := args[0]
	filePath := args[1]
	lineStr := args[2]
	message := args[3]

	revisionID := "current"
	if len(args) > 4 {
		revisionID = args[4]
	}

	// Parse line number
	line, err := strconv.Atoi(lineStr)
	if err != nil {
		return fmt.Errorf("invalid line number: %s", lineStr)
	}

	// Get flags
	resolvedFlag, _ := cmd.Flags().GetBool("resolved")
	unresolvedFlag, _ := cmd.Flags().GetBool("unresolved")
	inReplyTo, _ := cmd.Flags().GetString("in-reply-to")
	format := viper.GetString("output.format")

	// Determine unresolved status
	var unresolved *bool
	if unresolvedFlag {
		val := true
		unresolved = &val
	} else if resolvedFlag {
		val := false
		unresolved = &val
	} else {
		// Auto-detect from message priority prefix
		unresolved = determineUnresolved(message)
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
	return ExecuteCommand(format, "draft create", version, func() (interface{}, error) {
		ctx := context.Background()

		// Build draft input
		input := &gerrit.DraftInput{
			Path:       filePath,
			Line:       line,
			Message:    message,
			Unresolved: unresolved,
		}

		if inReplyTo != "" {
			input.InReplyTo = inReplyTo
		}

		// Create draft
		draft, err := client.CreateDraft(ctx, changeID, revisionID, input)
		if err != nil {
			return nil, fmt.Errorf("failed to create draft: %w", err)
		}

		return draft, nil
	})
}

// runDraftList executes the draft list command
func runDraftList(cmd *cobra.Command, args []string) error {
	changeID := args[0]
	revisionID := "current"
	if len(args) > 1 {
		revisionID = args[1]
	}

	fileFilter, _ := cmd.Flags().GetString("file")
	unresolvedOnly, _ := cmd.Flags().GetBool("unresolved")
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
	return ExecuteCommand(format, "draft list", version, func() (interface{}, error) {
		ctx := context.Background()

		// Get all drafts
		drafts, err := client.ListDrafts(ctx, changeID, revisionID)
		if err != nil {
			return nil, err
		}

		// Apply filters
		filteredDrafts := make(map[string][]gerrit.CommentInfo)

		for filePath, fileDrafts := range drafts {
			// Apply file filter if specified
			if fileFilter != "" && filePath != fileFilter {
				continue
			}

			// Apply unresolved filter if specified
			if unresolvedOnly {
				var unresolvedDrafts []gerrit.CommentInfo
				for _, draft := range fileDrafts {
					if draft.Unresolved {
						unresolvedDrafts = append(unresolvedDrafts, draft)
					}
				}
				if len(unresolvedDrafts) > 0 {
					filteredDrafts[filePath] = unresolvedDrafts
				}
			} else {
				filteredDrafts[filePath] = fileDrafts
			}
		}

		return filteredDrafts, nil
	})
}

// runDraftUpdate executes the draft update command
func runDraftUpdate(cmd *cobra.Command, args []string) error {
	changeID := args[0]
	draftID := args[1]
	message := args[2]

	revisionID := "current"
	if len(args) > 3 {
		revisionID = args[3]
	}

	// Get flags
	resolvedFlag, _ := cmd.Flags().GetBool("resolved")
	unresolvedFlag, _ := cmd.Flags().GetBool("unresolved")
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
	return ExecuteCommand(format, "draft update", version, func() (interface{}, error) {
		ctx := context.Background()

		// First, get the existing draft to preserve fields
		existingDraft, err := client.GetDraft(ctx, changeID, revisionID, draftID)
		if err != nil {
			return nil, fmt.Errorf("failed to get existing draft: %w", err)
		}

		// Build updated draft input
		input := &gerrit.DraftInput{
			Path:    existingDraft.Path,
			Line:    existingDraft.Line,
			Message: message,
		}

		// Set unresolved status
		if unresolvedFlag {
			val := true
			input.Unresolved = &val
		} else if resolvedFlag {
			val := false
			input.Unresolved = &val
		} else {
			// Auto-detect from message priority prefix
			input.Unresolved = determineUnresolved(message)
		}

		// Update draft
		updated, err := client.UpdateDraft(ctx, changeID, revisionID, draftID, input)
		if err != nil {
			return nil, fmt.Errorf("failed to update draft: %w", err)
		}

		return updated, nil
	})
}

// runDraftDelete executes the draft delete command
func runDraftDelete(cmd *cobra.Command, args []string) error {
	changeID := args[0]
	draftID := args[1]

	revisionID := "current"
	if len(args) > 2 {
		revisionID = args[2]
	}

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
	return ExecuteCommand(format, "draft delete", version, func() (interface{}, error) {
		ctx := context.Background()

		// Delete draft
		err := client.DeleteDraft(ctx, changeID, revisionID, draftID)
		if err != nil {
			return nil, fmt.Errorf("failed to delete draft: %w", err)
		}

		return map[string]interface{}{
			"deleted":  true,
			"draft_id": draftID,
		}, nil
	})
}
