package queue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrDuplicateTask = errors.New("task already in queue")
	ErrQueueFull     = errors.New("queue full")
	ErrObsoleteTask  = errors.New("obsolete task")
)

// Task represents a review task
//
// ChangeNumber + PatchsetNumber identifies the revision being reviewed.
type Task struct {
	ID             string
	Project        string
	ChangeNumber   int
	PatchsetNumber int
	Subject        string
	CreatedAt      time.Time
}

// QueueConfig configures queue behavior.
type QueueConfig struct {
	LazyMode bool // Keep only latest patchset per change
}

// Queue is an in-memory task queue
type Queue struct {
	tasks          chan Task
	inflight       map[string]bool
	latestByChange map[string]int
	lazyMode       bool
	mu             sync.RWMutex
}

// NewQueue creates a new task queue with the given capacity.
func NewQueue(size int, cfg QueueConfig) *Queue {
	return &Queue{
		tasks:          make(chan Task, size),
		inflight:       make(map[string]bool),
		latestByChange: make(map[string]int),
		lazyMode:       cfg.LazyMode,
	}
}

func changeKey(project string, changeNumber int) string {
	return fmt.Sprintf("%s-%d", project, changeNumber)
}

// Push adds a task to the queue.
// Returns typed errors for duplicate, full, or obsolete tasks.
func (q *Queue) Push(task Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Duplicate detection by unique task ID.
	if q.inflight[task.ID] {
		return fmt.Errorf("%w: %s", ErrDuplicateTask, task.ID)
	}

	if q.lazyMode {
		key := changeKey(task.Project, task.ChangeNumber)
		if latestPatchset, ok := q.latestByChange[key]; ok && task.PatchsetNumber <= latestPatchset {
			return fmt.Errorf("%w: %s (incoming=%d, latest=%d)", ErrObsoleteTask, key, task.PatchsetNumber, latestPatchset)
		}
		q.latestByChange[key] = task.PatchsetNumber
	}

	select {
	case q.tasks <- task:
		q.inflight[task.ID] = true
		return nil
	default:
		return ErrQueueFull
	}
}

// Pop retrieves a task from the queue.
// Blocks until a non-obsolete task is available or context is cancelled.
func (q *Queue) Pop(ctx context.Context) (Task, error) {
	for {
		select {
		case task := <-q.tasks:
			if q.lazyMode {
				q.mu.Lock()
				key := changeKey(task.Project, task.ChangeNumber)
				latestPatchset := q.latestByChange[key]
				if task.PatchsetNumber < latestPatchset {
					// This task has been superseded by a newer patchset for same change.
					delete(q.inflight, task.ID)
					q.mu.Unlock()
					continue
				}
				q.mu.Unlock()
			}

			return task, nil
		case <-ctx.Done():
			return Task{}, ctx.Err()
		}
	}
}

// MarkDone marks a task as completed and removes it from inflight tracking.
func (q *Queue) MarkDone(taskID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.inflight, taskID)
}

// Size returns the current number of tasks in the queue.
func (q *Queue) Size() int {
	return len(q.tasks)
}

// InFlight returns the number of tasks currently being processed or queued.
func (q *Queue) InFlight() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.inflight)
}
