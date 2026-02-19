package events

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/gerrit-ai-review/gerrit-tools/internal/logger"
)

// Listener listens to Gerrit stream-events via SSH
type Listener struct {
	sshAlias string
	log      *logger.Logger
}

// NewListener creates a new event listener
func NewListener(sshAlias string) *Listener {
	return &Listener{
		sshAlias: sshAlias,
		log:      logger.Get(),
	}
}

// StreamEvents opens SSH connection and returns channel of events
// It automatically reconnects on connection failures
func (l *Listener) StreamEvents(ctx context.Context) (<-chan Event, error) {
	eventCh := make(chan Event, 100)

	go func() {
		defer close(eventCh)

		retries := 0
		maxRetries := 100

		for retries < maxRetries {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if err := l.streamOnce(ctx, eventCh); err != nil {
				retries++
				waitTime := l.getBackoff(retries)
				l.log.Warnf("Connection lost (attempt %d/%d), reconnecting in %v",
					retries, maxRetries, waitTime)

				select {
				case <-time.After(waitTime):
				case <-ctx.Done():
					return
				}
			} else {
				// Reset retry count on successful connection
				retries = 0
			}
		}

		l.log.Errorf("Max retries (%d) reached", maxRetries)
	}()

	return eventCh, nil
}

// streamOnce establishes one SSH connection and streams events
func (l *Listener) streamOnce(ctx context.Context, eventCh chan<- Event) error {
	// Build SSH command
	// ssh gerrit-review -o ServerAliveInterval=30 -o ServerAliveCountMax=3 gerrit stream-events -s patchset-created
	cmd := exec.CommandContext(ctx,
		"ssh", l.sshAlias,
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
		"gerrit", "stream-events",
		"-s", "patchset-created",
	)

	l.log.Infof("Connecting to %s...", l.sshAlias)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start SSH: %w", err)
	}

	l.log.Infof("🎧 Connected, listening for events...")

	// Read events line by line
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			l.log.Warnf("Failed to parse event: %v", err)
			l.log.Debugf("Raw event: %s", line)
			continue
		}

		select {
		case eventCh <- event:
		case <-ctx.Done():
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
			return ctx.Err()
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return cmd.Wait()
}

// getBackoff returns the wait time before next retry
func (l *Listener) getBackoff(retries int) time.Duration {
	if retries < 5 {
		return 5 * time.Second
	}
	return 30 * time.Second
}
