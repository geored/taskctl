package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// dateLayout is the canonical date format accepted and displayed by taskctl.
const dateLayout = "2006-01-02"

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
//
// Note: both the reference time and the parsed due date are compared in UTC.
// time.Truncate operates in UTC, and time.Parse returns a UTC time, so the
// comparison is internally consistent. A task due "today" in UTC is not
// considered overdue until the following UTC day. Users in negative UTC offsets
// may observe a task as overdue while it is still their local calendar day.
func (t Task) IsOverdue(now time.Time) bool {
	if t.Done || t.DueDate == "" {
		return false
	}
	due, err := time.Parse(dateLayout, t.DueDate)
	if err != nil {
		return false
	}
	// Truncate both sides to date-only precision (UTC) so that a task due
	// today is not considered overdue until tomorrow.
	return now.Truncate(24 * time.Hour).After(due)
}

// Manager handles persistence and business logic for the task list.
// A mutex is embedded to serialise access when multiple goroutines (or, via
// OS-level file locks, multiple processes) share the same Manager instance.
type Manager struct {
	filePath string
	mu       sync.Mutex
}

// NewManager creates a Manager that stores tasks in the given file.
// filePath is cleaned with filepath.Clean before use. Paths that start with
// ".." (after cleaning) are rejected to prevent directory traversal attacks.
func NewManager(filePath string) (*Manager, error) {
	clean := filepath.Clean(filePath)
	// Reject obvious traversal attempts: if the cleaned path starts with ".."
	// it is pointing outside the current working directory.
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
// title must be a non-empty string.
// priority must be one of "high", "medium", or "low".
// dueDate must be in YYYY-MM-DD format or empty string for no due date.
// Returns an error if any input is invalid.
func (m *Manager) Add(title, priority, dueDate string) error {
	// Validate title at the public API boundary.
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("task title must not be empty")
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
		Title:    strings.TrimSpace(title),
		Done:     false,
		Priority: priority,
		DueDate:  dueDate,
	})
	return m.save(tasks)
}

// List returns all tasks, optionally filtered by priority and/or overdue status.
// When priority is non-empty only tasks with that priority are returned.
// When overdueOnly is true only incomplete tasks whose due date has passed are returned.
func (m *Manager) List(priority string, overdueOnly bool) ([]Task, error) {
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
func (m *Manager) Complete(id int) error {
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
