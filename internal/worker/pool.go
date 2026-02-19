package worker

import (
	"context"
	"sync"
	"time"

	"github.com/gerrit-ai-review/gerrit-tools/internal/logger"
	"github.com/gerrit-ai-review/gerrit-tools/internal/queue"
	"github.com/gerrit-ai-review/gerrit-tools/internal/reviewer"
)

// Pool manages a pool of workers that process review tasks
type Pool struct {
	workers  int
	queue    *queue.Queue
	reviewer *reviewer.Reviewer
	wg       sync.WaitGroup
	log      *logger.Logger
}

// NewPool creates a new worker pool
func NewPool(workers int, q *queue.Queue, rev *reviewer.Reviewer) *Pool {
	return &Pool{
		workers:  workers,
		queue:    q,
		reviewer: rev,
		log:      logger.Get(),
	}
}

// Start starts the worker pool
func (p *Pool) Start(ctx context.Context) {
	p.log.Infof("Starting %d worker(s)", p.workers)

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i+1)
	}
}

// worker is the main worker goroutine that processes tasks
func (p *Pool) worker(ctx context.Context, id int) {
	defer p.wg.Done()

	p.log.Infof("Worker %d started", id)

	for {
		task, err := p.queue.Pop(ctx)
		if err != nil {
			// Context cancelled
			p.log.Infof("Worker %d stopping", id)
			return
		}

		p.log.Infof("Worker %d processing: %s #%d/%d",
			id, task.Project, task.ChangeNumber, task.PatchsetNumber)

		start := time.Now()

		req := reviewer.ReviewRequest{
			Project:        task.Project,
			ChangeNumber:   task.ChangeNumber,
			PatchsetNumber: task.PatchsetNumber,
		}

		if err := p.reviewer.ReviewChange(ctx, req); err != nil {
			p.log.Errorf("Worker %d failed: %v", id, err)
		} else {
			duration := time.Since(start)
			p.log.Infof("Worker %d completed: %s #%d/%d (%.1fs)",
				id, task.Project, task.ChangeNumber, task.PatchsetNumber,
				duration.Seconds())
		}

		p.queue.MarkDone(task.ID)
	}
}

// Stop stops the worker pool gracefully
func (p *Pool) Stop(ctx context.Context) error {
	p.log.Info("Stopping worker pool...")

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.log.Info("All workers stopped")
		return nil
	case <-ctx.Done():
		p.log.Warn("Timeout waiting for workers")
		return ctx.Err()
	}
}
