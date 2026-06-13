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
		t.Errorf("IsOverdue: expected true for past due date %q, got false", task.DueDate)
	}
}

// TestIsOverdue_FutureDate verifies that an incomplete task with a future due
// date is not reported as overdue.
func TestIsOverdue_FutureDate(t *testing.T) {
	task := Task{
		ID:      2,
		Title:   "Future task",
		Done:    false,
		DueDate: "2099-12-31",
	}
	now := time.Now()
	if task.IsOverdue(now) {
		t.Errorf("IsOverdue: expected false for future due date %q, got true", task.DueDate)
	}
}

// TestIsOverdue_DoneTask verifies that a completed task is never considered
// overdue regardless of its due date.
func TestIsOverdue_DoneTask(t *testing.T) {
	task := Task{
		ID:      3,
		Title:   "Done old task",
		Done:    true,
		DueDate: "2000-01-01",
	}
	now := time.Now()
	if task.IsOverdue(now) {
		t.Errorf("IsOverdue: expected false for done task, got true")
	}
}

// TestIsOverdue_NoDueDate verifies that a task with no due date is never
// considered overdue.
func TestIsOverdue_NoDueDate(t *testing.T) {
	task := Task{
		ID:    4,
		Title: "No due date",
		Done:  false,
	}
	now := time.Now()
	if task.IsOverdue(now) {
		t.Errorf("IsOverdue: expected false for task with no due date, got true")
	}
}

func TestListOverdueFilter(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Overdue task", "high", "2000-01-01")
	_ = mgr.Add("Future task", "medium", "2099-01-01")
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

// TestClear_MixedTasks verifies that Clear removes only done tasks and returns
// correct cleared/remaining counts.
func TestClear_MixedTasks(t *testing.T) {
	mgr := newManager(t)

	_ = mgr.Add("Pending task 1", "high", "")
	_ = mgr.Add("Done task 1", "medium", "")
	_ = mgr.Add("Pending task 2", "low", "")
	_ = mgr.Add("Done task 2", "high", "")
	_ = mgr.Add("Pending task 3", "medium", "")

	tasks, _ := mgr.List("", false)
	// Mark tasks at index 1 and 3 as done.
	_ = mgr.Complete(tasks[1].ID)
	_ = mgr.Complete(tasks[3].ID)

	cleared, remaining, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}
	if cleared != 2 {
		t.Errorf("cleared: expected 2, got %d", cleared)
	}
	if remaining != 3 {
		t.Errorf("remaining: expected 3, got %d", remaining)
	}

	// Verify only pending tasks survive.
	after, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(after) != 3 {
		t.Errorf("expected 3 tasks after clear, got %d", len(after))
	}
	for _, task := range after {
		if task.Done {
			t.Errorf("Clear left a done task in the store: %+v", task)
		}
	}
}

// TestClear_AllDone verifies that Clear removes all tasks when every task is
// marked done, leaving remaining == 0.
func TestClear_AllDone(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Done 1", "high", "")
	_ = mgr.Add("Done 2", "medium", "")

	tasks, _ := mgr.List("", false)
	for _, task := range tasks {
		_ = mgr.Complete(task.ID)
	}

	cleared, remaining, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}
	if cleared != 2 {
		t.Errorf("cleared: expected 2, got %d", cleared)
	}
	if remaining != 0 {
		t.Errorf("remaining: expected 0, got %d", remaining)
	}

	after, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("expected 0 tasks after clear, got %d", len(after))
	}
}

// TestClear_NoneDone verifies that Clear returns cleared == 0 and all tasks
// remain when no tasks are marked done.
func TestClear_NoneDone(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Pending 1", "high", "")
	_ = mgr.Add("Pending 2", "low", "")
	_ = mgr.Add("Pending 3", "medium", "")

	cleared, remaining, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}
	if cleared != 0 {
		t.Errorf("cleared: expected 0, got %d", cleared)
	}
	if remaining != 3 {
		t.Errorf("remaining: expected 3, got %d", remaining)
	}

	after, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(after) != 3 {
		t.Errorf("expected 3 tasks after clear, got %d", len(after))
	}
}

// TestClear_EmptyStore verifies that Clear on an empty store returns
// cleared == 0, remaining == 0, and no error.
func TestClear_EmptyStore(t *testing.T) {
	mgr := newManager(t)

	cleared, remaining, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear on empty store: unexpected error: %v", err)
	}
	if cleared != 0 {
		t.Errorf("cleared: expected 0, got %d", cleared)
	}
	if remaining != 0 {
		t.Errorf("remaining: expected 0, got %d", remaining)
	}
}

// TestClear_PendingTasksPreserved verifies that pending tasks retain their
// original titles and priorities after a Clear operation.
func TestClear_PendingTasksPreserved(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Keep me", "high", "")
	_ = mgr.Add("Remove me", "low", "")

	tasks, _ := mgr.List("", false)
	// Mark the second task done.
	_ = mgr.Complete(tasks[1].ID)

	_, _, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}

	after, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(after) != 1 {
		t.Fatalf("expected 1 task after clear, got %d", len(after))
	}
	if after[0].Title != "Keep me" {
		t.Errorf("expected surviving task title %q, got %q", "Keep me", after[0].Title)
	}
	if after[0].Priority != "high" {
		t.Errorf("expected surviving task priority %q, got %q", "high", after[0].Priority)
	}
}
