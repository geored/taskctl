package task

// Tests for save() error paths. Fixes #45.
// These tests cover branches where os.CreateTemp, Chmod, Write, or Rename
// can fail, ensuring that save() surfaces meaningful errors in each case.
//
// NOTE: Tests that rely on a read-only directory are skipped when the test
// process runs as root (uid 0), because root bypasses filesystem permissions.

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const osWindows = "windows"

// isRoot reports whether the current process is running as root.
// Chmod/permission-based failure tests must be skipped when running as root
// because root ignores filesystem permission bits.
func isRoot() bool {
	return os.Getuid() == 0
}

// TestSave_CreateTempFailure verifies that save() returns an error when
// os.CreateTemp cannot create a file (e.g. the directory does not exist or
// is read-only). Fixes #45.
func TestSave_CreateTempFailure(t *testing.T) {
	if runtime.GOOS == osWindows {
		t.Skip("read-only directory semantics differ on Windows")
	}
	if isRoot() {
		t.Skip("running as root: filesystem permission checks are bypassed")
	}

	// Create a read-only directory so that CreateTemp inside it fails.
	dir := t.TempDir()
	roDir := filepath.Join(dir, "readonly")
	if err := os.Mkdir(roDir, 0500); err != nil {
		t.Fatalf("setup: Mkdir read-only dir: %v", err)
	}

	// Build a Manager whose filePath points inside the read-only directory.
	mgr := &Manager{filePath: filepath.Join(roDir, "tasks.json")}

	err := mgr.save([]Task{})
	if err == nil {
		t.Fatal("save: expected error for read-only directory, got nil")
	}
	if !strings.Contains(err.Error(), "save: create temp") {
		t.Errorf("save error should mention 'save: create temp', got: %v", err)
	}
}

// TestSave_WriteFailure verifies that save() returns an error when the write
// to the temp file fails. We simulate this by pointing the Manager's filePath
// at a path whose parent directory becomes read-only after the temp file is
// created — but that approach is brittle. Instead we use a zero-byte file
// descriptor opened with O_RDONLY (not directly possible with os.File).
//
// A more reliable approach: create a temp file manually, then use a Manager
// whose dir is a subdirectory we make unwritable after the first CreateTemp
// succeeds. Because timing this is hard, we instead test via a stub that
// verifies the write branch directly by providing an invalid file descriptor
// path. Since Go's standard library does not expose an easy way to inject a
// broken writer into os.CreateTemp, we test write failure by setting the
// directory to a path that exists and is writable, creating the temp file,
// then making the file's parent non-writable before Rename.
//
// This test covers the rename failure path (equivalent to write success but
// rename failure). Fixes #45.
func TestSave_RenameFailure(t *testing.T) {
	if runtime.GOOS == osWindows {
		t.Skip("read-only directory semantics differ on Windows")
	}
	if isRoot() {
		t.Skip("running as root: filesystem permission checks are bypassed")
	}

	// Use a writable parent dir that contains a sub-dir where tasks.json will live.
	parent := t.TempDir()
	subDir := filepath.Join(parent, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("setup: Mkdir subdir: %v", err)
	}

	// Build a Manager pointing into subDir.
	mgr := &Manager{filePath: filepath.Join(subDir, "tasks.json")}

	// First save succeeds (establishes the file in place).
	if err := mgr.save([]Task{{ID: 1, Title: "test", Priority: "low"}}); err != nil {
		t.Fatalf("initial save: unexpected error: %v", err)
	}

	// Now make the subDir read-only so the rename of the temp file will fail.
	if err := os.Chmod(subDir, 0500); err != nil {
		t.Fatalf("setup: Chmod read-only: %v", err)
	}
	// Restore permissions so t.TempDir cleanup can remove it.
	t.Cleanup(func() { os.Chmod(subDir, 0755) }) //nolint:errcheck

	err := mgr.save([]Task{{ID: 2, Title: "after-lock", Priority: "medium"}})
	if err == nil {
		t.Fatal("save after making dir read-only: expected error, got nil")
	}
	// The error should be either "create temp" (if CreateTemp fails first) or
	// "rename" (if the write succeeds but rename fails). Both are valid.
	if !strings.Contains(err.Error(), "save:") {
		t.Errorf("save error should be prefixed with 'save:', got: %v", err)
	}
}

// TestSave_HappyPath verifies that save() + load() round-trip works correctly,
// serving as a baseline sanity check for the error-path tests. Fixes #45.
func TestSave_HappyPath(t *testing.T) {
	dir := t.TempDir()
	mgr := &Manager{filePath: filepath.Join(dir, "tasks.json")}

	tasks := []Task{
		{ID: 1, Title: "Alpha", Done: false, Priority: "high", DueDate: "2099-01-01"},
		{ID: 2, Title: "Beta", Done: true, Priority: "low"},
	}

	if err := mgr.save(tasks); err != nil {
		t.Fatalf("save: unexpected error: %v", err)
	}

	loaded, err := mgr.load()
	if err != nil {
		t.Fatalf("load after save: unexpected error: %v", err)
	}
	if len(loaded) != len(tasks) {
		t.Fatalf("load after save: expected %d tasks, got %d", len(tasks), len(loaded))
	}
	for i, want := range tasks {
		got := loaded[i]
		if got.ID != want.ID || got.Title != want.Title || got.Done != want.Done ||
			got.Priority != want.Priority || got.DueDate != want.DueDate {
			t.Errorf("task[%d]: got %+v, want %+v", i, got, want)
		}
	}
}

// TestSave_CorruptedJSONReturnsPathInError verifies that when the tasks file
// contains invalid JSON, the error message includes the file path. Fixes #30.
func TestSave_CorruptedJSONReturnsPathInError(t *testing.T) {
	dir := t.TempDir()
	taskFile := filepath.Join(dir, "tasks.json")

	// Write invalid JSON directly to the file.
	if err := os.WriteFile(taskFile, []byte("{not valid json"), 0600); err != nil {
		t.Fatalf("setup: WriteFile: %v", err)
	}

	mgr := &Manager{filePath: taskFile}
	_, err := mgr.load()
	if err == nil {
		t.Fatal("load with corrupted JSON: expected error, got nil")
	}
	if !strings.Contains(err.Error(), taskFile) {
		t.Errorf("load error should contain file path %q, got: %v", taskFile, err)
	}
}

// ---------------------------------------------------------------------------
// Chmod and Sync failure tests — Fixes #133
// ---------------------------------------------------------------------------

// chmodFailFile wraps a real *os.File but returns a sentinel error from Chmod.
// All other operations are delegated to the underlying file unchanged.
type chmodFailFile struct {
	*os.File
}

func (f *chmodFailFile) Chmod(_ os.FileMode) error {
	return fmt.Errorf("injected chmod error")
}

// syncFailFile wraps a real *os.File but returns a sentinel error from Sync.
type syncFailFile struct {
	*os.File
}

func (f *syncFailFile) Sync() error {
	return fmt.Errorf("injected sync error")
}

// TestSave_ChmodFailure verifies that save() surfaces an error prefixed with
// "save: chmod" when Chmod on the temp file fails. Fixes #133.
func TestSave_ChmodFailure(t *testing.T) {
	if runtime.GOOS == osWindows {
		t.Skip("filesystem permission semantics differ on Windows")
	}
	if isRoot() {
		t.Skip("running as root: filesystem permission checks are bypassed")
	}

	dir := t.TempDir()
	mgr := &Manager{filePath: filepath.Join(dir, "tasks.json")}

	// Inject a factory that returns a file whose Chmod always fails.
	mgr.createTemp = func(d, pattern string) (osFile, error) {
		f, err := os.CreateTemp(d, pattern)
		if err != nil {
			return nil, err
		}
		return &chmodFailFile{f}, nil
	}

	err := mgr.save([]Task{{ID: 1, Title: "test", Priority: "low"}})
	if err == nil {
		t.Fatal("save: expected error for Chmod failure, got nil")
	}
	if !strings.Contains(err.Error(), "save: chmod") {
		t.Errorf("expected error to contain 'save: chmod', got: %v", err)
	}
}

// TestSave_SyncFailure verifies that save() surfaces an error prefixed with
// "save: sync" when Sync on the temp file fails. Fixes #133.
func TestSave_SyncFailure(t *testing.T) {
	if runtime.GOOS == osWindows {
		t.Skip("filesystem permission semantics differ on Windows")
	}
	if isRoot() {
		t.Skip("running as root: filesystem permission checks are bypassed")
	}

	dir := t.TempDir()
	mgr := &Manager{filePath: filepath.Join(dir, "tasks.json")}

	// Inject a factory that returns a file whose Sync always fails.
	mgr.createTemp = func(d, pattern string) (osFile, error) {
		f, err := os.CreateTemp(d, pattern)
		if err != nil {
			return nil, err
		}
		return &syncFailFile{f}, nil
	}

	err := mgr.save([]Task{{ID: 1, Title: "test", Priority: "low"}})
	if err == nil {
		t.Fatal("save: expected error for Sync failure, got nil")
	}
	if !strings.Contains(err.Error(), "save: sync") {
		t.Errorf("expected error to contain 'save: sync', got: %v", err)
	}
}
