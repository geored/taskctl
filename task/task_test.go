package task

import (
	"encoding/json"
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
// rejected by NewManager. (Req 34)
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

// TestNewManagerValidPath verifies that a plain relative path is accepted. (Req 34)
func TestNewManagerValidPath(t *testing.T) {
	dir := t.TempDir()
	_, err := NewManager(filepath.Join(dir, "tasks.json"))
	if err != nil {
		t.Errorf("NewManager: unexpected error for valid path: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CRUD tests
// ---------------------------------------------------------------------------

// TestAdd verifies basic task creation and that title/priority are stored. (Req 35)
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
	if tasks[0].Priority != "low" {
		t.Errorf("expected priority %q, got %q", "low", tasks[0].Priority)
	}
}

// TestList verifies that an unfiltered list returns all tasks. (Req 36)
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

// TestComplete verifies that a task is marked Done = true. (Req 37)
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

// TestDelete verifies that a task is removed and the list returns empty. (Req 38)
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

// TestListFilterByPriority verifies List("high", false) returns only high-priority tasks. (Req 39)
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

// TestAddInvalidPriority verifies that Add rejects unrecognised priority values. (Req 40)
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
// stored correctly. (Req 41)
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
// Stats tests
// ---------------------------------------------------------------------------

// TestStatsEmpty verifies all fields are zero on an empty store. (Req 42)
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

// TestStatsMixed verifies correct Total, Completed, and Pending counts. (Req 43)
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

// TestAddWithDueDate verifies due date is stored and retrieved correctly. (Req 44)
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

// TestAddInvalidDueDate verifies Add with "not-a-date" returns an error. (Req 45)
func TestAddInvalidDueDate(t *testing.T) {
	mgr := newManager(t)
	err := mgr.Add("Bad date task", "low", "not-a-date")
	if err == nil {
		t.Fatal("expected error for invalid due date, got nil")
	}
}

// TestAddNoDueDate verifies that DueDate field is empty string when not set. (Req 46)
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
// is reported as overdue. (Req 47)
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

// TestIsOverdue_FutureDate verifies that a task due in the future is not overdue. (Req 48)
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
// if its due date has passed. (Req 49)
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

// TestIsOverdue_NoDueDate verifies that a task with no due date is never overdue. (Req 50)
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

// TestListOverdueFilter verifies that overdueOnly=true returns only incomplete tasks
// with a past due date. (Req 51)
func TestListOverdueFilter(t *testing.T) {
	mgr := newManager(t)

	// Overdue: past date, incomplete
	_ = mgr.Add("Overdue task", "high", "2000-06-15")
	// Not overdue: future date
	_ = mgr.Add("Future task", "low", "2099-01-01")
	// Not overdue: no due date
	_ = mgr.Add("No date task", "medium", "")
	// Not overdue: past date but completed
	_ = mgr.Add("Done old task", "medium", "2000-01-01")
	tasks, _ := mgr.List("", false)
	// Mark the last task done
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

// TestStatsOverdue verifies that Stats.Overdue counts only incomplete tasks
// with a past due date; completed tasks with past dates are excluded. (Req 52)
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
// due dates set. (Req 53)
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

// TestStatsAllCompleted verifies that when all tasks are done, Pending = 0 and
// Overdue = 0 and Completed = Total. (Req 54)
func TestStatsAllCompleted(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task 1", "high", "2000-01-01") // past due, but will be completed
	_ = mgr.Add("Task 2", "medium", "")
	_ = mgr.Add("Task 3", "low", "")

	tasks, _ := mgr.List("", false)
	for _, tk := range tasks {
		_ = mgr.Complete(tk.ID)
	}

	s, err := mgr.Stats()
	if err != nil {
		t.Fatalf("Stats: unexpected error: %v", err)
	}
	if s.Pending != 0 {
		t.Errorf("Pending: expected 0, got %d", s.Pending)
	}
	if s.Overdue != 0 {
		t.Errorf("Overdue: expected 0 when all complete, got %d", s.Overdue)
	}
	if s.Completed != s.Total {
		t.Errorf("Completed (%d) should equal Total (%d)", s.Completed, s.Total)
	}
}

// TestStatsCompletionRate verifies 25% completion rate for 1-of-4 completed. (Req 55)
func TestStatsCompletionRate(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task 1", "high", "")
	_ = mgr.Add("Task 2", "medium", "")
	_ = mgr.Add("Task 3", "low", "")
	_ = mgr.Add("Task 4", "high", "")

	tasks, _ := mgr.List("", false)
	// Complete only the first task
	_ = mgr.Complete(tasks[0].ID)

	s, err := mgr.Stats()
	if err != nil {
		t.Fatalf("Stats: unexpected error: %v", err)
	}

	// Completion rate: Completed * 100 / Total (integer division)
	var pct int
	if s.Total > 0 {
		pct = s.Completed * 100 / s.Total
	}
	if pct != 25 {
		t.Errorf("completion rate: expected 25%%, got %d%%", pct)
	}
}

// TestStatsPriorityBreakdown verifies HighPriority + MediumPriority + LowPriority == Total. (Req 56)
func TestStatsPriorityBreakdown(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("H1", "high", "")
	_ = mgr.Add("H2", "high", "")
	_ = mgr.Add("M1", "medium", "")
	_ = mgr.Add("L1", "low", "")
	_ = mgr.Add("L2", "low", "")

	// Mark one of each priority done to verify breakdown counts all tasks
	tasks, _ := mgr.List("", false)
	_ = mgr.Complete(tasks[0].ID) // H1

	s, err := mgr.Stats()
	if err != nil {
		t.Fatalf("Stats: unexpected error: %v", err)
	}

	sum := s.HighPriority + s.MediumPriority + s.LowPriority
	if sum != s.Total {
		t.Errorf("HighPriority(%d) + MediumPriority(%d) + LowPriority(%d) = %d, expected Total(%d)",
			s.HighPriority, s.MediumPriority, s.LowPriority, sum, s.Total)
	}
	if s.HighPriority != 2 {
		t.Errorf("HighPriority: expected 2, got %d", s.HighPriority)
	}
	if s.MediumPriority != 1 {
		t.Errorf("MediumPriority: expected 1, got %d", s.MediumPriority)
	}
	if s.LowPriority != 2 {
		t.Errorf("LowPriority: expected 2, got %d", s.LowPriority)
	}
}

// ---------------------------------------------------------------------------
// Atomic save and file permission tests
// ---------------------------------------------------------------------------

// TestSaveFilePermissions verifies that the tasks file is written with mode
// 0600 (owner read/write only) after an Add operation. (Req 57)
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
// validates there are no data races in the save path. (Req 58)
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

	// Verify that the final file is valid JSON (no corruption from atomic writes)
	data, err := os.ReadFile(mgr.filePath)
	if err != nil {
		t.Fatalf("ReadFile: unexpected error: %v", err)
	}
	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		t.Fatalf("final file is not valid JSON: %v", err)
	}
	if len(tasks) != n {
		t.Errorf("expected %d tasks in file, got %d", n, len(tasks))
	}
}

// TestInvalidIDs verifies that Complete and Delete return errors for
// non-existent IDs. (Req 59)
func TestInvalidIDs(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Existing task", "medium", "")

	// Try Complete with non-existent ID
	if err := mgr.Complete(9999); err == nil {
		t.Error("Complete with non-existent ID: expected error, got nil")
	}

	// Try Delete with non-existent ID
	if err := mgr.Delete(9999); err == nil {
		t.Error("Delete with non-existent ID: expected error, got nil")
	}

	// Verify the existing task is untouched
	tasks, _ := mgr.List("", false)
	if len(tasks) != 1 {
		t.Errorf("expected 1 task to remain untouched, got %d", len(tasks))
	}
}

// TestAddEmptyTitle verifies that Add with a blank or whitespace-only title
// returns an error. (Req 60)
func TestAddEmptyTitle(t *testing.T) {
	mgr := newManager(t)
	cases := []string{"", "   ", "\t", "\n"}
	for _, title := range cases {
		err := mgr.Add(title, "medium", "")
		if err == nil {
			t.Errorf("Add with title %q: expected error for empty title, got nil", title)
		}
	}
}
