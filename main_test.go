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

// captureStdout redirects os.Stdout during fn(), returns what was printed.
// It uses defer to restore os.Stdout even if fn() panics, making it safe
// for use in tests that may encounter unexpected errors.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	return buf.String()
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
	// Verify the CLI-layer error message has the required format (Issue #32).
	if !strings.Contains(err.Error(), "invalid date format for --due") {
		t.Errorf("expected error to contain %q, got: %v", "invalid date format for --due", err)
	}
	if !strings.Contains(err.Error(), "not-a-date") {
		t.Errorf("expected error to mention the bad value %q, got: %v", "not-a-date", err)
	}
}

func TestRunAdd_InvalidDueDateEqualsFormat(t *testing.T) {
	mgr := newTestManager(t)
	err := runAdd(mgr, []string{"--due=bad-date", "My task"})
	if err == nil {
		t.Fatal("runAdd with invalid due date (= syntax): expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid date format for --due") {
		t.Errorf("expected error to contain %q, got: %v", "invalid date format for --due", err)
	}
}

func TestRunAdd_ValidDueDate(t *testing.T) {
	mgr := newTestManager(t)
	err := runAdd(mgr, []string{"--due", "2025-12-31", "My task"})
	if err != nil {
		t.Fatalf("runAdd with valid due date: unexpected error: %v", err)
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
	_ = runAdd(mgr, []string{"--priority", "high", "--due", "2000-01-01", "Old task"})
	_ = runAdd(mgr, []string{"--priority", "medium", "Normal task"})
	_ = runAdd(mgr, []string{"--priority", "low", "Low task"})

	tasks, _ := mgr.List("", false)
	_ = mgr.Complete(tasks[0].ID)

	out := captureStdout(t, func() {
		if err := runStats(mgr); err != nil {
			t.Errorf("runStats: unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Total tasks:") {
		t.Errorf("expected output to contain 'Total tasks:', got: %s", out)
	}
	if !strings.Contains(out, "Completion rate:") {
		t.Errorf("expected output to contain 'Completion rate:', got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// runClear tests
// ---------------------------------------------------------------------------

func TestRunClear_HappyPath(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "high", "Task A"})
	_ = runAdd(mgr, []string{"--priority", "low", "Task B"})
	_ = runAdd(mgr, []string{"--priority", "medium", "Task C"})

	tasks, _ := mgr.List("", false)
	_ = mgr.Complete(tasks[0].ID)
	_ = mgr.Complete(tasks[1].ID)

	out := captureStdout(t, func() {
		if err := runClear(mgr); err != nil {
			t.Errorf("runClear: unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Cleared 2 completed tasks") {
		t.Errorf("expected 'Cleared 2 completed tasks' in output, got: %s", out)
	}
	if !strings.Contains(out, "1 tasks remaining") {
		t.Errorf("expected '1 tasks remaining' in output, got: %s", out)
	}

	remaining, _ := mgr.List("", false)
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining task after clear, got %d", len(remaining))
	}
}

func TestRunClear_NothingToClear(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "medium", "Active task"})

	out := captureStdout(t, func() {
		if err := runClear(mgr); err != nil {
			t.Errorf("runClear: unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Cleared 0 completed tasks") {
		t.Errorf("expected 'Cleared 0 completed tasks', got: %s", out)
	}
}

func TestRunClear_EmptyStore(t *testing.T) {
	mgr := newTestManager(t)
	out := captureStdout(t, func() {
		if err := runClear(mgr); err != nil {
			t.Errorf("runClear on empty store: unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Cleared 0 completed tasks") {
		t.Errorf("expected 'Cleared 0 completed tasks', got: %s", out)
	}
}

func TestRunClear_Error(t *testing.T) {
	// Use a path that cannot be created to force a load error.
	// We construct a manager pointing at a directory (not a file) so that
	// reading it as JSON will fail.
	dir := t.TempDir()
	// Create a subdirectory where the file should be — os.ReadFile on a dir
	// will fail, causing load() to return an error.
	badPath := filepath.Join(dir, "subdir")
	if err := os.Mkdir(badPath, 0755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	mgr, err := task.NewManager(badPath)
	if err != nil {
		t.Fatalf("NewManager: unexpected error: %v", err)
	}

	err = runClear(mgr)
	if err == nil {
		t.Fatal("runClear with bad path: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// captureStdout panic-safety test (Issue #56)
// ---------------------------------------------------------------------------

// TestCaptureStdout_PanicSafety verifies that captureStdout restores os.Stdout
// even when the wrapped function panics.
func TestCaptureStdout_PanicSafety(t *testing.T) {
	original := os.Stdout

	// captureStdout should restore os.Stdout even if fn() panics.
	func() {
		defer func() {
			// Swallow the panic so the test doesn't fail due to it.
			recover() //nolint:errcheck
		}()
		captureStdout(t, func() {
			panic("simulated panic inside captureStdout")
		})
	}()

	if os.Stdout != original {
		// Restore manually so subsequent tests aren't broken, then fail.
		os.Stdout = original
		t.Fatal("captureStdout did not restore os.Stdout after panic")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func intStr(n int) string {
	return fmt.Sprintf("%d", n)
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

var _ = fmt.Sprintf
