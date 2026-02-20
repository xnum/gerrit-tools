package cli

import (
	"fmt"

	"github.com/gerrit-ai-review/gerrit-tools/internal/config"
	"github.com/gerrit-ai-review/gerrit-tools/internal/logger"
)

// ConfigureGlobalLogger initializes the process-wide logger from configuration.
func ConfigureGlobalLogger(cfg *config.Config) error {
	l, err := logger.NewLogger(cfg.LogVerbose(), cfg.Logging.File)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	logger.SetGlobal(l)
	return nil
}
