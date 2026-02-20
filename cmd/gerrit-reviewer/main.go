package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/gerrit-ai-review/gerrit-tools/internal/cli"
	"github.com/gerrit-ai-review/gerrit-tools/internal/config"
	"github.com/gerrit-ai-review/gerrit-tools/internal/reviewer"
)

var (
	Version = "dev"
)

func main() {
	// Check if we're being called with subcommands
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		if err := cli.ExecuteReviewer(Version); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// One-shot mode
	runOneShot()
}

func runOneShot() {
	project := flag.String("project", "", "Project name (required)")
	changeNum := flag.Int("change-number", 0, "Change number (required)")
	patchsetNum := flag.Int("patchset-number", 0, "Patchset number (required)")
	skipPermissions := flag.Bool("dangerously-skip-permissions", false, "Bypass permission/sandbox checks in the selected review CLI (unsafe)")
	reviewCLI := flag.String("review-cli", "", "AI CLI backend: claude or codex")
	version := flag.Bool("version", false, "Show version")

	flag.Parse()

	if *version {
		fmt.Printf("gerrit-reviewer version %s\n", Version)
		os.Exit(0)
	}

	if *project == "" || *changeNum == 0 || *patchsetNum == 0 {
		fmt.Fprintf(os.Stderr, "Error: --project, --change-number, and --patchset-number are required\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  One-shot review:\n")
		fmt.Fprintf(os.Stderr, "    gerrit-reviewer --project <project> --change-number <num> --patchset-number <num>\n\n")
		fmt.Fprintf(os.Stderr, "  Serve mode:\n")
		fmt.Fprintf(os.Stderr, "    gerrit-reviewer serve\n\n")
		flag.Usage()
		os.Exit(1)
	}

	// Load configuration (config.yaml + env vars)
	if flagWasSet("dangerously-skip-permissions") {
		if err := os.Setenv("CLAUDE_SKIP_PERMISSIONS", strconv.FormatBool(*skipPermissions)); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting CLAUDE_SKIP_PERMISSIONS: %v\n", err)
			os.Exit(1)
		}
	}
	if flagWasSet("review-cli") {
		if err := os.Setenv("REVIEW_CLI", *reviewCLI); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting REVIEW_CLI: %v\n", err)
			os.Exit(1)
		}
	}

	cfg, err := config.LoadFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	rev := reviewer.NewReviewer(cfg)

	ctx := context.Background()
	req := reviewer.ReviewRequest{
		Project:        *project,
		ChangeNumber:   *changeNum,
		PatchsetNumber: *patchsetNum,
	}

	if err := rev.ReviewChange(ctx, req); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func flagWasSet(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
