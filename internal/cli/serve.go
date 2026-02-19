package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/gerrit-ai-review/gerrit-tools/internal/config"
	"github.com/gerrit-ai-review/gerrit-tools/internal/events"
	"github.com/gerrit-ai-review/gerrit-tools/internal/logger"
	"github.com/gerrit-ai-review/gerrit-tools/internal/queue"
	"github.com/gerrit-ai-review/gerrit-tools/internal/reviewer"
	"github.com/gerrit-ai-review/gerrit-tools/internal/worker"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run as a long-running service that listens to Gerrit events",
	Long: `Listen to Gerrit stream-events via SSH and automatically review new patchsets.

This replaces the bash scripts (listener.sh + reviewer-worker.sh) with a single
Go binary that:
  - Listens to Gerrit SSH stream-events
  - Filters patchset-created events
  - Dispatches review tasks to a worker pool
  - Processes reviews automatically

Example:
  gerrit-reviewer serve

The number of workers and queue size can be configured in config.yaml:
  serve:
    workers: 1
    queue_size: 100
`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	log := logger.Get()

	// Print banner
	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Println("║       Gerrit AI Reviewer - Serve Mode               ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Println("")
	fmt.Printf("SSH Alias:    %s\n", cfg.Gerrit.SSHAlias)
	fmt.Printf("Workers:      %d\n", cfg.Serve.Workers)
	fmt.Printf("Queue size:   %d\n", cfg.Serve.QueueSize)
	fmt.Printf("Lazy mode:    %t\n", cfg.Serve.LazyMode)
	if len(cfg.Serve.Filter.Projects) > 0 {
		fmt.Printf("Watch:        %v\n", cfg.Serve.Filter.Projects)
	} else {
		fmt.Printf("Watch:        ALL\n")
	}
	if len(cfg.Serve.Filter.Exclude) > 0 {
		fmt.Printf("Exclude:      %v\n", cfg.Serve.Filter.Exclude)
	}
	fmt.Println("")

	// Run preflight checks
	log.Info("Running preflight checks...")
	if err := runPreflightChecks(log, cfg); err != nil {
		return fmt.Errorf("preflight checks failed: %w", err)
	}
	log.Info("✓ All preflight checks passed")
	fmt.Println("")

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Infof("Received signal %v, shutting down...", sig)
		cancel()
	}()

	// Create components
	listener := events.NewListener(cfg.Gerrit.SSHAlias)
	filter := events.NewFilter(events.FilterConfig{
		Projects: cfg.Serve.Filter.Projects,
		Exclude:  cfg.Serve.Filter.Exclude,
	})
	q := queue.NewQueue(cfg.Serve.QueueSize, queue.QueueConfig{LazyMode: cfg.Serve.LazyMode})
	rev := reviewer.NewReviewer(cfg)
	pool := worker.NewPool(cfg.Serve.Workers, q, rev)

	// Start worker pool
	go pool.Start(ctx)

	// Start listening to events
	eventCh, err := listener.StreamEvents(ctx)
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}

	log.Info("🎧 Listening for patchset-created events...")
	log.Info("Ready to process reviews")
	fmt.Println("")

	// Main event loop
	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				log.Warn("Event channel closed")
				return nil
			}

			if !filter.ShouldProcess(event) {
				log.Debugf("Filtered out: %s", event.Type)
				continue
			}

			// Validate event has required fields
			if event.Change == nil || event.PatchSet == nil {
				log.Warnf("Event missing required fields, skipping")
				continue
			}

			// Convert event to task
			task := queue.Task{
				ID:             fmt.Sprintf("%s-%d-%d", event.Change.Project, event.Change.Number, event.PatchSet.Number),
				Project:        event.Change.Project,
				ChangeNumber:   event.Change.Number,
				PatchsetNumber: event.PatchSet.Number,
				Subject:        event.Change.Subject,
				CreatedAt:      time.Now(),
			}

			if err := q.Push(task); err != nil {
				if errors.Is(err, queue.ErrQueueFull) {
					log.Warnf("Queue full, dropping task: %s", task.ID)
				} else if errors.Is(err, queue.ErrObsoleteTask) {
					log.Debugf("Task superseded by newer patchset, dropping: %s", task.ID)
				} else {
					// Already queued (duplicate)
					log.Debugf("Task already queued: %s", task.ID)
				}
				continue
			}

			// Truncate subject for display
			subject := task.Subject
			if len(subject) > 60 {
				subject = subject[:60] + "..."
			}

			log.Infof("📥 Queued: %s #%d/%d - %s",
				event.Change.Project,
				event.Change.Number,
				event.PatchSet.Number,
				subject)

		case <-ctx.Done():
			log.Info("Context cancelled, shutting down...")

			// Give workers time to finish current tasks
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer shutdownCancel()

			if err := pool.Stop(shutdownCtx); err != nil {
				log.Errorf("Error stopping workers: %v", err)
			}

			return nil
		}
	}
}

// runPreflightChecks runs startup checks before starting serve mode
func runPreflightChecks(log *logger.Logger, cfg *config.Config) error {
	cliCmd := "gerrit-cli"

	// 1. Check if gerrit-cli exists in PATH
	log.Info("  Checking gerrit-cli...")
	cliPath, err := exec.LookPath(cliCmd)
	if err != nil {
		return fmt.Errorf("gerrit-cli not found in PATH (run: make build-gerrit-cli)")
	}
	log.Infof("  ✓ gerrit-cli found: %s", cliPath)

	// 2. Test gerrit-cli connectivity
	log.Info("  Testing gerrit-cli connectivity...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, cliCmd, "change", "list", "status:open", "--limit", "1")
	cmd.Env = append(os.Environ(), cfg.GerritEnvVars()...)
	output, err := cmd.Output()
	if err != nil {
		log.Warnf("  ✗ gerrit-cli test failed: %v", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.Warnf("  Stderr captured (%d bytes)", len(exitErr.Stderr))
		}
		return fmt.Errorf("gerrit-cli connectivity test failed: %w", err)
	}

	// Parse JSON response
	var result struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return fmt.Errorf("failed to parse gerrit-cli output: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("gerrit-cli returned success=false")
	}

	log.Info("  ✓ gerrit-cli connectivity test passed")

	// 3. Test SSH connection to Gerrit
	log.Info("  Testing SSH connection to Gerrit...")
	sshCmd := exec.CommandContext(ctx, "ssh", cfg.Gerrit.SSHAlias, "gerrit", "version")
	if output, err := sshCmd.CombinedOutput(); err != nil {
		log.Warnf("  ✗ SSH test failed: %v", err)
		log.Warnf("  Output: %s", string(output))
		return fmt.Errorf("SSH connection test failed: %w\nEnsure SSH alias '%s' is configured in ~/.ssh/config", err, cfg.Gerrit.SSHAlias)
	}
	log.Info("  ✓ SSH connection test passed")

	// 4. Check claude CLI
	log.Info("  Checking claude CLI...")
	claudeCmd := exec.CommandContext(ctx, "claude", "--version")
	if output, err := claudeCmd.CombinedOutput(); err != nil {
		log.Warnf("  ✗ claude not found: %v", err)
		return fmt.Errorf("claude CLI not found in PATH: %w", err)
	} else {
		log.Infof("  ✓ claude CLI found: %s", string(output))
	}

	return nil
}
