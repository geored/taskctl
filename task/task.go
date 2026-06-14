package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// dateLayout is the canonical date format accepted and displayed by taskctl.
const dateLayout = "2006-01-02"

// maxTitleLength is the maximum number of Unicode characters (runes) allowed
// in a task title. Titles longer than this are rejected at the Add boundary
// to prevent unbounded storage growth and UI display issues.
// Fixes #48: raised from 256 to 1000 per security requirement.
const maxTitleLength = 1000

// Task represents a single to-do item.
type Task struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Done     bool   `json:"done"`
	Priority string `json:"priority"`
	// DueDate holds an optional due date in YYYY-MM-DD format.
	// An empty string means no due date has been set.
	DueDate string `json:"due_date,omitempty"`
}

// IsOverdue reports whether the task is incomplete and its due date has passed
// relative to the given reference time (typically time.Now()).
// Tasks with no due date are never considered overdue.
// If DueDate is a non-empty string that cannot be parsed, IsOverdue logs a
// warning and returns false.
//
// Comparison is performed in UTC so that the result is consistent regardless
// of the local timezone: a task due on date D is considered overdue only when
// the UTC date of `now` is strictly after D.
func (t Task) IsOverdue(now time.Time) bool {
	if t.Done || t.DueDate == "" {
		return false
	}
	due, err := time.Parse(dateLayout, t.DueDate)
	if err != nil {
		log.Printf("warning: task %d has malformed due date %q: %v", t.ID, t.DueDate, err)
		return false
	}
	// Normalise `now` to UTC midnight so the comparison is purely date-based
	// and is not affected by the host's local timezone.
	nowUTC := now.UTC()
	nowDate := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)
	// `due` is already in UTC midnight because time.Parse uses UTC when no
	// timezone is embedded in the layout/value.
	return nowDate.After(due)
}

// Manager handles persistence and business logic for the task list.
// A mutex is embedded to serialise access when multiple goroutines (or, via
// OS-level file locks, multiple processes) share the same Manager instance.
//
// TODO(#17): add OS-level file locking for multi-process safety.
type Manager struct {
	filePath string
	mu       sync.Mutex
}

// NewManager creates a Manager that stores tasks in the given file.
// filePath is cleaned with filepath.Clean before use. The library enforces
// that filePath must be a relative path that does not escape the current
// working directory: absolute paths and paths beginning with ".." are both
// rejected with a descriptive error.
//
// Defence-in-depth: after the textual traversal check, NewManager resolves
// symlinks via filepath.EvalSymlinks and verifies that the real path stays
// within the current working directory. Fixes #87.
func NewManager(filePath string) (*Manager, error) {
	clean := filepath.Clean(filePath)

	// Reject absolute paths — the library only supports relative paths within
	// the current working directory.
	if filepath.IsAbs(clean) {
		return nil, fmt.Errorf("NewManager: path %q must be a relative path", filePath)
	}

	// Reject obvious traversal attempts: if the cleaned path starts with ".."
	// it points outside the current working directory.
	if strings.HasPrefix(clean, "..") {
		return nil, fmt.Errorf("NewManager: path %q attempts directory traversal", filePath)
	}

	// Defence-in-depth: resolve symlinks and verify the real path stays within
	// the current working directory. This catches symlink-based escapes that
	// bypass the textual check above (Fixes #87).
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("NewManager: unable to determine working directory: %w", err)
	}

	// Build the candidate absolute path (clean is already relative).
	candidate := filepath.Join(cwd, clean)

	// Attempt to resolve symlinks on the candidate path.
	realPath, evalErr := filepath.EvalSymlinks(candidate)
	if evalErr != nil {
		if errors.Is(evalErr, os.ErrNotExist) {
			// The file (or one of its path components) does not exist yet.
			// Try resolving the parent directory instead — an intermediate
			// symlink in the directory components could still escape.
			parentCandidate := filepath.Dir(candidate)
			realParent, parentErr := filepath.EvalSymlinks(parentCandidate)
			if parentErr != nil {
				// Parent also doesn't exist (new nested path) — skip the
				// symlink check gracefully to preserve NewManager's contract
				// for not-yet-created files.
				return &Manager{filePath: clean}, nil
			}
			// Verify the resolved parent stays within cwd.
			rel, relErr := filepath.Rel(cwd, realParent)
			if relErr != nil || strings.HasPrefix(rel, "..") {
				return nil, fmt.Errorf("manager: file path escapes working directory")
			}
			return &Manager{filePath: clean}, nil
		}
		// A non-ErrNotExist error (e.g. permission denied) must not be
		// swallowed — surface it with context.
		return nil, fmt.Errorf("NewManager: symlink resolution failed for %q: %w", filePath, evalErr)
	}

	// EvalSymlinks succeeded: verify the real path is still within cwd.
	rel, relErr := filepath.Rel(cwd, realPath)
	if relErr != nil || strings.HasPrefix(rel, "..") {
		return nil, fmt.Errorf("manager: file path escapes working directory")
	}

	return &Manager{filePath: clean}, nil
}

// load reads all tasks from disk. It returns an empty slice when the file does
// not yet exist. Callers must hold m.mu before calling load.
//
// Fixes #30: JSON parse errors now include the file path.
// Fixes #80: Post-deserialization validation rejects records with invalid IDs,
// empty titles, unknown priorities, malformed due dates, or duplicate IDs.
func (m *Manager) load() ([]Task, error) {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make([]Task, 0), nil
		}
		return nil, fmt.Errorf("load: %w", err)
	}
	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("failed to parse task file %s: %w", m.filePath, err)
	}

	// Post-deserialization validation: reject any file whose records violate
	// the invariants that Add() enforces so that a hand-crafted or externally
	// modified tasks.json cannot corrupt application state. Fixes #80.
	seenIDs := make(map[int]struct{}, len(tasks))
	for i, t := range tasks {
		// 1. ID must be a positive integer.
		if t.ID <= 0 {
			return nil, fmt.Errorf("load: record %d has invalid id %d: must be a positive integer", i, t.ID)
		}
		// 2. Title must be non-empty.
		if strings.TrimSpace(t.Title) == "" {
			return nil, fmt.Errorf("load: record %d (id %d) has empty title", i, t.ID)
		}
		// 3. Priority must be one of the accepted values.
		switch t.Priority {
		case "high", "medium", "low":
			// valid
		default:
			return nil, fmt.Errorf("load: record %d (id %d) has unknown priority %q: must be high, medium, or low", i, t.ID, t.Priority)
		}
		// 4. DueDate must be empty or in YYYY-MM-DD format.
		if t.DueDate != "" {
			if _, err := time.Parse(dateLayout, t.DueDate); err != nil {
				return nil, fmt.Errorf("load: record %d (id %d) has malformed due_date %q: expected YYYY-MM-DD", i, t.ID, t.DueDate)
			}
		}
		// 5. No duplicate IDs.
		if _, dup := seenIDs[t.ID]; dup {
			return nil, fmt.Errorf("load: duplicate task id %d found in %s", t.ID, m.filePath)
		}
		seenIDs[t.ID] = struct{}{}
	}

	return tasks, nil
}

// save writes tasks to disk atomically (write to temp file, then rename).
// Callers must hold m.mu before calling save.
func (m *Manager) save(tasks []Task) error {
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("save: marshal: %w", err)
	}

	dir := filepath.Dir(m.filePath)
	tmp, err := os.CreateTemp(dir, ".tasks-*.tmp")
	if err != nil {
		return fmt.Errorf("save: create temp file: %w", err)
	}
	tmpName := tmp.Name()

	// Ensure the temp file is cleaned up if anything goes wrong before rename.
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("save: write: %w", err)
	}
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("save: chmod: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("save: close: %w", err)
	}
	if err := os.Rename(tmpName, m.filePath); err != nil {
		return fmt.Errorf("save: rename: %w", err)
	}

	success = true
	return nil
}

// Add creates a new task with the given title, priority, and optional due date.
// The title must be non-empty and at most maxTitleLength runes long.
// Priority must be one of "low", "medium", or "high".
// dueDate must be either empty or in YYYY-MM-DD format.
func (m *Manager) Add(title, priority, dueDate string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("task title must not be empty")
	}
	if utf8.RuneCountInString(title) > maxTitleLength {
		return fmt.Errorf("task title must not exceed %d characters", maxTitleLength)
	}
	switch priority {
	case "low", "medium", "high":
		// valid
	default:
		return fmt.Errorf("invalid priority %q: must be low, medium, or high", priority)
	}
	if dueDate != "" {
		if _, err := time.Parse(dateLayout, dueDate); err != nil {
			return fmt.Errorf("invalid due date %q: expected format YYYY-MM-DD", dueDate)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	tasks, err := m.load()
	if err != nil {
		return err
	}

	maxID := 0
	for _, t := range tasks {
		if t.ID > maxID {
			maxID = t.ID
		}
	}

	tasks = append(tasks, Task{
		ID:       maxID + 1,
		Title:    title,
		Done:     false,
		Priority: priority,
		DueDate:  dueDate,
	})

	return m.save(tasks)
}

// List returns all tasks, optionally filtered by priority and/or overdue status.
// An empty priority string disables the priority filter.
// When overdueOnly is true, only incomplete tasks whose due date has passed are returned.
func (m *Manager) List(priority string, overdueOnly bool) ([]Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tasks, err := m.load()
	if err != nil {
		return nil, err
	}

	if priority == "" && !overdueOnly {
		return tasks, nil
	}

	now := time.Now()
	filtered := make([]Task, 0, len(tasks))
	for _, t := range tasks {
		if priority != "" && t.Priority != priority {
			continue
		}
		if overdueOnly && !t.IsOverdue(now) {
			continue
		}
		filtered = append(filtered, t)
	}
	return filtered, nil
}

// Complete marks the task with the given ID as done.
// Returns an error if the ID does not exist or the task is already done.
func (m *Manager) Complete(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tasks, err := m.load()
	if err != nil {
		return err
	}

	for i, t := range tasks {
		if t.ID == id {
			if t.Done {
				return fmt.Errorf("task %d is already marked as done", id)
			}
			tasks[i].Done = true
			return m.save(tasks)
		}
	}
	return fmt.Errorf("task %d not found", id)
}

// Delete removes the task with the given ID permanently.
// Returns an error if the ID does not exist.
func (m *Manager) Delete(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tasks, err := m.load()
	if err != nil {
		return err
	}

	for i, t := range tasks {
		if t.ID == id {
			tasks = append(tasks[:i], tasks[i+1:]...)
			return m.save(tasks)
		}
	}
	return fmt.Errorf("task %d not found", id)
}

// Stats returns counts of total, done, and pending tasks, plus the number of
// overdue incomplete tasks.
func (m *Manager) Stats() (total, done, pending, overdue int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tasks, err := m.load()
	if err != nil {
		return 0, 0, 0, 0, err
	}

	now := time.Now()
	for _, t := range tasks {
		total++
		if t.Done {
			done++
		} else {
			pending++
			if t.IsOverdue(now) {
				overdue++
			}
		}
	}
	return total, done, pending, overdue, nil
}

// Clear removes all completed tasks from the store.
// Returns the number of tasks removed.
func (m *Manager) Clear() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tasks, err := m.load()
	if err != nil {
		return 0, err
	}

	remaining := make([]Task, 0, len(tasks))
	removed := 0
	for _, t := range tasks {
		if t.Done {
			removed++
		} else {
			remaining = append(remaining, t)
		}
	}

	if removed == 0 {
		return 0, nil
	}

	return removed, m.save(remaining)
}
