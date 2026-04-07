package task

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Task represents a single task item with a priority level.
type Task struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Priority  string    `json:"priority"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
}

// Stats holds summary statistics for the current task list.
type Stats struct {
	Total          int
	Pending        int
	Completed      int
	HighPriority   int
	MediumPriority int
	LowPriority    int
	CompletionRate int // percentage, 0-100
}

// ClearResult holds the outcome of a Clear operation.
type ClearResult struct {
	Cleared   int // number of completed tasks removed
	Remaining int // number of tasks still in the store
}

// Manager handles persistence and operations on the task list.
type Manager struct {
	filepath string
	tasks    []Task
	nextID   int
}

// NewManager creates a Manager backed by the given JSON file path and
// loads any previously persisted tasks from disk.
func NewManager(filepath string) *Manager {
	m := &Manager{filepath: filepath}
	m.load()
	return m
}

// Add creates a new task with the given title and priority, persists it,
// and returns the created Task.
func (m *Manager) Add(title, priority string) Task {
	t := Task{
		ID:        m.nextID,
		Title:     title,
		Priority:  priority,
		Done:      false,
		CreatedAt: time.Now(),
	}
	m.nextID++
	m.tasks = append(m.tasks, t)
	m.save()
	return t
}

// List returns all tasks regardless of priority.
func (m *Manager) List() []Task {
	return m.tasks
}

// FilterByPriority returns only the tasks whose Priority field matches
// the supplied priority string (case-sensitive: "high", "medium", "low").
// An empty slice is returned when no tasks match.
func (m *Manager) FilterByPriority(priority string) []Task {
	var filtered []Task
	for _, t := range m.tasks {
		if t.Priority == priority {
			filtered = append(filtered, t)
		}
	}
	// Return an initialised empty slice instead of nil so callers can
	// safely range over the result without a nil-check.
	if filtered == nil {
		return []Task{}
	}
	return filtered
}

// Complete marks the task with the given ID as done and persists the change.
func (m *Manager) Complete(id int) error {
	for i := range m.tasks {
		if m.tasks[i].ID == id {
			m.tasks[i].Done = true
			m.save()
			return nil
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

// Delete removes the task with the given ID and persists the change.
func (m *Manager) Delete(id int) error {
	for i := range m.tasks {
		if m.tasks[i].ID == id {
			m.tasks = append(m.tasks[:i], m.tasks[i+1:]...)
			m.save()
			return nil
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

// Clear removes all tasks that are marked as done and persists the result.
// It returns a ClearResult describing how many tasks were removed and how
// many remain in the store after the operation.
func (m *Manager) Clear() ClearResult {
	var remaining []Task
	cleared := 0

	for _, t := range m.tasks {
		if t.Done {
			cleared++
		} else {
			remaining = append(remaining, t)
		}
	}

	// Replace the task list with only the pending tasks and persist.
	m.tasks = remaining
	m.save()

	return ClearResult{
		Cleared:   cleared,
		Remaining: len(remaining),
	}
}

// Stats computes and returns summary statistics for all tasks currently
// held by the Manager. The CompletionRate field is an integer percentage
// (0-100); it is 0 when there are no tasks.
func (m *Manager) Stats() Stats {
	s := Stats{}
	s.Total = len(m.tasks)

	for _, t := range m.tasks {
		if t.Done {
			s.Completed++
		} else {
			s.Pending++
		}
		switch t.Priority {
		case "high":
			s.HighPriority++
		case "medium":
			s.MediumPriority++
		case "low":
			s.LowPriority++
		}
	}

	if s.Total > 0 {
		s.CompletionRate = (s.Completed * 100) / s.Total
	}
	return s
}

func (m *Manager) load() {
	data, err := os.ReadFile(m.filepath)
	if err != nil {
		m.nextID = 1
		return
	}
	json.Unmarshal(data, &m.tasks)
	for _, t := range m.tasks {
		if t.ID >= m.nextID {
			m.nextID = t.ID + 1
		}
	}
}

func (m *Manager) save() {
	data, _ := json.MarshalIndent(m.tasks, "", "  ")
	os.WriteFile(m.filepath, data, 0644)
}
