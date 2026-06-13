# taskctl

> A lightweight, fast command-line task manager written in Go — with priority-based filtering and due-date tracking built in.

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Installation](#installation)
  - [From Source](#from-source)
  - [Docker](#docker)
- [CLI Reference](#cli-reference)
  - [add](#add)
  - [list](#list)
  - [done](#done)
  - [delete](#delete)
  - [stats](#stats)
  - [clear](#clear)
- [Examples](#examples)
- [Testing](#testing)
- [Project Structure](#project-structure)
- [Contributing](#contributing)

---

## Overview

**taskctl** is a minimal, dependency-free CLI tool for managing your tasks directly from the terminal. Tasks are stored locally and can be tagged with a priority level (`low`, `medium`, or `high`) and an optional due date (`YYYY-MM-DD`), making it easy to focus on what matters most. The `--priority` filter lets you instantly surface only the tasks that match a given urgency level, and `--overdue` shows tasks that have passed their due date.

---

## Features

| Feature | Description |
|---|---|
| ➕ **Add tasks** | Create a new task with a title and optional priority level |
| 📋 **List tasks** | Display all tasks with their ID, status, priority, due date, and title |
| 🔍 **Filter by priority** | Use `--priority` to show only `low`, `medium`, or `high` priority tasks |
| ✅ **Complete tasks** | Mark a task as done by its ID |
| 🗑️ **Delete tasks** | Remove a task permanently by its ID |
| 📊 **Task statistics** | View total, pending, completed, overdue counts and completion rate with `stats` |
| 📅 **Due dates** | Attach an optional due date (YYYY-MM-DD) to any task with `--due` |
| ⚠️ **Overdue detection** | Tasks past their due date are flagged `[OVERDUE]`; filter with `--overdue` |
| 🧹 **Clear completed** | Remove all done tasks in one command with `clear`; reports cleared and remaining counts |

---

## Installation

### From Source

**Prerequisites:** [Go 1.22+](https://golang.org/dl/)

```bash
# Clone the repository
git clone https://github.com/geored/taskctl.git
cd taskctl

# Build the binary
go build -o taskctl .

# (Optional) Move to a directory on your PATH
mv taskctl /usr/local/bin/taskctl
```

Verify the installation:

```bash
taskctl --help
```

### Docker

A `Dockerfile` is included for containerised usage.

```bash
# Build the Docker image
docker build -t taskctl .

# Run taskctl inside a container
docker run --rm taskctl --help

# Add a task
docker run --rm taskctl add "Review pull requests" --priority high

# Add a task with a due date
docker run --rm taskctl add "Submit report" --priority high --due 2025-12-31

# List all tasks
docker run --rm taskctl list

# Filter by priority
docker run --rm taskctl list --priority high
```

> **Tip:** Mount a local volume if you want tasks to persist between container runs:
> ```bash
> docker run --rm -v $(pwd)/data:/data taskctl list
> ```

---

## CLI Reference

### `add`

Create a new task with a title, an optional priority, and an optional due date.

```
taskctl add <title> [--priority <level>] [--due YYYY-MM-DD]
```

| Flag | Values | Default | Description |
|---|---|---|---|
| `--priority` | `low`, `medium`, `high` | `medium` | Set the urgency level of the task |
| `--due` | `YYYY-MM-DD` | _(none)_ | Set an optional due date for the task |

**Examples:**

```bash
# Add a task with default (medium) priority
taskctl add "Write unit tests"

# Add a high-priority task
taskctl add "Fix production bug" --priority high

# Add a low-priority task
taskctl add "Update dependencies" --priority low

# Add a medium-priority task explicitly
taskctl add "Refactor auth module" --priority medium

# Add a task with a due date (and default priority)
taskctl add "Submit quarterly report" --due 2025-03-31

# Add a high-priority task with a due date
taskctl add "Fix critical security patch" --priority high --due 2025-01-15
```

---

### `list`

Display tasks. Without flags, all tasks are shown. Use `--priority` to filter by urgency, or `--overdue` to show only tasks past their due date.

```
taskctl list [--priority <level>] [--overdue]
```

| Flag | Values | Default | Description |
|---|---|---|---|
| `--priority` | `low`, `medium`, `high` | _(none — shows all)_ | Filter tasks by priority level |
| `--overdue` | _(boolean flag)_ | `false` | Show only incomplete tasks whose due date has passed |

**Examples:**

```bash
# List all tasks
taskctl list

# List only high-priority tasks
taskctl list --priority high

# List only medium-priority tasks
taskctl list --priority medium

# List only low-priority tasks
taskctl list --priority low

# List only overdue tasks
taskctl list --overdue
```

**Sample output (`taskctl list --priority high`):**

```
ID   Done   Priority Due Date     Title
------------------------------------------------------
1    [ ]    high     2025-01-15   Fix critical security patch [OVERDUE]
4    [ ]    high     -            Deploy hotfix to staging
7    [x]    high     2024-12-01   Patch security vulnerability
```

> Tasks past their due date are flagged with `[OVERDUE]` in the title column. Tasks with no due date display `-` in the `Due Date` column.

---

### `done`

Mark a task as completed by its numeric ID.

```
taskctl done <id>
```

**Examples:**

```bash
# Mark task 3 as done
taskctl done 3

# Mark multiple tasks done (run sequentially)
taskctl done 1
taskctl done 5
```

---

### `delete`

Permanently remove a task by its numeric ID.

```
taskctl delete <id>
```

**Examples:**

```bash
# Delete task 2
taskctl delete 2

# Delete a completed task
taskctl delete 7
```

---

### `stats`

Display a summary of all tasks, including counts by status and priority, and the overall completion rate.

```
taskctl stats
```

No flags are accepted — `stats` always reports on the full task list.

**Example output:**

```
Total tasks:     12
  Pending:       7
  Completed:     5
  Overdue:       2
  High priority: 3
  Med priority:  6
  Low priority:  3
Completion rate: 41%
```

**Notes:**

- **Pending** = tasks not yet marked done.
- **Completed** = tasks marked done via `taskctl done <id>`.
- **Overdue** = incomplete tasks whose due date has passed.
- **Completion rate** is an integer percentage: `(completed / total) * 100`. It is `0%` when there are no tasks.
- Priority counts (`High priority`, `Med priority`, `Low priority`) include both pending and completed tasks so they always sum to the total.

---

### `clear`

Delete all tasks that are marked as done in a single operation. Pending (not-yet-done) tasks are left completely untouched.

```
taskctl clear
```

No flags are accepted. The command accepts no arguments.

**Output format:**

```
Cleared N completed tasks. M tasks remaining.
```

Where `N` is the number of completed tasks removed and `M` is the number of pending tasks still in the store after the operation.

**Examples:**

```bash
# Remove all completed tasks
taskctl clear
# Cleared 3 completed tasks. 5 tasks remaining.

# When there are no completed tasks to remove
taskctl clear
# Cleared 0 completed tasks. 5 tasks remaining.

# When the task list is empty
taskctl clear
# Cleared 0 completed tasks. 0 tasks remaining.
```

**Typical workflow:**

```bash
# 1. Add and complete some tasks
taskctl add "Write tests"       --priority high
taskctl add "Deploy to staging" --priority medium
taskctl add "Update changelog"  --priority low
taskctl done 1
taskctl done 2

# 2. Confirm which tasks are done
taskctl list
# ID   Done   Priority Due Date     Title
# ------------------------------------------------------
# 1    [x]    high     -            Write tests
# 2    [x]    medium   -            Deploy to staging
# 3    [ ]    low      -            Update changelog

# 3. Clear completed tasks
taskctl clear
# Cleared 2 completed tasks. 1 tasks remaining.

# 4. Only pending tasks remain
taskctl list
# ID   Done   Priority Due Date     Title
# ------------------------------------------------------
# 3    [ ]    low      -            Update changelog
```

**Notes:**

- IDs of remaining tasks are **not** renumbered after a `clear` — task 3 stays task 3.
- `clear` is atomic: it uses the same write-to-temp-then-rename pattern as all other write operations, so a crash mid-clear cannot corrupt the task store.
- Running `clear` on an empty store or a store with no completed tasks is safe and always returns `(0, nil)`.

---

## Examples

A full end-to-end workflow:

```bash
# 1. Add some tasks
taskctl add "Plan sprint" --priority high --due 2025-02-01
taskctl add "Write documentation" --priority medium --due 2025-03-15
taskctl add "Clean up old branches" --priority low
taskctl add "Fix login bug" --priority high --due 2025-01-20
taskctl add "Update README" --priority medium

# 2. View all tasks
taskctl list
# ID   Done   Priority Due Date     Title
# ------------------------------------------------------
# 1    [ ]    high     2025-02-01   Plan sprint
# 2    [ ]    medium   2025-03-15   Write documentation
# 3    [ ]    low      -            Clean up old branches
# 4    [ ]    high     2025-01-20   Fix login bug
# 5    [ ]    medium   -            Update README

# 3. Focus on high-priority items only
taskctl list --priority high
# ID   Done   Priority Due Date     Title
# ------------------------------------------------------
# 1    [ ]    high     2025-02-01   Plan sprint
# 4    [ ]    high     2025-01-20   Fix login bug

# 4. See which tasks are overdue
taskctl list --overdue
# ID   Done   Priority Due Date     Title
# ------------------------------------------------------
# 4    [ ]    high     2025-01-20   Fix login bug [OVERDUE]

# 5. Complete a task
taskctl done 1
# Task 1 marked as done.

# 6. Verify completion
taskctl list --priority high
# ID   Done   Priority Due Date     Title
# ------------------------------------------------------
# 1    [x]    high     2025-02-01   Plan sprint
# 4    [ ]    high     2025-01-20   Fix login bug [OVERDUE]

# 7. Check stats
taskctl stats
# Total tasks:     5
#   Pending:       4
#   Completed:     1
#   Overdue:       1
#   High priority: 2
#   Med priority:  2
#   Low priority:  1
# Completion rate: 20%

# 8. Delete a task
taskctl delete 3
# Task 3 deleted.

# 9. Clear all completed tasks
taskctl clear
# Cleared 1 completed tasks. 3 tasks remaining.
```

---

## Testing

```bash
# Run all tests
go test ./...

# Run tests with race detection
go test -race ./...

# Run tests with coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## Project Structure

```
taskctl/
├── main.go          # CLI entry point: argument parsing, command dispatch
├── main_test.go     # Integration-level tests for each CLI handler
├── go.mod           # Go module definition
├── Dockerfile       # Multi-stage Docker build
└── task/
    ├── task.go      # Manager: Add, List, Complete, Delete, Clear, Stats
    └── task_test.go # Unit tests for the task package
```

### Key types (`task/task.go`)

```go
// Task represents a single to-do item.
type Task struct {
    ID       int    `json:"id"`
    Title    string `json:"title"`
    Done     bool   `json:"done"`
    Priority string `json:"priority"`
    DueDate  string `json:"due_date,omitempty"`
}
```

---

## Contributing

Contributions are welcome! Here's how to get started:

1. **Fork** the repository on GitHub.
2. **Clone** your fork locally:
   ```bash
   git clone https://github.com/<your-username>/taskctl.git
   cd taskctl
   ```
3. **Create a feature branch:**
   ```bash
   git checkout -b feature/my-new-feature
   ```
4. **Make your changes** and add tests where appropriate.
5. **Run the test suite** to make sure everything passes:
   ```bash
   go test -race ./...
   ```
6. **Commit** with a descriptive message following [Conventional Commits](https://www.conventionalcommits.org/):
   ```bash
   git commit -m "feat: add due-date support to tasks"
   ```
7. **Push** your branch and open a **Pull Request** against `main`.

### Guidelines

- Keep PRs focused — one feature or fix per PR.
- Add or update tests for any changed behaviour.
- Follow standard Go formatting (`gofmt`/`goimports`).
- Update this README if you add new commands or flags.

---

> Made with ☕ and Go. Happy task managing!
