package task

import (
	"os"
	"testing"
)

func TestAddTask(t *testing.T) {
	mgr := NewManager("/tmp/test_tasks.json")
	defer os.Remove("/tmp/test_tasks.json")

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
	mgr := NewManager("/tmp/test_tasks.json")
	defer os.Remove("/tmp/test_tasks.json")

	mgr.Add("Task 1", "low")
	mgr.Add("Task 2", "high")

	tasks := mgr.List()
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestCompleteTask(t *testing.T) {
	mgr := NewManager("/tmp/test_tasks.json")
	defer os.Remove("/tmp/test_tasks.json")

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
	mgr := NewManager("/tmp/test_tasks.json")
	defer os.Remove("/tmp/test_tasks.json")

	err := mgr.Complete(999)
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestDeleteTask(t *testing.T) {
	mgr := NewManager("/tmp/test_tasks.json")
	defer os.Remove("/tmp/test_tasks.json")

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
	mgr := NewManager("/tmp/test_filter_tasks.json")
	defer os.Remove("/tmp/test_filter_tasks.json")

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
	mgr := NewManager("/tmp/test_filter_empty.json")
	defer os.Remove("/tmp/test_filter_empty.json")

	result := mgr.FilterByPriority("high")
	if result == nil {
		t.Error("FilterByPriority should return a non-nil slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(result))
	}
}
