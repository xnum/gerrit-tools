package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadFromEnv(t *testing.T) {
	// Reset viper state
	viper.Reset()

	originalReviewCLI, hadReviewCLI := os.LookupEnv("REVIEW_CLI")
	os.Unsetenv("REVIEW_CLI")

	// Set up test environment variables
	os.Setenv("GERRIT_SSH_ALIAS", "test-gerrit")
	os.Setenv("GERRIT_HTTP_URL", "https://gerrit.test.com")
	os.Setenv("GERRIT_HTTP_USER", "test-user")
	os.Setenv("GERRIT_HTTP_PASSWORD", "test-pass")
	os.Setenv("GIT_REPO_BASE_PATH", "/tmp/test-repos")
	os.Setenv("CLAUDE_SKIP_PERMISSIONS", "true")
	defer func() {
		os.Unsetenv("GERRIT_SSH_ALIAS")
		os.Unsetenv("GERRIT_HTTP_URL")
		os.Unsetenv("GERRIT_HTTP_USER")
		os.Unsetenv("GERRIT_HTTP_PASSWORD")
		os.Unsetenv("GIT_REPO_BASE_PATH")
		os.Unsetenv("CLAUDE_SKIP_PERMISSIONS")
		if hadReviewCLI {
			os.Setenv("REVIEW_CLI", originalReviewCLI)
		} else {
			os.Unsetenv("REVIEW_CLI")
		}
	}()

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() failed: %v", err)
	}

	// Verify Gerrit config
	if cfg.Gerrit.SSHAlias != "test-gerrit" {
		t.Errorf("Expected SSHAlias 'test-gerrit', got '%s'", cfg.Gerrit.SSHAlias)
	}
	if cfg.Gerrit.HTTPUrl != "https://gerrit.test.com" {
		t.Errorf("Expected HTTPUrl 'https://gerrit.test.com', got '%s'", cfg.Gerrit.HTTPUrl)
	}

	// Verify Git config
	if cfg.Git.RepoBasePath != "/tmp/test-repos" {
		t.Errorf("Expected RepoBasePath '/tmp/test-repos', got '%s'", cfg.Git.RepoBasePath)
	}

	// Verify defaults
	if cfg.Review.CLI != "claude" {
		t.Errorf("Expected CLI 'claude', got '%s'", cfg.Review.CLI)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Expected Logging.Level 'info', got '%s'", cfg.Logging.Level)
	}
	if cfg.Logging.Verbose {
		t.Errorf("Expected Logging.Verbose false by default")
	}
	if cfg.Review.ClaudeTimeout != 600 {
		t.Errorf("Expected ClaudeTimeout 600, got %d", cfg.Review.ClaudeTimeout)
	}
	if !cfg.Review.ClaudeSkipPermissionsCheck {
		t.Errorf("Expected ClaudeSkipPermissionsCheck true from env var")
	}
}

func TestGetGitURL(t *testing.T) {
	cfg := &Config{
		Gerrit: GerritConfig{
			SSHAlias: "gerrit-review",
		},
	}

	url := cfg.GetGitURL("my-project")
	expected := "gerrit-review:my-project"
	if url != expected {
		t.Errorf("Expected '%s', got '%s'", expected, url)
	}
}

func TestGetRepoPath(t *testing.T) {
	cfg := &Config{
		Git: GitConfig{
			RepoBasePath: "/tmp/repos",
		},
	}

	tests := []struct {
		project  string
		expected string
	}{
		{"simple-project", "/tmp/repos/simple-project"},
		{"group/nested-project", "/tmp/repos/nested-project"},
	}

	for _, tt := range tests {
		t.Run(tt.project, func(t *testing.T) {
			path := cfg.GetRepoPath(tt.project)
			if path != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, path)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "missing SSH alias",
			cfg: &Config{
				Gerrit: GerritConfig{
					HTTPUrl:  "https://gerrit.test.com",
					HTTPUser: "user",
					HTTPPass: "pass",
				},
			},
			wantErr: true,
		},
		{
			name: "missing HTTP URL",
			cfg: &Config{
				Gerrit: GerritConfig{
					SSHAlias: "gerrit",
					HTTPUser: "user",
					HTTPPass: "pass",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGerritEnvVars(t *testing.T) {
	cfg := &Config{
		Gerrit: GerritConfig{
			SSHAlias: "my-gerrit",
			HTTPUrl:  "https://gerrit.example.com",
			HTTPUser: "user1",
			HTTPPass: "pass1",
		},
		Git: GitConfig{
			RepoBasePath: "/tmp/repos",
		},
	}

	envVars := cfg.GerritEnvVars()

	expected := map[string]bool{
		"GERRIT_SSH_ALIAS=my-gerrit":                 false,
		"GERRIT_HTTP_URL=https://gerrit.example.com": false,
		"GERRIT_HTTP_USER=user1":                     false,
		"GERRIT_HTTP_PASSWORD=pass1":                 false,
		"GIT_REPO_BASE_PATH=/tmp/repos":              false,
	}

	for _, env := range envVars {
		if _, ok := expected[env]; ok {
			expected[env] = true
		}
	}

	for k, found := range expected {
		if !found {
			t.Errorf("Expected env var '%s' not found in output", k)
		}
	}
}

func TestClaudeSkipPermissionsDefaultFalse(t *testing.T) {
	viper.Reset()

	originalReviewCLI, hadReviewCLI := os.LookupEnv("REVIEW_CLI")
	os.Unsetenv("REVIEW_CLI")

	os.Setenv("GERRIT_SSH_ALIAS", "test-gerrit")
	os.Setenv("GERRIT_HTTP_URL", "https://gerrit.test.com")
	os.Setenv("GERRIT_HTTP_USER", "test-user")
	os.Setenv("GERRIT_HTTP_PASSWORD", "test-pass")
	os.Setenv("GIT_REPO_BASE_PATH", "/tmp/test-repos")
	defer func() {
		os.Unsetenv("GERRIT_SSH_ALIAS")
		os.Unsetenv("GERRIT_HTTP_URL")
		os.Unsetenv("GERRIT_HTTP_USER")
		os.Unsetenv("GERRIT_HTTP_PASSWORD")
		os.Unsetenv("GIT_REPO_BASE_PATH")
		if hadReviewCLI {
			os.Setenv("REVIEW_CLI", originalReviewCLI)
		} else {
			os.Unsetenv("REVIEW_CLI")
		}
	}()

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() failed: %v", err)
	}

	if cfg.Review.ClaudeSkipPermissionsCheck {
		t.Fatalf("expected ClaudeSkipPermissionsCheck default false")
	}
	if cfg.Review.CLI != "claude" {
		t.Fatalf("expected Review.CLI default claude, got %q", cfg.Review.CLI)
	}
}

func TestReviewCLIFromEnv(t *testing.T) {
	viper.Reset()

	os.Setenv("GERRIT_SSH_ALIAS", "test-gerrit")
	os.Setenv("GERRIT_HTTP_URL", "https://gerrit.test.com")
	os.Setenv("GERRIT_HTTP_USER", "test-user")
	os.Setenv("GERRIT_HTTP_PASSWORD", "test-pass")
	os.Setenv("GIT_REPO_BASE_PATH", "/tmp/test-repos")
	os.Setenv("REVIEW_CLI", "CoDeX")
	defer func() {
		os.Unsetenv("GERRIT_SSH_ALIAS")
		os.Unsetenv("GERRIT_HTTP_URL")
		os.Unsetenv("GERRIT_HTTP_USER")
		os.Unsetenv("GERRIT_HTTP_PASSWORD")
		os.Unsetenv("GIT_REPO_BASE_PATH")
		os.Unsetenv("REVIEW_CLI")
	}()

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() failed: %v", err)
	}

	if cfg.Review.CLI != "codex" {
		t.Fatalf("expected Review.CLI to normalize to codex, got %q", cfg.Review.CLI)
	}
}

func TestInvalidReviewCLI(t *testing.T) {
	cfg := &Config{
		Gerrit: GerritConfig{
			SSHAlias: "gerrit",
			HTTPUrl:  "https://gerrit.test.com",
			HTTPUser: "user",
			HTTPPass: "pass",
		},
		Git: GitConfig{
			RepoBasePath: "/tmp/test-repos",
		},
		Review: ReviewConfig{
			CLI: "invalid",
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected Validate() to fail for invalid review.cli")
	}
}

func TestInvalidLoggingLevel(t *testing.T) {
	cfg := &Config{
		Gerrit: GerritConfig{
			SSHAlias: "gerrit",
			HTTPUrl:  "https://gerrit.test.com",
			HTTPUser: "user",
			HTTPPass: "pass",
		},
		Git: GitConfig{
			RepoBasePath: "/tmp/test-repos",
		},
		Logging: LoggingConfig{
			Level: "nope",
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected Validate() to fail for invalid logging.level")
	}
}

func TestLogVerboseFromLevelAndFlag(t *testing.T) {
	cfg := &Config{
		Logging: LoggingConfig{
			Level: "info",
		},
	}
	if cfg.LogVerbose() {
		t.Fatalf("expected LogVerbose false for info level")
	}

	cfg.Logging.Level = "debug"
	if !cfg.LogVerbose() {
		t.Fatalf("expected LogVerbose true for debug level")
	}

	cfg.Logging.Level = "info"
	cfg.Logging.Verbose = true
	if !cfg.LogVerbose() {
		t.Fatalf("expected LogVerbose true when verbose flag set")
	}
}
