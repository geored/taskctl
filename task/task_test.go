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
