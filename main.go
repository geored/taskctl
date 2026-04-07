package main

import (
	"fmt"
	"os"

	"github.com/geored/taskctl/task"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	mgr := task.NewManager("tasks.json")

	switch os.Args[1] {
	case "add":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: taskctl add <title> [--priority high|medium|low]")
			os.Exit(1)
		}
		priority := "medium"
		for i, arg := range os.Args {
			if arg == "--priority" && i+1 < len(os.Args) {
				priority = os.Args[i+1]
			}
		}
		t := mgr.Add(os.Args[2], priority)
		fmt.Printf("Created task #%d: %s [%s]\n", t.ID, t.Title, t.Priority)

	case "list":
		// Parse the optional --priority flag to filter the output.
		// When omitted, all tasks are displayed.
		priorityFilter := ""
		for i, arg := range os.Args {
			if arg == "--priority" && i+1 < len(os.Args) {
				priorityFilter = os.Args[i+1]
			}
		}

		// Validate the supplied priority value when the flag is present.
		if priorityFilter != "" {
			switch priorityFilter {
			case "high", "medium", "low":
				// valid — proceed
			default:
				fmt.Fprintf(os.Stderr, "invalid priority %q: must be high, medium, or low\n", priorityFilter)
				os.Exit(1)
			}
		}

		// Retrieve tasks — filtered or full list.
		var tasks []task.Task
		if priorityFilter != "" {
			tasks = mgr.FilterByPriority(priorityFilter)
		} else {
			tasks = mgr.List()
		}

		if len(tasks) == 0 {
			if priorityFilter != "" {
				fmt.Printf("No %s-priority tasks.\n", priorityFilter)
			} else {
				fmt.Println("No tasks.")
			}
			return
		}
		for _, t := range tasks {
			status := " "
			if t.Done {
				status = "x"
			}
			fmt.Printf("[%s] #%d %s (%s)\n", status, t.ID, t.Title, t.Priority)
		}

	case "done":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: taskctl done <id>")
			os.Exit(1)
		}
		var id int
		fmt.Sscanf(os.Args[2], "%d", &id)
		if err := mgr.Complete(id); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("Task #%d marked as done.\n", id)

	case "delete":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: taskctl delete <id>")
			os.Exit(1)
		}
		var id int
		fmt.Sscanf(os.Args[2], "%d", &id)
		if err := mgr.Delete(id); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("Task #%d deleted.\n", id)

	case "stats":
		// Compute and display summary statistics for all tasks.
		s := mgr.Stats()
		fmt.Printf("Total tasks: %d\n", s.Total)
		fmt.Printf("  Pending:         %d\n", s.Pending)
		fmt.Printf("  Completed:       %d\n", s.Completed)
		fmt.Printf("  High priority:   %d\n", s.HighPriority)
		fmt.Printf("  Medium priority: %d\n", s.MediumPriority)
		fmt.Printf("  Low priority:    %d\n", s.LowPriority)
		fmt.Printf("Completion rate: %d%%\n", s.CompletionRate)

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("taskctl — simple task manager")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  taskctl add <title> [--priority high|medium|low]")
	fmt.Println("  taskctl list [--priority high|medium|low]")
	fmt.Println("  taskctl done <id>")
	fmt.Println("  taskctl delete <id>")
	fmt.Println("  taskctl stats")
}
