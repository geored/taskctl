package main

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/geored/taskctl/task"
)

// chdirTemp changes the working directory to a fresh temporary directory for
// the duration of the test and restores the original directory on cleanup.
// It returns the absolute path of the temporary directory.
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

// newTestManager creates a Manager backed by a temp file isolated per test.
// It changes the working directory to a fresh temp directory so that the
// relative path "tasks.json" resolves to an isolated, per-test location.
func newTestManager(t *testing.T) *task.Manager {
	t.Helper()
	chdirTemp(t)
	mgr, err := task.NewManager("tasks.json")
	if err != nil {
		t.Fatalf("NewManager: unexpected error: %v", err)
	}
	return mgr
}

// newBuf returns a fresh bytes.Buffer as an io.Writer for capturing output.
func newBuf() *bytes.Buffer {
	return &bytes.Buffer{}
}

// intStr converts an int to its decimal string representation.
func intStr(n int) string {
	return strconv.Itoa(n)
}

// ---------------------------------------------------------------------------
// run() tests
// ---------------------------------------------------------------------------

func TestRun_NoArgs(t *testing.T) {
	err := run([]string{"taskctl"}, newBuf())
	if err == nil {
		t.Fatal("run with no args: expected error, got nil")
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	chdirTemp(t)
	err := run([]string{"taskctl", "frobnicate"}, newBuf())
	if err == nil {
		t.Fatal("run with unknown command: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "frobnicate") {
		t.Errorf("error should mention unknown command, got: %v", err)
	}
}

func TestRun_InvalidFilePath(t *testing.T) {
	// Absolute path should be rejected by NewManager.
	err := run([]string{"taskctl", "--file", "/etc/shadow", "list"}, newBuf())
	if err == nil {
		t.Fatal("run with absolute file path: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to initialise task manager") {
		t.Errorf("error should mention manager init failure, got: %v", err)
	}
}

func TestRun_HelpFlag(t *testing.T) {
	err := run([]string{"taskctl", "--help"}, newBuf())
	if err != nil {
		t.Fatalf("run --help: unexpected error: %v", err)
	}
}

func TestRun_HFlag(t *testing.T) {
	err := run([]string{"taskctl", "-h"}, newBuf())
	if err != nil {
		t.Fatalf("run -h: unexpected error: %v", err)
	}
}

func TestRun_AddCommand(t *testing.T) {
	chdirTemp(t)
	err := run([]string{"taskctl", "add", "--priority", "high", "Test task"}, newBuf())
	if err != nil {
		t.Fatalf("run add: unexpected error: %v", err)
	}
}

func TestRun_FileFlag_BeforeCommand(t *testing.T) {
	chdirTemp(t)
	err := run([]string{"taskctl", "--file", "tasks.json", "add", "Test"}, newBuf())
	if err != nil {
		t.Fatalf("run --file before command: unexpected error: %v", err)
	}
}

func TestRun_FileFlag_AfterCommand(t *testing.T) {
	chdirTemp(t)
	err := run([]string{"taskctl", "add", "--file", "tasks.json", "Test"}, newBuf())
	if err != nil {
		t.Fatalf("run --file after command: unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runAdd tests
// ---------------------------------------------------------------------------

func TestRunAdd_Success(t *testing.T) {
	mgr := newTestManager(t)
	err := runAdd(mgr, []string{"--priority", "high", "Buy milk"}, newBuf())
	if err != nil {
		t.Fatalf("runAdd: unexpected error: %v", err)
	}
}

func TestRunAdd_MultiWordTitle(t *testing.T) {
	mgr := newTestManager(t)
	err := runAdd(mgr, []string{"Fix the broken", "CI pipeline"}, newBuf())
	if err != nil {
		t.Fatalf("runAdd multi-word: unexpected error: %v", err)
	}
}

func TestRunAdd_FlagEqualsValueSyntax(t *testing.T) {
	mgr := newTestManager(t)
	// --priority=low uses the flag=value syntax (requires flag.FlagSet support)
	err := runAdd(mgr, []string{"--priority=low", "--due=2099-12-31", "Syntax test"}, newBuf())
	if err != nil {
		t.Fatalf("runAdd with --flag=value syntax: unexpected error: %v", err)
	}
}

func TestRunAdd_MissingTitle(t *testing.T) {
	mgr := newTestManager(t)
	err := runAdd(mgr, []string{"--priority", "medium"}, newBuf())
	if err == nil {
		t.Fatal("runAdd with no title: expected error, got nil")
	}
}

func TestRunAdd_InvalidPriority(t *testing.T) {
	mgr := newTestManager(t)
	err := runAdd(mgr, []string{"--priority", "urgent", "My task"}, newBuf())
	if err == nil {
		t.Fatal("runAdd with invalid priority: expected error, got nil")
	}
}

func TestRunAdd_InvalidDueDate(t *testing.T) {
	mgr := newTestManager(t)
	err := runAdd(mgr, []string{"--due", "not-a-date", "My task"}, newBuf())
	if err == nil {
		t.Fatal("runAdd with invalid due date: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "expected format YYYY-MM-DD") {
		t.Errorf("error should mention YYYY-MM-DD format, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runList tests
// ---------------------------------------------------------------------------

func TestRunList_EmptyStore(t *testing.T) {
	mgr := newTestManager(t)
	err := runList(mgr, []string{}, newBuf())
	if err != nil {
		t.Fatalf("runList empty store: unexpected error: %v", err)
	}
}

func TestRunList_WithTasks(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "high", "Task A"}, newBuf())
	_ = runAdd(mgr, []string{"--priority", "low", "Task B"}, newBuf())

	err := runList(mgr, []string{}, newBuf())
	if err != nil {
		t.Fatalf("runList: unexpected error: %v", err)
	}
}

func TestRunList_FilterByPriority(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "high", "High task"}, newBuf())
	_ = runAdd(mgr, []string{"--priority", "low", "Low task"}, newBuf())

	err := runList(mgr, []string{"--priority", "high"}, newBuf())
	if err != nil {
		t.Fatalf("runList --priority high: unexpected error: %v", err)
	}
}

func TestRunList_FilterByPriorityEqualsValue(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority=medium", "Med task"}, newBuf())

	err := runList(mgr, []string{"--priority=medium"}, newBuf())
	if err != nil {
		t.Fatalf("runList --priority=medium: unexpected error: %v", err)
	}
}

func TestRunList_InvalidPriority(t *testing.T) {
	mgr := newTestManager(t)
	err := runList(mgr, []string{"--priority", "urgent"}, newBuf())
	if err == nil {
		t.Fatal("runList with invalid priority: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "urgent") {
		t.Errorf("error message should mention invalid priority, got: %v", err)
	}
}

func TestRunList_OverdueFlag(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--due", "2000-01-01", "--priority", "high", "Old task"}, newBuf())

	err := runList(mgr, []string{"--overdue"}, newBuf())
	if err != nil {
		t.Fatalf("runList --overdue: unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runDone tests
// ---------------------------------------------------------------------------

func TestRunDone_Success(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "medium", "Task to complete"}, newBuf())

	tasks, _ := mgr.List("", false)
	id := tasks[0].ID

	err := runDone(mgr, []string{intStr(id)}, newBuf())
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
	err := runDone(mgr, []string{}, newBuf())
	if err == nil {
		t.Fatal("runDone with no args: expected error, got nil")
	}
}

func TestRunDone_InvalidID(t *testing.T) {
	mgr := newTestManager(t)
	err := runDone(mgr, []string{"not-a-number"}, newBuf())
	if err == nil {
		t.Fatal("runDone with non-integer ID: expected error, got nil")
	}
}

func TestRunDone_NonExistentID(t *testing.T) {
	mgr := newTestManager(t)
	err := runDone(mgr, []string{"9999"}, newBuf())
	if err == nil {
		t.Fatal("runDone with non-existent ID: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// runDelete tests
// ---------------------------------------------------------------------------

func TestRunDelete_Success(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "low", "Task to delete"}, newBuf())

	tasks, _ := mgr.List("", false)
	id := tasks[0].ID

	err := runDelete(mgr, []string{intStr(id)}, newBuf())
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
	err := runDelete(mgr, []string{}, newBuf())
	if err == nil {
		t.Fatal("runDelete with no args: expected error, got nil")
	}
}

func TestRunDelete_InvalidID(t *testing.T) {
	mgr := newTestManager(t)
	err := runDelete(mgr, []string{"abc"}, newBuf())
	if err == nil {
		t.Fatal("runDelete with non-integer ID: expected error, got nil")
	}
}

func TestRunDelete_NonExistentID(t *testing.T) {
	mgr := newTestManager(t)
	err := runDelete(mgr, []string{"9999"}, newBuf())
	if err == nil {
		t.Fatal("runDelete with non-existent ID: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// runStats tests
// ---------------------------------------------------------------------------

func TestRunStats_EmptyStore(t *testing.T) {
	mgr := newTestManager(t)
	err := runStats(mgr, newBuf())
	if err != nil {
		t.Fatalf("runStats empty store: unexpected error: %v", err)
	}
}

func TestRunStats_WithTasks(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "high", "Task 1"}, newBuf())
	_ = runAdd(mgr, []string{"--priority", "low", "Task 2"}, newBuf())

	tasks, _ := mgr.List("", false)
	_ = runDone(mgr, []string{intStr(tasks[0].ID)}, newBuf())

	err := runStats(mgr, newBuf())
	if err != nil {
		t.Fatalf("runStats with tasks: unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runClear tests
// ---------------------------------------------------------------------------

// TestRunClear_HappyPath verifies that runClear prints the exact expected
// output for a known mixture of done and pending tasks.
func TestRunClear_HappyPath(t *testing.T) {
	mgr := newTestManager(t)
	// Add 4 tasks, mark 2 done.
	_ = runAdd(mgr, []string{"--priority", "high", "Task 1"}, newBuf())
	_ = runAdd(mgr, []string{"--priority", "medium", "Task 2"}, newBuf())
	_ = runAdd(mgr, []string{"--priority", "low", "Task 3"}, newBuf())
	_ = runAdd(mgr, []string{"--priority", "high", "Task 4"}, newBuf())

	all, _ := mgr.List("", false)
	_ = mgr.Complete(all[0].ID)
	_ = mgr.Complete(all[2].ID)

	buf := newBuf()
	if err := runClear(mgr, buf); err != nil {
		t.Errorf("runClear: unexpected error: %v", err)
	}

	want := "Cleared 2 completed tasks. 2 tasks remaining.\n"
	if got := buf.String(); got != want {
		t.Errorf("runClear output:\n  got:  %q\n  want: %q", got, want)
	}
}

// TestRunClear_NothingToClear verifies the output when no tasks are done.
func TestRunClear_NothingToClear(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "medium", "Pending task"}, newBuf())

	buf := newBuf()
	if err := runClear(mgr, buf); err != nil {
		t.Errorf("runClear: unexpected error: %v", err)
	}

	want := "Cleared 0 completed tasks. 1 tasks remaining.\n"
	if got := buf.String(); got != want {
		t.Errorf("runClear output:\n  got:  %q\n  want: %q", got, want)
	}
}

// TestRunClear_EmptyStore verifies the output when the store is empty.
func TestRunClear_EmptyStore(t *testing.T) {
	mgr := newTestManager(t)

	buf := newBuf()
	if err := runClear(mgr, buf); err != nil {
		t.Errorf("runClear on empty store: unexpected error: %v", err)
	}

	want := "Cleared 0 completed tasks. 0 tasks remaining.\n"
	if got := buf.String(); got != want {
		t.Errorf("runClear output:\n  got:  %q\n  want: %q", got, want)
	}
}

// TestRunClear_Error verifies that runClear returns a non-nil error (wrapped
// with "clear:") when Manager.Clear() fails due to a corrupted storage file.
func TestRunClear_Error(t *testing.T) {
	// Change into a fresh temp directory so we can use a relative path.
	chdirTemp(t)

	const relPath = "tasks.json"

	// Write invalid JSON so that load() will fail.
	if err := os.WriteFile(relPath, []byte("{bad json"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	mgr, err := task.NewManager(relPath)
	if err != nil {
		t.Fatalf("NewManager: unexpected error: %v", err)
	}

	runErr := runClear(mgr, newBuf())
	if runErr == nil {
		t.Fatal("runClear with bad file: expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "clear:") {
		t.Errorf("runClear error should be wrapped with \"clear:\", got: %v", runErr)
	}
}
