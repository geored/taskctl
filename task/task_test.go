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
// Add tests
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

// TestAddEmptyTitle verifies that Add rejects an empty title.
func TestAddEmptyTitle(t *testing.T) {
	mgr := newManager(t)
	err := mgr.Add("", "medium", "")
	if err == nil {
		t.Fatal("Add with empty title: expected error, got nil")
	}
}

// TestAddWhiteSpaceOnlyTitle verifies that a whitespace-only title is rejected.
func TestAddWhiteSpaceOnlyTitle(t *testing.T) {
	mgr := newManager(t)
	err := mgr.Add("   ", "medium", "")
	if err == nil {
		t.Fatal("Add with whitespace-only title: expected error, got nil")
	}
}

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
// stored correctly. Covers requirement #1 (all three priority levels).
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
// Due date tests
// ---------------------------------------------------------------------------

// TestAddWithDueDate verifies that a task with a valid due date is stored
// correctly. Covers requirement #2 (task creation with a valid due date).
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

// TestAddInvalidDueDate verifies that Add rejects malformed date strings.
// Covers requirement #4 (invalid due date format must return error).
func TestAddInvalidDueDate(t *testing.T) {
	mgr := newManager(t)
	invalidDates := []string{"not-a-date", "2025/01/01", "01-01-2025", "2025-13-01", "2025-00-00"}
	for _, d := range invalidDates {
		err := mgr.Add("Bad date task", "low", d)
		if err == nil {
			t.Errorf("Add with due date %q: expected error, got nil", d)
		}
	}
}

// TestAddNoDueDate verifies that a task without a due date stores an empty
// DueDate. Covers requirement #3 (task creation without a due date).
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

// ---------------------------------------------------------------------------
// List tests
// ---------------------------------------------------------------------------

// TestList verifies unfiltered listing. Covers requirement #5.
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

// TestListFilterByPriority verifies filtering by high priority. Covers part of
// requirement #6 (filtering by priority).
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

// TestListFilterByMediumPriority verifies filtering by medium priority.
// Covers requirement #6 (all three priority filter cases).
func TestListFilterByMediumPriority(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("High task", "high", "")
	_ = mgr.Add("Low task", "low", "")
	_ = mgr.Add("Medium task", "medium", "")

	tasks, err := mgr.List("medium", false)
	if err != nil {
		t.Fatalf("List medium: unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 medium-priority task, got %d", len(tasks))
	}
	if tasks[0].Priority != "medium" {
		t.Errorf("expected priority %q, got %q", "medium", tasks[0].Priority)
	}
}

// TestListFilterByLowPriority verifies filtering by low priority.
// Covers requirement #6 (all three priority filter cases).
func TestListFilterByLowPriority(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("High task", "high", "")
	_ = mgr.Add("Low task", "low", "")
	_ = mgr.Add("Medium task", "medium", "")

	tasks, err := mgr.List("low", false)
	if err != nil {
		t.Fatalf("List low: unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 low-priority task, got %d", len(tasks))
	}
	if tasks[0].Priority != "low" {
		t.Errorf("expected priority %q, got %q", "low", tasks[0].Priority)
	}
}

// TestListInvalidPriority verifies that List returns an error for an
// unrecognised priority filter (library-boundary validation).
func TestListInvalidPriority(t *testing.T) {
	mgr := newManager(t)
	_, err := mgr.List("critical", false)
	if err == nil {
		t.Fatal("List with invalid priority: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// IsOverdue unit tests
// ---------------------------------------------------------------------------

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

// TestIsOverdue_FutureDate verifies that a task due in the future is not overdue.
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
// if its due date has passed.
func TestIsOverdue_DoneTask(t *testing.T) {
	task := Task{
		ID:      3,
		Title:   "Done old task",
		Done:    true,
		DueDate: "2000-01-01",
	}
	now := time.Now()
	if task.IsOverdue(now) {
		t.Error("expected completed task to never be overdue")
	}
}

// TestIsOverdue_NoDueDate verifies that a task with no due date is never overdue.
func TestIsOverdue_NoDueDate(t *testing.T) {
	task := Task{
		ID:      4,
		Title:   "No due date",
		Done:    false,
		DueDate: "",
	}
	now := time.Now()
	if task.IsOverdue(now) {
		t.Error("expected task with no due date to never be overdue")
	}
}

// TestIsOverdue_BadDueDateString verifies that a task with a malformed due date
// stored on the struct is treated as not overdue (defensive path).
func TestIsOverdue_BadDueDateString(t *testing.T) {
	task := Task{
		ID:      5,
		Title:   "Corrupt date",
		Done:    false,
		DueDate: "not-a-date",
	}
	now := time.Now()
	if task.IsOverdue(now) {
		t.Error("expected task with corrupt due date string to not be overdue")
	}
}

// TestIsOverdue_DueToday verifies that a task due exactly today is NOT overdue
// (overdue means strictly after the due date).
func TestIsOverdue_DueToday(t *testing.T) {
	today := time.Now().Truncate(24 * time.Hour).Format("2006-01-02")
	task := Task{
		ID:      6,
		Title:   "Due today",
		Done:    false,
		DueDate: today,
	}
	now := time.Now()
	if task.IsOverdue(now) {
		t.Error("expected task due today to NOT be overdue")
	}
}

// ---------------------------------------------------------------------------
// Overdue filter (List) test
// ---------------------------------------------------------------------------

// TestListOverdueFilter verifies that --overdue returns only incomplete tasks
// with a past due date. Covers requirement #7.
func TestListOverdueFilter(t *testing.T) {
	mgr := newManager(t)

	_ = mgr.Add("Overdue task", "high", "2000-01-01")    // overdue
	_ = mgr.Add("Future task", "low", "2099-12-31")      // not overdue
	_ = mgr.Add("No date task", "medium", "")            // no date

	// Also add a completed overdue task — must NOT appear in overdue list.
	_ = mgr.Add("Done overdue", "high", "2000-06-01")
	tasks, _ := mgr.List("", false)
	_ = mgr.Complete(tasks[3].ID)

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

// ---------------------------------------------------------------------------
// Complete tests
// ---------------------------------------------------------------------------

// TestComplete verifies that marking a task done works correctly. Covers req #8.
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

// TestCompleteInvalidID verifies that Complete returns an error for a
// non-existent task ID. Covers requirement #10 (edge case: invalid ID).
func TestCompleteInvalidID(t *testing.T) {
	mgr := newManager(t)
	err := mgr.Complete(9999)
	if err == nil {
		t.Fatal("Complete with non-existent ID: expected error, got nil")
	}
}

// TestCompleteAlreadyDone verifies that marking an already-completed task done
// again succeeds without error (idempotent operation).
func TestCompleteAlreadyDone(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task", "medium", "")
	tasks, _ := mgr.List("", false)
	id := tasks[0].ID

	if err := mgr.Complete(id); err != nil {
		t.Fatalf("first Complete: unexpected error: %v", err)
	}
	if err := mgr.Complete(id); err != nil {
		t.Fatalf("second Complete (idempotent): unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Delete tests
// ---------------------------------------------------------------------------

// TestDelete verifies that deleting a task removes it from the list. Covers req #9.
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

// TestDeleteInvalidID verifies that Delete returns an error for a non-existent
// task ID. Covers requirement #10 (edge case: invalid ID).
func TestDeleteInvalidID(t *testing.T) {
	mgr := newManager(t)
	err := mgr.Delete(9999)
	if err == nil {
		t.Fatal("Delete with non-existent ID: expected error, got nil")
	}
}

// TestDeleteMiddleTask verifies that deleting a task in the middle of the list
// preserves the other tasks correctly.
func TestDeleteMiddleTask(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task A", "high", "")
	_ = mgr.Add("Task B", "medium", "")
	_ = mgr.Add("Task C", "low", "")

	tasks, _ := mgr.List("", false)
	middleID := tasks[1].ID // Task B

	if err := mgr.Delete(middleID); err != nil {
		t.Fatalf("Delete middle task: unexpected error: %v", err)
	}
	remaining, _ := mgr.List("", false)
	if len(remaining) != 2 {
		t.Fatalf("expected 2 tasks after deleting middle, got %d", len(remaining))
	}
	for _, task := range remaining {
		if task.ID == middleID {
			t.Errorf("deleted task %d still present in list", middleID)
		}
	}
}

// ---------------------------------------------------------------------------
// Stats tests
// ---------------------------------------------------------------------------

// TestStatsEmpty verifies all-zero stats on an empty store. Covers req #11.
func TestStatsEmpty(t *testing.T) {
	mgr := newManager(t)
	s, err := mgr.Stats()
	if err != nil {
		t.Fatalf("Stats: unexpected error: %v", err)
	}
	if s.Total != 0 || s.Completed != 0 || s.Pending != 0 || s.Overdue != 0 {
		t.Errorf("expected all-zero stats on empty store, got %+v", s)
	}
	// Derived completion rate should be 0% for empty store.
	pct := 0
	if s.Total > 0 {
		pct = s.Completed * 100 / s.Total
	}
	if pct != 0 {
		t.Errorf("completion rate: expected 0%%, got %d%%", pct)
	}
}

// TestStatsMixed verifies stats with a mix of pending/completed tasks.
// Covers requirement #11 (mixed tasks).
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

// TestStatsAllCompleted verifies stats when every task is done.
// Covers requirement #11 (all-completed tasks).
func TestStatsAllCompleted(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task A", "high", "")
	_ = mgr.Add("Task B", "low", "")

	tasks, _ := mgr.List("", false)
	for _, task := range tasks {
		_ = mgr.Complete(task.ID)
	}

	s, err := mgr.Stats()
	if err != nil {
		t.Fatalf("Stats: unexpected error: %v", err)
	}
	if s.Total != 2 {
		t.Errorf("Total: expected 2, got %d", s.Total)
	}
	if s.Completed != 2 {
		t.Errorf("Completed: expected 2, got %d", s.Completed)
	}
	if s.Pending != 0 {
		t.Errorf("Pending: expected 0, got %d", s.Pending)
	}
	if s.Overdue != 0 {
		t.Errorf("Overdue: expected 0 (all done), got %d", s.Overdue)
	}
	// Completion rate should be 100%.
	pct := s.Completed * 100 / s.Total
	if pct != 100 {
		t.Errorf("completion rate: expected 100%%, got %d%%", pct)
	}
}

// TestStatsCompletionRate verifies the completion rate calculation.
// Covers requirement #11 (completion rate = completed/total * 100).
func TestStatsCompletionRate(t *testing.T) {
	mgr := newManager(t)
	// Add 3 tasks, complete 1 → 33%
	_ = mgr.Add("Task 1", "high", "")
	_ = mgr.Add("Task 2", "medium", "")
	_ = mgr.Add("Task 3", "low", "")

	tasks, _ := mgr.List("", false)
	_ = mgr.Complete(tasks[0].ID)

	s, err := mgr.Stats()
	if err != nil {
		t.Fatalf("Stats: unexpected error: %v", err)
	}
	pct := s.Completed * 100 / s.Total
	if pct != 33 {
		t.Errorf("completion rate: expected 33%%, got %d%%", pct)
	}
}

// TestStatsPriorityCounts verifies that priority counts sum to total and cover
// both pending and completed tasks.
func TestStatsPriorityCounts(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("High 1", "high", "")
	_ = mgr.Add("High 2", "high", "")
	_ = mgr.Add("Med 1", "medium", "")
	_ = mgr.Add("Low 1", "low", "")

	// Complete one high task — it must still appear in the high count.
	tasks, _ := mgr.List("", false)
	_ = mgr.Complete(tasks[0].ID)

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
	// Priority counts must always sum to Total.
	sum := s.HighPriority + s.MediumPriority + s.LowPriority
	if sum != s.Total {
		t.Errorf("priority sum %d != total %d", sum, s.Total)
	}
}

// TestStatsOverdue verifies that Stats.Overdue counts only incomplete tasks
// with a past due date. Covers requirement #11 (overdue counting).
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
// 0600 (owner read/write only) after an Add operation. Covers requirement #12.
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
// validates there are no data races in the save path. Covers requirement #13.
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

// TestSaveConcurrentAccess verifies that concurrent Add operations do not
// cause race conditions (exercised by -race flag).
func TestSaveConcurrentAccess(t *testing.T) {
	mgr := newManager(t)

	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func(n int) {
			done <- mgr.Add("Concurrent task", "medium", "")
		}(i)
	}
	for i := 0; i < 5; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent Add: unexpected error: %v", err)
		}
	}
	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after concurrent adds: unexpected error: %v", err)
	}
	if len(tasks) != 5 {
		t.Errorf("expected 5 tasks after 5 concurrent adds, got %d", len(tasks))
	}
}
