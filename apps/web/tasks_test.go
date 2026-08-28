package main

import (
	"testing"
	"time"
)

func TestTaskManagerReturnsTaskSnapshots(t *testing.T) {
	tm := NewTaskManager()
	endedAt := time.Now()
	original := &Task{
		ID:        "task-1",
		Type:      "pull_kline",
		Status:    TaskStatusRunning,
		StartedAt: endedAt.Add(-time.Minute),
		EndedAt:   &endedAt,
	}
	tm.tasks[original.ID] = original

	got, ok := tm.Get(original.ID)
	if !ok {
		t.Fatal("Get() did not find task")
	}
	if got == original {
		t.Fatal("Get() returned internal task pointer")
	}
	if got.EndedAt == original.EndedAt {
		t.Fatal("Get() returned internal EndedAt pointer")
	}

	got.Status = TaskStatusFailed
	*got.EndedAt = got.EndedAt.Add(time.Hour)
	if original.Status != TaskStatusRunning {
		t.Fatalf("mutating snapshot changed original status to %q", original.Status)
	}
	if !original.EndedAt.Equal(endedAt) {
		t.Fatalf("mutating snapshot changed original EndedAt to %s", original.EndedAt)
	}
}

func TestTaskManagerListReturnsSnapshotsSortedByStartTime(t *testing.T) {
	tm := NewTaskManager()
	older := time.Now().Add(-time.Hour)
	newer := time.Now()
	tm.tasks["older"] = &Task{ID: "older", Type: "a", Status: TaskStatusRunning, StartedAt: older}
	tm.tasks["newer"] = &Task{ID: "newer", Type: "b", Status: TaskStatusRunning, StartedAt: newer}

	list := tm.List()
	if len(list) != 2 {
		t.Fatalf("List() length = %d, want 2", len(list))
	}
	if list[0].ID != "newer" || list[1].ID != "older" {
		t.Fatalf("List() order = [%s, %s], want [newer, older]", list[0].ID, list[1].ID)
	}

	list[0].Status = TaskStatusFailed
	if tm.tasks["newer"].Status != TaskStatusRunning {
		t.Fatalf("mutating List() result changed original status to %q", tm.tasks["newer"].Status)
	}
}
