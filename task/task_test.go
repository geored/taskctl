package task

import (
	"encoding/json"
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
		t.Error("expected IsOverdue to return true for past due date")
	}
}

// TestIsOverdue_FutureDate verifies that a task with a future due date is not
// considered overdue.
func TestIsOverdue_FutureDate(t *testing.T) {
	task := Task{
		ID:      2,
		Title:   "Future task",
		Done:    false,
		DueDate: "2099-12-31",
	}
	now := time.Now()
	if task.IsOverdue(now) {
		t.Error("expected IsOverdue to return false for future due date")
	}
}

// TestIsOverdue_DoneTask verifies that a completed task is never considered
// overdue, even if the due date has passed.
func TestIsOverdue_DoneTask(t *testing.T) {
	task := Task{
		ID:      3,
		Title:   "Done old task",
		Done:    true,
		DueDate: "2000-01-01",
	}
	now := time.Now()
	if task.IsOverdue(now) {
		t.Error("expected IsOverdue to return false for a completed task")
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
		t.Error("expected IsOverdue to return false for task without due date")
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

// TestClearMixedList verifies that Clear removes only done tasks and keeps
// pending tasks, returning the correct cleared and remaining counts.
func TestClearMixedList(t *testing.T) {
	mgr := newManager(t)

	// Add 5 tasks, mark 3 as done, leave 2 pending.
	titles := []string{"Task A", "Task B", "Task C", "Task D", "Task E"}
	for _, title := range titles {
		if err := mgr.Add(title, "medium", ""); err != nil {
			t.Fatalf("Add %q: unexpected error: %v", title, err)
		}
	}
	all, _ := mgr.List("", false)
	// Mark the first 3 done.
	for i := 0; i < 3; i++ {
		if err := mgr.Complete(all[i].ID); err != nil {
			t.Fatalf("Complete task %d: unexpected error: %v", all[i].ID, err)
		}
	}

	cleared, remaining, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}
	if cleared != 3 {
		t.Errorf("Clear: expected cleared=3, got %d", cleared)
	}
	if remaining != 2 {
		t.Errorf("Clear: expected remaining=2, got %d", remaining)
	}

	// Verify only pending tasks survive on disk.
	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("List after Clear: expected 2 tasks, got %d", len(tasks))
	}
	for _, task := range tasks {
		if task.Done {
			t.Errorf("List after Clear: task %d (%q) is done but should have been cleared", task.ID, task.Title)
		}
	}
}

// TestClearAllDone verifies that Clear removes every task when all are done,
// leaving remaining == 0.
func TestClearAllDone(t *testing.T) {
	mgr := newManager(t)

	for i := 0; i < 4; i++ {
		if err := mgr.Add("Done task", "low", ""); err != nil {
			t.Fatalf("Add: unexpected error: %v", err)
		}
	}
	all, _ := mgr.List("", false)
	for _, task := range all {
		if err := mgr.Complete(task.ID); err != nil {
			t.Fatalf("Complete task %d: unexpected error: %v", task.ID, err)
		}
	}

	cleared, remaining, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}
	if cleared != 4 {
		t.Errorf("Clear: expected cleared=4, got %d", cleared)
	}
	if remaining != 0 {
		t.Errorf("Clear: expected remaining=0, got %d", remaining)
	}

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("List after Clear: expected 0 tasks, got %d", len(tasks))
	}
}

// TestClearNoneDone verifies that Clear leaves all tasks intact when none are
// done, returning cleared == 0.
func TestClearNoneDone(t *testing.T) {
	mgr := newManager(t)

	if err := mgr.Add("Pending A", "high", ""); err != nil {
		t.Fatalf("Add: unexpected error: %v", err)
	}
	if err := mgr.Add("Pending B", "medium", ""); err != nil {
		t.Fatalf("Add: unexpected error: %v", err)
	}

	cleared, remaining, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}
	if cleared != 0 {
		t.Errorf("Clear: expected cleared=0, got %d", cleared)
	}
	if remaining != 2 {
		t.Errorf("Clear: expected remaining=2, got %d", remaining)
	}

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("List after Clear: expected 2 tasks, got %d", len(tasks))
	}
}

// TestClearEmptyList verifies that Clear on an empty store returns
// cleared == 0, remaining == 0 with no error.
func TestClearEmptyList(t *testing.T) {
	mgr := newManager(t)

	cleared, remaining, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear on empty list: unexpected error: %v", err)
	}
	if cleared != 0 {
		t.Errorf("Clear: expected cleared=0, got %d", cleared)
	}
	if remaining != 0 {
		t.Errorf("Clear: expected remaining=0, got %d", remaining)
	}
}

// TestClearErrorOnCorruptFile verifies that Clear returns an error (and does
// not silently swallow it) when the underlying JSON file is corrupt.
func TestClearErrorOnCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	mgr, err := NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: unexpected error: %v", err)
	}

	// Write corrupt JSON directly to the file.
	if err := os.WriteFile(path, []byte("{not valid json!!!"), 0600); err != nil {
		t.Fatalf("WriteFile corrupt JSON: unexpected error: %v", err)
	}

	_, _, err = mgr.Clear()
	if err == nil {
		t.Fatal("Clear with corrupt JSON: expected error, got nil")
	}
}

// TestClearPreservesPendingTaskFields verifies that the fields of pending tasks
// are not mutated by a Clear operation (title, priority, due date, ID intact).
func TestClearPreservesPendingTaskFields(t *testing.T) {
	mgr := newManager(t)

	if err := mgr.Add("Keep me", "high", "2099-01-01"); err != nil {
		t.Fatalf("Add: unexpected error: %v", err)
	}
	if err := mgr.Add("Remove me", "low", ""); err != nil {
		t.Fatalf("Add: unexpected error: %v", err)
	}

	all, _ := mgr.List("", false)
	var keepID int
	var removeID int
	for _, task := range all {
		if task.Title == "Keep me" {
			keepID = task.ID
		} else {
			removeID = task.ID
		}
	}
	if err := mgr.Complete(removeID); err != nil {
		t.Fatalf("Complete: unexpected error: %v", err)
	}

	cleared, remaining, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}
	if cleared != 1 || remaining != 1 {
		t.Fatalf("Clear: expected cleared=1 remaining=1, got cleared=%d remaining=%d", cleared, remaining)
	}

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task after clear, got %d", len(tasks))
	}
	kept := tasks[0]
	if kept.ID != keepID {
		t.Errorf("kept task ID: expected %d, got %d", keepID, kept.ID)
	}
	if kept.Title != "Keep me" {
		t.Errorf("kept task title: expected %q, got %q", "Keep me", kept.Title)
	}
	if kept.Priority != "high" {
		t.Errorf("kept task priority: expected %q, got %q", "high", kept.Priority)
	}
	if kept.DueDate != "2099-01-01" {
		t.Errorf("kept task DueDate: expected %q, got %q", "2099-01-01", kept.DueDate)
	}
	if kept.Done {
		t.Error("kept task should not be marked done")
	}
}

// TestClearFilePermissionsAfterClear verifies that the tasks file retains
// 0600 permissions after a Clear operation.
func TestClearFilePermissionsAfterClear(t *testing.T) {
	mgr := newManager(t)

	if err := mgr.Add("Task to clear", "medium", ""); err != nil {
		t.Fatalf("Add: unexpected error: %v", err)
	}
	tasks, _ := mgr.List("", false)
	if err := mgr.Complete(tasks[0].ID); err != nil {
		t.Fatalf("Complete: unexpected error: %v", err)
	}

	if _, _, err := mgr.Clear(); err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}

	info, err := os.Stat(mgr.filePath)
	if err != nil {
		t.Fatalf("Stat: unexpected error: %v", err)
	}
	got := info.Mode().Perm()
	const want = os.FileMode(0600)
	if got != want {
		t.Errorf("file permissions after Clear: expected %04o, got %04o", want, got)
	}
}

// ---------------------------------------------------------------------------
// Helper: seed a manager with raw JSON (bypasses validation, for error tests).
// ---------------------------------------------------------------------------

// writeTasks writes the given tasks directly to the manager's file as JSON,
// bypassing normal validation — used only in error-path tests.
func writeTasks(t *testing.T, mgr *Manager, tasks []Task) {
	t.Helper()
	data, err := json.Marshal(tasks)
	if err != nil {
		t.Fatalf("writeTasks: marshal: %v", err)
	}
	if err := os.WriteFile(mgr.filePath, data, 0600); err != nil {
		t.Fatalf("writeTasks: WriteFile: %v", err)
	}
}
