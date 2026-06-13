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
// Existing functionality tests (preserved + upgraded to t.TempDir())
// ---------------------------------------------------------------------------

func TestAdd(t *testing.T) {
	mgr := newManager(t)
	if err := mgr.Add("Buy milk", "low", ""); err != nil {
		t.Fatalf("Add: unexpected error: %v", err)
	}
	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Title != "Buy milk" {
		t.Errorf("expected title %q, got %q", "Buy milk", tasks[0].Title)
	}
}

func TestList(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task A", "high", "")
	_ = mgr.Add("Task B", "low", "")

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestComplete(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Finish report", "medium", "")

	tasks, _ := mgr.List("", false)
	id := tasks[0].ID

	if err := mgr.Complete(id); err != nil {
		t.Fatalf("Complete: unexpected error: %v", err)
	}
	tasks, _ = mgr.List("", false)
	if !tasks[0].Done {
		t.Error("expected task to be marked done")
	}
}

func TestDelete(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Temporary task", "medium", "")

	tasks, _ := mgr.List("", false)
	id := tasks[0].ID

	if err := mgr.Delete(id); err != nil {
		t.Fatalf("Delete: unexpected error: %v", err)
	}
	tasks, _ = mgr.List("", false)
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after delete, got %d", len(tasks))
	}
}

func TestListFilterByPriority(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("High task", "high", "")
	_ = mgr.Add("Low task", "low", "")
	_ = mgr.Add("Medium task", "medium", "")

	tasks, err := mgr.List("high", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 high-priority task, got %d", len(tasks))
	}
	if tasks[0].Priority != "high" {
		t.Errorf("expected priority %q, got %q", "high", tasks[0].Priority)
	}
}

// ---------------------------------------------------------------------------
// Priority validation tests
// ---------------------------------------------------------------------------

// TestAddInvalidPriority verifies that Add rejects unrecognised priority values.
func TestAddInvalidPriority(t *testing.T) {
	mgr := newManager(t)
	tests := []string{"critical", "urgent", "", "HIGH", "Low"}
	for _, p := range tests {
		err := mgr.Add("Some task", p, "")
		if err == nil {
			t.Errorf("Add with priority %q: expected error, got nil", p)
		}
	}
}

// TestAddValidPriorities verifies that all three accepted priority values are
// stored correctly.
func TestAddValidPriorities(t *testing.T) {
	for _, priority := range []string{"high", "medium", "low"} {
		mgr := newManager(t)
		if err := mgr.Add("Task", priority, ""); err != nil {
			t.Errorf("Add with priority %q: unexpected error: %v", priority, err)
		}
		tasks, _ := mgr.List("", false)
		if len(tasks) != 1 || tasks[0].Priority != priority {
			t.Errorf("priority %q: stored task has priority %q", priority, tasks[0].Priority)
		}
	}
}

// ---------------------------------------------------------------------------
// Stats tests (existing, preserved)
// ---------------------------------------------------------------------------

func TestStatsEmpty(t *testing.T) {
	mgr := newManager(t)
	s, err := mgr.Stats()
	if err != nil {
		t.Fatalf("Stats: unexpected error: %v", err)
	}
	if s.Total != 0 || s.Completed != 0 || s.Pending != 0 || s.Overdue != 0 {
		t.Errorf("expected all-zero stats on empty store, got %+v", s)
	}
}

func TestStatsMixed(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task 1", "high", "")
	_ = mgr.Add("Task 2", "low", "")
	_ = mgr.Add("Task 3", "medium", "")

	tasks, _ := mgr.List("", false)
	_ = mgr.Complete(tasks[0].ID)

	s, err := mgr.Stats()
	if err != nil {
		t.Fatalf("Stats: unexpected error: %v", err)
	}
	if s.Total != 3 {
		t.Errorf("Total: expected 3, got %d", s.Total)
	}
	if s.Completed != 1 {
		t.Errorf("Completed: expected 1, got %d", s.Completed)
	}
	if s.Pending != 2 {
		t.Errorf("Pending: expected 2, got %d", s.Pending)
	}
}

// ---------------------------------------------------------------------------
// Due date tests
// ---------------------------------------------------------------------------

func TestAddWithDueDate(t *testing.T) {
	mgr := newManager(t)
	if err := mgr.Add("Submit report", "high", "2030-12-31"); err != nil {
		t.Fatalf("Add with due date: unexpected error: %v", err)
	}
	tasks, _ := mgr.List("", false)
	if tasks[0].DueDate != "2030-12-31" {
		t.Errorf("expected DueDate %q, got %q", "2030-12-31", tasks[0].DueDate)
	}
}

func TestAddInvalidDueDate(t *testing.T) {
	mgr := newManager(t)
	err := mgr.Add("Bad date task", "low", "not-a-date")
	if err == nil {
		t.Fatal("expected error for invalid due date, got nil")
	}
}

func TestAddNoDueDate(t *testing.T) {
	mgr := newManager(t)
	if err := mgr.Add("No due date", "medium", ""); err != nil {
		t.Fatalf("Add without due date: unexpected error: %v", err)
	}
	tasks, _ := mgr.List("", false)
	if tasks[0].DueDate != "" {
		t.Errorf("expected empty DueDate, got %q", tasks[0].DueDate)
	}
}

// TestIsOverdue_PastDate verifies that an incomplete task with a past due date
// is reported as overdue.
func TestIsOverdue_PastDate(t *testing.T) {
	task := Task{
		ID:      1,
		Title:   "Old task",
		Done:    false,
		DueDate: "2000-01-01",
	}
	now := time.Now()
	if !task.IsOverdue(now) {
		t.Error("expected task with past due date to be overdue")
	}
}

// TestIsOverdue_FutureDate verifies that an incomplete task with a future due
// date is not considered overdue.
func TestIsOverdue_FutureDate(t *testing.T) {
	task := Task{
		ID:      2,
		Title:   "Future task",
		Done:    false,
		DueDate: "2099-12-31",
	}
	now := time.Now()
	if task.IsOverdue(now) {
		t.Error("expected task with future due date to not be overdue")
	}
}

// TestIsOverdue_DoneTask verifies that a completed task is never overdue even
// if its due date is in the past.
func TestIsOverdue_DoneTask(t *testing.T) {
	task := Task{
		ID:      3,
		Title:   "Done old task",
		Done:    true,
		DueDate: "2000-01-01",
	}
	now := time.Now()
	if task.IsOverdue(now) {
		t.Error("expected done task to not be overdue")
	}
}

// TestIsOverdue_NoDueDate verifies that a task without a due date is never
// considered overdue.
func TestIsOverdue_NoDueDate(t *testing.T) {
	task := Task{
		ID:    4,
		Title: "No due date task",
		Done:  false,
	}
	now := time.Now()
	if task.IsOverdue(now) {
		t.Error("expected task with no due date to not be overdue")
	}
}

func TestListOverdueFilter(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Overdue task", "high", "2000-01-01")
	_ = mgr.Add("Future task", "low", "2099-12-31")
	_ = mgr.Add("No due date", "medium", "")

	overdue, err := mgr.List("", true)
	if err != nil {
		t.Fatalf("List overdue: unexpected error: %v", err)
	}
	if len(overdue) != 1 {
		t.Fatalf("expected 1 overdue task, got %d", len(overdue))
	}
	if overdue[0].Title != "Overdue task" {
		t.Errorf("expected overdue task title %q, got %q", "Overdue task", overdue[0].Title)
	}
}

// TestStatsOverdue verifies that Stats.Overdue counts only incomplete tasks
// with a past due date.
func TestStatsOverdue(t *testing.T) {
	mgr := newManager(t)

	// 2 overdue (past date, incomplete)
	_ = mgr.Add("Overdue 1", "high", "2000-01-01")
	_ = mgr.Add("Overdue 2", "low", "1999-12-31")
	// 1 not overdue (future)
	_ = mgr.Add("Future", "medium", "2099-01-01")
	// 1 no due date
	_ = mgr.Add("No date", "medium", "")
	// 1 completed with past date — should NOT count as overdue
	_ = mgr.Add("Done old", "high", "2000-06-01")
	tasks, _ := mgr.List("", false)
	_ = mgr.Complete(tasks[4].ID)

	s, err := mgr.Stats()
	if err != nil {
		t.Fatalf("Stats: unexpected error: %v", err)
	}
	if s.Total != 5 {
		t.Errorf("Total: expected 5, got %d", s.Total)
	}
	if s.Overdue != 2 {
		t.Errorf("Overdue: expected 2, got %d", s.Overdue)
	}
}

// TestStatsOverdueZeroWhenNoDueDates verifies Overdue is 0 when no tasks have
// due dates set.
func TestStatsOverdueZeroWhenNoDueDates(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task A", "high", "")
	_ = mgr.Add("Task B", "low", "")

	s, err := mgr.Stats()
	if err != nil {
		t.Fatalf("Stats: unexpected error: %v", err)
	}
	if s.Overdue != 0 {
		t.Errorf("Overdue: expected 0, got %d", s.Overdue)
	}
}

// ---------------------------------------------------------------------------
// Atomic save and file permission tests
// ---------------------------------------------------------------------------

// TestSaveFilePermissions verifies that the tasks file is written with mode
// 0600 (owner read/write only) after an Add operation.
func TestSaveFilePermissions(t *testing.T) {
	mgr := newManager(t)
	if err := mgr.Add("Permission check task", "medium", ""); err != nil {
		t.Fatalf("Add: unexpected error: %v", err)
	}

	info, err := os.Stat(mgr.filePath)
	if err != nil {
		t.Fatalf("Stat: unexpected error: %v", err)
	}

	got := info.Mode().Perm()
	const want = os.FileMode(0600)
	if got != want {
		t.Errorf("file permissions: expected %04o, got %04o", want, got)
	}
}

// TestSaveAtomicity verifies that Add followed immediately by List returns a
// consistent task list with no partial-write corruption. Running with -race
// validates there are no data races in the save path.
func TestSaveAtomicity(t *testing.T) {
	mgr := newManager(t)

	const n = 20
	for i := 0; i < n; i++ {
		if err := mgr.Add("Task", "low", ""); err != nil {
			t.Fatalf("Add iteration %d: unexpected error: %v", i, err)
		}
		tasks, err := mgr.List("", false)
		if err != nil {
			t.Fatalf("List after Add %d: unexpected error: %v", i, err)
		}
		if len(tasks) != i+1 {
			t.Fatalf("after %d adds: expected %d tasks, got %d", i+1, i+1, len(tasks))
		}
	}
}

// ---------------------------------------------------------------------------
// Error-path tests (new — requirement #3)
// ---------------------------------------------------------------------------

// TestCompleteNonExistentID verifies that Complete returns an error when the
// given ID does not exist in the task store.
func TestCompleteNonExistentID(t *testing.T) {
	mgr := newManager(t)
	// Empty store — ID 9999 cannot exist.
	err := mgr.Complete(9999)
	if err == nil {
		t.Fatal("Complete with non-existent ID: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "9999") {
		t.Errorf("Complete error should mention the task ID, got: %v", err)
	}
}

// TestCompleteNonExistentIDWithTasks verifies that Complete returns an error
// when the ID does not match any task even when other tasks exist.
func TestCompleteNonExistentIDWithTasks(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Existing task", "medium", "")

	err := mgr.Complete(9999)
	if err == nil {
		t.Fatal("Complete with non-existent ID: expected error, got nil")
	}
}

// TestDeleteNonExistentID verifies that Delete returns an error when the given
// ID does not exist in the task store.
func TestDeleteNonExistentID(t *testing.T) {
	mgr := newManager(t)
	// Empty store — ID 9999 cannot exist.
	err := mgr.Delete(9999)
	if err == nil {
		t.Fatal("Delete with non-existent ID: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "9999") {
		t.Errorf("Delete error should mention the task ID, got: %v", err)
	}
}

// TestDeleteNonExistentIDWithTasks verifies that Delete returns an error when
// the ID does not match any task even when other tasks exist.
func TestDeleteNonExistentIDWithTasks(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Existing task", "high", "")

	err := mgr.Delete(9999)
	if err == nil {
		t.Fatal("Delete with non-existent ID: expected error, got nil")
	}
}

// TestAddEmptyTitle verifies that Add returns an error when the title is empty
// or consists only of whitespace.
func TestAddEmptyTitle(t *testing.T) {
	mgr := newManager(t)
	cases := []struct {
		name  string
		title string
	}{
		{"empty string", ""},
		{"spaces only", "   "},
		{"tabs only", "\t\t"},
		{"newline only", "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := mgr.Add(tc.title, "medium", "")
			if err == nil {
				t.Errorf("Add with title %q: expected error, got nil", tc.title)
			}
		})
	}
}

// TestListInvalidPriority verifies that List returns an error when called with
// an unrecognised priority string (library-boundary validation — requirement #1).
func TestListInvalidPriority(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("A task", "medium", "")

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
		{"", 3},     // no filter — all tasks
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

// ---------------------------------------------------------------------------
// Clear tests
// ---------------------------------------------------------------------------

// TestClearEmptyStore verifies that Clear on an empty task list returns (0, nil).
func TestClearEmptyStore(t *testing.T) {
	mgr := newManager(t)
	cleared, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear on empty store: unexpected error: %v", err)
	}
	if cleared != 0 {
		t.Errorf("Clear on empty store: expected 0 cleared, got %d", cleared)
	}
}

// TestClearNoDoneTasks verifies that Clear returns (0, nil) when no tasks are
// marked done and leaves all pending tasks intact.
func TestClearNoDoneTasks(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Pending A", "high", "")
	_ = mgr.Add("Pending B", "low", "")

	cleared, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear with no done tasks: unexpected error: %v", err)
	}
	if cleared != 0 {
		t.Errorf("expected 0 cleared, got %d", cleared)
	}

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks remaining, got %d", len(tasks))
	}
}

// TestClearRemovesDoneTasks verifies that Clear removes all done tasks and
// returns the correct count.
func TestClearRemovesDoneTasks(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task 1", "high", "")
	_ = mgr.Add("Task 2", "medium", "")
	_ = mgr.Add("Task 3", "low", "")

	tasks, _ := mgr.List("", false)
	_ = mgr.Complete(tasks[0].ID)
	_ = mgr.Complete(tasks[2].ID)

	cleared, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}
	if cleared != 2 {
		t.Errorf("expected 2 cleared, got %d", cleared)
	}

	remaining, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected 1 task remaining, got %d", len(remaining))
	}
	if remaining[0].Title != "Task 2" {
		t.Errorf("expected remaining task %q, got %q", "Task 2", remaining[0].Title)
	}
}

// TestClearAllDoneTasks verifies that Clear removes all tasks when every task
// is done and returns an empty list.
func TestClearAllDoneTasks(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Done 1", "high", "")
	_ = mgr.Add("Done 2", "low", "")

	tasks, _ := mgr.List("", false)
	for _, tk := range tasks {
		_ = mgr.Complete(tk.ID)
	}

	cleared, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear all done: unexpected error: %v", err)
	}
	if cleared != 2 {
		t.Errorf("expected 2 cleared, got %d", cleared)
	}

	remaining, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear all: unexpected error: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected 0 tasks remaining, got %d", len(remaining))
	}
}

// TestClearLeavesPendingTasksIntact verifies that Clear only removes done tasks
// and leaves all pending tasks with their original data unchanged.
func TestClearLeavesPendingTasksIntact(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Keep me", "high", "2099-12-31")
	_ = mgr.Add("Remove me", "low", "")
	_ = mgr.Add("Also keep me", "medium", "")

	tasks, _ := mgr.List("", false)
	// Mark "Remove me" (index 1) as done.
	_ = mgr.Complete(tasks[1].ID)

	cleared, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}
	if cleared != 1 {
		t.Errorf("expected 1 cleared, got %d", cleared)
	}

	remaining, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining tasks, got %d", len(remaining))
	}

	// Verify titles and fields of surviving tasks are intact.
	titles := map[string]bool{}
	for _, tk := range remaining {
		titles[tk.Title] = true
		if tk.Done {
			t.Errorf("remaining task %q should not be done", tk.Title)
		}
	}
	if !titles["Keep me"] {
		t.Error("expected 'Keep me' to remain after Clear")
	}
	if !titles["Also keep me"] {
		t.Error("expected 'Also keep me' to remain after Clear")
	}
}

// TestClearErrorOnUnreadableFile verifies that Clear propagates an error when
// the backing file is unreadable (e.g., after permissions are revoked).
func TestClearErrorOnUnreadableFile(t *testing.T) {
	mgr := newManager(t)
	// Add a task so the file is written.
	if err := mgr.Add("A task", "medium", ""); err != nil {
		t.Fatalf("Add: unexpected error: %v", err)
	}

	// Remove read permissions on the file so load() will fail.
	if err := os.Chmod(mgr.filePath, 0000); err != nil {
		t.Fatalf("Chmod: unexpected error: %v", err)
	}
	// Restore permissions after the test so the temp dir can be cleaned up.
	t.Cleanup(func() { os.Chmod(mgr.filePath, 0600) }) //nolint:errcheck

	_, err := mgr.Clear()
	if err == nil {
		t.Fatal("Clear on unreadable file: expected error, got nil")
	}
}
