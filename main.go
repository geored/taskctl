package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/geored/taskctl/task"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("cannot determine home directory: %v", err)
	}

	mgr, err := task.NewManager(home + "/.taskctl/tasks.json")
	if err != nil {
		log.Fatalf("cannot create task manager: %v", err)
	}

	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	cmd := args[0]
	rest := args[1:]

	var runErr error
	switch cmd {
	case "add":
		runErr = runAdd(mgr, rest)
	case "list":
		runErr = runList(mgr, rest)
	case "done":
		runErr = runDone(mgr, rest)
	case "delete":
		runErr = runDelete(mgr, rest)
	case "stats":
		runErr = runStats(mgr)
	case "clear":
		runErr = runClear(mgr)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %q\n\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if runErr != nil {
		log.Printf("error: %v", runErr)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`taskctl — a minimal task manager

Usage:
  taskctl <command> [arguments]

Commands:
  add     Add a new task       (--priority high|medium|low  --due YYYY-MM-DD  <title>)
  list    List tasks            (--priority high|medium|low  --overdue)
  done    Mark a task complete  <id>
  delete  Delete a task         <id>
  stats   Show task statistics
  clear   Delete all completed tasks`)
}

// runAdd parses flags and adds a new task.
func runAdd(mgr *task.Manager, args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	priority := fs.String("priority", "medium", "Task priority: high, medium, or low")
	due := fs.String("due", "", "Due date in YYYY-MM-DD format (optional)")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("add: %w", err)
	}

	titleParts := fs.Args()
	if len(titleParts) == 0 {
		return fmt.Errorf("add: title is required")
	}
	title := strings.Join(titleParts, " ")

	if err := mgr.Add(title, *priority, *due); err != nil {
		return fmt.Errorf("add: %w", err)
	}
	fmt.Printf("Task added: %q (priority: %s)\n", title, *priority)
	return nil
}

// runList parses flags and prints matching tasks.
func runList(mgr *task.Manager, args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	priority := fs.String("priority", "", "Filter by priority: high, medium, or low")
	overdueOnly := fs.Bool("overdue", false, "Show only overdue tasks")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("list: %w", err)
	}

	tasks, err := mgr.List(*priority, *overdueOnly)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		return nil
	}

	for _, t := range tasks {
		status := "[ ]"
		if t.Done {
			status = "[x]"
		}
		due := ""
		if t.DueDate != "" {
			due = "  due:" + t.DueDate
		}
		fmt.Printf("%s #%d  %-8s  %s%s\n", status, t.ID, t.Priority, t.Title, due)
	}
	return nil
}

// runDone marks a task as complete.
func runDone(mgr *task.Manager, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("done: task ID is required")
	}
	id, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("done: invalid task ID %q: %w", args[0], err)
	}
	if err := mgr.Complete(id); err != nil {
		return fmt.Errorf("done: %w", err)
	}
	fmt.Printf("Task #%d marked as done.\n", id)
	return nil
}

// runDelete removes a task by ID.
func runDelete(mgr *task.Manager, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("delete: task ID is required")
	}
	id, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("delete: invalid task ID %q: %w", args[0], err)
	}
	if err := mgr.Delete(id); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	fmt.Printf("Task #%d deleted.\n", id)
	return nil
}

// runStats prints aggregate statistics about the task list.
func runStats(mgr *task.Manager) error {
	s, err := mgr.Stats()
	if err != nil {
		return fmt.Errorf("stats: %w", err)
	}
	pct := 0
	if s.Total > 0 {
		pct = s.Completed * 100 / s.Total
	}
	fmt.Printf("Total:     %d\n", s.Total)
	fmt.Printf("Completed: %d (%d%%)\n", s.Completed, pct)
	fmt.Printf("Pending:   %d\n", s.Pending)
	fmt.Printf("Overdue:   %d\n", s.Overdue)
	fmt.Printf("Priority:  high=%d  medium=%d  low=%d\n",
		s.HighPriority, s.MediumPriority, s.LowPriority)
	return nil
}

// runClear removes all completed tasks and prints a summary.
func runClear(mgr *task.Manager) error {
	cleared, remaining, err := mgr.Clear()
	if err != nil {
		return fmt.Errorf("clear: %w", err)
	}
	fmt.Printf("Cleared %d completed tasks. %d tasks remaining.\n", cleared, remaining)
	return nil
}
