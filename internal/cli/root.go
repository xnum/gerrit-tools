package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	version string
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "gerrit-tool",
	Short: "Gerrit code review CLI tool",
	Long: `A comprehensive CLI tool for interacting with Gerrit code review system.

This tool provides commands to query changes, compare patchsets, fetch code,
and post reviews. It's designed to enable AI-powered code reviews by providing
structured, composable operations.`,
	Version: version,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute(ver string) error {
	version = ver
	rootCmd.Version = ver
	return rootCmd.Execute()
}

// ExecuteGerritCLI executes the gerrit-cli tool
func ExecuteGerritCLI(ver string) error {
	version = ver
	grCmd := createGrRootCmd()
	grCmd.Version = ver
	return grCmd.Execute()
}

// ExecuteReviewer executes the gerrit-reviewer CLI tool
func ExecuteReviewer(ver string) error {
	version = ver
	reviewerCmd := createReviewerRootCmd()
	reviewerCmd.Version = ver
	return reviewerCmd.Execute()
}

// createReviewerRootCmd creates the root command for gerrit-reviewer CLI
func createReviewerRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gerrit-reviewer",
		Short: "AI-powered code review for Gerrit",
		Long: `gerrit-reviewer is an AI-powered code review tool for Gerrit.

It can run in two modes:
  - One-shot mode: Review a specific patchset (use flags directly)
  - Serve mode: Listen to Gerrit events and review automatically (use 'serve' subcommand)`,
		Version: version,
	}

	// Global flags
	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
	cmd.PersistentFlags().Bool("dangerously-skip-permissions", false, "Bypass permission/sandbox checks in the selected review CLI (unsafe)")
	viper.BindPFlag("review.claude_skip_permissions", cmd.PersistentFlags().Lookup("dangerously-skip-permissions"))

	// Initialize config on command initialization
	cobra.OnInitialize(initConfig)

	// Add subcommands (only serve for now)
	cmd.AddCommand(serveCmd)

	return cmd
}

// createGrRootCmd creates the root command for gerrit-cli
func createGrRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gerrit-cli",
		Short: "Gerrit CLI tool for LLM integration",
		Long: `gerrit-cli is a minimal CLI tool for interacting with Gerrit code review system.

Designed for LLM integration, it provides structured JSON output by default
and focuses on querying changes, fetching diffs, and managing reviews.`,
		Version: version,
	}

	// Global flags
	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/gerrit-cli/config.yaml)")
	cmd.PersistentFlags().String("ssh-alias", "", "SSH config alias for Gerrit")
	cmd.PersistentFlags().String("host", "", "Gerrit host")
	cmd.PersistentFlags().Int("port", 29418, "Gerrit SSH port")
	cmd.PersistentFlags().String("user", "", "Gerrit SSH user")
	cmd.PersistentFlags().String("http-url", "", "Gerrit HTTP URL for REST API")
	cmd.PersistentFlags().String("http-user", "", "HTTP username for authentication")
	cmd.PersistentFlags().String("format", "json", "Output format: json or text")

	// Bind flags to viper
	viper.BindPFlag("gerrit.ssh_alias", cmd.PersistentFlags().Lookup("ssh-alias"))
	viper.BindPFlag("gerrit.host", cmd.PersistentFlags().Lookup("host"))
	viper.BindPFlag("gerrit.port", cmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("gerrit.user", cmd.PersistentFlags().Lookup("user"))
	viper.BindPFlag("gerrit.http_url", cmd.PersistentFlags().Lookup("http-url"))
	viper.BindPFlag("gerrit.http_user", cmd.PersistentFlags().Lookup("http-user"))
	viper.BindPFlag("output.format", cmd.PersistentFlags().Lookup("format"))

	// Initialize config on command initialization
	cobra.OnInitialize(initConfig)

	// Add subcommands
	cmd.AddCommand(changeCmd)
	cmd.AddCommand(patchsetCmd)
	cmd.AddCommand(commentCmd)
	cmd.AddCommand(draftCmd)
	cmd.AddCommand(reviewCmd)
	cmd.AddCommand(summaryCmd)
	cmd.AddCommand(repoCmd)

	return cmd
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/gerrit-cli/config.yaml)")
	rootCmd.PersistentFlags().String("ssh-alias", "", "SSH config alias for Gerrit")
	rootCmd.PersistentFlags().String("host", "", "Gerrit host")
	rootCmd.PersistentFlags().Int("port", 29418, "Gerrit SSH port")
	rootCmd.PersistentFlags().String("user", "", "Gerrit SSH user")
	rootCmd.PersistentFlags().String("http-url", "", "Gerrit HTTP URL for REST API")
	rootCmd.PersistentFlags().String("format", "text", "Output format: json, text, or compact")

	// Bind flags to viper
	viper.BindPFlag("gerrit.ssh_alias", rootCmd.PersistentFlags().Lookup("ssh-alias"))
	viper.BindPFlag("gerrit.host", rootCmd.PersistentFlags().Lookup("host"))
	viper.BindPFlag("gerrit.port", rootCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("gerrit.user", rootCmd.PersistentFlags().Lookup("user"))
	viper.BindPFlag("gerrit.http_url", rootCmd.PersistentFlags().Lookup("http-url"))
	viper.BindPFlag("output.format", rootCmd.PersistentFlags().Lookup("format"))

	// Add subcommands (placeholder - will be implemented in phases)
	// rootCmd.AddCommand(getChangeCmd)
	// rootCmd.AddCommand(listPatchsetsCmd)
	// rootCmd.AddCommand(diffPatchsetsCmd)
	// rootCmd.AddCommand(getCommentsCmd)
	// rootCmd.AddCommand(fetchCodeCmd)
	// rootCmd.AddCommand(postReviewCmd)
}

// initConfig reads in config file and ENV variables if set
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for config in standard locations
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error finding home directory:", err)
			return
		}

		// Search config in:
		// 1. Current directory (highest priority for local config)
		// 2. $HOME/.config/gerrit-cli/
		// 3. $HOME/
		viper.AddConfigPath(".")
		viper.AddConfigPath(home + "/.config/gerrit-cli")
		viper.AddConfigPath(home)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// Manually bind environment variables to viper keys
	// This ensures GERRIT_HTTP_URL maps to gerrit.http_url, etc.
	bindEnvVariables()

	// If a config file is found, read it
	if err := viper.ReadInConfig(); err == nil {
		// Config file found and successfully parsed
		// We don't print this in normal operation to keep output clean
	}
}

// bindEnvVariables manually binds environment variables to viper keys
// This ensures backward compatibility with existing environment variable names
func bindEnvVariables() {
	// Gerrit configuration
	viper.BindEnv("gerrit.ssh_alias", "GERRIT_SSH_ALIAS")
	viper.BindEnv("gerrit.host", "GERRIT_HOST")
	viper.BindEnv("gerrit.port", "GERRIT_PORT")
	viper.BindEnv("gerrit.user", "GERRIT_USER")
	viper.BindEnv("gerrit.http_url", "GERRIT_HTTP_URL")
	viper.BindEnv("gerrit.http_user", "GERRIT_HTTP_USER")
	viper.BindEnv("gerrit.http_password", "GERRIT_HTTP_PASSWORD")

	// Git configuration
	viper.BindEnv("git.repo_base_path", "GIT_REPO_BASE_PATH")

	// Review configuration
	viper.BindEnv("review.cli", "REVIEW_CLI")
	viper.BindEnv("review.claude_timeout", "CLAUDE_TIMEOUT")
	viper.BindEnv("review.claude_skip_permissions", "CLAUDE_SKIP_PERMISSIONS")

	// Output configuration
	viper.BindEnv("output.format", "OUTPUT_FORMAT")
}
