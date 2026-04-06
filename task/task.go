package task

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Task struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Priority  string    `json:"priority"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
}

type Manager struct {
	filepath string
	tasks    []Task
	nextID   int
}

func NewManager(filepath string) *Manager {
	m := &Manager{filepath: filepath}
	m.load()
	return m
}

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

func (m *Manager) List() []Task {
	return m.tasks
}

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
