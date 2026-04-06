# taskctl

A simple CLI task manager written in Go.

---

## Installation

```bash
git clone https://github.com/geored/taskctl.git
cd taskctl
go build -o taskctl .
```

---

## Usage

```
taskctl add <title> [--priority high|medium|low]
taskctl list [--priority high|medium|low]
taskctl done <id>
taskctl delete <id>
```

---

## Commands

### `add` — Create a new task

```bash
taskctl add "Buy groceries"
taskctl add "Fix critical bug" --priority high
taskctl add "Write tests"      --priority medium
taskctl add "Update changelog" --priority low
```

- `--priority` is optional; omitting it stores the task with no priority label.
- Accepted values: `high`, `medium`, `low` (case-sensitive).

---

### `list` — List tasks

Display all tasks, or filter by priority using the `--priority` flag.

#### Syntax

```
taskctl list [--priority high|medium|low]
```

#### Examples

```bash
# List every task (no filter applied)
taskctl list

# List only high-priority tasks
taskctl list --priority high

# List only medium-priority tasks
taskctl list --priority medium

# List only low-priority tasks
taskctl list --priority low
```

#### Priority filter behaviour

| Scenario | Output |
|---|---|
| Flag omitted | All tasks are displayed |
| `--priority high` | Only tasks with priority `high` |
| `--priority medium` | Only tasks with priority `medium` |
| `--priority low` | Only tasks with priority `low` |
| No matching tasks | Contextual empty-state message (e.g. `No high-priority tasks.`) |
| Invalid value | Error printed to stderr, exit code `1` |

#### Invalid priority value

Supplying a value outside the allowed set prints a descriptive error and exits with code `1`:

```bash
taskctl list --priority urgent
# invalid priority "urgent": must be high, medium, or low
```

---

### `done` — Mark a task as complete

```bash
taskctl done 3
```

---

### `delete` — Delete a task

```bash
taskctl delete 3
```

---

## Priority levels

| Value | Meaning |
|---|---|
| `high` | Urgent / must be done soon |
| `medium` | Normal importance |
| `low` | Nice to have / can wait |

Priority values are **case-sensitive**. Always use lowercase.

---

## Running tests

```bash
# Run all tests
go test ./...

# Run only the priority-filter tests with verbose output
go test ./task/... -v -run TestFilter
```

Expected output for the filter tests:

```
=== RUN   TestFilterByPriority
=== RUN   TestFilterByPriority/priority=high
=== RUN   TestFilterByPriority/priority=medium
=== RUN   TestFilterByPriority/priority=low
=== RUN   TestFilterByPriority/priority=unknown
--- PASS: TestFilterByPriority (0.00s)
=== RUN   TestFilterByPriorityEmptyStore
--- PASS: TestFilterByPriorityEmptyStore (0.00s)
PASS
```

---

## Project structure

```
taskctl/
├── main.go          # CLI entry point — commands, flag parsing, output
├── task/
│   ├── task.go      # Task model, Manager, FilterByPriority
│   └── task_test.go # Unit tests
└── README.md
```

---

## Changelog

### v0.2.0
- **feat**: `list --priority` flag — filter tasks by `high`, `medium`, or `low` priority (`FilterByPriority` method on `Manager`).
- **feat**: Contextualised empty-state messages when a priority filter is active.
- **feat**: Input validation for `--priority` with a clear error message and exit code `1` on invalid values.
- **docs**: GoDoc comments added to all exported types and methods.

### v0.1.0
- Initial release: `add`, `list`, `done`, `delete` commands with JSON persistence.
