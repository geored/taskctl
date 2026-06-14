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

const dateLayout = "2006-01-02"
const maxTitleLength = 1000

type Task struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Done     bool   `json:"done"`
	Priority string `json:"priority"`
	DueDate  string `json:"due_date,omitempty"`
}

func (t Task) IsOverdue(now time.Time) bool {
	if t.Done || t.DueDate == "" {
		return false
	}
	due, err := time.Parse(dateLayout, t.DueDate)
	if err != nil {
		log.Printf("warning: task %d has malformed due date %q: %v", t.ID, t.DueDate, err)
		return false
	}
	nowUTC := now.UTC()
	nowDate := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)
	return nowDate.After(due)
}

type Manager struct {
	filePath string
	mu       sync.Mutex
}

// evalSymlinksPartial resolves symlinks for as much of the path as exists on
// disk. It walks the path components from longest to shortest, calling
// filepath.EvalSymlinks on each prefix until one succeeds, then appends the
// remaining (non-existent) suffix. This handles the case where a directory
// component is a symlink but the full path does not yet exist.
func evalSymlinksPartial(path string) (string, error) {
	// Fast path: full path exists.
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved, nil
	}

	// Walk from the full path upward, looking for the longest existing prefix.
	current := path
	var suffix []string
	for {
		parent := filepath.Dir(current)
		if parent == current {
			// Reached root without finding an existing component — return path as-is.
			return path, nil
		}
		suffix = append([]string{filepath.Base(current)}, suffix...)
		current = parent
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			// Reattach the non-existent suffix onto the resolved parent.
			return filepath.Join(append([]string{resolved}, suffix...)...), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
}

// NewManager creates a Manager that stores tasks in the given file.
// filePath must be a relative path that does not escape the current working
// directory. Absolute paths, paths beginning with "..", and paths that use
// symlinks to escape the working directory are all rejected.
//
// Fixes #87: symlink-based directory traversal is now detected and rejected.
func NewManager(filePath string) (*Manager, error) {
	clean := filepath.Clean(filePath)

	if filepath.IsAbs(clean) {
		return nil, fmt.Errorf("NewManager: path %q must be a relative path", filePath)
	}

	if strings.HasPrefix(clean, "..") {
		return nil, fmt.Errorf("NewManager: path %q attempts directory traversal", filePath)
	}

	// Defense-in-depth (Fixes #87): resolve symlinks in any path component and
	// verify the real path stays within the current working directory.
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("NewManager: unable to determine working directory: %w", err)
	}

	candidate := filepath.Join(cwd, clean)

	resolved, err := evalSymlinksPartial(candidate)
	if err != nil {
		return nil, fmt.Errorf("NewManager: cannot resolve path %q: %w", filePath, err)
	}

	// Ensure the resolved path is rooted inside cwd. Append separator so that
	// a sibling directory sharing a prefix with cwd is not falsely accepted.
	cwdWithSep := cwd + string(os.PathSeparator)
	if resolved != cwd && !strings.HasPrefix(resolved, cwdWithSep) {
		return nil, fmt.Errorf("NewManager: path %q attempts directory traversal", filePath)
	}

	return &Manager{filePath: clean}, nil
}

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

	seenIDs := make(map[int]struct{}, len(tasks))
	for i, t := range tasks {
		if t.ID <= 0 {
			return nil, fmt.Errorf("load: record %d has invalid id %d: must be a positive integer", i, t.ID)
		}
		if strings.TrimSpace(t.Title) == "" {
			return nil, fmt.Errorf("load: record %d (id %d) has empty title", i, t.ID)
		}
		switch t.Priority {
		case "high", "medium", "low":
		default:
			return nil, fmt.Errorf("load: record %d (id %d) has unknown priority %q: must be high, medium, or low", i, t.ID, t.Priority)
		}
		if t.DueDate != "" {
			if _, err := time.Parse(dateLayout, t.DueDate); err != nil {
				return nil, fmt.Errorf("load: record %d (id %d) has malformed due_date %q: expected YYYY-MM-DD", i, t.ID, t.DueDate)
			}
		}
		if _, dup := seenIDs[t.ID]; dup {
			return nil, fmt.Errorf("load: duplicate task id %d found in %s", t.ID, m.filePath)
		}
		seenIDs[t.ID] = struct{}{}
	}

	return tasks, nil
}

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
	defer func() {
		os.Remove(tmpName) //nolint:errcheck
	}()

	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return fmt.Errorf("save: chmod: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("save: write: %w", err)
	}
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

func (m *Manager) Add(title, priority, dueDate string) error {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return fmt.Errorf("task title must not be empty")
	}
	if utf8.RuneCountInString(trimmed) > maxTitleLength {
		return fmt.Errorf("task title exceeds maximum length of %d characters", maxTitleLength)
	}

	switch priority {
	case "high", "medium", "low":
	default:
		return fmt.Errorf("invalid priority %q: must be high, medium, or low", priority)
	}

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

func isValidPriority(p string) bool {
	switch p {
	case "", "low", "medium", "high":
		return true
	default:
		return false
	}
}

func (m *Manager) List(priority string, overdueOnly bool) ([]Task, error) {
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

	if cleared == 0 {
		return 0, remaining, nil
	}

	if err := m.save(kept); err != nil {
		return 0, 0, err
	}
	return cleared, remaining, nil
}

type Stats struct {
	Total          int
	Completed      int
	Pending        int
	Overdue        int
	HighPriority   int
	MediumPriority int
	LowPriority    int
}

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
		if t.Done {
			s.Completed++
		} else {
			s.Pending++
			if t.IsOverdue(now) {
				s.Overdue++
			}
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
	return s, nil
}
