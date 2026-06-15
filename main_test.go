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

// TestRun_TraversalPathRejected verifies that a --file value containing a
// directory traversal sequence (e.g. "../etc/passwd") is rejected.
// The authoritative check lives in NewManager (Fixes #83): main.go delegates
// entirely to NewManager rather than duplicating a weaker strings.Contains check.
func TestRun_TraversalPathRejected(t *testing.T) {
	err := run([]string{"taskctl", "--file", "../etc/passwd", "list"}, newBuf())
	if err == nil {
		t.Fatal("run with traversal path: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to initialise task manager") {
		t.Errorf("traversal path: error should mention manager init failure, got: %v", err)
	}
}

// TestRun_FooDotDotBarAccepted verifies that a filename like "foo..bar"
// (which contains ".." as a substring but is NOT a traversal) is accepted.
// The old strings.Contains check in main.go would incorrectly reject this;
// the authoritative NewManager check uses filepath.Clean + HasPrefix which
// correctly allows it (Fixes #83).
func TestRun_FooDotDotBarAccepted(t *testing.T) {
	chdirTemp(t)
	// "foo..bar" is a valid relative filename — it must not be rejected as
	// traversal. The call may still return an error for other reasons (e.g.
	// empty command args), but any error must not mention "traversal".
	err := run([]string{"taskctl", "--file", "foo..bar", "list"}, newBuf())
	if err != nil && strings.Contains(err.Error(), "traversal") {
		t.Errorf("foo..bar should be accepted as a valid filename, got: %v", err)
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

func TestRunAdd_DueDateExceedsMaxLength(t *testing.T) {
	mgr := newTestManager(t)
	// Use a 100-character string to exceed the 10-character YYYY-MM-DD limit.
	oversized := strings.Repeat("x", 100)
	err := runAdd(mgr, []string{"--due", oversized, "My task"}, newBuf())
	if err == nil {
		t.Fatal("runAdd with oversized --due value: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum length") {
		t.Errorf("error should mention exceeds maximum length, got: %v", err)
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

	err := runDone(mgr, []string{strconv.Itoa(id)}, newBuf())
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
func TestRunDone_ZeroID(t *testing.T) {
	mgr := newTestManager(t)
	err := runDone(mgr, []string{"0"}, newBuf())
	if err == nil {
		t.Fatal("runDone with zero ID: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "task ID must be a positive integer") {
		t.Errorf("runDone zero ID: wrong error message, got: %v", err)
	}
}

func TestRunDone_NegativeID(t *testing.T) {
	mgr := newTestManager(t)
	err := runDone(mgr, []string{"--", "-5"}, newBuf())
	if err == nil {
		t.Fatal("runDone with negative ID: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "task ID must be a positive integer") {
		t.Errorf("runDone negative ID: wrong error message, got: %v", err)
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

	err := runDelete(mgr, []string{strconv.Itoa(id)}, newBuf())
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
func TestRunDelete_ZeroID(t *testing.T) {
	mgr := newTestManager(t)
	err := runDelete(mgr, []string{"0"}, newBuf())
	if err == nil {
		t.Fatal("runDelete with zero ID: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "task ID must be a positive integer") {
		t.Errorf("runDelete zero ID: wrong error message, got: %v", err)
	}
}

func TestRunDelete_NegativeID(t *testing.T) {
	mgr := newTestManager(t)
	err := runDelete(mgr, []string{"--", "-5"}, newBuf())
	if err == nil {
		t.Fatal("runDelete with negative ID: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "task ID must be a positive integer") {
		t.Errorf("runDelete negative ID: wrong error message, got: %v", err)
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
	_ = runDone(mgr, []string{strconv.Itoa(tasks[0].ID)}, newBuf())

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

	want := "Cleared 0 completed tasks. 1 task remaining.\n"
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

// ---------------------------------------------------------------------------
// --file flag placement and edge-case tests (Fixes #31)
// ---------------------------------------------------------------------------

// TestRun_FileFlag_EqualsSign_BeforeCommand verifies that --file=<path>
// (equals-sign syntax) before the subcommand is parsed correctly.
func TestRun_FileFlag_EqualsSign_BeforeCommand(t *testing.T) {
	chdirTemp(t)
	err := run([]string{"taskctl", "--file=work.json", "add", "EqBefore"}, newBuf())
	if err != nil {
		t.Fatalf("--file=work.json before subcommand: unexpected error: %v", err)
	}
}

// TestRun_FileFlag_EqualsSign_AfterCommand verifies that --file=<path>
// (equals-sign syntax) after the subcommand is parsed correctly.
func TestRun_FileFlag_EqualsSign_AfterCommand(t *testing.T) {
	chdirTemp(t)
	err := run([]string{"taskctl", "add", "--file=work.json", "EqAfter"}, newBuf())
	if err != nil {
		t.Fatalf("--file=work.json after subcommand: unexpected error: %v", err)
	}
}

// TestRun_FileFlag_SingleDash_BeforeCommand verifies that -file <path>
// (single-dash space-separated) before the subcommand is accepted.
func TestRun_FileFlag_SingleDash_BeforeCommand(t *testing.T) {
	chdirTemp(t)
	err := run([]string{"taskctl", "-file", "work.json", "add", "SingleDash"}, newBuf())
	if err != nil {
		t.Fatalf("-file work.json before subcommand: unexpected error: %v", err)
	}
}

// TestRun_FileFlag_SingleDashEquals_BeforeCommand verifies that -file=<path>
// (single-dash equals-sign) before the subcommand is parsed correctly.
func TestRun_FileFlag_SingleDashEquals_BeforeCommand(t *testing.T) {
	chdirTemp(t)
	err := run([]string{"taskctl", "-file=work.json", "add", "SingleDashEq"}, newBuf())
	if err != nil {
		t.Fatalf("-file=work.json before subcommand: unexpected error: %v", err)
	}
}

// TestRun_FileFlag_CorrectFileUsed verifies that the file named by --file is
// actually used for storage — a round-trip test: write to custom file via
// flag-before-subcommand, read it back via flag-after-subcommand, confirm the
// default tasks.json was never created.
func TestRun_FileFlag_CorrectFileUsed(t *testing.T) {
	chdirTemp(t)

	// Add a task to custom.json using --file before the subcommand.
	if err := run([]string{"taskctl", "--file", "custom.json", "add", "Custom file task"}, newBuf()); err != nil {
		t.Fatalf("add to custom.json: %v", err)
	}

	// List from custom.json using --file after the subcommand.
	buf := newBuf()
	if err := run([]string{"taskctl", "list", "--file", "custom.json"}, buf); err != nil {
		t.Fatalf("list from custom.json: %v", err)
	}
	if !strings.Contains(buf.String(), "Custom file task") {
		t.Errorf("expected 'Custom file task' in output, got: %s", buf.String())
	}

	// Verify the default tasks.json was NOT created.
	if _, err := os.Stat("tasks.json"); err == nil {
		t.Error("tasks.json should not exist when --file custom.json was used throughout")
	}
}

// TestRun_FileFlag_EqualsSign_CorrectFileUsed verifies the equals-sign syntax
// also routes to the correct custom file (round-trip).
func TestRun_FileFlag_EqualsSign_CorrectFileUsed(t *testing.T) {
	chdirTemp(t)

	if err := run([]string{"taskctl", "--file=eq.json", "add", "EqSyntax task"}, newBuf()); err != nil {
		t.Fatalf("add to eq.json: %v", err)
	}

	buf := newBuf()
	if err := run([]string{"taskctl", "--file=eq.json", "list"}, buf); err != nil {
		t.Fatalf("list from eq.json: %v", err)
	}
	if !strings.Contains(buf.String(), "EqSyntax task") {
		t.Errorf("expected 'EqSyntax task' in output, got: %s", buf.String())
	}
}

// TestRun_FileFlag_PathTooLong verifies that a --file value exceeding 4096
// characters is rejected before NewManager is called (Fixes #76 / Issue #31).
func TestRun_FileFlag_PathTooLong(t *testing.T) {
	longPath := strings.Repeat("a", maxFilePathLen+1)
	err := run([]string{"taskctl", "--file", longPath, "list"}, newBuf())
	if err == nil {
		t.Fatal("expected error for file path > maxFilePathLen, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum length") {
		t.Errorf("error should mention 'exceeds maximum length', got: %v", err)
	}
}

// TestRun_FileFlag_PathExactlyMaxLen verifies that a --file value of exactly
// maxFilePathLen characters is not rejected by the length check itself.
// (NewManager may still reject it for other reasons; we only test the length gate.)
func TestRun_FileFlag_PathExactlyMaxLen(t *testing.T) {
	// Use a path that is exactly maxFilePathLen chars. It will fail at
	// NewManager (path too long for OS), but NOT with "exceeds maximum length".
	exactPath := strings.Repeat("a", maxFilePathLen)
	err := run([]string{"taskctl", "--file", exactPath, "list"}, newBuf())
	if err != nil && strings.Contains(err.Error(), "exceeds maximum length") {
		t.Errorf("a %d-char path should NOT trigger the length gate, got: %v", maxFilePathLen, err)
	}
}

// TestRun_FileFlag_TraversalRejected verifies that a --file path containing
// directory traversal ("..") is rejected. The rejection is delegated to
// NewManager (single authoritative layer, per design).
func TestRun_FileFlag_TraversalRejected(t *testing.T) {
	chdirTemp(t)
	err := run([]string{"taskctl", "--file", "../sibling.json", "list"}, newBuf())
	if err == nil {
		t.Fatal("expected error for traversal path '../sibling.json', got nil")
	}
	if !strings.Contains(err.Error(), "failed to initialise task manager") {
		t.Errorf("traversal error should be wrapped with 'failed to initialise task manager', got: %v", err)
	}
}

// TestRun_FileFlag_AbsolutePathRejected ensures the existing absolute-path
// rejection (delegated to NewManager) is exercised through the full run() path
// including the pre-scan loop (complements TestRun_InvalidFilePath).
func TestRun_FileFlag_AbsolutePathRejected(t *testing.T) {
	err := run([]string{"taskctl", "--file", "/tmp/evil.json", "list"}, newBuf())
	if err == nil {
		t.Fatal("expected error for absolute --file path, got nil")
	}
	if !strings.Contains(err.Error(), "failed to initialise task manager") {
		t.Errorf("error should wrap manager init failure, got: %v", err)
	}
}

// TestRun_FileFlag_MissingValue verifies that `--file` with no following value
// (i.e. it is the last argument before the subcommand in a way that the
// subcommand name gets consumed as the filename) produces an error.
// This exercises the edge case where args[i+1] is the subcommand name.
func TestRun_FileFlag_MissingValue(t *testing.T) {
	chdirTemp(t)
	// "--file" is last; no value follows → no subcommand left → error.
	err := run([]string{"taskctl", "--file"}, newBuf())
	if err == nil {
		t.Fatal("expected error when --file has no value argument, got nil")
	}
}

// TestRun_FileFlag_BeforeAndAfterConsistent verifies that placing --file
// before vs after the subcommand produces the same observable result:
// both writes end up in the same file and reads return the same data.
func TestRun_FileFlag_BeforeAndAfterConsistent(t *testing.T) {
	chdirTemp(t)

	// Write using --file BEFORE subcommand.
	if err := run([]string{"taskctl", "--file", "shared.json", "add", "Shared task"}, newBuf()); err != nil {
		t.Fatalf("add (flag before): %v", err)
	}

	// Read using --file AFTER subcommand — should see the same task.
	buf := newBuf()
	if err := run([]string{"taskctl", "list", "--file", "shared.json"}, buf); err != nil {
		t.Fatalf("list (flag after): %v", err)
	}
	if !strings.Contains(buf.String(), "Shared task") {
		t.Errorf("flag-before and flag-after do not share storage: output=%s", buf.String())
	}
}

// ---------------------------------------------------------------------------
// Uncovered run() branches: version flag/subcommand, no-command-after-file
// ---------------------------------------------------------------------------

// TestRun_VersionFlag verifies that `taskctl --version` prints version info
// and returns nil (no error). Fixes #55.
func TestRun_VersionFlag(t *testing.T) {
	buf := newBuf()
	err := run([]string{"taskctl", "--version"}, buf)
	if err != nil {
		t.Fatalf("run --version: unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "taskctl version") {
		t.Errorf("run --version: expected output to contain 'taskctl version', got: %q", buf.String())
	}
}

// TestRun_VersionFlagSingleDash verifies that `taskctl -version` also works.
func TestRun_VersionFlagSingleDash(t *testing.T) {
	buf := newBuf()
	err := run([]string{"taskctl", "-version"}, buf)
	if err != nil {
		t.Fatalf("run -version: unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "taskctl version") {
		t.Errorf("run -version: expected output to contain 'taskctl version', got: %q", buf.String())
	}
}

// TestRun_VersionSubcommand verifies that `taskctl version` prints version
// info and returns nil. Fixes #55 / #74.
func TestRun_VersionSubcommand(t *testing.T) {
	buf := newBuf()
	err := run([]string{"taskctl", "version"}, buf)
	if err != nil {
		t.Fatalf("run version: unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "taskctl version") {
		t.Errorf("run version: expected output to contain 'taskctl version', got: %q", buf.String())
	}
}

// TestRun_NoCommandAfterFileFlag verifies that stripping --file <val> and
// leaving no remaining args produces a "no command specified" error. Fixes #55.
func TestRun_NoCommandAfterFileFlag(t *testing.T) {
	chdirTemp(t)
	// After pre-scan strips "--file tasks.json", remaining is empty → error.
	err := run([]string{"taskctl", "--file", "tasks.json"}, newBuf())
	if err == nil {
		t.Fatal("run --file <val> with no subcommand: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no command specified") {
		t.Errorf("expected 'no command specified' error, got: %v", err)
	}
}

// TestRun_NoCommandAfterFileFlagEqualsSign verifies the equals-sign variant
// also leaves remaining empty and returns "no command specified".
func TestRun_NoCommandAfterFileFlagEqualsSign(t *testing.T) {
	chdirTemp(t)
	err := run([]string{"taskctl", "--file=tasks.json"}, newBuf())
	if err == nil {
		t.Fatal("run --file=<val> with no subcommand: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no command specified") {
		t.Errorf("expected 'no command specified' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// run() integration tests for done/delete/stats/clear subcommands (Fixes #55)
// These cover the switch-case dispatch lines in run() that were previously
// only tested via direct runXxx() calls, leaving the switch arms uncovered.
// ---------------------------------------------------------------------------

// TestRun_DoneSubcommand exercises the "done" switch arm in run().
func TestRun_DoneSubcommand(t *testing.T) {
	chdirTemp(t)
	// Add a task first.
	if err := run([]string{"taskctl", "add", "Done subcommand task"}, newBuf()); err != nil {
		t.Fatalf("add: %v", err)
	}
	mgr, _ := task.NewManager("tasks.json")
	tasks, _ := mgr.List("", false)
	id := strconv.Itoa(tasks[0].ID)

	err := run([]string{"taskctl", "done", id}, newBuf())
	if err != nil {
		t.Fatalf("run done: unexpected error: %v", err)
	}
}

// TestRun_DeleteSubcommand exercises the "delete" switch arm in run().
func TestRun_DeleteSubcommand(t *testing.T) {
	chdirTemp(t)
	if err := run([]string{"taskctl", "add", "Delete subcommand task"}, newBuf()); err != nil {
		t.Fatalf("add: %v", err)
	}
	mgr, _ := task.NewManager("tasks.json")
	tasks, _ := mgr.List("", false)
	id := strconv.Itoa(tasks[0].ID)

	err := run([]string{"taskctl", "delete", id}, newBuf())
	if err != nil {
		t.Fatalf("run delete: unexpected error: %v", err)
	}
}

// TestRun_StatsSubcommand exercises the "stats" switch arm in run().
func TestRun_StatsSubcommand(t *testing.T) {
	chdirTemp(t)
	if err := run([]string{"taskctl", "add", "Stats subcommand task"}, newBuf()); err != nil {
		t.Fatalf("add: %v", err)
	}
	buf := newBuf()
	err := run([]string{"taskctl", "stats"}, buf)
	if err != nil {
		t.Fatalf("run stats: unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "Total tasks") {
		t.Errorf("run stats: expected 'Total tasks' in output, got: %q", buf.String())
	}
}

// TestRun_ClearSubcommand exercises the "clear" switch arm in run().
func TestRun_ClearSubcommand(t *testing.T) {
	chdirTemp(t)
	if err := run([]string{"taskctl", "add", "Clear subcommand task"}, newBuf()); err != nil {
		t.Fatalf("add: %v", err)
	}
	buf := newBuf()
	err := run([]string{"taskctl", "clear"}, buf)
	if err != nil {
		t.Fatalf("run clear: unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Missing branch coverage: runAdd empty-after-trim, runList display [x],
// runAdd/runDone/runDelete/runStats/runClear error paths, runClear singular
// ---------------------------------------------------------------------------

// TestRunAdd_WhitespaceTitleOnly covers the "title must not be empty" branch
// after TrimSpace produces an empty string.
func TestRunAdd_WhitespaceTitleOnly(t *testing.T) {
	mgr := newTestManager(t)
	err := runAdd(mgr, []string{"   "}, newBuf())
	if err == nil {
		t.Fatal("runAdd with whitespace-only title: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "title must not be empty") {
		t.Errorf("wrong error: %v", err)
	}
}

// TestRunList_DisplaysDoneMarker covers the "[x]" branch in runList display loop.
func TestRunList_DisplaysDoneMarker(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "high", "Done task"}, newBuf())
	tasks, _ := mgr.List("", false)
	_ = mgr.Complete(tasks[0].ID)

	buf := newBuf()
	err := runList(mgr, []string{}, buf)
	if err != nil {
		t.Fatalf("runList: unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "[x]") {
		t.Errorf("runList: expected '[x]' marker for done task, got: %q", buf.String())
	}
}

// TestRunClear_SingularTask covers the "task" (singular) branch in runClear.
func TestRunClear_SingularTask(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "medium", "Solo task"}, newBuf())
	tasks, _ := mgr.List("", false)
	_ = mgr.Complete(tasks[0].ID)

	buf := newBuf()
	if err := runClear(mgr, buf); err != nil {
		t.Fatalf("runClear singular: unexpected error: %v", err)
	}
	want := "Cleared 1 completed task. 0 tasks remaining.\n"
	if got := buf.String(); got != want {
		t.Errorf("runClear singular:\n  got:  %q\n  want: %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// Bug #130: --file / -file with no following value must return a clear error
// ---------------------------------------------------------------------------

// TestRun_FileFlag_BareAtEnd_DoubleDash verifies that `taskctl --file` (bare,
// last token) returns "--file requires a value" instead of "unknown command: --file".
func TestRun_FileFlag_BareAtEnd_DoubleDash(t *testing.T) {
	err := run([]string{"taskctl", "--file"}, newBuf())
	if err == nil {
		t.Fatal("expected error when --file has no value, got nil")
	}
	if !strings.Contains(err.Error(), "--file requires a value") {
		t.Errorf("expected '--file requires a value', got: %v", err)
	}
}

// TestRun_FileFlag_BareAtEnd_SingleDash verifies that `taskctl -file` (bare,
// last token) returns "--file requires a value" instead of "unknown command: -file".
func TestRun_FileFlag_BareAtEnd_SingleDash(t *testing.T) {
	err := run([]string{"taskctl", "-file"}, newBuf())
	if err == nil {
		t.Fatal("expected error when -file has no value, got nil")
	}
	if !strings.Contains(err.Error(), "--file requires a value") {
		t.Errorf("expected '--file requires a value', got: %v", err)
	}
}

// TestRun_FileFlag_ConsumesSubcommand verifies that `taskctl --file list`
// (where list is incorrectly consumed as the filename) returns an error
// indicating --file requires a value rather than the misleading "no command specified".
func TestRun_FileFlag_ConsumesSubcommand(t *testing.T) {
	// When --file eats "list" as its value, remaining becomes empty.
	// The fix should detect the bare --file before the subcommand eats it.
	// Since "list" gets consumed as the file value, we get "no command specified";
	// that's the footgun documented in #130. The bare-at-end case above is the
	// primary fix; this test documents the footgun.
	err := run([]string{"taskctl", "--file", "list"}, newBuf())
	if err == nil {
		t.Fatal("expected error for taskctl --file list (list consumed as filename), got nil")
	}
	// At minimum an error must be returned; the exact message documents the footgun.
	if err.Error() == "" {
		t.Errorf("expected a non-empty error message, got empty string")
	}
}

// TestRun_FileFlag_EqualsSignNoRegression verifies that --file=tasks.json
// (equals-sign syntax) still works correctly after the fix (no regression).
func TestRun_FileFlag_EqualsSignNoRegression(t *testing.T) {
	chdirTemp(t)
	buf := newBuf()
	err := run([]string{"taskctl", "--file=tasks.json", "list"}, buf)
	if err != nil {
		t.Fatalf("--file=tasks.json list: unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestRunStats_CompletionRateRounding verifies that completion rate is
// rounded rather than truncated (Issue #134).
// ---------------------------------------------------------------------------

func TestRunStats_CompletionRateRounding(t *testing.T) {
	// 2 of 3 completed: truncated integer gives 66%, rounded gives 67%
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"Task A"}, newBuf())
	_ = runAdd(mgr, []string{"Task B"}, newBuf())
	_ = runAdd(mgr, []string{"Task C"}, newBuf())

	tasks, _ := mgr.List("", false)
	_ = runDone(mgr, []string{strconv.Itoa(tasks[0].ID)}, newBuf())
	_ = runDone(mgr, []string{strconv.Itoa(tasks[1].ID)}, newBuf())

	buf := newBuf()
	if err := runStats(mgr, buf); err != nil {
		t.Fatalf("runStats: unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Completion rate: 67%") {
		t.Errorf("expected 'Completion rate: 67%%' (rounded), got: %q", out)
	}
}
