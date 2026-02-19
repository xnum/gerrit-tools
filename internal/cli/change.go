package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/gerrit-ai-review/gerrit-tools/internal/gerrit"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// changeCmd represents the change command group
var changeCmd = &cobra.Command{
	Use:   "change",
	Short: "Manage Gerrit changes",
	Long: `Query and retrieve information about Gerrit changes.

Changes represent code reviews in Gerrit. Use this command group to
list changes matching a query or get detailed information about a
specific change.`,
}

// changeListCmd lists changes matching a query
var changeListCmd = &cobra.Command{
	Use:   "list <query>",
	Short: "List changes matching a query",
	Long: `List changes matching a Gerrit search query.

Examples:
  # List open changes in a project
  gerrit-cli change list "status:open project:myproject"

  # List changes by owner
  gerrit-cli change list "owner:john@example.com"

  # List changes with specific topic
  gerrit-cli change list "topic:feature-x"

  # Combine multiple criteria
  gerrit-cli change list "status:open project:myproject branch:main"

Query Operators:
  status:open/merged/abandoned
  project:<project-name>
  branch:<branch-name>
  owner:<email>
  reviewer:<email>
  topic:<topic>
  label:Code-Review=+2
  is:mergeable

For full query syntax, see Gerrit documentation.`,
	Args: cobra.ExactArgs(1),
	RunE: runChangeList,
}

// changeGetCmd gets detailed information about a change
var changeGetCmd = &cobra.Command{
	Use:   "get <change-id>",
	Short: "Get detailed information about a change",
	Long: `Get detailed information about a specific change.

The change-id can be:
  - Numeric ID (e.g., 12345)
  - Change-Id (e.g., I1234567890abcdef1234567890abcdef12345678)
  - Project~Branch~Change-Id triplet

Examples:
  # Get change by numeric ID
  gerrit-cli change get 12345

  # Get change by Change-Id
  gerrit-cli change get I1234567890abcdef1234567890abcdef12345678

  # Get change with specific options
  gerrit-cli change get 12345 --options CURRENT_REVISION --options MESSAGES`,
	Args: cobra.ExactArgs(1),
	RunE: runChangeGet,
}

func init() {
	// Add flags for changeListCmd
	changeListCmd.Flags().IntP("limit", "n", 25, "Maximum number of results")
	changeListCmd.Flags().StringSliceP("options", "o", []string{"LABELS"}, "Additional options (e.g., CURRENT_REVISION, DETAILED_ACCOUNTS)")

	// Add flags for changeGetCmd
	changeGetCmd.Flags().StringSliceP("options", "o", []string{"CURRENT_REVISION", "LABELS", "DETAILED_ACCOUNTS"}, "Additional options")

	// Add subcommands to changeCmd
	changeCmd.AddCommand(changeListCmd)
	changeCmd.AddCommand(changeGetCmd)
}

// runChangeList executes the change list command
func runChangeList(cmd *cobra.Command, args []string) error {
	query := args[0]
	limit, _ := cmd.Flags().GetInt("limit")
	options, _ := cmd.Flags().GetStringSlice("options")
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
	return ExecuteCommand(format, "change list", version, func() (interface{}, error) {
		ctx := context.Background()
		changes, err := client.ListChanges(ctx, query, options, limit)
		if err != nil {
			return nil, err
		}

		return changes, nil
	})
}

// runChangeGet executes the change get command
func runChangeGet(cmd *cobra.Command, args []string) error {
	changeID := args[0]
	options, _ := cmd.Flags().GetStringSlice("options")
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
	return ExecuteCommand(format, "change get", version, func() (interface{}, error) {
		ctx := context.Background()
		change, err := client.GetChangeDetail(ctx, changeID, options)
		if err != nil {
			return nil, err
		}

		return change, nil
	})
}
