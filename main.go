package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"taskctl/task"
)

const defaultStore = "tasks.json"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	mgr, err := task.NewManager(defaultStore)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "add":
		cmdAdd(mgr, os.Args[2:])
	case "list":
		cmdList(mgr, os.Args[2:])
	case "done":
		cmdDone(mgr, os.Args[2:])
	case "delete":
		cmdDelete(mgr, os.Args[2:])
	case "stats":
		cmdStats(mgr)
	case "search":
		cmdSearch(mgr, os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

// cmdAdd handles: taskctl add <title> [--priority low|medium|high]
func cmdAdd(mgr *task.Manager, args []string) {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	priority := fs.String("priority", task.PriorityMedium, "task priority (low, medium, high)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "add: %v\n", err)
		os.Exit(1)
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "add: task title is required")
		os.Exit(1)
	}
	title := fs.Arg(0)
	t, err := mgr.Add(title, *priority)
	if err != nil {
		fmt.Fprintf(os.Stderr, "add: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Added task #%d: %s [%s]\n", t.ID, t.Title, t.Priority)
}

// cmdList handles: taskctl list [--priority low|medium|high]
func cmdList(mgr *task.Manager, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	priority := fs.String("priority", "", "filter by priority (low, medium, high)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "list: %v\n", err)
		os.Exit(1)
	}

	var tasks []task.Task
	if *priority != "" {
		tasks = mgr.FilterByPriority(*priority)
	} else {
		tasks = mgr.List()
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		return
	}
	for _, t := range tasks {
		status := "[ ]"
		if t.Done {
			status = "[x]"
		}
		fmt.Printf("%s #%d: %s [%s]\n", status, t.ID, t.Title, t.Priority)
	}
}

// cmdDone handles: taskctl done <id>
func cmdDone(mgr *task.Manager, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "done: task ID is required")
		os.Exit(1)
	}
	id, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "done: invalid task ID %q\n", args[0])
		os.Exit(1)
	}
	if err := mgr.Complete(id); err != nil {
		fmt.Fprintf(os.Stderr, "done: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Task #%d marked as done.\n", id)
}

// cmdDelete handles: taskctl delete <id>
func cmdDelete(mgr *task.Manager, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "delete: task ID is required")
		os.Exit(1)
	}
	id, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "delete: invalid task ID %q\n", args[0])
		os.Exit(1)
	}
	if err := mgr.Delete(id); err != nil {
		fmt.Fprintf(os.Stderr, "delete: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Task #%d deleted.\n", id)
}

// cmdStats handles: taskctl stats
func cmdStats(mgr *task.Manager) {
	s := mgr.Stats()
	fmt.Printf("Total tasks:      %d\n", s.Total)
	fmt.Printf("Completed:        %d\n", s.Completed)
	fmt.Printf("Pending:          %d\n", s.Pending)
	fmt.Printf("Completion rate:  %.1f%%\n", s.CompletionRate)
}

// cmdSearch handles: taskctl search <keyword>
//
// Performs a case-insensitive substring search across all task titles and
// prints each matching task in the same format used by `list`.  When no tasks
// match, a friendly message is printed and the command exits with status 0.
func cmdSearch(mgr *task.Manager, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "search: keyword is required")
		fmt.Fprintln(os.Stderr, "usage: taskctl search <keyword>")
		os.Exit(1)
	}
	keyword := args[0]
	tasks := mgr.Search(keyword)
	if len(tasks) == 0 {
		fmt.Printf("No tasks found matching %q.\n", keyword)
		return
	}
	for _, t := range tasks {
		status := "[ ]"
		if t.Done {
			status = "[x]"
		}
		fmt.Printf("%s #%d: %s [%s]\n", status, t.ID, t.Title, t.Priority)
	}
}

// printUsage prints a short help message listing all available commands.
func printUsage() {
	fmt.Fprintln(os.Stderr, `usage: taskctl <command> [arguments]

Commands:
  add <title> [--priority low|medium|high]   Add a new task
  list [--priority low|medium|high]          List tasks (optionally filtered)
  done <id>                                  Mark a task as completed
  delete <id>                                Delete a task
  stats                                      Show task summary statistics
  search <keyword>                           Search tasks by keyword`)
}
