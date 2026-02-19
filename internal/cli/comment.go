package cli

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/gerrit-ai-review/gerrit-tools/internal/gerrit"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// commentCmd represents the comment command group
var commentCmd = &cobra.Command{
	Use:   "comment",
	Short: "Manage comments on changes",
	Long: `Retrieve and manage comments on Gerrit changes.

Comments are inline code review feedback attached to specific files
and lines in a patchset.`,
}

// commentListCmd lists comments for a change
var commentListCmd = &cobra.Command{
	Use:   "list <change-id> [revision-id]",
	Short: "List comments for a change",
	Long: `List all comments for a specific patchset.

The revision-id can be:
  - "current" (default) - the latest patchset
  - Numeric patchset number (e.g., 1, 2, 3)
  - Commit SHA

Examples:
  # List all comments on current patchset
  gerrit-cli comment list 12345

  # List comments on specific patchset
  gerrit-cli comment list 12345 2

  # List comments for specific file only
  gerrit-cli comment list 12345 current --file src/main.go

  # List only unresolved comments
  gerrit-cli comment list 12345 --unresolved`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runCommentList,
}

// commentThreadsCmd shows comment threads with conversation history
var commentThreadsCmd = &cobra.Command{
	Use:   "threads <change-id>",
	Short: "Show comment threads with conversation history",
	Long: `Show comment threads across all patchsets with full conversation history and resolution status.

This command retrieves ALL comments from ALL patchsets and reconstructs conversation threads
by following reply chains. Each thread shows:
  - The original comment and all replies
  - Which patchset each comment was made on
  - Resolution status (based on the last comment in the thread)
  - Comment count

Examples:
  # Show all comment threads
  gerrit-cli comment threads 12345

  # Show only unresolved threads
  gerrit-cli comment threads 12345 --unresolved`,
	Args: cobra.ExactArgs(1),
	RunE: runCommentThreads,
}

func init() {
	// Add flags for commentListCmd
	commentListCmd.Flags().StringP("file", "f", "", "Filter comments for specific file")
	commentListCmd.Flags().BoolP("unresolved", "u", false, "Show only unresolved comments")

	// Add flags for commentThreadsCmd
	commentThreadsCmd.Flags().BoolP("unresolved", "u", false, "Show only unresolved threads")

	// Add subcommands to commentCmd
	commentCmd.AddCommand(commentListCmd)
	commentCmd.AddCommand(commentThreadsCmd)
}

// runCommentList executes the comment list command
func runCommentList(cmd *cobra.Command, args []string) error {
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
	return ExecuteCommand(format, "comment list", version, func() (interface{}, error) {
		ctx := context.Background()

		// Get all comments
		comments, err := client.ListComments(ctx, changeID, revisionID)
		if err != nil {
			return nil, err
		}

		// Apply filters
		filteredComments := make(map[string][]gerrit.CommentInfo)

		for filePath, fileComments := range comments {
			// Apply file filter if specified
			if fileFilter != "" && filePath != fileFilter {
				continue
			}

			// Apply unresolved filter if specified
			if unresolvedOnly {
				var unresolvedComments []gerrit.CommentInfo
				for _, comment := range fileComments {
					if comment.Unresolved {
						unresolvedComments = append(unresolvedComments, comment)
					}
				}
				if len(unresolvedComments) > 0 {
					filteredComments[filePath] = unresolvedComments
				}
			} else {
				filteredComments[filePath] = fileComments
			}
		}

		return filteredComments, nil
	})
}

// ThreadComment represents a comment within a thread with simplified fields
type ThreadComment struct {
	ID         string `json:"id"`
	Author     string `json:"author"`
	Patchset   int    `json:"patchset"`
	Date       string `json:"date"`
	Message    string `json:"message"`
	Unresolved bool   `json:"unresolved"`
}

// CommentThread represents a complete conversation thread
type CommentThread struct {
	ID           string          `json:"id"`
	File         string          `json:"file"`
	Line         int             `json:"line"`
	Patchset     int             `json:"patchset"`
	Resolved     bool            `json:"resolved"`
	CommentCount int             `json:"comment_count"`
	Comments     []ThreadComment `json:"comments"`
}

// ThreadsResponse represents the response structure for threads command
type ThreadsResponse struct {
	Threads []CommentThread `json:"threads"`
	Summary ThreadsSummary  `json:"summary"`
}

// ThreadsSummary provides summary statistics about threads
type ThreadsSummary struct {
	TotalThreads      int `json:"total_threads"`
	UnresolvedThreads int `json:"unresolved_threads"`
	ResolvedThreads   int `json:"resolved_threads"`
}

// runCommentThreads executes the comment threads command
func runCommentThreads(cmd *cobra.Command, args []string) error {
	changeID := args[0]
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
	return ExecuteCommand(format, "comment threads", version, func() (interface{}, error) {
		ctx := context.Background()

		// Get all comments across all patchsets
		allComments, err := client.ListAllComments(ctx, changeID)
		if err != nil {
			return nil, err
		}

		// Build threads from comments
		threads := buildThreads(allComments)

		// Apply filters
		filteredThreads := threads
		if unresolvedOnly {
			filteredThreads = filterUnresolvedThreads(threads)
		}

		// Calculate summary
		summary := calculateThreadsSummary(threads)

		return ThreadsResponse{
			Threads: filteredThreads,
			Summary: summary,
		}, nil
	})
}

// buildThreads reconstructs conversation threads from flat comment list
func buildThreads(allComments map[string][]gerrit.CommentInfo) []CommentThread {
	// Step 1: Flatten all comments from all files into a single list
	var flatComments []gerrit.CommentInfo
	fileMap := make(map[string]string) // comment ID -> file path

	for filePath, comments := range allComments {
		for _, comment := range comments {
			flatComments = append(flatComments, comment)
			fileMap[comment.ID] = filePath
		}
	}

	// Step 2: Build a map of comment ID -> comment for quick lookup
	commentMap := make(map[string]gerrit.CommentInfo)
	for _, comment := range flatComments {
		commentMap[comment.ID] = comment
	}

	// Step 3: Find root comments (those with empty InReplyTo)
	var rootComments []gerrit.CommentInfo
	for _, comment := range flatComments {
		if comment.InReplyTo == "" {
			rootComments = append(rootComments, comment)
		}
	}

	// Step 4: For each root comment, build a thread by collecting all replies
	threads := make([]CommentThread, 0)
	for _, root := range rootComments {
		thread := buildThread(root, commentMap, fileMap)
		threads = append(threads, thread)
	}

	// Step 5: Sort threads by patchset number, then by file name
	sort.Slice(threads, func(i, j int) bool {
		if threads[i].Patchset != threads[j].Patchset {
			return threads[i].Patchset < threads[j].Patchset
		}
		return threads[i].File < threads[j].File
	})

	return threads
}

// buildThread builds a single thread from a root comment
func buildThread(root gerrit.CommentInfo, commentMap map[string]gerrit.CommentInfo, fileMap map[string]string) CommentThread {
	// Collect all comments in the thread
	threadComments := []gerrit.CommentInfo{root}
	threadComments = append(threadComments, collectReplies(root.ID, commentMap)...)

	// Sort comments by timestamp to ensure chronological order
	sort.Slice(threadComments, func(i, j int) bool {
		return threadComments[i].Updated.Time.Before(threadComments[j].Updated.Time)
	})

	// Convert to simplified format
	var comments []ThreadComment
	for _, c := range threadComments {
		authorName := "unknown"
		if c.Author != nil {
			if c.Author.Name != "" {
				authorName = c.Author.Name
			} else if c.Author.Username != "" {
				authorName = c.Author.Username
			} else if c.Author.Email != "" {
				authorName = c.Author.Email
			}
		}

		comments = append(comments, ThreadComment{
			ID:         c.ID,
			Author:     authorName,
			Patchset:   c.PatchSet,
			Date:       c.Updated.Time.Format("2006-01-02 15:04:05"),
			Message:    c.Message,
			Unresolved: c.Unresolved,
		})
	}

	// Determine thread resolution: use the Unresolved field of the LAST comment
	lastComment := threadComments[len(threadComments)-1]
	resolved := !lastComment.Unresolved

	return CommentThread{
		ID:           root.ID,
		File:         fileMap[root.ID],
		Line:         root.Line,
		Patchset:     root.PatchSet,
		Resolved:     resolved,
		CommentCount: len(threadComments),
		Comments:     comments,
	}
}

// collectReplies recursively collects all replies to a comment
func collectReplies(commentID string, commentMap map[string]gerrit.CommentInfo) []gerrit.CommentInfo {
	var replies []gerrit.CommentInfo

	for _, comment := range commentMap {
		if comment.InReplyTo == commentID {
			replies = append(replies, comment)
			// Recursively collect replies to this reply
			replies = append(replies, collectReplies(comment.ID, commentMap)...)
		}
	}

	return replies
}

// filterUnresolvedThreads filters threads to only include unresolved ones
func filterUnresolvedThreads(threads []CommentThread) []CommentThread {
	filtered := make([]CommentThread, 0)
	for _, thread := range threads {
		if !thread.Resolved {
			filtered = append(filtered, thread)
		}
	}
	return filtered
}

// calculateThreadsSummary calculates summary statistics about threads
func calculateThreadsSummary(threads []CommentThread) ThreadsSummary {
	summary := ThreadsSummary{
		TotalThreads: len(threads),
	}

	for _, thread := range threads {
		if thread.Resolved {
			summary.ResolvedThreads++
		} else {
			summary.UnresolvedThreads++
		}
	}

	return summary
}
