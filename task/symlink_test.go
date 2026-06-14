package task

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewManagerSymlinkEscape verifies that NewManager rejects a relative path
// where a directory component is a symlink pointing outside the CWD.
// Fixes #87.
func TestNewManagerSymlinkEscape(t *testing.T) {
	// Create an "outside" directory that the symlink will point to.
	outsideDir := t.TempDir()

	// cd into a fresh CWD for this test.
	chdirTemp(t)

	// Create a symlink inside CWD pointing to the outside directory.
	if err := os.Symlink(outsideDir, "evil_link"); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	// NewManager via the symlink must be rejected.
	_, err := NewManager(filepath.Join("evil_link", "tasks.json"))
	if err == nil {
		t.Error("NewManager with symlink escaping CWD: expected error, got nil")
	}
}

// TestNewManagerNonExistentPathAccepted verifies that a plain relative path
// that does not yet exist on disk is still accepted (no symlink involved).
// Fixes #87: EvalSymlinks ErrNotExist must be handled gracefully.
func TestNewManagerNonExistentPathAccepted(t *testing.T) {
	chdirTemp(t)
	mgr, err := NewManager("newdir/tasks.json")
	if err != nil {
		t.Errorf("NewManager with non-existent path: unexpected error: %v", err)
	}
	if mgr == nil {
		t.Error("NewManager with non-existent path: expected non-nil *Manager, got nil")
	}
}
