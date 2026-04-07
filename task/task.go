package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// dateLayout is the canonical date format accepted and displayed by taskctl.
const dateLayout = "2006-01-02"

// Task represents a single to-do item.
type Task struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Done      bool   `json:"done"`
	Priority  string `json:"priority"`
	// DueDate holds an optional due date in YYYY-MM-DD format.
	// An empty string means no due date has been set.
	DueDate   string `json:"due_date,omitempty"`
}

// IsOverdue reports whether the task is incomplete and its due date has passed
// relative to the given reference time (typically time.Now()).
// Tasks with no due date are never considered overdue.
func (t Task) IsOverdue(now time.Time) bool {
	if t.Done || t.DueDate == "" {
		return false
	}
	due, err := time.Parse(dateLayout, t.DueDate)
	if err != nil {
		return false
	}
	// Truncate both sides to date-only precision so that a task due today is
	// not considered overdue until tomorrow.
	return now.Truncate(24 * time.Hour).After(due)
}

// Manager handles persistence and business logic for the task list.
type Manager struct {
	filePath string
}

// NewManager creates a Manager that stores tasks in the given file.
func NewManager(filePath string) *Manager {
	return &Manager{filePath: filePath}
}

// load reads all tasks from disk. It returns an empty slice when the file does
// not yet exist.
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

// save writes the task slice to disk as JSON.
func (m *Manager) save(tasks []Task) error {
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("save: marshal: %w", err)
	}
	if err := os.WriteFile(m.filePath, data, 0644); err != nil {
		return fmt.Errorf("save: write: %w", err)
	}
	return nil
}

// Add creates a new task with the given title, priority, and optional due date.
// dueDate must be in YYYY-MM-DD format or empty string for no due date.
// Returns an error if dueDate is non-empty but cannot be parsed.
func (m *Manager) Add(title, priority, dueDate string) error {
	// Validate due date format when provided.
	if dueDate != "" {
		if _, err := time.Parse(dateLayout, dueDate); err != nil {
			return fmt.Errorf("invalid due date %q: expected YYYY-MM-DD", dueDate)
		}
	}

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
		Title:    title,
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
}

// Stats computes summary statistics for all tasks in a single pass.
// The completion percentage is safe to derive from the returned struct:
//
//	pct := 0
//	if s.Total > 0 { pct = s.Completed * 100 / s.Total }
func (m *Manager) Stats() (Stats, error) {
	tasks, err := m.load()
	if err != nil {
		return Stats{}, err
	}

	now := time.Now()
	var s Stats
	s.Total = len(tasks)
	for _, t := range tasks {
		if t.Done {
			s.Completed++
		} else {
			s.Pending++
			if t.IsOverdue(now) {
				s.Overdue++
			}
		}
	}
	return s, nil
}
