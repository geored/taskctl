// taskctl is a minimal command-line task manager.
//
// Usage:
//
//	taskctl [--file <path>] <command> [flags] [args]
//
// Commands: add, list, complete, delete, stats, clear
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"taskctl/task"
)

func main() {
	log.SetFlags(0)

	// Top-level --file flag selects the backing store.
	fileFlag := flag.String("file", "tasks.json", "Path to the tasks JSON file")
	flag.Parse()

	if flag.NArg() == 0 {
		printUsage(os.Stderr)
		os.Exit(1)
	}

	mgr, err := task.NewManager(*fileFlag)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	subcommand := flag.Arg(0)
	args := flag.Args()[1:]

	var runErr error
	switch subcommand {
	case "add":
		runErr = runAdd(mgr, args, os.Stdout)
	case "list":
		runErr = runList(mgr, args, os.Stdout)
	case "complete":
		runErr = runComplete(mgr, args, os.Stdout)
	case "delete":
		runErr = runDelete(mgr, args, os.Stdout)
	case "stats":
		runErr = runStats(mgr, args, os.Stdout)
	case "clear":
		runErr = runClear(mgr, args, os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %q\n", subcommand)
		printUsage(os.Stderr)
		os.Exit(1)
	}

	if runErr != nil {
		log.Fatalf("error: %v", runErr)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: taskctl [--file <path>] <command> [flags] [args]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  add        Add a new task")
	fmt.Fprintln(w, "  list       List tasks")
	fmt.Fprintln(w, "  complete   Mark a task as done")
	fmt.Fprintln(w, "  delete     Delete a task")
	fmt.Fprintln(w, "  stats      Show task statistics")
	fmt.Fprintln(w, "  clear      Remove all completed tasks")
}

// runAdd handles the "add" sub-command.
// Flags: --priority (default "medium"), --due (optional YYYY-MM-DD date).
// Positional args are joined as the task title.
// Returns an error instead of calling os.Exit, enabling unit testing.
func runAdd(mgr *task.Manager, args []string, w io.Writer) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	priority := fs.String("priority", "medium", "Task priority: low, medium, high")
	due := fs.String("due", "", "Optional due date in YYYY-MM-DD format")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("add: %w", err)
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("add: task title is required")
	}

	// Join remaining positional arguments as the title so that users do not
	// need to quote multi-word titles.
	title := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if title == "" {
		return fmt.Errorf("add: task title must not be empty")
	}

	// Validate the --due flag value at the CLI boundary before calling the library.
	// Fixes #85: enforce a maximum length of 10 characters (the exact length of
	// YYYY-MM-DD) before calling time.Parse, to reject arbitrarily long strings.
	if *due != "" {
		if len(*due) > 10 {
			return fmt.Errorf("add: invalid due date, expected format YYYY-MM-DD")
		}
		if _, err := time.Parse("2006-01-02", *due); err != nil {
			return fmt.Errorf("add: invalid due date, expected format YYYY-MM-DD")
		}
	}

	if err := mgr.Add(title, *priority, *due); err != nil {
		return fmt.Errorf("add: %w", err)
	}
	fmt.Fprintln(w, "Task added.")
	return nil
}

// runList handles the "list" sub-command.
// Flags: --priority (filter by priority), --overdue (show only overdue tasks).
// Returns an error instead of calling os.Exit, enabling unit testing.
func runList(mgr *task.Manager, args []string, w io.Writer) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	priority := fs.String("priority", "", "Filter by priority: low, medium, high")
	overdueOnly := fs.Bool("overdue", false, "Show only overdue incomplete tasks")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("list: %w", err)
	}

	tasks, err := mgr.List(*priority, *overdueOnly)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}

	if len(tasks) == 0 {
		fmt.Fprintln(w, "No tasks found.")
		return nil
	}

	for _, t := range tasks {
		status := " "
		if t.Done {
			status = "✓"
		}
		due := ""
		if t.DueDate != "" {
			due = fmt.Sprintf(" (due: %s)", t.DueDate)
		}
		fmt.Fprintf(w, "[%s] #%d %s [%s]%s\n", status, t.ID, t.Title, t.Priority, due)
	}
	return nil
}

// runComplete handles the "complete" sub-command.
// Requires exactly one positional argument: the task ID.
// Returns an error instead of calling os.Exit, enabling unit testing.
func runComplete(mgr *task.Manager, args []string, w io.Writer) error {
	fs := flag.NewFlagSet("complete", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("complete: %w", err)
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("complete: task ID is required")
	}

	id, err := strconv.Atoi(fs.Arg(0))
	if err != nil || id <= 0 {
		return fmt.Errorf("complete: invalid task ID %q: must be a positive integer", fs.Arg(0))
	}

	if err := mgr.Complete(id); err != nil {
		return fmt.Errorf("complete: %w", err)
	}
	fmt.Fprintf(w, "Task #%d marked as done.\n", id)
	return nil
}

// runDelete handles the "delete" sub-command.
// Requires exactly one positional argument: the task ID.
// Returns an error instead of calling os.Exit, enabling unit testing.
func runDelete(mgr *task.Manager, args []string, w io.Writer) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("delete: task ID is required")
	}

	id, err := strconv.Atoi(fs.Arg(0))
	if err != nil || id <= 0 {
		return fmt.Errorf("delete: invalid task ID %q: must be a positive integer", fs.Arg(0))
	}

	if err := mgr.Delete(id); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	fmt.Fprintf(w, "Task #%d deleted.\n", id)
	return nil
}

// runStats handles the "stats" sub-command.
// Returns an error instead of calling os.Exit, enabling unit testing.
func runStats(mgr *task.Manager, args []string, w io.Writer) error {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("stats: %w", err)
	}

	total, done, pending, overdue, err := mgr.Stats()
	if err != nil {
		return fmt.Errorf("stats: %w", err)
	}

	fmt.Fprintf(w, "Total:   %d\n", total)
	fmt.Fprintf(w, "Done:    %d\n", done)
	fmt.Fprintf(w, "Pending: %d\n", pending)
	fmt.Fprintf(w, "Overdue: %d\n", overdue)
	return nil
}

// runClear handles the "clear" sub-command.
// Removes all completed tasks from the backing store.
// Returns an error instead of calling os.Exit, enabling unit testing.
func runClear(mgr *task.Manager, args []string, w io.Writer) error {
	fs := flag.NewFlagSet("clear", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("clear: %w", err)
	}

	removed, err := mgr.Clear()
	if err != nil {
		return fmt.Errorf("clear: %w", err)
	}

	if removed == 0 {
		fmt.Fprintln(w, "No completed tasks to remove.")
		return nil
	}
	fmt.Fprintf(w, "Removed %d completed task(s).\n", removed)
	return nil
}
