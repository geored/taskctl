package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/geored/taskctl/task"
)

func init() {
	// Remove the default timestamp prefix from log messages so that the CLI
	// output is clean; the caller sees only the message itself.
	log.SetFlags(0)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	mgr, err := task.NewManager("tasks.json")
	if err != nil {
		log.Printf("failed to initialise task manager: %v", err)
		os.Exit(1)
	}
	cmd := os.Args[1]

	var runErr error
	switch cmd {
	case "add":
		runErr = runAdd(mgr, os.Args[2:])
	case "list":
		runErr = runList(mgr, os.Args[2:])
	case "done":
		runErr = runDone(mgr, os.Args[2:])
	case "delete":
		runErr = runDelete(mgr, os.Args[2:])
	case "stats":
		runErr = runStats(mgr)
	case "clear":
		runErr = runClear(mgr)
	default:
		log.Printf("unknown command: %s", cmd)
		printUsage()
		os.Exit(1)
	}

	if runErr != nil {
		log.Printf("%v", runErr)
		os.Exit(1)
	}
}

// printUsage writes a short help message to stderr.
func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: taskctl <command> [options]

Commands:
  add     [--priority <low|medium|high>] [--due YYYY-MM-DD] <title>
  list    [--priority <low|medium|high>] [--overdue]
  done    <id>
  delete  <id>
  stats
  clear   Delete all completed tasks`)
}

// runAdd handles the "add" sub-command.
// Flags: --priority (default "medium"), --due (optional, YYYY-MM-DD).
// Remaining args after flag parsing are joined as the task title.
// Returns an error instead of calling os.Exit, enabling unit testing.
func runAdd(mgr *task.Manager, args []string) error {
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

	if err := mgr.Add(title, *priority, *due); err != nil {
		return fmt.Errorf("add: %w", err)
	}
	fmt.Println("Task added.")
	return nil
}

// runList handles the "list" sub-command.
// Flags: --priority (filter by priority), --overdue (show only overdue tasks).
// Returns an error instead of calling os.Exit, enabling unit testing.
func runList(mgr *task.Manager, args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	priority := fs.String("priority", "", "Filter by priority: low, medium, high")
	overdueOnly := fs.Bool("overdue", false, "Show only overdue incomplete tasks")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("list: %w", err)
	}

	// Validate the --priority flag value at the CLI boundary.
	// (Library also validates, but this gives a cleaner CLI-level error message.)
	if *priority != "" {
		switch *priority {
		case "low", "medium", "high":
			// valid
		default:
			return fmt.Errorf("list: invalid priority %q: must be low, medium, or high", *priority)
		}
	}

	tasks, err := mgr.List(*priority, *overdueOnly)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		return nil
	}

	now := time.Now()

	// Header
	fmt.Printf("%-4s %-6s %-8s %-12s %s\n", "ID", "Done", "Priority", "Due Date", "Title")
	fmt.Println("------------------------------------------------------")

	for _, t := range tasks {
		done := "[ ]"
		if t.Done {
			done = "[x]"
		}

		due := t.DueDate
		if due == "" {
			due = "-"
		}

		// Append an [OVERDUE] marker for incomplete tasks past their due date.
		title := t.Title
		if t.IsOverdue(now) {
			title += " [OVERDUE]"
		}

		fmt.Printf("%-4d %-6s %-8s %-12s %s\n", t.ID, done, t.Priority, due, title)
	}
	return nil
}

// runDone handles the "done" sub-command.
// Returns an error instead of calling os.Exit, enabling unit testing.
func runDone(mgr *task.Manager, args []string) error {
	fs := flag.NewFlagSet("done", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("done: %w", err)
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("done: task ID is required")
	}
	id, err := strconv.Atoi(fs.Arg(0))
	if err != nil {
		return fmt.Errorf("done: invalid task ID: %s", fs.Arg(0))
	}
	if err := mgr.Complete(id); err != nil {
		return fmt.Errorf("done: %w", err)
	}
	fmt.Printf("Task %d marked as done.\n", id)
	return nil
}

// runDelete handles the "delete" sub-command.
// Returns an error instead of calling os.Exit, enabling unit testing.
func runDelete(mgr *task.Manager, args []string) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("delete: task ID is required")
	}
	id, err := strconv.Atoi(fs.Arg(0))
	if err != nil {
		return fmt.Errorf("delete: invalid task ID: %s", fs.Arg(0))
	}
	if err := mgr.Delete(id); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	fmt.Printf("Task %d deleted.\n", id)
	return nil
}

// runStats handles the "stats" sub-command.
// It prints a summary that includes completion counts, overdue count,
// per-priority task counts, and the overall completion rate.
// Returns an error instead of calling os.Exit, enabling unit testing.
func runStats(mgr *task.Manager) error {
	s, err := mgr.Stats()
	if err != nil {
		return fmt.Errorf("stats: %w", err)
	}

	pct := 0
	if s.Total > 0 {
		pct = s.Completed * 100 / s.Total
	}

	fmt.Printf("Total tasks:     %d\n", s.Total)
	fmt.Printf("  Pending:       %d\n", s.Pending)
	fmt.Printf("  Completed:     %d\n", s.Completed)
	fmt.Printf("  Overdue:       %d\n", s.Overdue)
	fmt.Printf("  High priority: %d\n", s.HighPriority)
	fmt.Printf("  Med priority:  %d\n", s.MediumPriority)
	fmt.Printf("  Low priority:  %d\n", s.LowPriority)
	fmt.Printf("Completion rate: %d%%\n", pct)
	return nil
}

// runClear handles the "clear" sub-command.
// It removes all completed tasks from the store and prints a summary of how
// many tasks were cleared and how many remain.
// Returns an error instead of calling os.Exit, enabling unit testing.
func runClear(mgr *task.Manager) error {
	cleared, remaining, err := mgr.Clear()
	if err != nil {
		return fmt.Errorf("clear: %w", err)
	}
	fmt.Printf("Cleared %d completed tasks. %d tasks remaining.\n", cleared, remaining)
	return nil
}
