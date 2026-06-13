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
		"../tasks.json",
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			mgr, err := NewManager(p)
			if err == nil {
				t.Errorf("NewManager(%q): expected traversal error, got nil (mgr=%v)", p, mgr)
			}
		})
	}
}

// TestNewManagerValidPath verifies that a normal (non-traversal) path is
// accepted by NewManager.
func TestNewManagerValidPath(t *testing.T) {
	dir := t.TempDir()
	_, err := NewManager(filepath.Join(dir, "tasks.json"))
	if err != nil {
		t.Fatalf("NewManager with valid path: unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Add tests
// ---------------------------------------------------------------------------

// TestAddAndList verifies that added tasks appear in List output and that the
// returned slice length matches the number of tasks added.
func TestAddAndList(t *testing.T) {
	mgr := newManager(t)

	titles := []string{"Buy milk", "Write code", "Walk the dog"}
	for _, title := range titles {
		if err := mgr.Add(title, "medium", ""); err != nil {
			t.Fatalf("Add(%q): unexpected error: %v", title, err)
		}
	}

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}

	if len(tasks) != len(titles) {
		t.Fatalf("expected %d tasks, got %d", len(titles), len(tasks))
	}

	for i, task := range tasks {
		if task.Title != titles[i] {
			t.Errorf("task %d: expected title %q, got %q", i, titles[i], task.Title)
		}
		if task.Done {
			t.Errorf("task %d: expected Done=false, got true", i)
		}
	}
}

// TestAddPriorityFilter verifies that List correctly filters tasks by priority.
func TestAddPriorityFilter(t *testing.T) {
	mgr := newManager(t)

	if err := mgr.Add("High task", "high", ""); err != nil {
		t.Fatalf("Add high: unexpected error: %v", err)
	}
	if err := mgr.Add("Medium task", "medium", ""); err != nil {
		t.Fatalf("Add medium: unexpected error: %v", err)
	}
	if err := mgr.Add("Low task", "low", ""); err != nil {
		t.Fatalf("Add low: unexpected error: %v", err)
	}

	cases := []struct {
		priority string
		wantLen  int
	}{
		{"high", 1},
		{"medium", 1},
		{"low", 1},
		{"", 3},
	}

	for _, tc := range cases {
		tasks, err := mgr.List(tc.priority, false)
		if err != nil {
			t.Fatalf("List(%q): unexpected error: %v", tc.priority, err)
		}
		if len(tasks) != tc.wantLen {
			t.Errorf("List(%q): expected %d tasks, got %d", tc.priority, tc.wantLen, len(tasks))
		}
	}
}

// TestAddInvalidPriority verifies that Add rejects unknown priority strings.
func TestAddInvalidPriority(t *testing.T) {
	mgr := newManager(t)
	invalidPriorities := []string{"urgent", "critical", "HIGH", "Low", "MEDIUM", "none", ""}
	for _, p := range invalidPriorities {
		if p == "" {
			continue // empty string is handled by TestAddEmptyTitle
		}
		t.Run(p, func(t *testing.T) {
			err := mgr.Add("Some task", p, "")
			if err == nil {
				t.Errorf("Add with priority %q: expected error, got nil", p)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Complete tests
// ---------------------------------------------------------------------------

// TestComplete verifies that Complete marks the correct task as done without
// altering other tasks.
func TestComplete(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task 1", "low", "")
	_ = mgr.Add("Task 2", "medium", "")
	_ = mgr.Add("Task 3", "high", "")

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}

	targetID := tasks[1].ID
	if err := mgr.Complete(targetID); err != nil {
		t.Fatalf("Complete(%d): unexpected error: %v", targetID, err)
	}

	tasks, err = mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Complete: unexpected error: %v", err)
	}

	for _, task := range tasks {
		if task.ID == targetID {
			if !task.Done {
				t.Errorf("task %d: expected Done=true, got false", targetID)
			}
		} else {
			if task.Done {
				t.Errorf("task %d: expected Done=false, got true (should be unaffected)", task.ID)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Delete tests
// ---------------------------------------------------------------------------

// TestDelete verifies that Delete removes the correct task without affecting
// the others.
func TestDelete(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task 1", "low", "")
	_ = mgr.Add("Task 2", "medium", "")
	_ = mgr.Add("Task 3", "high", "")

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}

	targetID := tasks[1].ID
	if err := mgr.Delete(targetID); err != nil {
		t.Fatalf("Delete(%d): unexpected error: %v", targetID, err)
	}

	tasks, err = mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Delete: unexpected error: %v", err)
	}

	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks after Delete, got %d", len(tasks))
	}

	for _, task := range tasks {
		if task.ID == targetID {
			t.Errorf("task %d still present after Delete", targetID)
		}
	}
}

// ---------------------------------------------------------------------------
// Stats tests
// ---------------------------------------------------------------------------

// TestStats verifies the aggregate counts returned by Stats.
func TestStats(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("High 1", "high", "")
	_ = mgr.Add("High 2", "high", "")
	_ = mgr.Add("Medium 1", "medium", "")
	_ = mgr.Add("Low 1", "low", "")

	tasks, _ := mgr.List("", false)
	_ = mgr.Complete(tasks[0].ID) // complete High 1
	_ = mgr.Complete(tasks[2].ID) // complete Medium 1

	s, err := mgr.Stats()
	if err != nil {
		t.Fatalf("Stats: unexpected error: %v", err)
	}

	if s.Total != 4 {
		t.Errorf("Total: expected 4, got %d", s.Total)
	}
	if s.Completed != 2 {
		t.Errorf("Completed: expected 2, got %d", s.Completed)
	}
	if s.Pending != 2 {
		t.Errorf("Pending: expected 2, got %d", s.Pending)
	}
	if s.HighPriority != 2 {
		t.Errorf("HighPriority: expected 2, got %d", s.HighPriority)
	}
	if s.MediumPriority != 1 {
		t.Errorf("MediumPriority: expected 1, got %d", s.MediumPriority)
	}
	if s.LowPriority != 1 {
		t.Errorf("LowPriority: expected 1, got %d", s.LowPriority)
	}
}

// TestStatsEmpty verifies that Stats returns zeroed fields on an empty store.
func TestStatsEmpty(t *testing.T) {
	mgr := newManager(t)

	s, err := mgr.Stats()
	if err != nil {
		t.Fatalf("Stats: unexpected error: %v", err)
	}

	if s.Total != 0 || s.Completed != 0 || s.Pending != 0 {
		t.Errorf("expected all zero stats, got %+v", s)
	}
}

// ---------------------------------------------------------------------------
// Due date tests
// ---------------------------------------------------------------------------

// TestAddAndListDueDate verifies that the due date is stored and retrieved
// correctly.
func TestAddAndListDueDate(t *testing.T) {
	mgr := newManager(t)
	dueDate := "2099-12-31"
	if err := mgr.Add("Future task", "high", dueDate); err != nil {
		t.Fatalf("Add: unexpected error: %v", err)
	}

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].DueDate != dueDate {
		t.Errorf("DueDate: expected %q, got %q", dueDate, tasks[0].DueDate)
	}
}

// TestAddInvalidDueDate verifies that Add rejects malformed due-date strings.
func TestAddInvalidDueDate(t *testing.T) {
	mgr := newManager(t)
	invalids := []string{
		"31-12-2099", // wrong order
		"2099/12/31", // wrong separator
		"yesterday",
		"2099-13-01", // month > 12
	}
	for _, d := range invalids {
		t.Run(d, func(t *testing.T) {
			if err := mgr.Add("Task", "medium", d); err == nil {
				t.Errorf("Add with due %q: expected error, got nil", d)
			}
		})
	}
}

// TestIsOverdue verifies that IsOverdue correctly reports past, present, and
// future due dates relative to a known reference time.
func TestIsOverdue(t *testing.T) {
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		task    Task
		want    bool
	}{
		{
			"past date incomplete",
			Task{Done: false, DueDate: "2024-06-14"},
			true,
		},
		{
			"past date complete",
			Task{Done: true, DueDate: "2024-06-14"},
			false,
		},
		{
			"today incomplete",
			Task{Done: false, DueDate: "2024-06-15"},
			false,
		},
		{
			"future date incomplete",
			Task{Done: false, DueDate: "2024-06-16"},
			false,
		},
		{
			"no due date",
			Task{Done: false, DueDate: ""},
			false,
		},
		{
			"malformed date",
			Task{Done: false, DueDate: "not-a-date"},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.task.IsOverdue(now)
			if got != tc.want {
				t.Errorf("IsOverdue: expected %v, got %v (task=%+v)", tc.want, got, tc.task)
			}
		})
	}
}

// TestListOverdueOnly verifies that passing overdueOnly=true returns only
// incomplete tasks with a past due date.
func TestListOverdueOnly(t *testing.T) {
	mgr := newManager(t)

	_ = mgr.Add("Overdue task", "high", "2000-01-01")   // past, incomplete → overdue
	_ = mgr.Add("Future task", "medium", "2099-01-01")  // future, incomplete → not overdue
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

// ---------------------------------------------------------------------------
// Clear tests
// ---------------------------------------------------------------------------

// TestClear_RemovesOnlyCompleted verifies that Clear removes only tasks marked
// as done, leaving pending tasks intact, and returns the correct cleared count.
func TestClear_RemovesOnlyCompleted(t *testing.T) {
	mgr := newManager(t)

	_ = mgr.Add("Task 1", "high", "")
	_ = mgr.Add("Task 2", "medium", "")
	_ = mgr.Add("Task 3", "low", "")
	_ = mgr.Add("Task 4", "high", "")

	// Complete tasks 1 and 3 (indices 0 and 2).
	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if err := mgr.Complete(tasks[0].ID); err != nil {
		t.Fatalf("Complete task 1: %v", err)
	}
	if err := mgr.Complete(tasks[2].ID); err != nil {
		t.Fatalf("Complete task 3: %v", err)
	}

	cleared, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}
	if cleared != 2 {
		t.Errorf("cleared count: expected 2, got %d", cleared)
	}

	remaining, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("remaining count: expected 2, got %d", len(remaining))
	}
	for _, task := range remaining {
		if task.Done {
			t.Errorf("task %d (%q) is done but should have been cleared", task.ID, task.Title)
		}
	}
}

// TestClear_EmptyStore verifies that Clear on an empty store returns no error
// and a cleared count of 0.
func TestClear_EmptyStore(t *testing.T) {
	mgr := newManager(t)

	cleared, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear on empty store: unexpected error: %v", err)
	}
	if cleared != 0 {
		t.Errorf("cleared count on empty store: expected 0, got %d", cleared)
	}
}

// TestClear_NoneCompleted verifies that Clear leaves all tasks intact when
// none are marked as done, and returns a cleared count of 0.
func TestClear_NoneCompleted(t *testing.T) {
	mgr := newManager(t)

	_ = mgr.Add("Task A", "high", "")
	_ = mgr.Add("Task B", "medium", "")
	_ = mgr.Add("Task C", "low", "")

	cleared, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}
	if cleared != 0 {
		t.Errorf("cleared count: expected 0, got %d", cleared)
	}

	remaining, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(remaining) != 3 {
		t.Errorf("remaining count: expected 3, got %d", len(remaining))
	}
}

// TestClear_AllCompleted verifies that Clear removes all tasks when every task
// is marked as done, leaving an empty store.
func TestClear_AllCompleted(t *testing.T) {
	mgr := newManager(t)

	_ = mgr.Add("Task X", "high", "")
	_ = mgr.Add("Task Y", "medium", "")

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	for _, task := range tasks {
		if err := mgr.Complete(task.ID); err != nil {
			t.Fatalf("Complete task %d: %v", task.ID, err)
		}
	}

	cleared, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}
	if cleared != 2 {
		t.Errorf("cleared count: expected 2, got %d", cleared)
	}

	remaining, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("remaining count: expected 0, got %d", len(remaining))
	}
}
