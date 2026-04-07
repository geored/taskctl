package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"taskctl/task"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	mgr := task.NewManager("tasks.json")
	cmd := os.Args[1]

	switch cmd {
	case "add":
		runAdd(mgr, os.Args[2:])
	case "list":
		runList(mgr, os.Args[2:])
	case "done":
		runDone(mgr, os.Args[2:])
	case "delete":
		runDelete(mgr, os.Args[2:])
	case "stats":
		runStats(mgr)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
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
  stats`)
}

// runAdd handles the "add" sub-command.
// Flags: --priority (default "medium"), --due (optional, YYYY-MM-DD).
// Remaining args after flag parsing are joined as the task title.
func runAdd(mgr *task.Manager, args []string) {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	priority := fs.String("priority", "medium", "Task priority: low, medium, high")
	due := fs.String("due", "", "Optional due date in YYYY-MM-DD format")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "add: task title is required")
		os.Exit(1)
	}

	// Join remaining positional arguments as the title so that users do not
	// need to quote multi-word titles.
	title := ""
	for i, a := range fs.Args() {
		if i > 0 {
			title += " "
		}
		title += a
	}

	if err := mgr.Add(title, *priority, *due); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	fmt.Println("Task added.")
}

// runList handles the "list" sub-command.
// Flags: --priority (filter by priority), --overdue (show only overdue tasks).
func runList(mgr *task.Manager, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	priority := fs.String("priority", "", "Filter by priority: low, medium, high")
	overdueOnly := fs.Bool("overdue", false, "Show only overdue incomplete tasks")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	tasks, err := mgr.List(*priority, *overdueOnly)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		return
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
}

// runDone handles the "done" sub-command.
func runDone(mgr *task.Manager, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "done: task ID is required")
		os.Exit(1)
	}
	id, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "done: invalid task ID:", args[0])
		os.Exit(1)
	}
	if err := mgr.Complete(id); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	fmt.Printf("Task %d marked as done.\n", id)
}

// runDelete handles the "delete" sub-command.
func runDelete(mgr *task.Manager, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "delete: task ID is required")
		os.Exit(1)
	}
	id, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "delete: invalid task ID:", args[0])
		os.Exit(1)
	}
	if err := mgr.Delete(id); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	fmt.Printf("Task %d deleted.\n", id)
}

// runStats handles the "stats" sub-command.
func runStats(mgr *task.Manager) {
	s, err := mgr.Stats()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	pct := 0
	if s.Total > 0 {
		pct = s.Completed * 100 / s.Total
	}

	fmt.Printf("Total tasks:     %d\n", s.Total)
	fmt.Printf("Completed:       %d\n", s.Completed)
	fmt.Printf("Pending:         %d\n", s.Pending)
	fmt.Printf("Overdue:         %d\n", s.Overdue)
	fmt.Printf("Completion:      %d%%\n", pct)
}
