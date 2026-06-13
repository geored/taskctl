package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newManager creates a Manager backed by a temp file that is cleaned up
// automatically when the test ends. It calls t.Fatal if setup fails.
func newManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "tasks.json")
	mgr, err := NewManager(filePath)
	if err != nil {
		t.Fatalf("NewManager: unexpected error: %v", err)
	}
	return mgr
}

// ---------------------------------------------------------------------------
// NewManager tests
// ---------------------------------------------------------------------------

// TestNewManager_TraversalRejected verifies that paths containing ".." are
// rejected by NewManager.
func TestNewManager_TraversalRejected(t *testing.T) {
	cases := []string{
		"../tasks.json",
		"../../etc/shadow",
		"subdir/../../tasks.json",
	}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			mgr, err := NewManager(p)
			if err == nil {
				t.Errorf("NewManager(%q): expected traversal error, got nil (mgr=%v)", p, mgr)
			}
		})
	}
}

// TestNewManager_ValidPath verifies that a plain file path is accepted.
func TestNewManager_ValidPath(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(filepath.Join(dir, "tasks.json"))
	if err != nil {
		t.Fatalf("NewManager: unexpected error: %v", err)
	}
	if mgr == nil {
		t.Fatal("NewManager: expected non-nil manager")
	}
}

// ---------------------------------------------------------------------------
// Add tests
// ---------------------------------------------------------------------------

// TestAdd_Basic verifies that a single task can be added and then retrieved.
func TestAdd_Basic(t *testing.T) {
	mgr := newManager(t)
	if err := mgr.Add("Buy milk", "medium", ""); err != nil {
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
		t.Errorf("Title: expected %q, got %q", "Buy milk", tasks[0].Title)
	}
	if tasks[0].Priority != "medium" {
		t.Errorf("Priority: expected %q, got %q", "medium", tasks[0].Priority)
	}
	if tasks[0].Done {
		t.Errorf("Done: expected false, got true")
	}
}

// TestAdd_IDAutoIncrement verifies that successive tasks get sequential IDs.
func TestAdd_IDAutoIncrement(t *testing.T) {
	mgr := newManager(t)
	titles := []string{"Task A", "Task B", "Task C"}
	for _, title := range titles {
		if err := mgr.Add(title, "low", ""); err != nil {
			t.Fatalf("Add(%q): unexpected error: %v", title, err)
		}
	}

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}
	for i, task := range tasks {
		if task.ID != i+1 {
			t.Errorf("tasks[%d].ID: expected %d, got %d", i, i+1, task.ID)
		}
	}
}

// TestAdd_InvalidPriority verifies that Add returns an error for unrecognised
// priority strings.
func TestAdd_InvalidPriority(t *testing.T) {
	mgr := newManager(t)
	cases := []string{"urgent", "critical", "HIGH", "Low", "MEDIUM", "none", ""}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			err := mgr.Add("Some task", p, "")
			if err == nil {
				t.Errorf("Add with priority %q: expected error, got nil", p)
			}
		})
	}
}

// TestAdd_ValidDueDate verifies that Add succeeds with a correctly formatted
// due date.
func TestAdd_ValidDueDate(t *testing.T) {
	mgr := newManager(t)
	if err := mgr.Add("Deadline task", "high", "2025-12-31"); err != nil {
		t.Fatalf("Add with valid due date: unexpected error: %v", err)
	}
	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].DueDate != "2025-12-31" {
		t.Errorf("DueDate: expected %q, got %q", "2025-12-31", tasks[0].DueDate)
	}
}

// TestAdd_InvalidDueDate verifies that Add returns an error for malformed due
// date strings.
func TestAdd_InvalidDueDate(t *testing.T) {
	mgr := newManager(t)
	cases := []string{"31-12-2025", "2025/12/31", "not-a-date", "20251231"}
	for _, d := range cases {
		t.Run(d, func(t *testing.T) {
			err := mgr.Add("Task", "medium", d)
			if err == nil {
				t.Errorf("Add with due date %q: expected error, got nil", d)
			}
		})
	}
}

// TestAdd_TitleTrimmed verifies that leading/trailing whitespace in the title
// is stripped before persistence.
func TestAdd_TitleTrimmed(t *testing.T) {
	mgr := newManager(t)
	if err := mgr.Add("  Buy milk  ", "medium", ""); err != nil {
		t.Fatalf("Add: unexpected error: %v", err)
	}
	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if tasks[0].Title != "Buy milk" {
		t.Errorf("Title: expected %q, got %q", "Buy milk", tasks[0].Title)
	}
}

// ---------------------------------------------------------------------------
// Complete tests
// ---------------------------------------------------------------------------

// TestComplete_Basic verifies that a task can be marked done.
func TestComplete_Basic(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task", "medium", "")

	tasks, _ := mgr.List("", false)
	if err := mgr.Complete(tasks[0].ID); err != nil {
		t.Fatalf("Complete: unexpected error: %v", err)
	}

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if !tasks[0].Done {
		t.Errorf("Done: expected true, got false")
	}
}

// TestComplete_NotFound verifies that completing a non-existent task returns
// an error.
func TestComplete_NotFound(t *testing.T) {
	mgr := newManager(t)
	err := mgr.Complete(999)
	if err == nil {
		t.Error("Complete(999): expected error for non-existent task, got nil")
	}
}

// TestComplete_InvalidID verifies that Complete returns an error for non-positive
// IDs (0 and negative values).
func TestComplete_InvalidID(t *testing.T) {
	mgr := newManager(t)
	cases := []int{0, -1, -100}
	for _, id := range cases {
		t.Run("id="+itoa(id), func(t *testing.T) {
			err := mgr.Complete(id)
			if err == nil {
				t.Errorf("Complete(%d): expected error for non-positive ID, got nil", id)
			}
			if err != nil && !strings.Contains(err.Error(), "invalid task ID") {
				t.Errorf("Complete(%d): expected error message to contain %q, got: %q", id, "invalid task ID", err.Error())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Delete tests
// ---------------------------------------------------------------------------

// TestDelete_Basic verifies that a task can be removed.
func TestDelete_Basic(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Task A", "high", "")
	_ = mgr.Add("Task B", "low", "")

	tasks, _ := mgr.List("", false)
	deleteID := tasks[0].ID
	if err := mgr.Delete(deleteID); err != nil {
		t.Fatalf("Delete: unexpected error: %v", err)
	}

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task after delete, got %d", len(tasks))
	}
	if tasks[0].ID == deleteID {
		t.Errorf("deleted task (ID=%d) is still present", deleteID)
	}
}

// TestDelete_NotFound verifies that deleting a non-existent task returns an
// error.
func TestDelete_NotFound(t *testing.T) {
	mgr := newManager(t)
	err := mgr.Delete(999)
	if err == nil {
		t.Error("Delete(999): expected error for non-existent task, got nil")
	}
}

// TestDelete_InvalidID verifies that Delete returns an error for non-positive
// IDs (0 and negative values).
func TestDelete_InvalidID(t *testing.T) {
	mgr := newManager(t)
	cases := []int{0, -1, -100}
	for _, id := range cases {
		t.Run("id="+itoa(id), func(t *testing.T) {
			err := mgr.Delete(id)
			if err == nil {
				t.Errorf("Delete(%d): expected error for non-positive ID, got nil", id)
			}
			if err != nil && !strings.Contains(err.Error(), "invalid task ID") {
				t.Errorf("Delete(%d): expected error message to contain %q, got: %q", id, "invalid task ID", err.Error())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// IsOverdue tests
// ---------------------------------------------------------------------------

// TestIsOverdue_PastDate verifies that a task with a past due date is overdue.
func TestIsOverdue_PastDate(t *testing.T) {
	task := Task{Done: false, DueDate: "2000-01-01"}
	if !task.IsOverdue(time.Now()) {
		t.Error("expected task with past due date to be overdue")
	}
}

// TestIsOverdue_FutureDate verifies that a task with a future due date is not
// overdue.
func TestIsOverdue_FutureDate(t *testing.T) {
	task := Task{Done: false, DueDate: "2099-12-31"}
	if task.IsOverdue(time.Now()) {
		t.Error("expected task with future due date to not be overdue")
	}
}

// TestIsOverdue_NoDueDate verifies that a task without a due date is never
// considered overdue.
func TestIsOverdue_NoDueDate(t *testing.T) {
	task := Task{Done: false, DueDate: ""}
	if task.IsOverdue(time.Now()) {
		t.Error("expected task without due date to not be overdue")
	}
}

// TestIsOverdue_DoneTask verifies that a done task is never considered overdue,
// even if its due date is in the past.
func TestIsOverdue_DoneTask(t *testing.T) {
	task := Task{Done: true, DueDate: "2000-01-01"}
	if task.IsOverdue(time.Now()) {
		t.Error("expected done task to not be overdue regardless of due date")
	}
}

// TestIsOverdue_InvalidDueDate verifies that a task with a malformed due date
// is not considered overdue.
func TestIsOverdue_InvalidDueDate(t *testing.T) {
	task := Task{Done: false, DueDate: "not-a-date"}
	if task.IsOverdue(time.Now()) {
		t.Error("expected task with invalid due date to not be overdue")
	}
}

// ---------------------------------------------------------------------------
// List tests
// ---------------------------------------------------------------------------

// TestList_FilterByPriority verifies that the priority filter returns only
// matching tasks.
func TestList_FilterByPriority(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("High task 1", "high", "")
	_ = mgr.Add("High task 2", "high", "")
	_ = mgr.Add("Medium task", "medium", "")
	_ = mgr.Add("Low task", "low", "")

	tasks, err := mgr.List("high", false)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 high-priority tasks, got %d", len(tasks))
	}
	for _, task := range tasks {
		if task.Priority != "high" {
			t.Errorf("expected priority %q, got %q", "high", task.Priority)
		}
	}
}

// TestList_OverdueOnly verifies that the overdueOnly flag returns only overdue
// tasks.
func TestList_OverdueOnly(t *testing.T) {
	mgr := newManager(t)
	_ = mgr.Add("Overdue task", "high", "2000-01-01")
	_ = mgr.Add("Future task", "medium", "2099-12-31")
	_ = mgr.Add("No due date", "low", "")

	tasks, err := mgr.List("", true)
	if err != nil {
		t.Fatalf("List(overdueOnly): unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 overdue task, got %d", len(tasks))
	}
	if tasks[0].Title != "Overdue task" {
		t.Errorf("expected %q, got %q", "Overdue task", tasks[0].Title)
	}
}

// TestList_EmptyStore verifies that listing an empty store returns an empty
// slice, not nil or an error.
func TestList_EmptyStore(t *testing.T) {
	mgr := newManager(t)
	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List on empty store: unexpected error: %v", err)
	}
	if tasks == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
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
			// Verify error message contains the expected text.
			if err != nil && !strings.Contains(err.Error(), "invalid priority filter") {
				t.Errorf("List(%q): expected error message to contain %q, got: %q", p, "invalid priority filter", err.Error())
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
// load() error message tests
// ---------------------------------------------------------------------------

// TestLoad_CorruptedJSON verifies that load() returns an error containing the
// file path when the backing file contains malformed JSON (Issue #30).
func TestLoad_CorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "corrupt.json")

	// Write corrupt JSON directly.
	if err := os.WriteFile(filePath, []byte("not-valid-json{{{"), 0600); err != nil {
		t.Fatalf("WriteFile: unexpected error: %v", err)
	}

	mgr, err := NewManager(filePath)
	if err != nil {
		t.Fatalf("NewManager: unexpected error: %v", err)
	}

	// Trigger load via a public method.
	_, err = mgr.List("", false)
	if err == nil {
		t.Fatal("List on corrupted file: expected error, got nil")
	}
	if !strings.Contains(err.Error(), filePath) {
		t.Errorf("expected error message to contain file path %q, got: %q", filePath, err.Error())
	}
	if !strings.Contains(err.Error(), "corrupted") {
		t.Errorf("expected error message to contain %q, got: %q", "corrupted", err.Error())
	}
}

// TestLoad_CorruptedJSON_ViaAdd verifies that load() error propagates through Add().
func TestLoad_CorruptedJSON_ViaAdd(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "corrupt.json")

	if err := os.WriteFile(filePath, []byte("{bad json"), 0600); err != nil {
		t.Fatalf("WriteFile: unexpected error: %v", err)
	}

	mgr, err := NewManager(filePath)
	if err != nil {
		t.Fatalf("NewManager: unexpected error: %v", err)
	}

	err = mgr.Add("New task", "medium", "")
	if err == nil {
		t.Fatal("Add on corrupted file: expected error, got nil")
	}
	if !strings.Contains(err.Error(), filePath) {
		t.Errorf("expected error message to contain file path %q, got: %q", filePath, err.Error())
	}
}

// ---------------------------------------------------------------------------
// Clear tests
// ---------------------------------------------------------------------------

// TestClear_MixedTasks verifies that Clear removes done tasks and keeps pending
// ones, returning correct cleared/remaining counts.
func TestClear_MixedTasks(t *testing.T) {
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
func TestClear_CorruptedFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "tasks.json")

	// Write corrupt JSON.
	if err := os.WriteFile(filePath, []byte("not-valid-json{{{"), 0600); err != nil {
		t.Fatalf("WriteFile: unexpected error: %v", err)
	}

	mgr, err := NewManager(filePath)
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
// Helpers
// ---------------------------------------------------------------------------

// itoa converts an int to its string representation (avoids importing strconv).
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
