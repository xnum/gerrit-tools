package queue

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPushDuplicateTask(t *testing.T) {
	q := NewQueue(10, QueueConfig{})
	task := Task{ID: "proj-100-1", Project: "proj", ChangeNumber: 100, PatchsetNumber: 1}

	if err := q.Push(task); err != nil {
		t.Fatalf("first push failed: %v", err)
	}

	err := q.Push(task)
	if !errors.Is(err, ErrDuplicateTask) {
		t.Fatalf("expected ErrDuplicateTask, got: %v", err)
	}
}

func TestLazyModeRejectsOlderOrEqualPatchset(t *testing.T) {
	q := NewQueue(10, QueueConfig{LazyMode: true})

	if err := q.Push(Task{ID: "proj-100-2", Project: "proj", ChangeNumber: 100, PatchsetNumber: 2}); err != nil {
		t.Fatalf("push patchset 2 failed: %v", err)
	}

	err := q.Push(Task{ID: "proj-100-1", Project: "proj", ChangeNumber: 100, PatchsetNumber: 1})
	if !errors.Is(err, ErrObsoleteTask) {
		t.Fatalf("expected ErrObsoleteTask for older patchset, got: %v", err)
	}

	err = q.Push(Task{ID: "proj-100-2b", Project: "proj", ChangeNumber: 100, PatchsetNumber: 2})
	if !errors.Is(err, ErrObsoleteTask) {
		t.Fatalf("expected ErrObsoleteTask for equal patchset, got: %v", err)
	}
}

func TestLazyModePopSkipsSupersededQueuedTask(t *testing.T) {
	q := NewQueue(10, QueueConfig{LazyMode: true})

	if err := q.Push(Task{ID: "proj-100-1", Project: "proj", ChangeNumber: 100, PatchsetNumber: 1}); err != nil {
		t.Fatalf("push patchset 1 failed: %v", err)
	}
	if err := q.Push(Task{ID: "proj-100-2", Project: "proj", ChangeNumber: 100, PatchsetNumber: 2}); err != nil {
		t.Fatalf("push patchset 2 failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	task, err := q.Pop(ctx)
	if err != nil {
		t.Fatalf("pop failed: %v", err)
	}
	if task.PatchsetNumber != 2 {
		t.Fatalf("expected latest patchset (2), got: %d", task.PatchsetNumber)
	}
}

func TestNonLazyModePopsInOrder(t *testing.T) {
	q := NewQueue(10, QueueConfig{LazyMode: false})

	if err := q.Push(Task{ID: "proj-100-1", Project: "proj", ChangeNumber: 100, PatchsetNumber: 1}); err != nil {
		t.Fatalf("push patchset 1 failed: %v", err)
	}
	if err := q.Push(Task{ID: "proj-100-2", Project: "proj", ChangeNumber: 100, PatchsetNumber: 2}); err != nil {
		t.Fatalf("push patchset 2 failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	first, err := q.Pop(ctx)
	if err != nil {
		t.Fatalf("first pop failed: %v", err)
	}
	if first.PatchsetNumber != 1 {
		t.Fatalf("expected first patchset 1, got: %d", first.PatchsetNumber)
	}

	second, err := q.Pop(ctx)
	if err != nil {
		t.Fatalf("second pop failed: %v", err)
	}
	if second.PatchsetNumber != 2 {
		t.Fatalf("expected second patchset 2, got: %d", second.PatchsetNumber)
	}
}
