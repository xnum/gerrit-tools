package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/gerrit-ai-review/gerrit-tools/internal/gerrit"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// summaryCmd represents the summary command
var summaryCmd = &cobra.Command{
	Use:   "summary <change-id>",
	Short: "Get a comprehensive summary of a change",
	Long: `Get a comprehensive overview of a Gerrit change including:
  - Basic information (status, owner, dates)
  - Patchset history
  - Statistics (files changed, lines added/deleted)
  - Comment summary (total count, unresolved count)
  - Review votes and labels

Examples:
  # Get summary for change 12345
  gerrit-cli summary 12345

  # Get summary with JSON output
  gerrit-cli summary 12345 --format json`,
	Args: cobra.ExactArgs(1),
	RunE: runSummary,
}

// ChangeSummary represents a comprehensive summary of a change
type ChangeSummary struct {
	Basic      BasicInfo              `json:"basic"`
	Patchsets  []PatchsetSummary      `json:"patchsets"`
	Statistics ChangeStatistics       `json:"statistics"`
	Comments   CommentsSummary        `json:"comments"`
	Votes      map[string]interface{} `json:"votes"`
}

type BasicInfo struct {
	Number  int    `json:"number"`
	ID      string `json:"id"`
	Subject string `json:"subject"`
	Status  string `json:"status"`
	Project string `json:"project"`
	Branch  string `json:"branch"`
	Owner   string `json:"owner"`
	Created string `json:"created"`
	Updated string `json:"updated"`
	Topic   string `json:"topic,omitempty"`
}

type PatchsetSummary struct {
	Number    int    `json:"number"`
	Ref       string `json:"ref"`
	Uploader  string `json:"uploader"`
	Created   string `json:"created"`
	IsCurrent bool   `json:"is_current"`
}

type ChangeStatistics struct {
	FilesChanged  int `json:"files_changed"`
	LinesInserted int `json:"lines_inserted"`
	LinesDeleted  int `json:"lines_deleted"`
}

type CommentsSummary struct {
	Total      int                     `json:"total"`
	Unresolved int                     `json:"unresolved"`
	ByFile     map[string]FileComments `json:"by_file"`
}

type FileComments struct {
	Total      int `json:"total"`
	Unresolved int `json:"unresolved"`
}

func init() {
	summaryCmd.Flags().Bool("include-messages", false, "Include change messages in summary")
}

// runSummary executes the summary command
func runSummary(cmd *cobra.Command, args []string) error {
	changeID := args[0]
	format := viper.GetString("output.format")
	includeMessages, _ := cmd.Flags().GetBool("include-messages")

	// Get Gerrit configuration
	httpURL := viper.GetString("gerrit.http_url")
	httpUser := viper.GetString("gerrit.http_user")
	httpPassword := viper.GetString("gerrit.http_password")

	if httpURL == "" || httpUser == "" || httpPassword == "" {
		fmt.Fprintln(os.Stderr, FormatErrorResponse(format, "Gerrit HTTP configuration not found", "CONFIG_ERROR"))
		return fmt.Errorf("configuration error")
	}

	// Create Gerrit client
	client := gerrit.NewClient(httpURL, httpUser, httpPassword)

	// Execute command with standard formatting
	return ExecuteCommand(format, "summary", version, func() (interface{}, error) {
		ctx := context.Background()

		// Get change details with all necessary options
		options := []string{
			"CURRENT_REVISION",
			"ALL_REVISIONS",
			"LABELS",
			"DETAILED_ACCOUNTS",
			"DETAILED_LABELS",
		}

		if includeMessages {
			options = append(options, "MESSAGES")
		}

		change, err := client.GetChangeDetail(ctx, changeID, options)
		if err != nil {
			return nil, fmt.Errorf("failed to get change details: %w", err)
		}

		// Build summary
		summary := &ChangeSummary{
			Basic: BasicInfo{
				Number:  change.Number,
				ID:      change.ID,
				Subject: change.Subject,
				Status:  change.Status,
				Project: change.Project,
				Branch:  change.Branch,
				Owner:   change.Owner.Name,
				Created: change.Created.Time.Format("2006-01-02 15:04:05"),
				Updated: change.Updated.Time.Format("2006-01-02 15:04:05"),
				Topic:   change.Topic,
			},
			Statistics: ChangeStatistics{
				LinesInserted: change.Insertions,
				LinesDeleted:  change.Deletions,
			},
			Comments: CommentsSummary{
				Total:      change.TotalCommentCount,
				Unresolved: change.UnresolvedCommentCount,
				ByFile:     make(map[string]FileComments),
			},
			Votes: make(map[string]interface{}),
		}

		// Extract patchset information
		if len(change.Revisions) > 0 {
			summary.Patchsets = extractPatchsetSummary(change)
			summary.Statistics.FilesChanged = countFilesInCurrentRevision(change)
		}

		// Get comments details
		if change.TotalCommentCount > 0 {
			comments, err := client.ListComments(ctx, changeID, "current")
			if err == nil {
				summary.Comments.ByFile = summarizeCommentsByFile(comments)
			}
		}

		// Extract votes from labels
		if change.Labels != nil {
			summary.Votes = extractVotes(change.Labels)
		}

		return summary, nil
	})
}

// extractPatchsetSummary extracts patchset information from revisions
func extractPatchsetSummary(change *gerrit.ChangeInfo) []PatchsetSummary {
	var patchsets []PatchsetSummary

	for revID, rev := range change.Revisions {
		ps := PatchsetSummary{
			Number:    rev.Number,
			Ref:       rev.Ref,
			Created:   rev.Created.Time.Format("2006-01-02 15:04:05"),
			IsCurrent: revID == change.CurrentRevision,
			Uploader:  rev.Uploader.Name,
		}

		patchsets = append(patchsets, ps)
	}

	// Sort by patchset number
	sort.Slice(patchsets, func(i, j int) bool {
		return patchsets[i].Number < patchsets[j].Number
	})

	return patchsets
}

// countFilesInCurrentRevision counts files in current revision
func countFilesInCurrentRevision(change *gerrit.ChangeInfo) int {
	if change.CurrentRevision == "" {
		return 0
	}

	rev, ok := change.Revisions[change.CurrentRevision]
	if !ok || rev.Files == nil {
		return 0
	}

	// Exclude /COMMIT_MSG and /MERGE_LIST
	count := 0
	for file := range rev.Files {
		if file != "/COMMIT_MSG" && file != "/MERGE_LIST" {
			count++
		}
	}

	return count
}

// summarizeCommentsByFile summarizes comments grouped by file
func summarizeCommentsByFile(comments map[string][]gerrit.CommentInfo) map[string]FileComments {
	byFile := make(map[string]FileComments)

	for file, fileComments := range comments {
		// Skip commit message
		if file == "/COMMIT_MSG" || file == "/MERGE_LIST" {
			continue
		}

		fc := FileComments{
			Total: len(fileComments),
		}

		for _, comment := range fileComments {
			if comment.Unresolved {
				fc.Unresolved++
			}
		}

		byFile[file] = fc
	}

	return byFile
}

// extractVotes extracts vote information from labels
func extractVotes(labels map[string]*gerrit.LabelInfo) map[string]interface{} {
	votes := make(map[string]interface{})

	for labelName, labelInfo := range labels {
		if labelInfo == nil {
			continue
		}

		voteInfo := make(map[string]interface{})

		// Get all votes
		if len(labelInfo.All) > 0 {
			var allVotes []map[string]interface{}
			for _, approval := range labelInfo.All {
				if approval.Value != 0 {
					allVotes = append(allVotes, map[string]interface{}{
						"value": approval.Value,
						"user":  approval.Name,
						"date":  approval.Date.Time.Format("2006-01-02 15:04:05"),
					})
				}
			}
			voteInfo["votes"] = allVotes
		}

		// Get approved/rejected status
		if labelInfo.Approved != nil {
			voteInfo["approved_by"] = labelInfo.Approved.Name
		}
		if labelInfo.Rejected != nil {
			voteInfo["rejected_by"] = labelInfo.Rejected.Name
		}

		votes[labelName] = voteInfo
	}

	return votes
}

// formatSummaryText formats the summary as human-readable text
func formatSummaryText(summary *ChangeSummary) string {
	var b strings.Builder

	// Basic info
	b.WriteString(fmt.Sprintf("Change #%d: %s\n", summary.Basic.Number, summary.Basic.Subject))
	b.WriteString(fmt.Sprintf("Status: %s\n", summary.Basic.Status))
	b.WriteString(fmt.Sprintf("Project: %s (%s)\n", summary.Basic.Project, summary.Basic.Branch))
	b.WriteString(fmt.Sprintf("Owner: %s\n", summary.Basic.Owner))
	b.WriteString(fmt.Sprintf("Created: %s\n", summary.Basic.Created))
	b.WriteString(fmt.Sprintf("Updated: %s\n", summary.Basic.Updated))
	if summary.Basic.Topic != "" {
		b.WriteString(fmt.Sprintf("Topic: %s\n", summary.Basic.Topic))
	}
	b.WriteString("\n")

	// Statistics
	b.WriteString("Statistics:\n")
	b.WriteString(fmt.Sprintf("  Files changed: %d\n", summary.Statistics.FilesChanged))
	b.WriteString(fmt.Sprintf("  Lines: +%d -%d\n", summary.Statistics.LinesInserted, summary.Statistics.LinesDeleted))
	b.WriteString("\n")

	// Patchsets
	b.WriteString(fmt.Sprintf("Patchsets (%d):\n", len(summary.Patchsets)))
	for _, ps := range summary.Patchsets {
		current := ""
		if ps.IsCurrent {
			current = " (current)"
		}
		b.WriteString(fmt.Sprintf("  PS%d: %s by %s%s\n", ps.Number, ps.Created, ps.Uploader, current))
	}
	b.WriteString("\n")

	// Comments
	b.WriteString(fmt.Sprintf("Comments: %d total, %d unresolved\n", summary.Comments.Total, summary.Comments.Unresolved))
	if len(summary.Comments.ByFile) > 0 {
		for file, fc := range summary.Comments.ByFile {
			b.WriteString(fmt.Sprintf("  %s: %d total, %d unresolved\n", file, fc.Total, fc.Unresolved))
		}
	}
	b.WriteString("\n")

	// Votes
	if len(summary.Votes) > 0 {
		b.WriteString("Votes:\n")
		for label, info := range summary.Votes {
			b.WriteString(fmt.Sprintf("  %s:\n", label))
			if voteMap, ok := info.(map[string]interface{}); ok {
				if votes, ok := voteMap["votes"].([]map[string]interface{}); ok {
					for _, vote := range votes {
						b.WriteString(fmt.Sprintf("    %+d by %s (%s)\n", vote["value"], vote["user"], vote["date"]))
					}
				}
			}
		}
	}

	return b.String()
}
