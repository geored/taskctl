package task

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// chdirTemp changes the working directory to a fresh temporary directory for
// the duration of the test and restores the original directory on cleanup.
// It returns the absolute path of the temporary directory.
//
// Only a small number of tests legitimately require chdirTemp: those that test
// CWD-relative path resolution (NewManager's relative-path validation) or
// symlink behaviour. All other tests should use newManager() instead, which
// does NOT mutate the global process working directory and is safe with
// t.Parallel().
func chdirTemp(t *testing.T) string {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("chdirTemp: Getwd: %v", err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdirTemp: Chdir(%q): %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("chdirTemp cleanup: Chdir(%q): %v", orig, err)
		}
	})
	return dir
}

// newManager is a test helper that creates a Manager backed by an isolated
// temporary file per test. It uses t.TempDir() to obtain a per-test directory
// and newManagerFromPath() to create the Manager directly from an absolute
// path, without touching the global process working directory. This makes it
// safe to call t.Parallel() in any test that uses newManager().
func newManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	return newManagerFromPath(filepath.Join(dir, "tasks.json"))
}

// ---------------------------------------------------------------------------
// NewManager path validation tests
// ---------------------------------------------------------------------------

// TestNewManagerAbsolutePathRejected verifies that absolute paths are rejected
// by NewManager with a clear error message.
func TestNewManagerAbsolutePathRejected(t *testing.T) {
	t.Parallel()
	paths := []string{
		"/tmp/evil.json",
		"/etc/shadow",
		"/",
	}
	for _, p := range paths {
		p := p // capture loop variable
		t.Run(p, func(t *testing.T) {
			t.Parallel()
			_, err := NewManager(p)
			if err == nil {
				t.Errorf("NewManager(%q): expected error for absolute path, got nil", p)
			}
			if err != nil && !strings.Contains(err.Error(), "must be a relative path") {
				t.Errorf("NewManager(%q): error should mention 'must be a relative path', got: %v", p, err)
			}
		})
	}
}

// TestNewManagerTraversalRejected verifies that paths beginning with ".." are
// rejected by NewManager.
func TestNewManagerTraversalRejected(t *testing.T) {
	t.Parallel()
	paths := []string{
		"../../etc/shadow",
		"../sibling/tasks.json",
		"..",
	}
	for _, p := range paths {
		p := p // capture loop variable
		t.Run(p, func(t *testing.T) {
			t.Parallel()
			_, err := NewManager(p)
			if err == nil {
				t.Errorf("NewManager(%q): expected error for traversal path, got nil", p)
			}
		})
	}
}

// TestNewManagerValidPath verifies that plain relative paths are accepted by
// NewManager — both a simple filename and a subdirectory path.
// NOTE: This test uses chdirTemp because NewManager's behaviour depends on the
// current working directory when resolving and validating relative paths.
func TestNewManagerValidPath(t *testing.T) {
	// Cannot be parallel: uses chdirTemp which mutates global process CWD.
	validPaths := []string{
		"tasks.json",
		"data/tasks.json",
	}
	for _, p := range validPaths {
		p := p // capture loop variable
		t.Run(p, func(t *testing.T) {
			// Cannot be parallel: uses chdirTemp which mutates global process CWD.
			chdirTemp(t)
			mgr, err := NewManager(p)
			if err != nil {
				t.Errorf("NewManager(%q): unexpected error for valid relative path: %v", p, err)
			}
			if mgr == nil {
				t.Errorf("NewManager(%q): expected non-nil *Manager, got nil", p)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Existing functionality tests (preserved + upgraded to t.TempDir())
// ---------------------------------------------------------------------------

func TestAdd(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	for _, priority := range []string{"high", "medium", "low"} {
		priority := priority // capture loop variable
		t.Run(priority, func(t *testing.T) {
			t.Parallel()
			mgr := newManager(t)
			if err := mgr.Add("Task", priority, ""); err != nil {
				t.Errorf("Add with priority %q: unexpected error: %v", priority, err)
			}
			tasks, _ := mgr.List("", false)
			if len(tasks) != 1 || tasks[0].Priority != priority {
				t.Errorf("priority %q: stored task has priority %q", priority, tasks[0].Priority)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Stats tests (existing, preserved)
// ---------------------------------------------------------------------------

func TestStatsEmpty(t *testing.T) {
	t.Parallel()
	mgr := newManager(t)
	s, err := mgr.Stats()
	if err != nil {
		t.Fatalf("Stats: unexpected error: %v", err)
	}
	if s.Total != 0 || s.Completed != 0 || s.Pending != 0 || s.Overdue != 0 {
		t.Errorf("expected all-zero Stats on empty store, got %+v", s)
	}
}

func TestStatsMixed(t *testing.T) {
	t.Parallel()
	mgr := newManager(t)
	_ = mgr.Add("Task A", "high", "")
	_ = mgr.Add("Task B", "medium", "")
	_ = mgr.Add("Task C", "low", "")

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
	if s.HighPriority != 1 {
		t.Errorf("HighPriority: expected 1, got %d", s.HighPriority)
	}
	if s.MediumPriority != 1 {
		t.Errorf("MediumPriority: expected 1, got %d", s.MediumPriority)
	}
	if s.LowPriority != 1 {
		t.Errorf("LowPriority: expected 1, got %d", s.LowPriority)
	}
}

func TestAddWithDueDate(t *testing.T) {
	t.Parallel()
	mgr := newManager(t)
	if err := mgr.Add("Due task", "high", "2099-12-31"); err != nil {
		t.Fatalf("Add with due date: unexpected error: %v", err)
	}
	tasks, _ := mgr.List("", false)
	if tasks[0].DueDate != "2099-12-31" {
		t.Errorf("expected DueDate %q, got %q", "2099-12-31", tasks[0].DueDate)
	}
}

func TestAddInvalidDueDate(t *testing.T) {
	t.Parallel()
	mgr := newManager(t)
	err := mgr.Add("Bad date", "medium", "not-a-date")
	if err == nil {
		t.Fatal("expected error for invalid due date, got nil")
	}
}

func TestAddNoDueDate(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

// TestIsOverdue_FutureDate verifies that a task with a future due date is not
// overdue.
func TestIsOverdue_FutureDate(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:      2,
		Title:   "Future task",
		Done:    false,
		DueDate: "2099-12-31",
	}
	now := time.Now()
	if task.IsOverdue(now) {
		t.Error("expected task with future due date NOT to be overdue")
	}
}

// TestIsOverdue_DoneTask verifies that a completed task is never considered
// overdue even if its due date has passed.
func TestIsOverdue_DoneTask(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:      3,
		Title:   "Done task",
		Done:    true,
		DueDate: "2000-01-01",
	}
	now := time.Now()
	if task.IsOverdue(now) {
		t.Error("expected completed task NOT to be overdue")
	}
}

// TestIsOverdue_NoDueDate verifies that a task without a due date is never
// considered overdue.
func TestIsOverdue_NoDueDate(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:    4,
		Title: "No due date",
		Done:  false,
	}
	now := time.Now()
	if task.IsOverdue(now) {
		t.Error("expected task with no due date NOT to be overdue")
	}
}

// TestIsOverdue_TodayNotOverdue verifies that a task due today (in UTC) is
// never flagged as overdue, regardless of the local timezone. This guards
// against the Truncate(24h) UTC-midnight bug (Fixes #58).
func TestIsOverdue_TodayNotOverdue(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	todayStr := now.Format(dateLayout)
	task := Task{
		ID:      5,
		Title:   "Due today",
		Done:    false,
		DueDate: todayStr,
	}
	// Pass a now value that is in a far-ahead timezone to stress timezone handling.
	loc := time.FixedZone("UTC+14", 14*60*60) // UTC+14 is the furthest ahead
	nowLocal := now.In(loc)
	if task.IsOverdue(nowLocal) {
		t.Errorf("task due today (%s) should NOT be overdue; now=%v", todayStr, nowLocal)
	}
}

// TestIsOverdue_YesterdayIsOverdue verifies that a task due yesterday is always
// flagged as overdue (Fixes #58).
func TestIsOverdue_YesterdayIsOverdue(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1)
	yesterdayStr := yesterday.Format(dateLayout)
	task := Task{
		ID:      6,
		Title:   "Due yesterday",
		Done:    false,
		DueDate: yesterdayStr,
	}
	if !task.IsOverdue(now) {
		t.Errorf("task due yesterday (%s) should be overdue", yesterdayStr)
	}
}

func TestListOverdueFilter(t *testing.T) {
	t.Parallel()
	mgr := newManager(t)
	_ = mgr.Add("Overdue task", "high", "2000-01-01")
	_ = mgr.Add("Future task", "low", "2099-01-01")
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
// Error-path tests
// ---------------------------------------------------------------------------

// TestCompleteNonExistentID verifies that Complete returns an error when the
// given ID does not exist in the task store.
func TestCompleteNonExistentID(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
		tc := tc // capture loop variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := mgr.Add(tc.title, "medium", "")
			if err == nil {
				t.Errorf("Add with title %q: expected error, got nil", tc.title)
			}
		})
	}
}

// TestListInvalidPriority verifies that List returns an error when called with
// an unrecognised priority string (library-boundary validation).
func TestListInvalidPriority(t *testing.T) {
	t.Parallel()
	mgr := newManager(t)
	_ = mgr.Add("A task", "medium", "")

	invalidPriorities := []string{"urgent", "critical", "HIGH", "Low", "MEDIUM", "none"}
	for _, p := range invalidPriorities {
		p := p // capture loop variable
		t.Run(p, func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()
	mgr := newManager(t)
	_ = mgr.Add("Task A", "high", "")
	_ = mgr.Add("Task B", "medium", "")
	_ = mgr.Add("Task C", "low", "")

	validPriorities := []string{"", "high", "medium", "low"}
	for _, p := range validPriorities {
		p := p // capture loop variable
		t.Run("priority="+p, func(t *testing.T) {
			t.Parallel()
			tasks, err := mgr.List(p, false)
			if err != nil {
				t.Errorf("List(%q): unexpected error: %v", p, err)
			}
			if tasks == nil {
				t.Errorf("List(%q): expected non-nil task slice, got nil", p)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Clear tests
// ---------------------------------------------------------------------------

// TestClear_MixedTasks verifies that Clear removes all completed tasks and
// leaves pending ones, returning correct cleared/remaining counts.
func TestClear_MixedTasks(t *testing.T) {
	t.Parallel()
	mgr := newManager(t)
	// Add 5 tasks, mark 3 done.
	_ = mgr.Add("Task 1", "high", "")
	_ = mgr.Add("Task 2", "medium", "")
	_ = mgr.Add("Task 3", "low", "")
	_ = mgr.Add("Task 4", "high", "")
	_ = mgr.Add("Task 5", "low", "")

	all, _ := mgr.List("", false)
	_ = mgr.Complete(all[0].ID)
	_ = mgr.Complete(all[2].ID)
	_ = mgr.Complete(all[4].ID)

	cleared, remaining, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}
	if cleared != 3 {
		t.Errorf("cleared: expected 3, got %d", cleared)
	}
	if remaining != 2 {
		t.Errorf("remaining: expected 2, got %d", remaining)
	}

	// Verify only pending tasks remain and none are done.
	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks after Clear, got %d", len(tasks))
	}
	for _, task := range tasks {
		if task.Done {
			t.Errorf("task %d (%q) should have been cleared but still exists as done", task.ID, task.Title)
		}
	}
}

// TestClear_AllDone verifies that Clear with all tasks done returns cleared=N,
// remaining=0, and leaves an empty task list.
func TestClear_AllDone(t *testing.T) {
	t.Parallel()
	mgr := newManager(t)
	_ = mgr.Add("Task A", "high", "")
	_ = mgr.Add("Task B", "medium", "")
	_ = mgr.Add("Task C", "low", "")

	all, _ := mgr.List("", false)
	for _, task := range all {
		_ = mgr.Complete(task.ID)
	}

	cleared, remaining, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}
	if cleared != 3 {
		t.Errorf("cleared: expected 3, got %d", cleared)
	}
	if remaining != 0 {
		t.Errorf("remaining: expected 0, got %d", remaining)
	}

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected empty task list after clearing all done tasks, got %d", len(tasks))
	}
}

// TestClear_NoneDone verifies that Clear with no completed tasks returns
// cleared=0, remaining=N, and leaves the task list unchanged.
func TestClear_NoneDone(t *testing.T) {
	t.Parallel()
	mgr := newManager(t)
	_ = mgr.Add("Task A", "high", "")
	_ = mgr.Add("Task B", "medium", "")
	_ = mgr.Add("Task C", "low", "")

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

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks to remain after Clear with none done, got %d", len(tasks))
	}
}

// TestClear_EmptyStore verifies that Clear on an empty store returns
// cleared=0, remaining=0 without error.
func TestClear_EmptyStore(t *testing.T) {
	t.Parallel()
	mgr := newManager(t)

	cleared, remaining, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear on empty store: unexpected error: %v", err)
	}
	if cleared != 0 {
		t.Errorf("cleared: expected 0 on empty store, got %d", cleared)
	}
	if remaining != 0 {
		t.Errorf("remaining: expected 0 on empty store, got %d", remaining)
	}
}

// TestClear_CorruptedFile verifies that Clear returns a non-nil error when the
// backing file is corrupt (invalid JSON) and does NOT modify the store.
//
// NOTE: This test uses chdirTemp because it must construct a Manager via
// NewManager (which requires a relative path), so the CWD must be set to the
// temp directory. It cannot safely call t.Parallel().
func TestClear_CorruptedFile(t *testing.T) {
	// Cannot be parallel: uses chdirTemp which mutates global process CWD.
	dir := chdirTemp(t)
	filePath := filepath.Join(dir, "tasks.json")
	relPath := "tasks.json"

	// Write corrupt JSON directly to the absolute path so we can control content.
	if err := os.WriteFile(filePath, []byte("not-valid-json{{{"), 0600); err != nil {
		t.Fatalf("WriteFile: unexpected error: %v", err)
	}

	mgr, err := NewManager(relPath)
	if err != nil {
		t.Fatalf("NewManager: unexpected error: %v", err)
	}

	cleared, remaining, err := mgr.Clear()
	if err == nil {
		t.Fatalf("Clear on corrupted file: expected error, got nil (cleared=%d, remaining=%d)", cleared, remaining)
	}

	// State must be unmodified — the corrupt content should still be there.
	content, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("ReadFile after failed Clear: unexpected error: %v", readErr)
	}
	if string(content) != "not-valid-json{{{" {
		t.Errorf("file content was modified despite Clear failing; got: %s", content)
	}
}

// ---------------------------------------------------------------------------
// New tests for bug fixes (Issues #58, #66, #67, #68, #69, #70)
// ---------------------------------------------------------------------------

// TestAddIDOverflow verifies that Add returns an error when the maximum existing
// task ID is math.MaxInt, preventing integer overflow (Fixes #70).
func TestAddIDOverflow(t *testing.T) {
	t.Parallel()
	mgr := newManager(t)

	// Manually write a task file containing a task with ID = math.MaxInt.
	maxTasks := []Task{
		{
			ID:       math.MaxInt,
			Title:    "Max ID task",
			Done:     false,
			Priority: "low",
		},
	}
	data, err := json.MarshalIndent(maxTasks, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent: unexpected error: %v", err)
	}
	if err := os.WriteFile(mgr.filePath, data, 0600); err != nil {
		t.Fatalf("WriteFile: unexpected error: %v", err)
	}

	// Attempting to add another task should return an overflow error.
	err = mgr.Add("Another task", "low", "")
	if err == nil {
		t.Fatal("Add with max ID task: expected overflow error, got nil")
	}
	if !strings.Contains(err.Error(), "overflow") {
		t.Errorf("expected error message to mention 'overflow', got: %v", err)
	}
}

// TestAddUnicodeTitleAtLimit verifies that a title consisting of exactly
// maxTitleLength emoji runes (which are 4 bytes each) is accepted without
// error (Fixes #69).
func TestAddUnicodeTitleAtLimit(t *testing.T) {
	t.Parallel()
	mgr := newManager(t)

	// Build a title of exactly maxTitleLength emoji runes.
	// Each emoji is 4 bytes in UTF-8, so the byte length will be 4×maxTitleLength.
	emoji := "😀"
	title := strings.Repeat(emoji, maxTitleLength)

	if err := mgr.Add(title, "low", ""); err != nil {
		t.Errorf("Add with %d emoji runes (maxTitleLength): unexpected error: %v", maxTitleLength, err)
	}
}

// TestAddUnicodeTitleOverLimit verifies that a title exceeding maxTitleLength
// runes is rejected (Fixes #69).
func TestAddUnicodeTitleOverLimit(t *testing.T) {
	t.Parallel()
	mgr := newManager(t)

	emoji := "😀"
	title := strings.Repeat(emoji, maxTitleLength+1)

	err := mgr.Add(title, "low", "")
	if err == nil {
		t.Errorf("Add with %d emoji runes (over limit): expected error, got nil", maxTitleLength+1)
	}
}

// TestClear_NoOpWhenNoneDone verifies that calling Clear() when no tasks are
// done does not write to disk (Fixes #66).
func TestClear_NoOpWhenNoneDone(t *testing.T) {
	t.Parallel()
	mgr := newManager(t)
	_ = mgr.Add("Task A", "high", "")
	_ = mgr.Add("Task B", "medium", "")

	// Record file mtime before Clear.
	infoBefore, err := os.Stat(mgr.filePath)
	if err != nil {
		t.Fatalf("Stat before Clear: %v", err)
	}
	mtimeBefore := infoBefore.ModTime()

	// Small sleep to ensure mtime would differ if the file was written.
	time.Sleep(10 * time.Millisecond)

	cleared, remaining, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}
	if cleared != 0 {
		t.Errorf("cleared: expected 0, got %d", cleared)
	}
	if remaining != 2 {
		t.Errorf("remaining: expected 2, got %d", remaining)
	}

	// Verify mtime has NOT changed — no write occurred.
	infoAfter, err := os.Stat(mgr.filePath)
	if err != nil {
		t.Fatalf("Stat after Clear: %v", err)
	}
	if !infoAfter.ModTime().Equal(mtimeBefore) {
		t.Errorf("file mtime changed after Clear with no done tasks: before=%v after=%v",
			mtimeBefore, infoAfter.ModTime())
	}
}

// ---------------------------------------------------------------------------
// Symlink-based traversal tests (Issue #87)
// ---------------------------------------------------------------------------

// TestNewManagerSymlinkTraversalRejected verifies that NewManager rejects a
// relative path that passes the ".." prefix check but resolves — via a
// symbolic link — to a location outside the current working directory.
// This is the defense-in-depth EvalSymlinks check (Fixes #87).
//
// NOTE: This test uses chdirTemp because it tests NewManager's CWD-relative
// symlink resolution logic, which inherently requires controlling the CWD.
// It cannot safely call t.Parallel().
func TestNewManagerSymlinkTraversalRejected(t *testing.T) {
	// Cannot be parallel: uses chdirTemp which mutates global process CWD.
	// Create a fresh working directory and cd into it.
	chdirTemp(t)

	// Create a real "outside" target directory to point the symlink at.
	outside := t.TempDir()

	// Create a symlink inside cwd that points outside cwd.
	if err := os.Symlink(outside, "escape_link"); err != nil {
		t.Fatalf("Symlink: unexpected error: %v", err)
	}

	// "escape_link/tasks.json" passes the existing ".." and IsAbs checks,
	// but should be rejected because escape_link resolves outside cwd.
	_, err := NewManager("escape_link/tasks.json")
	if err == nil {
		t.Fatal("NewManager(\"escape_link/tasks.json\") with symlink pointing outside cwd: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "traversal") {
		t.Errorf("expected error message to mention 'traversal', got: %v", err)
	}
}

// TestNewManagerSymlinkWithinCwdAccepted verifies that a symlink pointing to a
// location *inside* the current working directory is accepted by NewManager.
//
// NOTE: This test uses chdirTemp because it tests NewManager's CWD-relative
// symlink resolution logic, which inherently requires controlling the CWD.
// It cannot safely call t.Parallel().
func TestNewManagerSymlinkWithinCwdAccepted(t *testing.T) {
	// Cannot be parallel: uses chdirTemp which mutates global process CWD.
	// Create a fresh working directory and cd into it.
	chdirTemp(t)

	// Create a real subdirectory inside cwd.
	if err := os.Mkdir("subdir", 0750); err != nil {
		t.Fatalf("Mkdir: unexpected error: %v", err)
	}

	// Create a symlink inside cwd that points to the subdir (also inside cwd).
	if err := os.Symlink("subdir", "safe_link"); err != nil {
		t.Fatalf("Symlink: unexpected error: %v", err)
	}

	// "safe_link/tasks.json" should be accepted — symlink resolves within cwd.
	mgr, err := NewManager("safe_link/tasks.json")
	if err != nil {
		t.Errorf("NewManager(\"safe_link/tasks.json\") with symlink pointing inside cwd: unexpected error: %v", err)
	}
	if mgr == nil {
		t.Error("expected non-nil *Manager, got nil")
	}
}
