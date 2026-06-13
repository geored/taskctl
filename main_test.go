package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geored/taskctl/task"
)

// newTestManager creates a Manager backed by a temp file isolated per test.
func newTestManager(t *testing.T) *task.Manager {
	t.Helper()
	mgr, err := task.NewManager(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatalf("NewManager: unexpected error: %v", err)
	}
	return mgr
}

// ---------------------------------------------------------------------------
// runAdd tests
// ---------------------------------------------------------------------------

func TestRunAdd_Success(t *testing.T) {
	mgr := newTestManager(t)
	err := runAdd(mgr, []string{"--priority", "high", "Buy milk"})
	if err != nil {
		t.Fatalf("runAdd: unexpected error: %v", err)
	}
}

func TestRunAdd_MultiWordTitle(t *testing.T) {
	mgr := newTestManager(t)
	err := runAdd(mgr, []string{"Fix the broken", "CI pipeline"})
	if err != nil {
		t.Fatalf("runAdd multi-word: unexpected error: %v", err)
	}
}

func TestRunAdd_FlagEqualsValueSyntax(t *testing.T) {
	mgr := newTestManager(t)
	// --priority=low uses the flag=value syntax (requires flag.FlagSet support)
	err := runAdd(mgr, []string{"--priority=low", "--due=2099-12-31", "Syntax test"})
	if err != nil {
		t.Fatalf("runAdd with --flag=value syntax: unexpected error: %v", err)
	}
}

func TestRunAdd_MissingTitle(t *testing.T) {
	mgr := newTestManager(t)
	err := runAdd(mgr, []string{"--priority", "medium"})
	if err == nil {
		t.Fatal("runAdd with no title: expected error, got nil")
	}
}

func TestRunAdd_InvalidPriority(t *testing.T) {
	mgr := newTestManager(t)
	err := runAdd(mgr, []string{"--priority", "urgent", "My task"})
	if err == nil {
		t.Fatal("runAdd with invalid priority: expected error, got nil")
	}
}

func TestRunAdd_InvalidDueDate(t *testing.T) {
	mgr := newTestManager(t)
	err := runAdd(mgr, []string{"--due", "not-a-date", "My task"})
	if err == nil {
		t.Fatal("runAdd with invalid due date: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// runList tests
// ---------------------------------------------------------------------------

func TestRunList_EmptyStore(t *testing.T) {
	mgr := newTestManager(t)
	err := runList(mgr, []string{})
	if err != nil {
		t.Fatalf("runList empty store: unexpected error: %v", err)
	}
}

func TestRunList_WithTasks(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "high", "Task A"})
	_ = runAdd(mgr, []string{"--priority", "low", "Task B"})

	err := runList(mgr, []string{})
	if err != nil {
		t.Fatalf("runList: unexpected error: %v", err)
	}
}

func TestRunList_FilterByPriority(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "high", "High task"})
	_ = runAdd(mgr, []string{"--priority", "low", "Low task"})

	err := runList(mgr, []string{"--priority", "high"})
	if err != nil {
		t.Fatalf("runList --priority high: unexpected error: %v", err)
	}
}

func TestRunList_FilterByPriorityEqualsValue(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority=medium", "Med task"})

	err := runList(mgr, []string{"--priority=medium"})
	if err != nil {
		t.Fatalf("runList --priority=medium: unexpected error: %v", err)
	}
}

func TestRunList_InvalidPriority(t *testing.T) {
	mgr := newTestManager(t)
	err := runList(mgr, []string{"--priority", "urgent"})
	if err == nil {
		t.Fatal("runList with invalid priority: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "urgent") {
		t.Errorf("error message should mention invalid priority, got: %v", err)
	}
}

func TestRunList_OverdueFlag(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--due", "2000-01-01", "--priority", "high", "Old task"})

	err := runList(mgr, []string{"--overdue"})
	if err != nil {
		t.Fatalf("runList --overdue: unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runDone tests
// ---------------------------------------------------------------------------

func TestRunDone_Success(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "medium", "Task to complete"})

	tasks, _ := mgr.List("", false)
	id := tasks[0].ID

	err := runDone(mgr, []string{intStr(id)})
	if err != nil {
		t.Fatalf("runDone: unexpected error: %v", err)
	}

	tasks, _ = mgr.List("", false)
	if !tasks[0].Done {
		t.Error("expected task to be marked done after runDone")
	}
}

func TestRunDone_MissingID(t *testing.T) {
	mgr := newTestManager(t)
	err := runDone(mgr, []string{})
	if err == nil {
		t.Fatal("runDone with no args: expected error, got nil")
	}
}

func TestRunDone_InvalidID(t *testing.T) {
	mgr := newTestManager(t)
	err := runDone(mgr, []string{"not-a-number"})
	if err == nil {
		t.Fatal("runDone with non-integer ID: expected error, got nil")
	}
}

func TestRunDone_NonExistentID(t *testing.T) {
	mgr := newTestManager(t)
	err := runDone(mgr, []string{"9999"})
	if err == nil {
		t.Fatal("runDone with non-existent ID: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// runDelete tests
// ---------------------------------------------------------------------------

func TestRunDelete_Success(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "low", "Task to delete"})

	tasks, _ := mgr.List("", false)
	id := tasks[0].ID

	err := runDelete(mgr, []string{intStr(id)})
	if err != nil {
		t.Fatalf("runDelete: unexpected error: %v", err)
	}

	tasks, _ = mgr.List("", false)
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after delete, got %d", len(tasks))
	}
}

func TestRunDelete_MissingID(t *testing.T) {
	mgr := newTestManager(t)
	err := runDelete(mgr, []string{})
	if err == nil {
		t.Fatal("runDelete with no args: expected error, got nil")
	}
}

func TestRunDelete_InvalidID(t *testing.T) {
	mgr := newTestManager(t)
	err := runDelete(mgr, []string{"abc"})
	if err == nil {
		t.Fatal("runDelete with non-integer ID: expected error, got nil")
	}
}

func TestRunDelete_NonExistentID(t *testing.T) {
	mgr := newTestManager(t)
	err := runDelete(mgr, []string{"9999"})
	if err == nil {
		t.Fatal("runDelete with non-existent ID: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// runStats tests
// ---------------------------------------------------------------------------

func TestRunStats_EmptyStore(t *testing.T) {
	mgr := newTestManager(t)
	err := runStats(mgr)
	if err != nil {
		t.Fatalf("runStats empty store: unexpected error: %v", err)
	}
}

func TestRunStats_WithTasks(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "high", "Task 1"})
	_ = runAdd(mgr, []string{"--priority", "low", "Task 2"})

	tasks, _ := mgr.List("", false)
	_ = runDone(mgr, []string{intStr(tasks[0].ID)})

	err := runStats(mgr)
	if err != nil {
		t.Fatalf("runStats with tasks: unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runClear tests
// ---------------------------------------------------------------------------

// TestRunClear_EmptyStore verifies that runClear on an empty store returns nil.
func TestRunClear_EmptyStore(t *testing.T) {
	mgr := newTestManager(t)
	err := runClear(mgr)
	if err != nil {
		t.Fatalf("runClear on empty store: unexpected error: %v", err)
	}
}

// TestRunClear_NoDoneTasks verifies that runClear with only pending tasks
// returns nil and all tasks are still present.
func TestRunClear_NoDoneTasks(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "high", "Pending 1"})
	_ = runAdd(mgr, []string{"--priority", "medium", "Pending 2"})
	_ = runAdd(mgr, []string{"--priority", "low", "Pending 3"})

	err := runClear(mgr)
	if err != nil {
		t.Fatalf("runClear with no done tasks: unexpected error: %v", err)
	}

	tasks, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after runClear: unexpected error: %v", err)
	}
	if len(tasks) != 3 {
		t.Errorf("runClear with no done tasks: expected 3 tasks remaining, got %d", len(tasks))
	}
}

// TestRunClear_ClearsCompleted verifies that runClear removes completed tasks
// and only pending tasks remain after the call.
func TestRunClear_ClearsCompleted(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "high", "Keep me"})
	_ = runAdd(mgr, []string{"--priority", "medium", "Delete me"})
	_ = runAdd(mgr, []string{"--priority", "low", "Also keep me"})

	tasks, _ := mgr.List("", false)
	// Mark the second task as done.
	_ = runDone(mgr, []string{intStr(tasks[1].ID)})

	err := runClear(mgr)
	if err != nil {
		t.Fatalf("runClear: unexpected error: %v", err)
	}

	remaining, err := mgr.List("", false)
	if err != nil {
		t.Fatalf("List after runClear: unexpected error: %v", err)
	}
	if len(remaining) != 2 {
		t.Errorf("expected 2 tasks remaining after runClear, got %d", len(remaining))
	}
	for _, task := range remaining {
		if task.Done {
			t.Errorf("task %d (%q) should not be done after runClear", task.ID, task.Title)
		}
	}
}

// TestRunClear_OutputFormat verifies that runClear prints the expected output
// to stdout in the exact format: "Cleared N completed tasks. M tasks remaining.\n".
func TestRunClear_OutputFormat(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "high", "Task A"})
	_ = runAdd(mgr, []string{"--priority", "medium", "Task B"})
	_ = runAdd(mgr, []string{"--priority", "low", "Task C"})

	tasks, _ := mgr.List("", false)
	_ = runDone(mgr, []string{intStr(tasks[0].ID)}) // mark Task A done
	_ = runDone(mgr, []string{intStr(tasks[1].ID)}) // mark Task B done

	// Capture stdout.
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: unexpected error: %v", err)
	}
	os.Stdout = w

	runErr := runClear(mgr)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy: unexpected error: %v", err)
	}

	if runErr != nil {
		t.Fatalf("runClear: unexpected error: %v", runErr)
	}

	got := buf.String()
	want := fmt.Sprintf("Cleared %d completed tasks. %d tasks remaining.\n", 2, 1)
	if got != want {
		t.Errorf("runClear output:\n  got:  %q\n  want: %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// intStr converts an int to its string representation — avoids importing strconv
// in the test file directly.
func intStr(n int) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.Repeat("x", n), "x", "")[0:0]) + itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
