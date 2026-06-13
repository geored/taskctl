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

// ---------------------------------------------------------------------------
// Add tests
// ---------------------------------------------------------------------------

// TestAddSingle verifies that adding a single task persists it correctly.
func TestAddSingle(t *testing.T) {
	mgr := newManager(t)
	if err := mgr.Add("Buy groceries", "medium", ""); err != nil {
		t.Fatalf("Add: unexpected error: %v", err)
	}

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Title != "Buy groceries" {
		t.Errorf("expected title %q, got %q", "Buy groceries", tasks[0].Title)
	}
	if tasks[0].Priority != "medium" {
		t.Errorf("expected priority %q, got %q", "medium", tasks[0].Priority)
	}
	if tasks[0].Done {
		t.Errorf("expected Done=false, got true")
	}
}

// TestAddMultiple verifies that IDs are assigned sequentially across multiple
// Add calls and that all tasks are persisted.
func TestAddMultiple(t *testing.T) {
	mgr := newManager(t)
	titles := []string{"Alpha", "Beta", "Gamma"}
	for _, title := range titles {
		if err := mgr.Add(title, "low", ""); err != nil {
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
	for i, title := range titles {
		if tasks[i].Title != title {
			t.Errorf("task %d: expected title %q, got %q", i, title, tasks[i].Title)
		}
		if tasks[i].ID != i+1 {
			t.Errorf("task %d: expected ID=%d, got %d", i, i+1, tasks[i].ID)
		}
	}
}

// TestAddWithDueDate verifies that a task added with a due date is persisted
// correctly.
func TestAddWithDueDate(t *testing.T) {
	mgr := newManager(t)
	if err := mgr.Add("Due task", "high", "2099-12-31"); err != nil {
		t.Fatalf("Add: unexpected error: %v", err)
	}

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].DueDate != "2099-12-31" {
		t.Errorf("expected DueDate %q, got %q", "2099-12-31", tasks[0].DueDate)
	}
}

// TestAddInvalidDueDate verifies that an invalid due date format is rejected.
func TestAddInvalidDueDate(t *testing.T) {
	mgr := newManager(t)
	cases := []string{"31-12-2099", "2099/12/31", "tomorrow", "now", "not-a-date"}
	for _, d := range cases {
		t.Run(d, func(t *testing.T) {
			err := mgr.Add("Task", "medium", d)
			if err == nil {
				t.Errorf("Add with due=%q: expected error, got nil", d)
			}
		})
	}
}

// TestAddInvalidPriority verifies that an invalid priority is rejected.
func TestAddInvalidPriority(t *testing.T) {
	mgr := newManager(t)
	invalidPriorities := []string{"urgent", "critical", "HIGH", "Med", "none", "0"}
	for _, p := range invalidPriorities {
		t.Run(p, func(t *testing.T) {
			err := mgr.Add("Task", p, "")
			if err == nil {
				t.Errorf("Add with priority=%q: expected error, got nil", p)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Complete tests
// ---------------------------------------------------------------------------

// TestComplete verifies that marking a task done persists the state.
func TestComplete(t *testing.T) {
	mgr := newManager(t)
	if err := mgr.Add("Do laundry", "low", ""); err != nil {
		t.Fatalf("Add: unexpected error: %v", err)
	}

	tasks, _ := mgr.List("", false)
	id := tasks[0].ID

	if err := mgr.Complete(id); err != nil {
		t.Fatalf("Complete(%d): unexpected error: %v", id, err)
	}

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task after Complete, got %d", len(tasks))
	}
	if !tasks[0].Done {
		t.Errorf("expected Done=true after Complete, got false")
	}
}

// ---------------------------------------------------------------------------
// Delete tests
// ---------------------------------------------------------------------------

// TestDelete verifies that deleting a task removes it from the store.
func TestDelete(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task A", "high", "")
	_ = mgr.Add("Task B", "medium", "")

	tasks, _ := mgr.List("", false)
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks before Delete, got %d", len(tasks))
	}

	idToDelete := tasks[0].ID
	if err := mgr.Delete(idToDelete); err != nil {
		t.Fatalf("Delete(%d): unexpected error: %v", idToDelete, err)
	}

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Delete: unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task after Delete, got %d", len(tasks))
	}
	if tasks[0].ID == idToDelete {
		t.Errorf("deleted task ID=%d still present", idToDelete)
	}
}

// ---------------------------------------------------------------------------
// List / filter tests
// ---------------------------------------------------------------------------

// TestListByPriority verifies that priority filtering works correctly.
func TestListByPriority(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("High task", "high", "")
	_ = mgr.Add("Medium task", "medium", "")
	_ = mgr.Add("Low task", "low", "")

	for _, tc := range []struct {
		priority string
		want     int
	}{
		{"high", 1},
		{"medium", 1},
		{"low", 1},
		{"", 3},
	} {
		tasks, err := mgr.List(tc.priority, false)
		if err != nil {
			t.Fatalf("List(%q): unexpected error: %v", tc.priority, err)
		}
		if len(tasks) != tc.want {
			t.Errorf("List(%q): expected %d tasks, got %d", tc.priority, tc.want, len(tasks))
		}
	}
}

// TestListOverdue verifies that the overdue filter returns only tasks with a
// past due date that are not yet done.
func TestListOverdue(t *testing.T) {
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
// IsOverdue tests
// ---------------------------------------------------------------------------

// TestIsOverdue validates the IsOverdue helper directly.
func TestIsOverdue(t *testing.T) {
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		task    Task
		want    bool
	}{
		{
			name: "past due date, not done",
			task: Task{Done: false, DueDate: "2024-06-14"},
			want: true,
		},
		{
			name: "due today, not done",
			task: Task{Done: false, DueDate: "2024-06-15"},
			want: false,
		},
		{
			name: "future due date, not done",
			task: Task{Done: false, DueDate: "2024-06-16"},
			want: false,
		},
		{
			name: "past due date, done",
			task: Task{Done: true, DueDate: "2024-06-14"},
			want: false,
		},
		{
			name: "no due date, not done",
			task: Task{Done: false, DueDate: ""},
			want: false,
		},
		{
			name: "invalid due date format",
			task: Task{Done: false, DueDate: "not-a-date"},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.task.IsOverdue(now)
			if got != tc.want {
				t.Errorf("IsOverdue() = %v, want %v", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Clear tests
// ---------------------------------------------------------------------------

// TestClear_NothingToClear verifies that Clear on a store with only pending
// tasks returns cleared=0 and remaining=N without modifying the tasks.
func TestClear_NothingToClear(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Pending 1", "high", "")
	_ = mgr.Add("Pending 2", "medium", "")
	_ = mgr.Add("Pending 3", "low", "")

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

	// Verify tasks are still present in the store.
	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(tasks) != 3 {
		t.Errorf("task count after Clear: expected 3, got %d", len(tasks))
	}
}

// TestClear_AllDone verifies that Clear removes all tasks when every task is
// done, returning cleared=N and remaining=0 with an empty store.
func TestClear_AllDone(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task A", "high", "")
	_ = mgr.Add("Task B", "medium", "")

	// Mark all tasks as done.
	tasks, _ := mgr.List("", false)
	for _, task := range tasks {
		if err := mgr.Complete(task.ID); err != nil {
			t.Fatalf("Complete(%d): unexpected error: %v", task.ID, err)
		}
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

	// Verify the store is now empty.
	tasks, err = mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("task count after Clear: expected 0, got %d", len(tasks))
	}
}

// TestClear_Mixed verifies that Clear removes only done tasks and preserves
// pending tasks, returning correct counts.
func TestClear_Mixed(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Pending 1", "high", "")
	_ = mgr.Add("Done 1", "medium", "")
	_ = mgr.Add("Pending 2", "low", "")
	_ = mgr.Add("Done 2", "high", "")
	_ = mgr.Add("Pending 3", "medium", "")

	// Mark tasks at index 1 and 3 (Done 1, Done 2) as complete.
	allTasks, _ := mgr.List("", false)
	_ = mgr.Complete(allTasks[1].ID) // Done 1
	_ = mgr.Complete(allTasks[3].ID) // Done 2

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

	// Verify only the pending tasks remain and none are done.
	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(tasks) != 3 {
		t.Errorf("task count after Clear: expected 3, got %d", len(tasks))
	}
	for _, task := range tasks {
		if task.Done {
			t.Errorf("task %d (%q) is Done after Clear — should have been removed", task.ID, task.Title)
		}
	}
}

// TestClear_EmptyStore verifies that Clear on an empty store (no file on disk)
// returns cleared=0, remaining=0, and no error.
func TestClear_EmptyStore(t *testing.T) {
	mgr := newManager(t)
	// Do NOT add any tasks — the file does not exist on disk yet.

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
