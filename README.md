# taskctl

A simple command-line task manager written in Go.

## Commands

| Command | Description |
|---|---|
| `taskctl add <title> [--priority <level>]` | Add a new task |
| `taskctl list [--priority <level>]` | List tasks (optionally filtered by priority) |
| `taskctl done <id>` | Mark a task as completed |
| `taskctl delete <id>` | Delete a task |

## Usage

```bash
# Add tasks with a priority level
taskctl add "Fix login bug" --priority high
taskctl add "Write unit tests" --priority medium
taskctl add "Update changelog" --priority low

# List all tasks
taskctl list

# Mark a task as done
taskctl done 1

# Delete a task
taskctl delete 2
```

## Filtering Tasks by Priority (`--priority`)

The `list` command supports a `--priority` flag that narrows the output to tasks
matching a specific priority level.

### Syntax

```
taskctl list --priority <level>
```

### Allowed values

| Value | Description |
|---|---|
| `high` | Show only high-priority tasks |
| `medium` | Show only medium-priority tasks |
| `low` | Show only low-priority tasks |

> **Note:** The flag value is **case-sensitive**. `High`, `HIGH`, etc. are not valid.

### Usage examples

```bash
# List all tasks (no filter — existing behaviour unchanged)
taskctl list

# List only high-priority tasks
taskctl list --priority high

# List only medium-priority tasks
taskctl list --priority medium

# List only low-priority tasks
taskctl list --priority low
```

### Empty results

When the filter matches no tasks, a contextualised message is printed:

```
No high-priority tasks.
```

Compare this with the generic message shown when no filter is active and the
task store is empty:

```
No tasks.
```

### Invalid priority value

Supplying a value outside the allowed set prints a descriptive error to stderr
and exits with code `1`:

```bash
taskctl list --priority urgent
# invalid priority "urgent": must be high, medium, or low
```

### Implementation notes

- Filtering is performed by `Manager.FilterByPriority(priority string) []Task`
  in `task/task.go`.
- The method always returns an **initialised, non-nil slice** so callers can
  safely `range` over the result without a nil-check.
- When `--priority` is omitted, `mgr.List()` is called as before — no
  behaviour change for existing callers.

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

## Docker

```bash
# Build the image
docker build -t taskctl .

# Run a command inside the container
docker run --rm taskctl list
```
