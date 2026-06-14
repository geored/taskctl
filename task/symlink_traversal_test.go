package task

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// TestNewManagerSymlinkTraversalRejected verifies that NewManager rejects a
// path where a symlink component resolves to a location outside the working
// directory (symlink-based directory traversal). Fixes #87.
func TestNewManagerSymlinkTraversalRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on Windows")
	}

	// Establish a fresh working directory.
	chdirTemp(t)

	// Create an "outside" directory that lives outside the working directory.
	outsideDir := t.TempDir()

	// Create a symlink inside the working directory that points to outsideDir.
	if err := os.Symlink(outsideDir, "escape"); err != nil {
		t.Fatalf("setup: Symlink: %v", err)
	}

	// "escape/tasks.json" passes the ".." and abs-path checks cleanly,
	// but EvalSymlinks should reveal it escapes the working directory.
	_, err := NewManager("escape/tasks.json")
	if err == nil {
		t.Error("NewManager(\"escape/tasks.json\") with symlink pointing outside: expected error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "resolves outside working directory") {
		t.Errorf("expected error to mention 'resolves outside working directory', got: %v", err)
	}
}

// TestNewManagerSymlinkWithinWorkdirAccepted verifies that a symlink that
// stays within the working directory is still accepted. Fixes #87.
func TestNewManagerSymlinkWithinWorkdirAccepted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on Windows")
	}

	// Establish a fresh working directory.
	chdirTemp(t)

	// Create a subdirectory and symlink that both stay within the working dir.
	if err := os.MkdirAll("data", 0755); err != nil {
		t.Fatalf("setup: Mkdir data: %v", err)
	}
	// "link" -> "data" (both inside working dir)
	if err := os.Symlink("data", "link"); err != nil {
		t.Fatalf("setup: Symlink: %v", err)
	}

	// "link/tasks.json" should be accepted because "link" resolves within cwd.
	mgr, err := NewManager("link/tasks.json")
	if err != nil {
		t.Errorf("NewManager(\"link/tasks.json\") with in-bounds symlink: unexpected error: %v", err)
	}
	if mgr == nil {
		t.Error("expected non-nil *Manager, got nil")
	}
}

// TestNewManagerNonExistentPathAccepted verifies that a non-existent but valid
// relative path is still accepted (EvalSymlinks returns ErrNotExist for the
// final component, which should be tolerated). Fixes #87.
func TestNewManagerNonExistentPathAccepted(t *testing.T) {
	chdirTemp(t)

	// "data/tasks.json" doesn't exist yet; that's the normal new-store case.
	mgr, err := NewManager("data/tasks.json")
	if err != nil {
		t.Errorf("NewManager(\"data/tasks.json\") (non-existent): unexpected error: %v", err)
	}
	if mgr == nil {
		t.Error("expected non-nil *Manager, got nil")
	}
}
