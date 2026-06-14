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
// Fixes #74: support taskctl --version and taskctl version.
var version = "dev"

// maxFilePathLen is the maximum allowed length for the --file flag value.
// Fixes #76: reject excessively long file paths.
const maxFilePathLen = 4096

// maxDueDateLen is the maximum allowed length for the --due flag value.
// YYYY-MM-DD is exactly 10 characters.
// Fixes #85: reject excessively long due date strings before time.Parse.
const maxDueDateLen = 10

func init() {
	// Remove the default timestamp prefix from log messages so that the CLI
	// output is clean; the caller sees only the message itself.
	log.SetFlags(0)
}

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

// run is the testable entry-point for the CLI. args is the full argument list
// (including the program name at index 0). w receives any output that would
// normally go to stdout.
func run(args []string, w io.Writer) error {
	if len(args) < 2 {
		printUsage()
		return fmt.Errorf("no command specified")
	}

	// Support top-level --help / -h before any subcommand. Fixes #62.
	if args[1] == "--help" || args[1] == "-h" {
		printUsage()
		return nil
	}

	// Support --version / version subcommand. Fixes #74.
	if args[1] == "--version" || args[1] == "-version" || args[1] == "version" {
		fmt.Fprintf(w, "taskctl version %s\n", version)
		return nil
	}

	// Parse the top-level --file flag, which may appear before or after the
	// subcommand. We do a quick pre-scan for --file / --file=<val> so that
	// `taskctl --file x.json add ...` works as well as `taskctl add --file x.json ...`.
	filePath := "tasks.json"
	remaining := args[1:]
	for i := 0; i < len(remaining); i++ {
		a := remaining[i]
		if a == "--file" || a == "-file" {
			if i+1 < len(remaining) {
				filePath = remaining[i+1]
				remaining = append(remaining[:i], remaining[i+2:]...)
				i--
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

	// Validate --file path length. Fixes #76.
	if len(filePath) > maxFilePathLen {
		return fmt.Errorf("--file path exceeds maximum length of %d characters", maxFilePathLen)
	}

	// Fixes #83: path traversal and absolute-path validation is delegated
	// entirely to NewManager, which uses filepath.Clean + IsAbs + HasPrefix
	// checks as the single authoritative layer. We do not duplicate that logic
	// here so there is no risk of the two checks diverging.

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

// printUsage writes a short help message to stderr.
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
// Flags: --priority (default "medium"), --due (optional, YYYY-MM-DD).
// Remaining args after flag parsing are joined as the task title.
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
	// Fixes #85: reject oversized input before time.Parse is called.
	// Fixes #32: return a user-friendly error containing "expected format YYYY-MM-DD"
	// so that the substring is unambiguous for both users and tests.
	if *due != "" {
		if len(*due) > maxDueDateLen {
			return fmt.Errorf("add: --due value exceeds maximum length of %d characters", maxDueDateLen)
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

	// Validate the --priority flag value at the CLI boundary.
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
		fmt.Fprintln(w, "No tasks found.")
		return nil
	}

	now := time.Now()

	// Header
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

		// Append an [OVERDUE] marker for incomplete tasks past their due date.
		title := t.Title
		if t.IsOverdue(now) {
			title += " [OVERDUE]"
		}

		fmt.Fprintf(w, "%-4d %-6s %-8s %-12s %s\n", t.ID, done, t.Priority, due, title)
	}
	return nil
}

// runDone handles the "done" sub-command.
// Returns an error instead of calling os.Exit, enabling unit testing.
func runDone(mgr *task.Manager, args []string, w io.Writer) error {
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
	// Fixes #81: reject non-positive IDs at the CLI boundary.
	if id <= 0 {
		return fmt.Errorf("done: task ID must be a positive integer")
	}
	if err := mgr.Complete(id); err != nil {
		return fmt.Errorf("done: %w", err)
	}
	fmt.Fprintf(w, "Task %d marked as done.\n", id)
	return nil
}

// runDelete handles the "delete" sub-command.
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
	if err != nil {
		return fmt.Errorf("delete: invalid task ID: %s", fs.Arg(0))
	}
	// Fixes #81: reject non-positive IDs at the CLI boundary.
	if id <= 0 {
		return fmt.Errorf("delete: task ID must be a positive integer")
	}
	if err := mgr.Delete(id); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	fmt.Fprintf(w, "Task %d deleted.\n", id)
	return nil
}

// runStats handles the "stats" sub-command.
// It prints a summary that includes completion counts, overdue count,
// per-priority task counts, and the overall completion rate.
// Returns an error instead of calling os.Exit, enabling unit testing.
func runStats(mgr *task.Manager, w io.Writer) error {
	s, err := mgr.Stats()
	if err != nil {
		return fmt.Errorf("stats: %w", err)
	}

	pct := 0
	if s.Total > 0 {
		pct = s.Completed * 100 / s.Total
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

// runClear handles the "clear" sub-command.
// It removes all tasks marked as done and prints how many were cleared and
// how many tasks remain. Returns an error instead of calling os.Exit so that
// the caller (main) controls the exit code and tests can capture errors.
func runClear(mgr *task.Manager, w io.Writer) error {
	cleared, remaining, err := mgr.Clear()
	if err != nil {
		return fmt.Errorf("clear: %w", err)
	}
	clearedWord := "tasks"
	if cleared == 1 { clearedWord = "task" }
	remainingWord := "tasks"
	if remaining == 1 { remainingWord = "task" }
	fmt.Fprintf(w, "Cleared %d completed %s. %d %s remaining.\n", cleared, clearedWord, remaining, remainingWord)
	return nil
}
