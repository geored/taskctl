package task

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Priority levels supported by the task manager.
const (
	PriorityLow    = "low"
	PriorityMedium = "medium"
	PriorityHigh   = "high"
)

// Task represents a single to-do item.
type Task struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Done     bool   `json:"done"`
}

// Manager handles persistence and business logic for a collection of tasks.
// Tasks are stored as JSON in the file at FilePath.
type Manager struct {
	FilePath string
	tasks    []Task
}

// NewManager creates a Manager backed by the given file path and loads any
// previously persisted tasks from disk.
func NewManager(filePath string) (*Manager, error) {
	m := &Manager{FilePath: filePath}
	if err := m.load(); err != nil {
		return nil, err
	}
	return m, nil
}

// load reads tasks from the JSON file.  A missing file is treated as an empty
// store (not an error).
func (m *Manager) load() error {
	data, err := os.ReadFile(m.FilePath)
	if os.IsNotExist(err) {
		m.tasks = make([]Task, 0)
		return nil
	}
	if err != nil {
		return fmt.Errorf("load tasks: %w", err)
	}
	return json.Unmarshal(data, &m.tasks)
}

// save persists the current task list to disk as JSON.
func (m *Manager) save() error {
	data, err := json.MarshalIndent(m.tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tasks: %w", err)
	}
	if err := os.WriteFile(m.FilePath, data, 0644); err != nil {
		return fmt.Errorf("save tasks: %w", err)
	}
	return nil
}

// Add creates a new task with the given title and priority, appends it to the
// store, and persists the change.
func (m *Manager) Add(title, priority string) (Task, error) {
	id := 1
	if len(m.tasks) > 0 {
		id = m.tasks[len(m.tasks)-1].ID + 1
	}
	t := Task{ID: id, Title: title, Priority: priority}
	m.tasks = append(m.tasks, t)
	return t, m.save()
}

// List returns all tasks in the store.
func (m *Manager) List() []Task {
	return m.tasks
}

// FilterByPriority returns only the tasks whose Priority matches the given
// value (case-insensitive).
func (m *Manager) FilterByPriority(priority string) []Task {
	result := make([]Task, 0)
	lower := strings.ToLower(priority)
	for _, t := range m.tasks {
		if strings.ToLower(t.Priority) == lower {
			result = append(result, t)
		}
	}
	return result
}

// Search returns all tasks whose Title contains keyword as a case-insensitive
// substring.  The original store is never modified.
func (m *Manager) Search(keyword string) []Task {
	result := make([]Task, 0)
	lower := strings.ToLower(keyword)
	for _, t := range m.tasks {
		if strings.Contains(strings.ToLower(t.Title), lower) {
			result = append(result, t)
		}
	}
	return result
}

// Complete marks the task with the given ID as done and persists the change.
// It returns an error if no task with that ID exists.
func (m *Manager) Complete(id int) error {
	for i, t := range m.tasks {
		if t.ID == id {
			m.tasks[i].Done = true
			return m.save()
		}
	}
	return fmt.Errorf("task %d not found", id)
}

// Delete removes the task with the given ID from the store and persists the
// change.  It returns an error if no task with that ID exists.
func (m *Manager) Delete(id int) error {
	for i, t := range m.tasks {
		if t.ID == id {
			m.tasks = append(m.tasks[:i], m.tasks[i+1:]...)
			return m.save()
		}
	}
	return fmt.Errorf("task %d not found", id)
}

// Stats holds aggregate counts computed from the current task store.
type Stats struct {
	Total     int
	Completed int
	Pending   int
	// CompletionRate is the percentage of completed tasks (0 when Total == 0).
	CompletionRate float64
}

// Stats computes and returns summary statistics for the current task store.
// The calculation is a single O(n) pass and is safe when the store is empty.
func (m *Manager) Stats() Stats {
	var s Stats
	s.Total = len(m.tasks)
	for _, t := range m.tasks {
		if t.Done {
			s.Completed++
		}
	}
	s.Pending = s.Total - s.Completed
	if s.Total > 0 {
		s.CompletionRate = float64(s.Completed) / float64(s.Total) * 100
	}
	return s
}
