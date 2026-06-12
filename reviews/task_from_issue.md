# Task: Synchronise README documentation with actual implementation and fix code quality issues

## Summary

The `taskctl` README is significantly out of sync with the current codebase. It documents an older version of the API (missing `--due` / `--overdue` flags, wrong command name `complete` vs `done`, outdated struct layout, missing `Overdue` stat), and the storage layer has security and reliability defects that violate project architecture rules. Both the documentation and the code need to be brought in line with each other.

---

## Source of Truth

- Codebase: `main.go`, `task/task.go`, `task/task_test.go`
- Architecture rules: `architecture.md`
- Current module: `github.com/geored/taskctl` (Go 1.22, zero external dependencies)

---

## Requirements

### 1. Fix README — Command name mismatch (`complete` vs `done`)

The README CLI Reference documents a `complete` sub-command, but the binary actually exposes `done`.

- **Action:** Update every occurrence of `taskctl complete <id>` in the README to `taskctl done <id>`.
- Affected sections: *CLI Reference → complete*, *Examples* workflow.

---

### 2. Fix README — `add` sub-command is missing `--due` flag

The `add` command accepts a `--due YYYY-MM-DD` flag (implemented in `runAdd`), but it is absent from the README.

- **Action:** Add `--due` to the `add` command's flag table and usage line:
  ```
  taskctl add <title> [--priority <level>] [--due YYYY-MM-DD]
  ```
- Add at least two examples showing `--due` usage (with and without `--priority`).

---

### 3. Fix README — `list` sub-command is missing `--overdue` flag

The `list` command accepts `--overdue` (implemented in `runList`), which filters to incomplete tasks whose due date has passed. The README makes no mention of this flag.

- **Action:** Add `--overdue` to the `list` command's flag table and usage line:
  ```
  taskctl list [--priority <level>] [--overdue]
  ```
- Update the sample output to show the `Due Date` column that is actually rendered by the code.
- Add an example: `taskctl list --overdue`.

---

### 4. Fix README — `stats` output is incomplete

The README sample output for `stats` is missing the `Overdue` count and the per-priority breakdown that `runStats` actually prints.

- **Action:** Update the example output to match the real output format:
  ```
  Total tasks:     <n>
    Pending:       <n>
    Completed:     <n>
    Overdue:       <n>
    High priority: <n>
    Med priority:  <n>
    Low priority:  <n>
  Completion rate: <n>%
  ```

---

### 5. Fix README — Features table is missing due-date and overdue features

The Features table lists six capabilities but omits:
- Due-date assignment on tasks (`--due`)
- Overdue task detection and filtering (`--overdue`, `[OVERDUE]` marker in list output)

- **Action:** Add two new rows to the Features table:

  | Feature | Description |
  |---|---|
  | 📅 **Due dates** | Attach an optional due date (YYYY-MM-DD) to any task with `--due` |
  | ⚠️ **Overdue detection** | Tasks past their due date are flagged `[OVERDUE]`; filter with `--overdue` |

---

### 6. Fix README — `Project Structure → Key types` block is stale

The embedded Go struct in the README uses the old layout (`Completed bool`, a `Priority` type alias, no `DueDate`). The actual struct is:

```go
type Task struct {
    ID       int    `json:"id"`
    Title    string `json:"title"`
    Done     bool   `json:"done"`
    Priority string `json:"priority"`
    DueDate  string `json:"due_date,omitempty"`
}
```

- **Action:** Replace the stale struct block in the README with the actual definition above. Remove the `Priority` type-alias constants (`PriorityLow`, etc.) that do not exist in the code.

---

### 7. Fix code — Insecure file permissions in `save()`

`task/task.go` → `Manager.save()` writes the tasks file with permission `0644`:

```go
if err := os.WriteFile(m.filePath, data, 0644); err != nil {
```

Per architecture rules, sensitive files must use `0600`.

- **Action:** Change `0644` → `0600`.

---

### 8. Fix code — Non-atomic file write in `save()`

The current `save()` calls `os.WriteFile` which overwrites the target in place. If the process is interrupted mid-write, the file is corrupted. Architecture rules require atomic writes (write-to-temp + rename).

- **Action:** Rewrite `save()` to:
  1. Write to a temporary file in the same directory as `m.filePath` (use `os.CreateTemp` or a `*.tmp` name).
  2. `os.Rename` the temp file over `m.filePath`.
  3. Set file permissions to `0600` on the temp file before rename.
  4. Ensure the temp file is cleaned up on any error path.

- Example skeleton:
  ```go
  func (m *Manager) save(tasks []Task) error {
      data, err := json.MarshalIndent(tasks, "", "  ")
      if err != nil {
          return fmt.Errorf("save: marshal: %w", err)
      }
      dir := filepath.Dir(m.filePath)
      tmp, err := os.CreateTemp(dir, "tasks-*.tmp")
      if err != nil {
          return fmt.Errorf("save: create temp: %w", err)
      }
      tmpName := tmp.Name()
      defer func() {
          // Clean up temp file on error; ignore error if already renamed.
          os.Remove(tmpName)
      }()
      if err := tmp.Chmod(0600); err != nil {
          tmp.Close()
          return fmt.Errorf("save: chmod: %w", err)
      }
      if _, err := tmp.Write(data); err != nil {
          tmp.Close()
          return fmt.Errorf("save: write: %w", err)
      }
      if err := tmp.Close(); err != nil {
          return fmt.Errorf("save: close: %w", err)
      }
      if err := os.Rename(tmpName, m.filePath); err != nil {
          return fmt.Errorf("save: rename: %w", err)
      }
      return nil
  }
  ```
- Add `"path/filepath"` to imports in `task.go`.

---

### 9. Add tests for atomic-save and permission behaviour

After implementing requirement 8, add tests to `task/task_test.go`:

- **`TestSaveFilePermissions`** — after `Add()`, verify the task file has mode `0600`.
- **`TestSaveAtomicity`** — verify that a read of tasks immediately after `save()` returns consistent data (no partial writes). This can be done by calling `Add()` and then `List()` in a tight loop with `-race`.

---

### 10. Verify all tests pass after changes

Run the full test suite (including race detector) before considering the work complete:

```bash
go test -race ./...
go vet ./...
```

All tests must pass with zero failures and zero race conditions detected.

---

## Acceptance Criteria

| # | Criterion |
|---|---|
| AC1 | README uses `done` (not `complete`) everywhere a task-completion command is shown |
| AC2 | README `add` section documents `--due YYYY-MM-DD` flag with examples |
| AC3 | README `list` section documents `--overdue` flag; sample output shows `Due Date` column |
| AC4 | README `stats` section shows correct output format including `Overdue` and per-priority lines |
| AC5 | README Features table includes due-date and overdue-detection rows |
| AC6 | README `Key types` block matches the actual `Task` struct in `task.go` |
| AC7 | `task/task.go` `save()` uses permission `0600` |
| AC8 | `task/task.go` `save()` uses atomic write-to-temp + rename pattern |
| AC9 | New tests for file permissions and save atomicity added to `task/task_test.go` |
| AC10 | `go test -race ./...` and `go vet ./...` pass with zero errors |

---

## Files to Change

| File | Change type |
|---|---|
| `README.md` | Documentation update (requirements 1–6) |
| `task/task.go` | Code fix — permissions + atomic save (requirements 7–8) |
| `task/task_test.go` | New tests (requirement 9) |

---

## Constraints

- **Zero new external dependencies** — the module must remain dependency-free (`go.mod` stays as-is).
- **No changes to the public API** — `Manager`, `Task`, `Stats`, and all exported methods keep their current signatures.
- **Go 1.22** — use only standard-library features available in Go 1.22.
- Do not introduce `print`/`fmt.Print` for logging in production code paths — all errors are already surfaced via `return fmt.Errorf(...)`.
