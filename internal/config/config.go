package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all configuration for the gerrit-reviewer
type Config struct {
	Gerrit GerritConfig
	Git    GitConfig
	Review ReviewConfig
	Serve  ServeConfig
}

// GerritConfig holds Gerrit connection settings
type GerritConfig struct {
	SSHAlias string // SSH alias from ~/.ssh/config
	HTTPUrl  string // Base URL for REST API (e.g., https://gerrit.stranity.dev)
	HTTPUser string // Username for HTTP basic auth
	HTTPPass string // Password for HTTP basic auth
}

// GitConfig holds Git repository settings
type GitConfig struct {
	RepoBasePath string // Base path for cloning repositories (e.g., /tmp/ai-review-repos)
}

// ReviewConfig holds review-specific settings
type ReviewConfig struct {
	ClaudeTimeout              int  // Timeout in seconds for Claude execution (default: 600)
	ClaudeSkipPermissionsCheck bool // Whether to pass --dangerously-skip-permissions to Claude CLI
}

// ServeConfig holds serve mode specific settings
type ServeConfig struct {
	Workers   int          // Number of concurrent workers
	QueueSize int          // Maximum queue size
	LazyMode  bool         // Keep only latest patchset per change in queue
	Filter    FilterConfig // Event filtering rules
}

// FilterConfig holds event filtering rules
type FilterConfig struct {
	Projects []string // Projects to review (empty = all)
	Exclude  []string // Projects to exclude
}

// LoadFromEnv loads configuration from environment variables
// Kept for backward compatibility, but prefers config file via Viper
func LoadFromEnv() (*Config, error) {
	// Initialize Viper to also search for config files
	initViperDefaults()

	// Try reading config file first
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	home, _ := os.UserHomeDir()
	if home != "" {
		viper.AddConfigPath(home + "/.config/gerrit-cli")
		viper.AddConfigPath(home + "/.config/gerrit-tool")
	}
	viper.ReadInConfig() // ignore error — env vars are fallback

	// Bind environment variables
	bindEnvVars()

	return buildConfig()
}

// LoadConfig loads configuration from Viper (config file or env vars)
// This is used by serve mode which uses the Viper-based CLI
func LoadConfig() (*Config, error) {
	bindEnvVars()
	return buildConfig()
}

// bindEnvVars binds environment variable names to viper keys
func bindEnvVars() {
	viper.BindEnv("gerrit.ssh_alias", "GERRIT_SSH_ALIAS")
	viper.BindEnv("gerrit.http_url", "GERRIT_HTTP_URL")
	viper.BindEnv("gerrit.http_user", "GERRIT_HTTP_USER")
	viper.BindEnv("gerrit.http_password", "GERRIT_HTTP_PASSWORD")
	viper.BindEnv("git.repo_base_path", "GIT_REPO_BASE_PATH")
	viper.BindEnv("review.claude_timeout", "CLAUDE_TIMEOUT")
	viper.BindEnv("review.claude_skip_permissions", "CLAUDE_SKIP_PERMISSIONS")
	viper.BindEnv("serve.lazy_mode", "SERVE_LAZY_MODE")
}

// initViperDefaults sets default values
func initViperDefaults() {
	viper.SetDefault("gerrit.ssh_alias", "gerrit-review")
	viper.SetDefault("git.repo_base_path", "/tmp/ai-review-repos")
	viper.SetDefault("review.claude_timeout", 600)
	viper.SetDefault("review.claude_skip_permissions", false)
	viper.SetDefault("serve.workers", 1)
	viper.SetDefault("serve.queue_size", 100)
	viper.SetDefault("serve.lazy_mode", false)
}

// buildConfig constructs a Config from current Viper state
func buildConfig() (*Config, error) {
	initViperDefaults()

	cfg := &Config{
		Gerrit: GerritConfig{
			SSHAlias: viper.GetString("gerrit.ssh_alias"),
			HTTPUrl:  viper.GetString("gerrit.http_url"),
			HTTPUser: viper.GetString("gerrit.http_user"),
			HTTPPass: viper.GetString("gerrit.http_password"),
		},
		Git: GitConfig{
			RepoBasePath: viper.GetString("git.repo_base_path"),
		},
		Review: ReviewConfig{
			ClaudeTimeout:              viper.GetInt("review.claude_timeout"),
			ClaudeSkipPermissionsCheck: viper.GetBool("review.claude_skip_permissions"),
		},
		Serve: ServeConfig{
			Workers:   viper.GetInt("serve.workers"),
			QueueSize: viper.GetInt("serve.queue_size"),
			LazyMode:  viper.GetBool("serve.lazy_mode"),
			Filter: FilterConfig{
				Projects: viper.GetStringSlice("serve.filter.projects"),
				Exclude:  viper.GetStringSlice("serve.filter.exclude"),
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Gerrit.SSHAlias == "" {
		return fmt.Errorf("gerrit.ssh_alias is required")
	}

	if c.Gerrit.HTTPUrl == "" {
		return fmt.Errorf("gerrit.http_url is required")
	}

	if c.Gerrit.HTTPUser == "" {
		return fmt.Errorf("gerrit.http_user is required")
	}

	if c.Gerrit.HTTPPass == "" {
		return fmt.Errorf("gerrit.http_password is required")
	}

	if c.Git.RepoBasePath == "" {
		return fmt.Errorf("git.repo_base_path is required")
	}

	return nil
}

// GetGitURL returns the SSH URL for cloning a project
func (c *Config) GetGitURL(project string) string {
	return fmt.Sprintf("%s:%s", c.Gerrit.SSHAlias, project)
}

// GetRepoPath returns the local path for a project's repository
func (c *Config) GetRepoPath(project string) string {
	safeName := filepath.Base(project)
	return filepath.Join(c.Git.RepoBasePath, safeName)
}

// GerritEnvVars returns the environment variables needed by gerrit-cli
func (c *Config) GerritEnvVars() []string {
	return []string{
		fmt.Sprintf("GERRIT_SSH_ALIAS=%s", c.Gerrit.SSHAlias),
		fmt.Sprintf("GERRIT_HTTP_URL=%s", c.Gerrit.HTTPUrl),
		fmt.Sprintf("GERRIT_HTTP_USER=%s", c.Gerrit.HTTPUser),
		fmt.Sprintf("GERRIT_HTTP_PASSWORD=%s", c.Gerrit.HTTPPass),
		fmt.Sprintf("GIT_REPO_BASE_PATH=%s", c.Git.RepoBasePath),
	}
}
