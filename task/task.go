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
// the UTC date of now is strictly after D.
func (t Task) IsOverdue(now time.Time) bool {
	if t.Done || t.DueDate == "" {
		return false
	}
	due, err := time.Parse(dateLayout, t.DueDate)
	if err != nil {
		log.Printf("warning: task %d has malformed due date %q: %v", t.ID, t.DueDate, err)
		return false
	}
	// Normalise now to UTC midnight so the comparison is purely date-based
	// and is not affected by the host's local timezone.
	nowUTC := now.UTC()
	nowDate := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)
	// due is already in UTC midnight because time.Parse uses UTC when no
	// timezone is embedded in the layout/value.
	return nowDate.After(due)
}

// osFile is the subset of *os.File operations used by save().
// It is defined as an interface so that tests can inject fakes.
type osFile interface {
	Chmod(mode os.FileMode) error
	Write(b []byte) (int, error)
	Sync() error
	Close() error
	Name() string
}

// Manager handles persistence and business logic for the task list.
// A mutex is embedded to serialise access when multiple goroutines (or, via
// OS-level file locks, multiple processes) share the same Manager instance.
//
// TODO(#17): add OS-level file locking for multi-process safety.
type Manager struct {
	filePath string
	mu       sync.Mutex
	// createTempFn is the factory used by save() to create temp files.
	// When nil, os.CreateTemp is used. Tests may inject a replacement.
	createTemp func(dir, pattern string) (osFile, error)
}

// NewManager creates a Manager that stores tasks in the given file.
// filePath is cleaned with filepath.Clean before use. The library enforces
// that filePath must be a relative path that does not escape the current
// working directory: absolute paths and paths beginning with ".." are both
// rejected with a descriptive error.
//
// As defense-in-depth against symlink-based traversal (Fixes #87), the
// resolved absolute path (after EvalSymlinks on any existing components) is
// verified to still be rooted inside the current working directory.
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

	// Defense-in-depth: resolve symlinks and verify the result stays within
	// the current working directory (Fixes #87).
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("NewManager: could not determine working directory: %w", err)
	}

	joined := filepath.Join(cwd, clean)

	// Attempt to resolve symlinks on the full path. If the file (or a
	// component of its path) does not yet exist, EvalSymlinks will return an
	// error. In that case we fall back to resolving only the existing prefix
	// of the path so that newly-created files are still accepted while
	// symlinks in existing parent directories are still detected.
	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		resolved, err = evalSymlinksPartial(joined)
		if err != nil {
			return nil, fmt.Errorf("NewManager: cannot resolve path %q: %w", filePath, err)
		}
	}

	// The resolved path must be rooted inside cwd (with a separator to avoid
	// false positives where cwd is a prefix of an unrelated sibling path).
	cwdWithSep := cwd + string(filepath.Separator)
	if resolved != cwd && !strings.HasPrefix(resolved, cwdWithSep) {
		return nil, fmt.Errorf("NewManager: path %q attempts directory traversal via symlink", filePath)
	}

	return &Manager{filePath: clean}, nil
}

// evalSymlinksPartial resolves symlinks on the longest existing prefix of
// absPath and returns the fully resolved path (existing prefix resolved +
// non-existing suffix re-appended). absPath must be an absolute path.
func evalSymlinksPartial(absPath string) (string, error) {
	dir := absPath
	suffix := ""

	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the filesystem root without finding an existing component.
			return "", fmt.Errorf("evalSymlinksPartial: could not find existing prefix for %q", absPath)
		}

		resolved, err := filepath.EvalSymlinks(dir)
		if err == nil {
			// dir exists and is fully resolved; reattach any trailing suffix.
			if suffix == "" {
				return resolved, nil
			}
			return filepath.Join(resolved, suffix), nil
		}

		// dir does not exist yet; strip the last component into suffix.
		base := filepath.Base(dir)
		if suffix == "" {
			suffix = base
		} else {
			suffix = filepath.Join(base, suffix)
		}
		dir = parent
	}
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

// save writes the task slice to disk as JSON using an atomic write-to-temp +
// rename pattern. The file is created with mode 0600 (owner read/write only).
// An fsync is performed on the temp file before the rename to ensure data is
// durable on disk even in the event of a crash.
// Callers must hold m.mu before calling save.
func (m *Manager) save(tasks []Task) error {
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("save: marshal: %w", err)
	}

	dir := filepath.Dir(m.filePath)
	createTemp := m.createTemp
	if createTemp == nil {
		createTemp = func(dir, pattern string) (osFile, error) {
			return os.CreateTemp(dir, pattern)
		}
	}
	tmp, err := createTemp(dir, "tasks-*.tmp")
	if err != nil {
		return fmt.Errorf("save: create temp: %w", err)
	}
	tmpName := tmp.Name()
	// Clean up the temp file on any error path; if Rename succeeded the file
	// no longer exists under tmpName and Remove is a harmless no-op.
	defer func() {
		os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
	}()

	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return fmt.Errorf("save: chmod: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("save: write: %w", err)
	}
	// Sync flushes kernel buffers to disk, ensuring durability before rename.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("save: sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("save: close: %w", err)
	}
	if err := os.Rename(tmpName, m.filePath); err != nil {
		return fmt.Errorf("save: rename: %w", err)
	}
	return nil
}

// Add creates a new task with the given title, priority, and optional due date.
// title must be a non-empty string of at most maxTitleLength Unicode characters.
// priority must be one of "high", "medium", or "low".
// dueDate must be in YYYY-MM-DD format or empty string for no due date.
// Returns an error if any input is invalid.
// Fixes #48: enforces maximum title length of 1000 characters.
func (m *Manager) Add(title, priority, dueDate string) error {
	// Validate title at the public API boundary.
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return fmt.Errorf("task title must not be empty")
	}
	// Count Unicode code points (runes), not bytes, so that multibyte
	// characters such as emoji are counted correctly (Fixes #69).
	if utf8.RuneCountInString(trimmed) > maxTitleLength {
		return fmt.Errorf("task title exceeds maximum length of %d characters", maxTitleLength)
	}

	// Validate priority at the public API boundary.
	switch priority {
	case "high", "medium", "low":
		// valid
	default:
		return fmt.Errorf("invalid priority %q: must be high, medium, or low", priority)
	}

	// Validate due date format when provided.
	if dueDate != "" {
		if _, err := time.Parse(dateLayout, dueDate); err != nil {
			return fmt.Errorf("invalid due date %q: expected YYYY-MM-DD", dueDate)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	tasks, err := m.load()
	if err != nil {
		return err
	}

	// Determine next ID (max existing ID + 1, or 1 for an empty list).
	// Guard against integer overflow: if any existing ID equals math.MaxInt,
	// incrementing would produce a negative value (Fixes #70).
	nextID := 1
	for _, t := range tasks {
		if t.ID >= nextID {
			nextID = t.ID + 1
		}
	}
	if nextID <= 0 {
		return fmt.Errorf("id overflow: task store is full")
	}

	tasks = append(tasks, Task{
		ID:       nextID,
		Title:    trimmed,
		Done:     false,
		Priority: priority,
		DueDate:  dueDate,
	})
	return m.save(tasks)
}

// isValidPriority reports whether p is a valid priority value for filtering.
// An empty string is also valid (meaning "no filter").
func isValidPriority(p string) bool {
	switch p {
	case "", "low", "medium", "high":
		return true
	default:
		return false
	}
}

// List returns all tasks, optionally filtered by priority and/or overdue status.
// priority must be one of "low", "medium", "high", or "" (empty = no filter)
// When overdueOnly is true only incomplete tasks whose due date has passed are returned.
// Returns an error if priority is not a valid value.
//
// Fixes #73: the mutex is held for the full duration including the filtering
// step to prevent TOCTOU races.
// Fixes #82: uses defer m.mu.Unlock() consistent with all other Manager methods.
func (m *Manager) List(priority string, overdueOnly bool) ([]Task, error) {
	// Validate priority at the library boundary — consistent with Add().
	if !isValidPriority(priority) {
		return nil, fmt.Errorf("invalid priority filter %q: must be low, medium, or high", priority)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	tasks, err := m.load()
	if err != nil {
		return nil, err
	}

	// Filter while holding the lock to prevent TOCTOU races (Fixes #73).
	now := time.Now()
	result := make([]Task, 0, len(tasks))
	for _, t := range tasks {
		if priority != "" && t.Priority != priority {
			continue
		}
		if overdueOnly && !t.IsOverdue(now) {
			continue
		}
		result = append(result, t)
	}
	return result, nil
}

// Complete marks the task with the given ID as done and persists the change.
// Returns an error if id is <= 0 or if no task with that ID exists.
// Fixes #50: validates that id is a positive integer.
func (m *Manager) Complete(id int) error {
	if id <= 0 {
		return fmt.Errorf("invalid task ID: %d", id)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	tasks, err := m.load()
	if err != nil {
		return err
	}

	for i, t := range tasks {
		if t.ID == id {
			tasks[i].Done = true
			return m.save(tasks)
		}
	}
	return fmt.Errorf("task %d not found", id)
}

// Delete removes the task with the given ID from the store.
// Returns an error if id is <= 0 or if no task with that ID exists.
// Fixes #50: validates that id is a positive integer.
func (m *Manager) Delete(id int) error {
	if id <= 0 {
		return fmt.Errorf("invalid task ID: %d", id)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	tasks, err := m.load()
	if err != nil {
		return err
	}

	filtered := make([]Task, 0, len(tasks))
	found := false
	for _, t := range tasks {
		if t.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, t)
	}
	if !found {
		return fmt.Errorf("task %d not found", id)
	}
	return m.save(filtered)
}

// Clear removes all completed tasks from the store and returns the count of
// tasks cleared and the count of tasks remaining. The mutex is held throughout
// to prevent races. If load or save fails, the store is not modified and a
// non-nil error is returned.
//
// When no tasks are completed, the backing file is not written (Fixes #66).
func (m *Manager) Clear() (cleared int, remaining int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tasks, err := m.load()
	if err != nil {
		return 0, 0, err
	}

	kept := make([]Task, 0, len(tasks))
	for _, t := range tasks {
		if t.Done {
			cleared++
		} else {
			kept = append(kept, t)
		}
	}
	remaining = len(kept)

	// Early return: skip the disk write when nothing was cleared (Fixes #66).
	if cleared == 0 {
		return 0, remaining, nil
	}

	if err := m.save(kept); err != nil {
		return 0, 0, err
	}
	return cleared, remaining, nil
}

// Stats holds aggregate counts for the task list.
type Stats struct {
	Total     int
	Completed int
	Pending   int
	// Overdue is the number of incomplete tasks whose due date has passed.
	Overdue int

	// Priority breakdown — counts include both pending and completed tasks.
	HighPriority   int
	MediumPriority int
	LowPriority    int
}

// Stats computes summary statistics for all tasks in a single pass.
// The completion percentage is safe to derive from the returned struct:
//
//	pct := 0
//	if s.Total > 0 { pct = s.Completed * 100 / s.Total }
//
// Priority counts (HighPriority, MediumPriority, LowPriority) include both
// pending and completed tasks so they always sum to Total.
func (m *Manager) Stats() (Stats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tasks, err := m.load()
	if err != nil {
		return Stats{}, err
	}

	now := time.Now()
	var s Stats
	s.Total = len(tasks)
	for _, t := range tasks {
		// Completion / pending / overdue counts.
		if t.Done {
			s.Completed++
		} else {
			s.Pending++
			if t.IsOverdue(now) {
				s.Overdue++
			}
		}

		// Priority breakdown — tallied regardless of done state.
		switch t.Priority {
		case "high":
			s.HighPriority++
		case "medium":
			s.MediumPriority++
		case "low":
			s.LowPriority++
		}
	}
	return s, nil
}
