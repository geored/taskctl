# PR #1 Review: feat: add --priority flag to list command for filtering tasks by priority

**PR Author:** geored  
**State:** MERGED  
**Files Changed:** `main.go`, `task/task.go`, `task/task_test.go`

---

## Important Observation

The PR diff is based on an earlier revision of the codebase. The current workspace shows a significantly refactored version (the `Manager` no longer holds in-memory state, `Add` returns `error` instead of `Task`, `List` accepts filter parameters, `FilterByPriority` no longer exists as a separate method, etc.). This review evaluates **the PR diff as submitted**, noting where code has since been superseded.

---

## Findings

### 1. [MEDIUM] Insecure file permissions — `task/task.go` (pre-existing, but unchanged in PR)

**File:** `task/task.go` (visible in diff context at `save` method)  
**Issue:** The `save` method uses `os.WriteFile(m.filePath, data, 0644)`, which grants world-readable permissions on the task data file.  
**Architecture Rule Violated:** _"Use least-privilege file permissions (0600 for sensitive files, not 0644)."_  
**Suggested Fix:**
```go
os.WriteFile(m.filePath, data, 0600)
```
**Note:** This is a pre-existing issue, not introduced by this PR, but worth flagging. *(The current codebase still has this issue.)*

### 2. [MEDIUM] Non-atomic file writes — `task/task.go` (pre-existing)

**File:** `task/task.go`, `save` method  
**Issue:** `os.WriteFile` directly overwrites the file. If the process crashes mid-write the data could be corrupted.  
**Architecture Rule Violated:** _"Prefer atomic file operations (write-to-temp + rename) over direct overwrites."_  
**Suggested Fix:**
```go
func (m *Manager) save(tasks []Task) error {
    data, err := json.MarshalIndent(tasks, "", "  ")
    if err != nil {
        return fmt.Errorf("save: marshal: %w", err)
    }
    tmp := m.filePath + ".tmp"
    if err := os.WriteFile(tmp, data, 0600); err != nil {
        return fmt.Errorf("save: write: %w", err)
    }
    if err := os.Rename(tmp, m.filePath); err != nil {
        return fmt.Errorf("save: rename: %w", err)
    }
    return nil
}
```
**Note:** Pre-existing issue, still present in the current codebase.

### 3. [MEDIUM] Manual `os.Args` parsing instead of `flag` package — `main.go` (diff lines ~35–50)

**File:** `main.go`, `case "list"` block  
**Lines:** Diff lines introducing `--priority` flag parsing  
**Issue:** The PR manually iterates `os.Args` to find `--priority` and its value:
```go
for i, arg := range os.Args {
    if arg == "--priority" && i+1 < len(os.Args) {
        priorityFilter = os.Args[i+1]
    }
}
```
This approach has several problems:
- It doesn't handle `--priority=high` (equals-sign syntax), which Go's `flag` package supports.
- It scans **all** of `os.Args`, not just the args after the `list` sub-command, meaning `taskctl --priority high list` would incorrectly pick up the flag.
- It silently ignores a trailing `--priority` with no value (the `i+1 < len(os.Args)` guard), rather than reporting an error to the user.
- It's inconsistent with the `add` command which already uses `flag.NewFlagSet` for argument parsing.

**Suggested Fix:** Use `flag.NewFlagSet("list", flag.ExitOnError)` consistent with the `add` sub-command pattern now visible in the current codebase. *(This has been fixed in the current workspace code.)*

### 4. [LOW] `FilterByPriority` accepts arbitrary strings without validation — `task/task.go` (diff lines ~50–66)

**File:** `task/task.go`, `FilterByPriority` method  
**Issue:** The method accepts any string for `priority` and silently returns an empty slice for unrecognized values. While the caller (`main.go`) validates the input before calling, the method itself offers no domain protection. A better design would either:
- Accept a typed `Priority` constant, or
- Return an error for invalid priority strings.

This is defensible as "caller validates, callee filters" but violates the architecture rule: _"Validate inputs at the boundaries (public APIs, CLI args, file parsing)."_ `FilterByPriority` is a public API (`Manager` method on exported type).

**Severity:** Low — the caller does validate, and the current codebase has already merged this into `List()` which also doesn't validate the priority parameter at the method level.

### 5. [LOW] Tests use hardcoded `/tmp` paths — `task/task_test.go` (diff lines ~78, ~105)

**File:** `task/task_test.go`, `TestFilterByPriority` and `TestFilterByPriorityEmptyStore`  
**Lines:** `NewManager("/tmp/test_filter_tasks.json")` and `NewManager("/tmp/test_filter_empty.json")`  
**Issue:** Hardcoded `/tmp` paths are problematic:
- Not portable to Windows.
- Concurrent test runs may interfere with each other (race condition on shared file).
- Relies on `defer os.Remove` which won't execute if the test panics, leaving stale files.

**Suggested Fix:** Use `t.TempDir()` which automatically creates a unique temp directory and cleans it up:
```go
mgr := NewManager(filepath.Join(t.TempDir(), "tasks.json"))
```
**Note:** The current codebase has already adopted this pattern via the `newManager` helper — good improvement.

### 6. [LOW] No validation of `priority` in `Add` — `task/task.go`

**File:** `task/task.go`, `Add` method  
**Issue:** The `Add` method accepts any string for `priority` and stores it without validation. A caller could create a task with `priority = "critical"` or an empty string, which would then never match any filter. The `DueDate` parameter is validated but `priority` is not — inconsistent.  
**Suggested Fix:** Add priority validation:
```go
func (m *Manager) Add(title, priority, dueDate string) error {
    switch priority {
    case "high", "medium", "low":
        // valid
    default:
        return fmt.Errorf("invalid priority %q: must be high, medium, or low", priority)
    }
    // ...
}
```

### 7. [INFO] Race condition on shared file — `task/task.go` (pre-existing)

**File:** `task/task.go`, `load` / `save` methods  
**Issue:** If multiple `taskctl` processes run concurrently (e.g., in shell scripts), there is no file locking. Two concurrent `Add` calls could read the same state, and the second write would overwrite the first addition.  
**Architecture Rule:** _"Avoid race conditions — use locking when multiple processes may access shared state."_  
**Suggested Fix:** Use advisory file locking (`syscall.Flock` or a lock-file pattern) around load+save operations. This is a pre-existing architectural issue, not introduced by this PR.

### 8. [INFO] GoDoc comments added — positive note

The PR adds GoDoc comments to all exported types and methods in `task/task.go`. This is a nice improvement to code quality.

---

## Summary

| Severity | Count | Description |
|----------|-------|-------------|
| MEDIUM   | 3     | File permissions, non-atomic writes, manual arg parsing |
| LOW      | 3     | Missing input validation on public API, hardcoded /tmp in tests, no priority validation in Add |
| INFO     | 2     | Race condition (pre-existing), good GoDoc additions |

**Verdict:** The PR achieves its stated goal of adding `--priority` filtering to the `list` command. The most concerning issue **introduced by this PR** is the manual `os.Args` parsing (#3), which has since been fixed in the current codebase. The remaining issues are either pre-existing or low-severity. The test coverage for the new feature is solid with both table-driven and edge-case tests.

Several pre-existing issues (file permissions, non-atomic writes, race conditions) remain in the current codebase and should be addressed in follow-up PRs.
