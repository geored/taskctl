package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newManager is a test helper that creates a Manager backed by a temp file.
func newManager(t *testing.T) *Manager {
	t.Helper()
	mgr, err := NewManager(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatalf("NewManager: unexpected error: %v", err)
	}
	return mgr
}

// ---------------------------------------------------------------------------
// NewManager path validation tests
// ---------------------------------------------------------------------------

// TestNewManagerTraversalRejected verifies that paths beginning with ".." are
// rejected by NewManager.
func TestNewManagerTraversalRejected(t *testing.T) {
	paths := []string{
		"../../etc/shadow",
		"../sibling/tasks.json",
		"..",
	}
	for _, p := range paths {
		_, err := NewManager(p)
		if err == nil {
			t.Errorf("NewManager(%q): expected error for traversal path, got nil", p)
		}
	}
}

// TestNewManagerValidPath verifies that a plain relative path is accepted.
func TestNewManagerValidPath(t *testing.T) {
	dir := t.TempDir()
	_, err := NewManager(filepath.Join(dir, "tasks.json"))
	if err != nil {
		t.Errorf("NewManager: unexpected error for valid path: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Add tests
// ---------------------------------------------------------------------------

func TestAdd_EmptyTitle(t *testing.T) {
	mgr := newManager(t)
	err := mgr.Add("", "medium", "")
	if err == nil {
		t.Fatal("Add with empty title: expected error, got nil")
	}
}

func TestAdd_WhitespaceTitle(t *testing.T) {
	mgr := newManager(t)
	err := mgr.Add("   ", "medium", "")
	if err == nil {
		t.Fatal("Add with whitespace-only title: expected error, got nil")
	}
}

func TestAdd_InvalidPriority(t *testing.T) {
	mgr := newManager(t)
	err := mgr.Add("My task", "urgent", "")
	if err == nil {
		t.Fatal("Add with invalid priority: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "urgent") {
		t.Errorf("expected error to mention 'urgent', got: %v", err)
	}
}

func TestAdd_ValidPriorities(t *testing.T) {
	for _, p := range []string{"high", "medium", "low"} {
		mgr := newManager(t)
		if err := mgr.Add("Task", p, ""); err != nil {
			t.Errorf("Add with priority %q: unexpected error: %v", p, err)
		}
	}
}

func TestAdd_InvalidDueDate(t *testing.T) {
	mgr := newManager(t)
	err := mgr.Add("My task", "medium", "not-a-date")
	if err == nil {
		t.Fatal("Add with invalid due date: expected error, got nil")
	}
}

func TestAdd_ValidDueDate(t *testing.T) {
	mgr := newManager(t)
	err := mgr.Add("My task", "medium", "2099-12-31")
	if err != nil {
		t.Fatalf("Add with valid due date: unexpected error: %v", err)
	}
}

func TestAdd_IDAutoIncrement(t *testing.T) {
	mgr := newManager(t)
	for i := 0; i < 3; i++ {
		if err := mgr.Add("Task", "low", ""); err != nil {
			t.Fatalf("Add task %d: unexpected error: %v", i, err)
		}
	}
	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	for i, task := range tasks {
		if task.ID != i+1 {
			t.Errorf("expected task ID %d, got %d", i+1, task.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// List tests
// ---------------------------------------------------------------------------

func TestList_Empty(t *testing.T) {
	mgr := newManager(t)
	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List on empty store: unexpected error: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestList_AllTasks(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("A", "high", "")
	_ = mgr.Add("B", "low", "")
	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestList_FilterByPriority(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("High", "high", "")
	_ = mgr.Add("Low", "low", "")
	tasks, err := mgr.List("high", false)
	if err != nil {
		t.Fatalf("List(high): unexpected error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Priority != "high" {
		t.Errorf("expected 1 high-priority task, got %v", tasks)
	}
}

func TestList_OverdueFilter(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Past", "medium", "2000-01-01") // definitely overdue
	_ = mgr.Add("Future", "medium", "2099-12-31")
	tasks, err := mgr.List("", true)
	if err != nil {
		t.Fatalf("List overdue: unexpected error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Title != "Past" {
		t.Errorf("expected 1 overdue task, got %v", tasks)
	}
}

// ---------------------------------------------------------------------------
// Complete tests
// ---------------------------------------------------------------------------

func TestComplete_Success(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task", "medium", "")
	tasks, _ := mgr.List("", false)
	if err := mgr.Complete(tasks[0].ID); err != nil {
		t.Fatalf("Complete: unexpected error: %v", err)
	}
	tasks, _ = mgr.List("", false)
	if !tasks[0].Done {
		t.Error("expected task to be done after Complete")
	}
}

func TestComplete_NotFound(t *testing.T) {
	mgr := newManager(t)
	err := mgr.Complete(9999)
	if err == nil {
		t.Fatal("Complete non-existent ID: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Delete tests
// ---------------------------------------------------------------------------

func TestDelete_Success(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task", "medium", "")
	tasks, _ := mgr.List("", false)
	if err := mgr.Delete(tasks[0].ID); err != nil {
		t.Fatalf("Delete: unexpected error: %v", err)
	}
	tasks, _ = mgr.List("", false)
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after Delete, got %d", len(tasks))
	}
}

func TestDelete_NotFound(t *testing.T) {
	mgr := newManager(t)
	err := mgr.Delete(9999)
	if err == nil {
		t.Fatal("Delete non-existent ID: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// IsOverdue tests
// ---------------------------------------------------------------------------

func TestIsOverdue_NoDueDate(t *testing.T) {
	tk := Task{Done: false, DueDate: ""}
	if tk.IsOverdue(time.Now()) {
		t.Error("task with no due date should not be overdue")
	}
}

func TestIsOverdue_DoneTask(t *testing.T) {
	tk := Task{Done: true, DueDate: "2000-01-01"}
	if tk.IsOverdue(time.Now()) {
		t.Error("done task should never be overdue")
	}
}

func TestIsOverdue_FutureDate(t *testing.T) {
	tk := Task{Done: false, DueDate: "2099-12-31"}
	if tk.IsOverdue(time.Now()) {
		t.Error("task with future due date should not be overdue")
	}
}

func TestIsOverdue_PastDate(t *testing.T) {
	tk := Task{Done: false, DueDate: "2000-01-01"}
	if !tk.IsOverdue(time.Now()) {
		t.Error("task with past due date should be overdue")
	}
}

func TestIsOverdue_Today(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	tk := Task{Done: false, DueDate: today}
	if tk.IsOverdue(time.Now()) {
		t.Error("task due today should not yet be overdue")
	}
}

// ---------------------------------------------------------------------------
// Stats tests
// ---------------------------------------------------------------------------

func TestStats_Empty(t *testing.T) {
	mgr := newManager(t)
	s, err := mgr.Stats()
	if err != nil {
		t.Fatalf("Stats on empty store: unexpected error: %v", err)
	}
	if s.Total != 0 || s.Completed != 0 || s.Pending != 0 {
		t.Errorf("unexpected stats on empty store: %+v", s)
	}
}

func TestStats_WithTasks(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("High task", "high", "")
	_ = mgr.Add("Medium task", "medium", "2000-01-01") // overdue
	_ = mgr.Add("Low task", "low", "")

	tasks, _ := mgr.List("", false)
	_ = mgr.Complete(tasks[0].ID) // complete the first task

	s, err := mgr.Stats()
	if err != nil {
		t.Fatalf("Stats: unexpected error: %v", err)
	}
	if s.Total != 3 {
		t.Errorf("expected Total=3, got %d", s.Total)
	}
	if s.Completed != 1 {
		t.Errorf("expected Completed=1, got %d", s.Completed)
	}
	if s.Pending != 2 {
		t.Errorf("expected Pending=2, got %d", s.Pending)
	}
	if s.Overdue != 1 {
		t.Errorf("expected Overdue=1, got %d", s.Overdue)
	}
	if s.HighPriority != 1 {
		t.Errorf("expected HighPriority=1, got %d", s.HighPriority)
	}
	if s.MediumPriority != 1 {
		t.Errorf("expected MediumPriority=1, got %d", s.MediumPriority)
	}
	if s.LowPriority != 1 {
		t.Errorf("expected LowPriority=1, got %d", s.LowPriority)
	}
}

// ---------------------------------------------------------------------------
// Clear tests
// ---------------------------------------------------------------------------

// TestClear_NoneCompleted verifies that Clear is a valid no-op when there are
// no completed tasks: cleared=0, remaining=total, tasks on disk are unchanged.
func TestClear_NoneCompleted(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task A", "high", "")
	_ = mgr.Add("Task B", "low", "")

	cleared, remaining, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear with no completed tasks: unexpected error: %v", err)
	}
	if cleared != 0 {
		t.Errorf("expected cleared=0, got %d", cleared)
	}
	if remaining != 2 {
		t.Errorf("expected remaining=2, got %d", remaining)
	}

	// Verify tasks on disk are unchanged.
	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks on disk after no-op Clear, got %d", len(tasks))
	}
}

// TestClear_SomeCompleted verifies that Clear removes only the completed tasks
// and reports correct cleared/remaining counts.
func TestClear_SomeCompleted(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task A", "high", "")
	_ = mgr.Add("Task B", "medium", "")
	_ = mgr.Add("Task C", "low", "")

	// Complete the first two tasks.
	all, _ := mgr.List("", false)
	_ = mgr.Complete(all[0].ID)
	_ = mgr.Complete(all[1].ID)

	cleared, remaining, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}
	if cleared != 2 {
		t.Errorf("expected cleared=2, got %d", cleared)
	}
	if remaining != 1 {
		t.Errorf("expected remaining=1, got %d", remaining)
	}

	// Verify only the incomplete task remains.
	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task after Clear, got %d", len(tasks))
	}
	if tasks[0].Done {
		t.Error("remaining task should not be done")
	}
	if tasks[0].Title != "Task C" {
		t.Errorf("expected remaining task title 'Task C', got %q", tasks[0].Title)
	}
}

// TestClear_AllCompleted verifies that when all tasks are done, Clear removes
// them all and the resulting list is empty.
func TestClear_AllCompleted(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task A", "high", "")
	_ = mgr.Add("Task B", "low", "")

	all, _ := mgr.List("", false)
	for _, tk := range all {
		_ = mgr.Complete(tk.ID)
	}

	cleared, remaining, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear all completed: unexpected error: %v", err)
	}
	if cleared != 2 {
		t.Errorf("expected cleared=2, got %d", cleared)
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0, got %d", remaining)
	}

	// Verify the store is empty.
	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear all: unexpected error: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after Clear all, got %d", len(tasks))
	}
}

// TestClear_ErrorOnUnreadableFile verifies that Clear surfaces a load error
// when the backing file is unreadable.
func TestClear_ErrorOnUnreadableFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root — file permission restrictions do not apply")
	}

	dir := t.TempDir()
	filePath := filepath.Join(dir, "tasks.json")

	// Write a valid JSON file first.
	if err := os.WriteFile(filePath, []byte("[]"), 0600); err != nil {
		t.Fatalf("setup: WriteFile: %v", err)
	}

	mgr, err := NewManager(filePath)
	if err != nil {
		t.Fatalf("NewManager: unexpected error: %v", err)
	}

	// Remove read permission so load() will fail.
	if err := os.Chmod(filePath, 0000); err != nil {
		t.Fatalf("setup: Chmod: %v", err)
	}
	// Restore permissions after the test so TempDir cleanup can remove the file.
	t.Cleanup(func() { os.Chmod(filePath, 0600) }) //nolint:errcheck

	cleared, remaining, err := mgr.Clear()
	if err == nil {
		t.Fatal("Clear with unreadable file: expected error, got nil")
	}
	if cleared != 0 || remaining != 0 {
		t.Errorf("expected (0, 0) on error, got (%d, %d)", cleared, remaining)
	}
}

// ---------------------------------------------------------------------------
// TestListInvalidPriorities verifies that List rejects invalid priority values.
// ---------------------------------------------------------------------------

func TestListInvalidPriorities(t *testing.T) {
	mgr := newManager(t)

	invalidPriorities := []string{"urgent", "critical", "HIGH", "Low", "MEDIUM", "none"}
	for _, p := range invalidPriorities {
		t.Run(p, func(t *testing.T) {
			tasks, err := mgr.List(p, false)
			if err == nil {
				t.Errorf("List(%q): expected error for invalid priority, got nil (tasks=%v)", p, tasks)
			}
			if tasks != nil {
				t.Errorf("List(%q): expected nil tasks on error, got %v", p, tasks)
			}
		})
	}
}

// TestListValidPriorities verifies that List accepts all valid priority values
// including empty string (no filter) without returning an error.
func TestListValidPriorities(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("High task", "high", "")
	_ = mgr.Add("Medium task", "medium", "")
	_ = mgr.Add("Low task", "low", "")

	validPriorities := []struct {
		filter string
		want   int
	}{
		{"", 3},      // no filter — all tasks
		{"high", 1},
		{"medium", 1},
		{"low", 1},
	}
	for _, tc := range validPriorities {
		t.Run("priority="+tc.filter, func(t *testing.T) {
			tasks, err := mgr.List(tc.filter, false)
			if err != nil {
				t.Fatalf("List(%q): unexpected error: %v", tc.filter, err)
			}
			if len(tasks) != tc.want {
				t.Errorf("List(%q): expected %d tasks, got %d", tc.filter, tc.want, len(tasks))
			}
		})
	}
}
