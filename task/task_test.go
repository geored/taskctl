package task

import (
	"testing"
)

func TestAddTask(t *testing.T) {
	mgr := NewManager(t.TempDir() + "/tasks.json")

	task := mgr.Add("Test task", "high")
	if task.Title != "Test task" {
		t.Errorf("expected title 'Test task', got '%s'", task.Title)
	}
	if task.Priority != "high" {
		t.Errorf("expected priority 'high', got '%s'", task.Priority)
	}
	if task.Done {
		t.Error("new task should not be done")
	}
}

func TestListTasks(t *testing.T) {
	mgr := NewManager(t.TempDir() + "/tasks.json")

	mgr.Add("Task 1", "low")
	mgr.Add("Task 2", "high")

	tasks := mgr.List()
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestCompleteTask(t *testing.T) {
	mgr := NewManager(t.TempDir() + "/tasks.json")

	task := mgr.Add("Complete me", "medium")
	err := mgr.Complete(task.ID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	tasks := mgr.List()
	if !tasks[0].Done {
		t.Error("task should be marked as done")
	}
}

func TestCompleteNonExistent(t *testing.T) {
	mgr := NewManager(t.TempDir() + "/tasks.json")

	err := mgr.Complete(999)
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestDeleteTask(t *testing.T) {
	mgr := NewManager(t.TempDir() + "/tasks.json")

	task := mgr.Add("Delete me", "low")
	err := mgr.Delete(task.ID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	tasks := mgr.List()
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after delete, got %d", len(tasks))
	}
}

// TestFilterByPriority verifies that FilterByPriority returns only tasks
// whose Priority field matches the requested level.
func TestFilterByPriority(t *testing.T) {
	mgr := NewManager(t.TempDir() + "/tasks.json")

	mgr.Add("High task 1", "high")
	mgr.Add("Medium task 1", "medium")
	mgr.Add("Low task 1", "low")
	mgr.Add("High task 2", "high")
	mgr.Add("Medium task 2", "medium")

	tests := []struct {
		priority string
		wantLen  int
	}{
		{"high", 2},
		{"medium", 2},
		{"low", 1},
		{"unknown", 0},
	}

	for _, tc := range tests {
		t.Run("priority="+tc.priority, func(t *testing.T) {
			result := mgr.FilterByPriority(tc.priority)
			if len(result) != tc.wantLen {
				t.Errorf("FilterByPriority(%q): expected %d tasks, got %d",
					tc.priority, tc.wantLen, len(result))
			}
			// Verify every returned task actually has the requested priority.
			for _, task := range result {
				if task.Priority != tc.priority {
					t.Errorf("FilterByPriority(%q): got task with priority %q",
						tc.priority, task.Priority)
				}
			}
		})
	}
}

// TestFilterByPriorityEmptyStore ensures FilterByPriority returns an empty
// (non-nil) slice when the task store is empty.
func TestFilterByPriorityEmptyStore(t *testing.T) {
	mgr := NewManager(t.TempDir() + "/tasks.json")

	result := mgr.FilterByPriority("high")
	if result == nil {
		t.Error("FilterByPriority should return a non-nil slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(result))
	}
}

// ---------------------------------------------------------------------------
// Stats tests
// ---------------------------------------------------------------------------

// TestStats_EmptyStore verifies that Stats() returns all-zero values when
// there are no tasks in the store.
func TestStats_EmptyStore(t *testing.T) {
	mgr := NewManager(t.TempDir() + "/tasks.json")

	s := mgr.Stats()

	if s.Total != 0 {
		t.Errorf("Total: want 0, got %d", s.Total)
	}
	if s.Pending != 0 {
		t.Errorf("Pending: want 0, got %d", s.Pending)
	}
	if s.Completed != 0 {
		t.Errorf("Completed: want 0, got %d", s.Completed)
	}
	if s.HighPriority != 0 {
		t.Errorf("HighPriority: want 0, got %d", s.HighPriority)
	}
	if s.MediumPriority != 0 {
		t.Errorf("MediumPriority: want 0, got %d", s.MediumPriority)
	}
	if s.LowPriority != 0 {
		t.Errorf("LowPriority: want 0, got %d", s.LowPriority)
	}
	if s.CompletionRate != 0 {
		t.Errorf("CompletionRate: want 0, got %d", s.CompletionRate)
	}
}

// TestStats_MixedTasks verifies Stats() counts correctly across a realistic
// mix of tasks with different priorities and completion states.
// This mirrors the exact example from issue #3:
//
//	Total tasks: 12 | Pending: 8 | Completed: 4 | Completion rate: 33%
//	High priority: 3 | Medium priority: 6 | Low priority: 3
func TestStats_MixedTasks(t *testing.T) {
	mgr := NewManager(t.TempDir() + "/tasks.json")

	// Add 12 tasks: 3 high, 6 medium, 3 low
	h1 := mgr.Add("High 1", "high")
	h2 := mgr.Add("High 2", "high")
	mgr.Add("High 3", "high")
	m1 := mgr.Add("Medium 1", "medium")
	m2 := mgr.Add("Medium 2", "medium")
	mgr.Add("Medium 3", "medium")
	mgr.Add("Medium 4", "medium")
	mgr.Add("Medium 5", "medium")
	mgr.Add("Medium 6", "medium")
	mgr.Add("Low 1", "low")
	mgr.Add("Low 2", "low")
	mgr.Add("Low 3", "low")

	// Complete 4 tasks (h1, h2, m1, m2)
	mgr.Complete(h1.ID)
	mgr.Complete(h2.ID)
	mgr.Complete(m1.ID)
	mgr.Complete(m2.ID)

	s := mgr.Stats()

	if s.Total != 12 {
		t.Errorf("Total: want 12, got %d", s.Total)
	}
	if s.Completed != 4 {
		t.Errorf("Completed: want 4, got %d", s.Completed)
	}
	if s.Pending != 8 {
		t.Errorf("Pending: want 8, got %d", s.Pending)
	}
	if s.HighPriority != 3 {
		t.Errorf("HighPriority: want 3, got %d", s.HighPriority)
	}
	if s.MediumPriority != 6 {
		t.Errorf("MediumPriority: want 6, got %d", s.MediumPriority)
	}
	if s.LowPriority != 3 {
		t.Errorf("LowPriority: want 3, got %d", s.LowPriority)
	}
	// 4/12 = 33% (integer division)
	if s.CompletionRate != 33 {
		t.Errorf("CompletionRate: want 33, got %d", s.CompletionRate)
	}
}

// TestStats_AllCompleted verifies that CompletionRate is 100 when every
// task is marked done.
func TestStats_AllCompleted(t *testing.T) {
	mgr := NewManager(t.TempDir() + "/tasks.json")

	t1 := mgr.Add("Task A", "high")
	t2 := mgr.Add("Task B", "low")
	mgr.Complete(t1.ID)
	mgr.Complete(t2.ID)

	s := mgr.Stats()

	if s.Total != 2 {
		t.Errorf("Total: want 2, got %d", s.Total)
	}
	if s.Completed != 2 {
		t.Errorf("Completed: want 2, got %d", s.Completed)
	}
	if s.Pending != 0 {
		t.Errorf("Pending: want 0, got %d", s.Pending)
	}
	if s.CompletionRate != 100 {
		t.Errorf("CompletionRate: want 100, got %d", s.CompletionRate)
	}
}

// TestStats_NoPending verifies counts when all tasks are pending (none done).
func TestStats_NoPending(t *testing.T) {
	mgr := NewManager(t.TempDir() + "/tasks.json")

	mgr.Add("Task 1", "high")
	mgr.Add("Task 2", "medium")
	mgr.Add("Task 3", "low")

	s := mgr.Stats()

	if s.Total != 3 {
		t.Errorf("Total: want 3, got %d", s.Total)
	}
	if s.Pending != 3 {
		t.Errorf("Pending: want 3, got %d", s.Pending)
	}
	if s.Completed != 0 {
		t.Errorf("Completed: want 0, got %d", s.Completed)
	}
	if s.CompletionRate != 0 {
		t.Errorf("CompletionRate: want 0, got %d", s.CompletionRate)
	}
}

// TestStats_PriorityCounts verifies that priority counters are independent
// of the Done state — a completed high-priority task still counts as high.
func TestStats_PriorityCounts(t *testing.T) {
	mgr := NewManager(t.TempDir() + "/tasks.json")

	t1 := mgr.Add("High done", "high")
	mgr.Add("High pending", "high")
	mgr.Add("Medium pending", "medium")
	mgr.Complete(t1.ID)

	s := mgr.Stats()

	if s.HighPriority != 2 {
		t.Errorf("HighPriority: want 2, got %d", s.HighPriority)
	}
	if s.MediumPriority != 1 {
		t.Errorf("MediumPriority: want 1, got %d", s.MediumPriority)
	}
	if s.LowPriority != 0 {
		t.Errorf("LowPriority: want 0, got %d", s.LowPriority)
	}
}

// TestStats_DeleteAffectsTotal verifies that deleting a task reduces the
// total count reflected in Stats().
func TestStats_DeleteAffectsTotal(t *testing.T) {
	mgr := NewManager(t.TempDir() + "/tasks.json")

	t1 := mgr.Add("Task 1", "high")
	mgr.Add("Task 2", "medium")
	mgr.Delete(t1.ID)

	s := mgr.Stats()

	if s.Total != 1 {
		t.Errorf("Total after delete: want 1, got %d", s.Total)
	}
	if s.HighPriority != 0 {
		t.Errorf("HighPriority after delete: want 0, got %d", s.HighPriority)
	}
	if s.MediumPriority != 1 {
		t.Errorf("MediumPriority after delete: want 1, got %d", s.MediumPriority)
	}
}
