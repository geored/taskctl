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

	"github.com/geored/taskctl/task"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

const maxFilePathLen = 4096
const maxDueDateLen = 10

func init() {
	log.SetFlags(0)
}

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run(args []string, w io.Writer) error {
	if len(args) < 2 {
		printUsage()
		return fmt.Errorf("no command specified")
	}
	if args[1] == "--help" || args[1] == "-h" {
		printUsage()
		return nil
	}
	if args[1] == "--version" || args[1] == "-version" || args[1] == "version" {
		fmt.Fprintf(w, "taskctl version %s\n", version)
		return nil
	}

	filePath := "tasks.json"
	remaining := args[1:]
	for i := 0; i < len(remaining); i++ {
		a := remaining[i]
		if a == "--file" || a == "-file" {
			if i+1 < len(remaining) {
				filePath = remaining[i+1]
				remaining = append(remaining[:i], remaining[i+2:]...)
				i--
			} else {
				// Finding #5: bare --file at end — return clear error immediately
				// instead of leaving the token in remaining and producing a
				// confusing downstream error.
				return fmt.Errorf("--file requires a value")
			}
		} else if strings.HasPrefix(a, "--file=") {
			filePath = strings.TrimPrefix(a, "--file=")
			remaining = append(remaining[:i], remaining[i+1:]...)
			i--
		} else if strings.HasPrefix(a, "-file=") {
			filePath = strings.TrimPrefix(a, "-file=")
			remaining = append(remaining[:i], remaining[i+1:]...)
			i--
		}
	}

	// Secondary guard for any remaining bare --file/-file tokens. Fixes #130.
	for _, tok := range remaining {
		if tok == "--file" || tok == "-file" {
			return fmt.Errorf("--file requires a value")
		}
	}

	if len(filePath) > maxFilePathLen {
		return fmt.Errorf("--file path exceeds maximum length of %d characters", maxFilePathLen)
	}

	if len(remaining) == 0 {
		printUsage()
		return fmt.Errorf("no command specified")
	}

	cmd := remaining[0]
	cmdArgs := remaining[1:]

	mgr, err := task.NewManager(filePath)
	if err != nil {
		return fmt.Errorf("failed to initialise task manager: %w", err)
	}

	switch cmd {
	case "add":
		return runAdd(mgr, cmdArgs, w)
	case "list":
		return runList(mgr, cmdArgs, w)
	case "done":
		return runDone(mgr, cmdArgs, w)
	case "delete":
		return runDelete(mgr, cmdArgs, w)
	case "stats":
		return runStats(mgr, w)
	case "clear":
		return runClear(mgr, w)
	default:
		printUsage()
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: taskctl [--file <path>] <command> [options]

Global flags:
  --file <path>   Path to the tasks JSON file (default: tasks.json)
  --version       Print the version string and exit
  --help, -h      Show this help message

Commands:
  add     [--priority <low|medium|high>] [--due YYYY-MM-DD] <title>
  list    [--priority <low|medium|high>] [--overdue]
  done    <id>
  delete  <id>
  stats
  clear   Remove all completed (done) tasks
  version Print the version string`)
}

// runAdd handles the "add" sub-command.
// Due-date format validation is the library's single authority (Finding #4).
// The CLI performs only a cheap length pre-filter (Fixes #85) and a
// user-friendly format check (Fixes #32) before delegating to the library.
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
	title := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if title == "" {
		return fmt.Errorf("add: task title must not be empty")
	}
	if *due != "" {
		if len(*due) > maxDueDateLen {
			return fmt.Errorf("add: --due value exceeds maximum length of %d characters", maxDueDateLen)
		}
		// User-friendly format check at CLI boundary (Fixes #32).
		// The library also validates; this provides a clearer error message.
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

func runList(mgr *task.Manager, args []string, w io.Writer) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	priority := fs.String("priority", "", "Filter by priority: low, medium, high")
	overdueOnly := fs.Bool("overdue", false, "Show only overdue incomplete tasks")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("list: %w", err)
	}
	if *priority != "" {
		switch *priority {
		case "low", "medium", "high":
		default:
			return fmt.Errorf("list: invalid priority %q: must be low, medium, or high", *priority)
		}
	}
	tasks, err := mgr.List(*priority, *overdueOnly)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	if len(tasks) == 0 {
		fmt.Fprintln(w, "No tasks found.")
		return nil
	}
	now := time.Now()
	fmt.Fprintf(w, "%-4s %-6s %-8s %-12s %s\n", "ID", "Done", "Priority", "Due Date", "Title")
	fmt.Fprintln(w, "------------------------------------------------------")
	for _, t := range tasks {
		done := "[ ]"
		if t.Done {
			done = "[x]"
		}
		due := t.DueDate
		if due == "" {
			due = "-"
		}
		title := t.Title
		if t.IsOverdue(now) {
			title += " [OVERDUE]"
		}
		fmt.Fprintf(w, "%-4d %-6s %-8s %-12s %s\n", t.ID, done, t.Priority, due, title)
	}
	return nil
}

// parseID parses a single positive integer task ID from args.
// This helper eliminates the structural duplication between runDone and
// runDelete (Finding #7): both commands share identical ID-parsing boilerplate.
func parseID(cmd string, args []string) (int, error) {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 0, fmt.Errorf("%s: %w", cmd, err)
	}
	if fs.NArg() == 0 {
		return 0, fmt.Errorf("%s: task ID is required", cmd)
	}
	id, err := strconv.Atoi(fs.Arg(0))
	if err != nil {
		return 0, fmt.Errorf("%s: invalid task ID: %s", cmd, fs.Arg(0))
	}
	if id <= 0 {
		return 0, fmt.Errorf("%s: task ID must be a positive integer", cmd)
	}
	return id, nil
}

func runDone(mgr *task.Manager, args []string, w io.Writer) error {
	id, err := parseID("done", args)
	if err != nil {
		return err
	}
	if err := mgr.Complete(id); err != nil {
		return fmt.Errorf("done: %w", err)
	}
	fmt.Fprintf(w, "Task %d marked as done.\n", id)
	return nil
}

func runDelete(mgr *task.Manager, args []string, w io.Writer) error {
	id, err := parseID("delete", args)
	if err != nil {
		return err
	}
	if err := mgr.Delete(id); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	fmt.Fprintf(w, "Task %d deleted.\n", id)
	return nil
}

func runStats(mgr *task.Manager, w io.Writer) error {
	s, err := mgr.Stats()
	if err != nil {
		return fmt.Errorf("stats: %w", err)
	}
	pct := 0
	if s.Total > 0 {
		pct = (s.Completed*100 + s.Total/2) / s.Total
	}
	fmt.Fprintf(w, "Total tasks:     %d\n", s.Total)
	fmt.Fprintf(w, "  Pending:       %d\n", s.Pending)
	fmt.Fprintf(w, "  Completed:     %d\n", s.Completed)
	fmt.Fprintf(w, "  Overdue:       %d\n", s.Overdue)
	fmt.Fprintf(w, "  High priority: %d\n", s.HighPriority)
	fmt.Fprintf(w, "  Med priority:  %d\n", s.MediumPriority)
	fmt.Fprintf(w, "  Low priority:  %d\n", s.LowPriority)
	fmt.Fprintf(w, "Completion rate: %d%%\n", pct)
	return nil
}

func runClear(mgr *task.Manager, w io.Writer) error {
	cleared, remaining, err := mgr.Clear()
	if err != nil {
		return fmt.Errorf("clear: %w", err)
	}
	clearedWord := "tasks"
	if cleared == 1 {
		clearedWord = "task"
	}
	remainingWord := "tasks"
	if remaining == 1 {
		remainingWord = "task"
	}
	fmt.Fprintf(w, "Cleared %d completed %s. %d %s remaining.\n", cleared, clearedWord, remaining, remainingWord)
	return nil
}
