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
)

// dateLayout is the canonical date format accepted and displayed by taskctl.
const dateLayout = "2006-01-02"

// maxTitleLength is the maximum number of characters allowed in a task title.
// Titles longer than this are rejected at the Add boundary to prevent
// unbounded storage growth and UI display issues.
const maxTitleLength = 256

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
	mu       sync.Mutex // TODO(#17): add OS-level file locking for multi-process safety
}

// NewManager creates a Manager that stores tasks in the given file.
// filePath is cleaned with filepath.Clean before use. The library enforces
// that filePath must be a relative path that does not escape the current
// working directory: absolute paths and paths beginning with ".." are both
// rejected with a descriptive error.
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

	return &Manager{filePath: clean}, nil
}

// load reads all tasks from disk. It returns an empty slice when the file does
// not yet exist. Callers must hold m.mu before calling load.
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
		return nil, fmt.Errorf("load: unmarshal: %w", err)
	}
	return tasks, nil
}

// save writes the task slice to disk as JSON using an atomic write-to-temp +
// rename pattern. The file is created with mode 0600 (owner read/write only).
// Callers must hold m.mu before calling save.
func (m *Manager) save(tasks []Task) error {
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("save: marshal: %w", err)
	}

	dir := filepath.Dir(m.filePath)
	tmp, err := os.CreateTemp(dir, "tasks-*.tmp")
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
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("save: close: %w", err)
	}
	if err := os.Rename(tmpName, m.filePath); err != nil {
		return fmt.Errorf("save: rename: %w", err)
	}
	return nil
}

// Add creates a new task with the given title, priority, and optional due date.
// title must be a non-empty string of at most maxTitleLength characters.
// priority must be one of "high", "medium", or "low".
// dueDate must be in YYYY-MM-DD format or empty string for no due date.
// Returns an error if any input is invalid.
func (m *Manager) Add(title, priority, dueDate string) error {
	// Validate title at the public API boundary.
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return fmt.Errorf("task title must not be empty")
	}
	if len(trimmed) > maxTitleLength {
		return fmt.Errorf("task title must not exceed %d characters (got %d)", maxTitleLength, len(trimmed))
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
	nextID := 1
	for _, t := range tasks {
		if t.ID >= nextID {
			nextID = t.ID + 1
		}
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
// priority must be one of "low", "medium", "high", or "" (empty = no filter).
// When overdueOnly is true only incomplete tasks whose due date has passed are returned.
// Returns an error if priority is not a valid value.
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

// Complete marks the task with the given ID as done.
// id must be a positive integer; returns an error for id <= 0.
func (m *Manager) Complete(id int) error {
	if id <= 0 {
		return fmt.Errorf("invalid id %d: must be a positive integer", id)
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

// Delete removes the task with the given ID.
// id must be a positive integer; returns an error for id <= 0.
func (m *Manager) Delete(id int) error {
	if id <= 0 {
		return fmt.Errorf("invalid id %d: must be a positive integer", id)
	}

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

// Clear removes all tasks marked as done (Done == true) and saves the result.
// It returns the count of tasks removed (cleared) and the count of tasks kept
// (remaining). The entire load-filter-save sequence is performed under m.mu to
// prevent races. If load or save fails, the store is not modified and a
// non-nil error is returned.
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
