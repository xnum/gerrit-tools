package queue

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Task represents a review task
type Task struct {
	ID             string
	Project        string
	ChangeNumber   int
	PatchsetNumber int
	Subject        string
	CreatedAt      time.Time
}

// Queue is an in-memory task queue
type Queue struct {
	tasks    chan Task
	inflight map[string]bool
	mu       sync.RWMutex
}

// NewQueue creates a new task queue with the given capacity
func NewQueue(size int) *Queue {
	return &Queue{
		tasks:    make(chan Task, size),
		inflight: make(map[string]bool),
	}
}

// Push adds a task to the queue
// Returns error if task already exists or queue is full
func (q *Queue) Push(task Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Duplicate detection
	if q.inflight[task.ID] {
		return fmt.Errorf("task %s already in queue", task.ID)
	}

	select {
	case q.tasks <- task:
		q.inflight[task.ID] = true
		return nil
	default:
		return fmt.Errorf("queue full")
	}
}

// Pop retrieves a task from the queue
// Blocks until a task is available or context is cancelled
func (q *Queue) Pop(ctx context.Context) (Task, error) {
	select {
	case task := <-q.tasks:
		return task, nil
	case <-ctx.Done():
		return Task{}, ctx.Err()
	}
}

// MarkDone marks a task as completed and removes it from inflight tracking
func (q *Queue) MarkDone(taskID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.inflight, taskID)
}

// Size returns the current number of tasks in the queue
func (q *Queue) Size() int {
	return len(q.tasks)
}

// InFlight returns the number of tasks currently being processed or queued
func (q *Queue) InFlight() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.inflight)
}
