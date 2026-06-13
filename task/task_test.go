package task

import (
	"os"
	"path/filepath"
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
	_ = mgr.Add("No date task", "medium", "")

	tasks, err := mgr.List("", true)
	if err != nil {
		t.Fatalf("List overdue: unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 overdue task, got %d", len(tasks))
	}
	if tasks[0].Title != "Overdue task" {
		t.Errorf("expected overdue task title %q, got %q", "Overdue task", tasks[0].Title)
	}
}

// ---------------------------------------------------------------------------
// Delete error path tests
// ---------------------------------------------------------------------------

// TestDeleteNonExistentTask verifies that deleting a task ID that does not
// exist returns an error.
func TestDeleteNonExistentTask(t *testing.T) {
	mgr := newManager(t)
	err := mgr.Delete(9999)
	if err == nil {
		t.Fatal("Delete of non-existent task: expected error, got nil")
	}
}

// TestCompleteNonExistentTask verifies that completing a task ID that does not
// exist returns an error.
func TestCompleteNonExistentTask(t *testing.T) {
	mgr := newManager(t)
	err := mgr.Complete(9999)
	if err == nil {
		t.Fatal("Complete of non-existent task: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Clear tests
// ---------------------------------------------------------------------------

// TestClearEmpty verifies that Clear on an empty task list returns (0, nil).
func TestClearEmpty(t *testing.T) {
	mgr := newManager(t)
	cleared, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear on empty list: unexpected error: %v", err)
	}
	if cleared != 0 {
		t.Errorf("Clear on empty list: expected 0 cleared, got %d", cleared)
	}
}

// TestClearNoDoneTasks verifies that Clear with no completed tasks returns
// (0, nil) and leaves all pending tasks intact.
func TestClearNoDoneTasks(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Pending A", "low", "")
	_ = mgr.Add("Pending B", "medium", "")

	cleared, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear with no done tasks: unexpected error: %v", err)
	}
	if cleared != 0 {
		t.Errorf("Clear with no done tasks: expected 0 cleared, got %d", cleared)
	}

	// Both pending tasks must still be present.
	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("Clear with no done tasks: expected 2 remaining tasks, got %d", len(tasks))
	}
}

// TestClearSomeDoneTasks verifies that Clear removes all done tasks and returns
// the correct cleared count, leaving pending tasks untouched.
func TestClearSomeDoneTasks(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Keep me", "low", "")
	_ = mgr.Add("Remove A", "medium", "")
	_ = mgr.Add("Remove B", "high", "")

	tasks, _ := mgr.List("", false)
	// Mark the second and third tasks as done.
	_ = mgr.Complete(tasks[1].ID)
	_ = mgr.Complete(tasks[2].ID)

	cleared, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}
	if cleared != 2 {
		t.Errorf("Clear: expected 2 cleared, got %d", cleared)
	}

	// Only the pending task should remain.
	remaining, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("Clear: expected 1 remaining task, got %d", len(remaining))
	}
	if remaining[0].Title != "Keep me" {
		t.Errorf("Clear: expected remaining task title %q, got %q", "Keep me", remaining[0].Title)
	}
}

// TestClearAllDoneTasks verifies that clearing a list where every task is done
// removes all tasks and returns the correct count.
func TestClearAllDoneTasks(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Done A", "low", "")
	_ = mgr.Add("Done B", "medium", "")

	tasks, _ := mgr.List("", false)
	for _, task := range tasks {
		_ = mgr.Complete(task.ID)
	}

	cleared, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear all done: unexpected error: %v", err)
	}
	if cleared != 2 {
		t.Errorf("Clear all done: expected 2 cleared, got %d", cleared)
	}

	remaining, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear all done: unexpected error: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("Clear all done: expected 0 remaining tasks, got %d", len(remaining))
	}
}

// TestClearPendingTasksUntouched verifies that Clear only removes done tasks
// and does not alter the content or order of pending tasks.
func TestClearPendingTasksUntouched(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Alpha", "high", "")
	_ = mgr.Add("Beta", "low", "")
	_ = mgr.Add("Gamma", "medium", "")

	tasks, _ := mgr.List("", false)
	// Mark only the middle task as done.
	_ = mgr.Complete(tasks[1].ID)

	_, err := mgr.Clear()
	if err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}

	remaining, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after Clear: unexpected error: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("Clear: expected 2 remaining tasks, got %d", len(remaining))
	}

	titles := make([]string, len(remaining))
	for i, task := range remaining {
		titles[i] = task.Title
	}
	for _, expected := range []string{"Alpha", "Gamma"} {
		found := false
		for _, title := range titles {
			if title == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Clear: expected task %q to remain, but it was not found in %v", expected, titles)
		}
	}
}

// TestClearErrorOnUnreadableFile verifies that Clear propagates an error when
// the backing file is unreadable (e.g., after permissions are revoked).
//
// NOTE: Root bypasses POSIX file-permission checks (common in Docker/CI), so
// this test is skipped when running as root.
func TestClearErrorOnUnreadableFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root to enforce file permissions; skipping when running as root")
	}

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

// ---------------------------------------------------------------------------
// Path validation tests (additional)
// ---------------------------------------------------------------------------

// TestListInvalidPriority verifies that List rejects an invalid priority filter.
func TestListInvalidPriority(t *testing.T) {
	mgr := newManager(t)
	_, err := mgr.List("invalid", false)
	if err == nil {
		t.Fatal("List with invalid priority: expected error, got nil")
	}
}

// TestAddTitleValidation verifies that Add rejects an empty title.
func TestAddTitleValidation(t *testing.T) {
	mgr := newManager(t)
	err := mgr.Add("", "medium", "")
	if err == nil {
		t.Fatal("Add with empty title: expected error, got nil")
	}
}

// TestAddTitleWhitespaceOnly verifies that Add rejects a whitespace-only title.
func TestAddTitleWhitespaceOnly(t *testing.T) {
	mgr := newManager(t)
	err := mgr.Add("   ", "medium", "")
	if err == nil {
		t.Fatal("Add with whitespace-only title: expected error, got nil")
	}
}

// TestDeleteIDZeroOrNegative verifies that Delete rejects non-positive IDs.
func TestDeleteIDZeroOrNegative(t *testing.T) {
	mgr := newManager(t)
	for _, id := range []int{0, -1, -100} {
		err := mgr.Delete(id)
		if err == nil {
			t.Errorf("Delete(%d): expected error for non-positive ID, got nil", id)
		}
	}
}

// TestCompleteIDZeroOrNegative verifies that Complete rejects non-positive IDs.
func TestCompleteIDZeroOrNegative(t *testing.T) {
	mgr := newManager(t)
	for _, id := range []int{0, -1, -100} {
		err := mgr.Complete(id)
		if err == nil {
			t.Errorf("Complete(%d): expected error for non-positive ID, got nil", id)
		}
	}
}

// TestNewManagerPathWithDotDot verifies directory traversal rejection using
// strings that contain ".." somewhere in the path (not just as a prefix).
func TestNewManagerPathWithDotDot(t *testing.T) {
	paths := []string{
		"tasks/../../../etc/passwd",
		"foo/../../bar/tasks.json",
	}
	for _, p := range paths {
		_, err := NewManager(p)
		if err == nil {
			t.Errorf("NewManager(%q): expected traversal rejection, got nil", p)
		}
	}
}

// TestNewManagerEmptyPath verifies that an empty path is rejected.
func TestNewManagerEmptyPath(t *testing.T) {
	_, err := NewManager("")
	if err == nil {
		t.Fatal("NewManager with empty path: expected error, got nil")
	}
}

// TestNewManagerAbsPath verifies that an absolute path not containing ".." is
// accepted (used when the manager is constructed with a proper abs path).
func TestNewManagerAbsPath(t *testing.T) {
	dir := t.TempDir()
	absPath := filepath.Join(dir, "tasks.json")
	_, err := NewManager(absPath)
	if err != nil {
		t.Errorf("NewManager with abs path: unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Stats — priority bucket tests
// ---------------------------------------------------------------------------

func TestStatsPriorityBuckets(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("H1", "high", "")
	_ = mgr.Add("H2", "high", "")
	_ = mgr.Add("M1", "medium", "")
	_ = mgr.Add("L1", "low", "")

	s, err := mgr.Stats()
	if err != nil {
		t.Fatalf("Stats: unexpected error: %v", err)
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

// TestStatsOverdueCount verifies that Stats correctly counts overdue tasks.
func TestStatsOverdueCount(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Overdue", "low", "2000-01-01")
	_ = mgr.Add("Future", "medium", "2099-12-31")
	_ = mgr.Add("No date", "high", "")

	s, err := mgr.Stats()
	if err != nil {
		t.Fatalf("Stats: unexpected error: %v", err)
	}
	if s.Overdue != 1 {
		t.Errorf("Overdue: expected 1, got %d", s.Overdue)
	}
}

// ---------------------------------------------------------------------------
// List with both priority filter and overdue filter
// ---------------------------------------------------------------------------

func TestListPriorityAndOverdue(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("High overdue", "high", "2000-01-01")
	_ = mgr.Add("High future", "high", "2099-12-31")
	_ = mgr.Add("Low overdue", "low", "2000-01-01")

	tasks, err := mgr.List("high", true)
	if err != nil {
		t.Fatalf("List(high, overdue): unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Title != "High overdue" {
		t.Errorf("expected %q, got %q", "High overdue", tasks[0].Title)
	}
}

// ---------------------------------------------------------------------------
// ID uniqueness test
// ---------------------------------------------------------------------------

// TestAddIDsAreUnique verifies that successive Add calls produce unique IDs.
func TestAddIDsAreUnique(t *testing.T) {
	mgr := newManager(t)
	for i := 0; i < 5; i++ {
		if err := mgr.Add("Task", "medium", ""); err != nil {
			t.Fatalf("Add iteration %d: unexpected error: %v", i, err)
		}
	}
	tasks, _ := mgr.List("", false)
	seen := make(map[int]bool)
	for _, task := range tasks {
		if seen[task.ID] {
			t.Errorf("duplicate task ID: %d", task.ID)
		}
		seen[task.ID] = true
	}
}

// ---------------------------------------------------------------------------
// Title sanitisation
// ---------------------------------------------------------------------------

// TestAddTitleTrimmed verifies that leading/trailing whitespace in the title
// is preserved as-is at the library layer (trimming is a CLI responsibility).
func TestAddTitleTrimmed(t *testing.T) {
	mgr := newManager(t)
	// Library should accept a title with internal spaces but reject empty/blank.
	if err := mgr.Add("Hello World", "medium", ""); err != nil {
		t.Fatalf("Add with spaced title: unexpected error: %v", err)
	}
	tasks, _ := mgr.List("", false)
	if tasks[0].Title != "Hello World" {
		t.Errorf("expected title %q, got %q", "Hello World", tasks[0].Title)
	}
}

// ---------------------------------------------------------------------------
// Persistence round-trip test
// ---------------------------------------------------------------------------

// TestPersistenceRoundTrip verifies that tasks written by one Manager instance
// are readable by a new Manager instance pointing to the same file.
func TestPersistenceRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")

	mgr1, err := NewManager(path)
	if err != nil {
		t.Fatalf("NewManager (first): unexpected error: %v", err)
	}
	if err := mgr1.Add("Persisted task", "high", ""); err != nil {
		t.Fatalf("Add: unexpected error: %v", err)
	}

	mgr2, err := NewManager(path)
	if err != nil {
		t.Fatalf("NewManager (second): unexpected error: %v", err)
	}
	tasks, err := mgr2.List("", false)
	if err != nil {
		t.Fatalf("List on second manager: unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 persisted task, got %d", len(tasks))
	}
	if tasks[0].Title != "Persisted task" {
		t.Errorf("expected %q, got %q", "Persisted task", tasks[0].Title)
	}
}

// ---------------------------------------------------------------------------
// Whitespace title rejection at the strings.TrimSpace boundary
// ---------------------------------------------------------------------------

// TestListInvalidPriorityEmpty verifies "" is rejected.
func TestListEmptyPriorityFilter(t *testing.T) {
	mgr := newManager(t)
	// An empty priority filter ("") should mean "no filter" — not an error.
	_, err := mgr.List("", false)
	if err != nil {
		t.Errorf("List with empty priority filter: unexpected error: %v", err)
	}
}
