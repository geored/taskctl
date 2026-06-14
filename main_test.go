package main

import (
	"bytes"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/geored/taskctl/task"
)

// chdirTemp changes the working directory to a fresh temporary directory for
// the duration of the test and restores the original directory on cleanup.
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
// TestMain — build the binary once so subprocess tests can invoke main().
// ---------------------------------------------------------------------------

var testBinary string

func TestMain(m *testing.M) {
	bin, err := os.CreateTemp("", "taskctl-test-*")
	if err != nil {
		panic("TestMain: failed to create temp file: " + err.Error())
	}
	binPath := bin.Name()
	bin.Close()

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = "/workspace"
	if out, buildErr := cmd.CombinedOutput(); buildErr != nil {
		panic("TestMain: go build failed: " + buildErr.Error() + "\n" + string(out))
	}
	testBinary = binPath

	code := m.Run()
	os.Remove(binPath)
	os.Exit(code)
}

// ---------------------------------------------------------------------------
// main() subprocess tests
// ---------------------------------------------------------------------------

func TestMain_NoArgs(t *testing.T) {
	cmd := exec.Command(testBinary)
	err := cmd.Run()
	if err == nil {
		t.Fatal("main() with no args: expected non-zero exit, got 0")
	}
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 {
		t.Errorf("main() with no args: expected exit 1, got: %v", err)
	}
}

func TestMain_HelpFlag(t *testing.T) {
	cmd := exec.Command(testBinary, "--help")
	if err := cmd.Run(); err != nil {
		t.Fatalf("main() --help: expected exit 0, got: %v", err)
	}
}

func TestMain_VersionFlag(t *testing.T) {
	cmd := exec.Command(testBinary, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("main() --version: expected exit 0, got: %v", err)
	}
	if !strings.Contains(string(out), "taskctl version") {
		t.Errorf("main() --version: output should contain 'taskctl version', got: %q", string(out))
	}
}

func TestMain_UnknownCommand(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(testBinary, "frobnicate")
	cmd.Dir = dir
	err := cmd.Run()
	if err == nil {
		t.Fatal("main() unknown command: expected non-zero exit, got 0")
	}
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 {
		t.Errorf("main() unknown command: expected exit 1, got: %v", err)
	}
}

func TestMain_AddAndList(t *testing.T) {
	dir := t.TempDir()

	cmd := exec.Command(testBinary, "--file", "tasks.json", "add", "My subprocess task")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("main() add: unexpected error: %v", err)
	}

	cmd2 := exec.Command(testBinary, "--file", "tasks.json", "list")
	cmd2.Dir = dir
	out, err := cmd2.Output()
	if err != nil {
		t.Fatalf("main() list: unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "My subprocess task") {
		t.Errorf("main() list: expected task in output, got: %q", string(out))
	}
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

func TestRun_VersionSubcommand(t *testing.T) {
	buf := newBuf()
	err := run([]string{"taskctl", "version"}, buf)
	if err != nil {
		t.Fatalf("run version: unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "taskctl version") {
		t.Errorf("run version: expected 'taskctl version' in output, got: %q", buf.String())
	}
}

func TestRun_VersionDoubleDashFlag(t *testing.T) {
	buf := newBuf()
	err := run([]string{"taskctl", "--version"}, buf)
	if err != nil {
		t.Fatalf("run --version: unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "taskctl version") {
		t.Errorf("run --version: expected 'taskctl version' in output, got: %q", buf.String())
	}
}

func TestRun_VersionSingleDashFlag(t *testing.T) {
	buf := newBuf()
	err := run([]string{"taskctl", "-version"}, buf)
	if err != nil {
		t.Fatalf("run -version: unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "taskctl version") {
		t.Errorf("run -version: expected 'taskctl version' in output, got: %q", buf.String())
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

func TestRun_FileFlag_DoubleDashEquals(t *testing.T) {
	chdirTemp(t)
	err := run([]string{"taskctl", "--file=tasks.json", "add", "Equals test"}, newBuf())
	if err != nil {
		t.Fatalf("run --file=tasks.json: unexpected error: %v", err)
	}
}

func TestRun_FileFlag_SingleDashEquals(t *testing.T) {
	chdirTemp(t)
	err := run([]string{"taskctl", "-file=tasks.json", "add", "Single dash equals test"}, newBuf())
	if err != nil {
		t.Fatalf("run -file=tasks.json: unexpected error: %v", err)
	}
}

func TestRun_FileFlag_SingleDashSpace(t *testing.T) {
	chdirTemp(t)
	err := run([]string{"taskctl", "-file", "tasks.json", "add", "Single dash space test"}, newBuf())
	if err != nil {
		t.Fatalf("run -file tasks.json: unexpected error: %v", err)
	}
}

func TestRun_FilePathTooLong(t *testing.T) {
	longPath := strings.Repeat("a", maxFilePathLen+1)
	err := run([]string{"taskctl", "--file", longPath, "list"}, newBuf())
	if err == nil {
		t.Fatal("run with overlong --file path: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum length") {
		t.Errorf("error should mention exceeds maximum length, got: %v", err)
	}
}

func TestRun_FileFlagOnly_NoCommandAfter(t *testing.T) {
	chdirTemp(t)
	err := run([]string{"taskctl", "--file=tasks.json"}, newBuf())
	if err == nil {
		t.Fatal("run with --file but no command: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no command specified") {
		t.Errorf("error should mention 'no command specified', got: %v", err)
	}
}

func TestRun_ListCommand(t *testing.T) {
	chdirTemp(t)
	err := run([]string{"taskctl", "list"}, newBuf())
	if err != nil {
		t.Fatalf("run list: unexpected error: %v", err)
	}
}

func TestRun_DoneCommand(t *testing.T) {
	chdirTemp(t)
	if err := run([]string{"taskctl", "add", "Task for done"}, newBuf()); err != nil {
		t.Fatalf("run add: unexpected error: %v", err)
	}
	mgr, err := task.NewManager("tasks.json")
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	tasks, err := mgr.List("", false)
	if err != nil || len(tasks) == 0 {
		t.Fatalf("expected at least one task, err=%v", err)
	}
	id := strconv.Itoa(tasks[0].ID)
	if err := run([]string{"taskctl", "done", id}, newBuf()); err != nil {
		t.Fatalf("run done: unexpected error: %v", err)
	}
}

func TestRun_DeleteCommand(t *testing.T) {
	chdirTemp(t)
	if err := run([]string{"taskctl", "add", "Task for delete"}, newBuf()); err != nil {
		t.Fatalf("run add: unexpected error: %v", err)
	}
	mgr, err := task.NewManager("tasks.json")
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	tasks, err := mgr.List("", false)
	if err != nil || len(tasks) == 0 {
		t.Fatalf("expected at least one task, err=%v", err)
	}
	id := strconv.Itoa(tasks[0].ID)
	if err := run([]string{"taskctl", "delete", id}, newBuf()); err != nil {
		t.Fatalf("run delete: unexpected error: %v", err)
	}
}

func TestRun_StatsCommand(t *testing.T) {
	chdirTemp(t)
	err := run([]string{"taskctl", "stats"}, newBuf())
	if err != nil {
		t.Fatalf("run stats: unexpected error: %v", err)
	}
}

func TestRun_ClearCommand(t *testing.T) {
	chdirTemp(t)
	err := run([]string{"taskctl", "clear"}, newBuf())
	if err != nil {
		t.Fatalf("run clear: unexpected error: %v", err)
	}
}

func TestRun_DispatchReturnsError(t *testing.T) {
	chdirTemp(t)
	err := run([]string{"taskctl", "done", "9999"}, newBuf())
	if err == nil {
		t.Fatal("run done 9999: expected error, got nil")
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
	oversized := strings.Repeat("x", 100)
	err := runAdd(mgr, []string{"--due", oversized, "My task"}, newBuf())
	if err == nil {
		t.Fatal("runAdd with oversized --due value: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum length") {
		t.Errorf("error should mention exceeds maximum length, got: %v", err)
	}
}

func TestRunAdd_EmptyTitleAfterTrim(t *testing.T) {
	mgr := newTestManager(t)
	err := runAdd(mgr, []string{"   "}, newBuf())
	if err == nil {
		t.Fatal("runAdd with whitespace-only title: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("error should mention empty title, got: %v", err)
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

func TestRunList_DoneTaskShowsX(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "medium", "Done task"}, newBuf())
	tasks, _ := mgr.List("", false)
	_ = mgr.Complete(tasks[0].ID)

	buf := newBuf()
	err := runList(mgr, []string{}, buf)
	if err != nil {
		t.Fatalf("runList with done task: unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "[x]") {
		t.Errorf("runList: expected [x] marker for done task, got: %q", buf.String())
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

func TestRunClear_HappyPath(t *testing.T) {
	mgr := newTestManager(t)
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

func TestRunClear_SingleTask(t *testing.T) {
	mgr := newTestManager(t)
	_ = runAdd(mgr, []string{"--priority", "high", "Only task"}, newBuf())

	all, _ := mgr.List("", false)
	_ = mgr.Complete(all[0].ID)

	buf := newBuf()
	if err := runClear(mgr, buf); err != nil {
		t.Errorf("runClear single task: unexpected error: %v", err)
	}

	want := "Cleared 1 completed task. 0 tasks remaining.\n"
	if got := buf.String(); got != want {
		t.Errorf("runClear single task output:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestRunClear_Error(t *testing.T) {
	chdirTemp(t)

	const relPath = "tasks.json"

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
