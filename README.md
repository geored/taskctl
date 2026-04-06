# taskctl

> A lightweight, fast command-line task manager written in Go — with priority-based filtering built in.

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org)
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
  - [complete](#complete)
  - [delete](#delete)
- [Examples](#examples)
- [Testing](#testing)
- [Project Structure](#project-structure)
- [Contributing](#contributing)

---

## Overview

**taskctl** is a minimal, dependency-free CLI tool for managing your tasks directly from the terminal. Tasks are stored locally and can be tagged with a priority level (`low`, `medium`, or `high`), making it easy to focus on what matters most. The `--priority` filter lets you instantly surface only the tasks that match a given urgency level.

---

## Features

| Feature | Description |
|---|---|
| ➕ **Add tasks** | Create a new task with a title and optional priority level |
| 📋 **List tasks** | Display all tasks with their ID, status, priority, and title |
| 🔍 **Filter by priority** | Use `--priority` to show only `low`, `medium`, or `high` priority tasks |
| ✅ **Complete tasks** | Mark a task as done by its ID |
| 🗑️ **Delete tasks** | Remove a task permanently by its ID |

---

## Installation

### From Source

**Prerequisites:** [Go 1.21+](https://golang.org/dl/)

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

Create a new task with a title and an optional priority.

```
taskctl add <title> [--priority <level>]
```

| Flag | Values | Default | Description |
|---|---|---|---|
| `--priority` | `low`, `medium`, `high` | `medium` | Set the urgency level of the task |

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
```

---

### `list`

Display tasks. Without flags, all tasks are shown. Use `--priority` to filter.

```
taskctl list [--priority <level>]
```

| Flag | Values | Default | Description |
|---|---|---|---|
| `--priority` | `low`, `medium`, `high` | _(none — shows all)_ | Filter tasks by priority level |

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
```

**Sample output (`taskctl list --priority high`):**

```
ID   STATUS      PRIORITY   TITLE
─────────────────────────────────────────────────
1    [ ]         high       Fix production bug
4    [ ]         high       Deploy hotfix to staging
7    [✓]         high       Patch security vulnerability
```

---

### `complete`

Mark a task as completed by its numeric ID.

```
taskctl complete <id>
```

**Examples:**

```bash
# Mark task 3 as complete
taskctl complete 3

# Mark multiple tasks complete (run sequentially)
taskctl complete 1
taskctl complete 5
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

## Examples

A full end-to-end workflow:

```bash
# 1. Add some tasks
taskctl add "Plan sprint" --priority high
taskctl add "Write documentation" --priority medium
taskctl add "Clean up old branches" --priority low
taskctl add "Fix login bug" --priority high
taskctl add "Update README" --priority medium

# 2. View all tasks
taskctl list
# ID   STATUS   PRIORITY   TITLE
# 1    [ ]      high       Plan sprint
# 2    [ ]      medium     Write documentation
# 3    [ ]      low        Clean up old branches
# 4    [ ]      high       Fix login bug
# 5    [ ]      medium     Update README

# 3. Focus on high-priority items only
taskctl list --priority high
# ID   STATUS   PRIORITY   TITLE
# 1    [ ]      high       Plan sprint
# 4    [ ]      high       Fix login bug

# 4. Complete a task
taskctl complete 1
# ✓ Task 1 marked as complete.

# 5. Verify completion
taskctl list --priority high
# ID   STATUS   PRIORITY   TITLE
# 1    [✓]      high       Plan sprint
# 4    [ ]      high       Fix login bug

# 6. Delete a task
taskctl delete 3
# ✓ Task 3 deleted.
```

---

## Testing

The test suite lives in `task/task_test.go` and uses the standard Go testing library — no external dependencies required.

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for the task package only
go test -v ./task/...

# Run tests with race-condition detection
go test -race ./...

# Generate a coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

The test suite covers:

- Task creation with all priority levels
- Listing tasks (unfiltered)
- Filtering tasks by `low`, `medium`, and `high` priority
- Marking tasks as complete
- Deleting tasks
- Edge cases (invalid IDs, unknown priority values)

---

## Project Structure

```
taskctl/
├── main.go          # CLI entry point — command parsing and dispatch
├── go.mod           # Go module definition
├── Dockerfile       # Multi-stage Docker build
├── .gitignore       # Git ignore rules
└── task/
    ├── task.go      # Core task logic: Task struct, storage, CRUD, filtering
    └── task_test.go # Unit tests for the task package
```

### Key types (`task/task.go`)

```go
// Priority represents the urgency level of a task.
type Priority string

const (
    PriorityLow    Priority = "low"
    PriorityMedium Priority = "medium"
    PriorityHigh   Priority = "high"
)

// Task represents a single to-do item.
type Task struct {
    ID        int
    Title     string
    Priority  Priority
    Completed bool
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
