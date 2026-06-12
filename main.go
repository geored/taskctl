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
		log.Printf("unknown command: %s", cmd)
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
		log.Printf("add: %v", err)
		os.Exit(1)
	}

	if fs.NArg() == 0 {
		log.Print("add: task title is required")
		os.Exit(1)
	}

	// Join remaining positional arguments as the title so that users do not
	// need to quote multi-word titles.
	title := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if title == "" {
		log.Print("add: task title must not be empty")
		os.Exit(1)
	}

	if err := mgr.Add(title, *priority, *due); err != nil {
		log.Printf("add: %v", err)
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
		log.Printf("list: %v", err)
		os.Exit(1)
	}

	// Validate the --priority flag value at the CLI boundary.
	if *priority != "" {
		switch *priority {
		case "low", "medium", "high":
			// valid
		default:
			log.Printf("list: invalid priority %q: must be low, medium, or high", *priority)
			os.Exit(1)
		}
	}

	tasks, err := mgr.List(*priority, *overdueOnly)
	if err != nil {
		log.Printf("list: %v", err)
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
		log.Print("done: task ID is required")
		os.Exit(1)
	}
	id, err := strconv.Atoi(args[0])
	if err != nil {
		log.Printf("done: invalid task ID: %s", args[0])
		os.Exit(1)
	}
	if err := mgr.Complete(id); err != nil {
		log.Printf("done: %v", err)
		os.Exit(1)
	}
	fmt.Printf("Task %d marked as done.\n", id)
}

// runDelete handles the "delete" sub-command.
func runDelete(mgr *task.Manager, args []string) {
	if len(args) == 0 {
		log.Print("delete: task ID is required")
		os.Exit(1)
	}
	id, err := strconv.Atoi(args[0])
	if err != nil {
		log.Printf("delete: invalid task ID: %s", args[0])
		os.Exit(1)
	}
	if err := mgr.Delete(id); err != nil {
		log.Printf("delete: %v", err)
		os.Exit(1)
	}
	fmt.Printf("Task %d deleted.\n", id)
}

// runStats handles the "stats" sub-command.
// It prints a summary that includes completion counts, overdue count,
// per-priority task counts, and the overall completion rate.
func runStats(mgr *task.Manager) {
	s, err := mgr.Stats()
	if err != nil {
		log.Printf("stats: %v", err)
		os.Exit(1)
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
}
