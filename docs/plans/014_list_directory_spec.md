# Specification: Tool Call M3 — `list_directory` Tool

**Status:** Draft
**Version:** 1.0
**Created:** 2026-03-02
**Scope:** Read-only directory listing tool with path security, `RegisterAll` pattern, and sub-agent tool wiring

---

## 1. Purpose

Implement the `list_directory` tool — the first concrete tool executor registered in the `Registry` (M2). This tool reads directory entries relative to the agent's workdir and returns them to the LLM. It is read-only with no side effects.

This milestone also introduces two pieces of infrastructure used by all future tool milestones (M4–M7):

1. **`RegisterAll` function** — a single function in `internal/tool/` that registers all implemented tool entries into a `*Registry`. Both call sites (`cmd/run.go` and `ExecuteCallAgent` in `tool.go`) call `RegisterAll` after `NewRegistry()`.
2. **`validatePath` function** — a package-private path validation function that resolves a relative path against a workdir and rejects absolute paths, `..` traversal, and symlinks that escape the workdir boundary. Reused by M4–M7 file tools.

Additionally, this milestone wires up `cfg.Tools` resolution for sub-agents in `ExecuteCallAgent`. M2 deferred this because no tools existed to register. Now that `list_directory` exists, sub-agents with `tools = ["list_directory"]` in their config must have those tools resolved and injected.

---

## 2. Design Decisions

The following decisions were made during planning and are binding for implementation:

1. **`RegisterAll` is the single registration point.** A new exported function `RegisterAll(r *Registry)` registers all implemented tools. Both `cmd/run.go` (after `NewRegistry()`) and `ExecuteCallAgent` (after `NewRegistry()`) call it. Future milestones add lines to `RegisterAll` — they do not touch call sites.

2. **`validatePath` uses `filepath.EvalSymlinks`.** After `filepath.Join` + `filepath.Clean`, the resolved path is passed through `filepath.EvalSymlinks` to dereference symlinks. The resulting real path is checked with `strings.HasPrefix` against the cleaned workdir. This catches both `..` traversal and symlink-based escapes.

3. **`path="."` is valid input.** It resolves to the workdir root. The LLM can list the top-level project directory.

4. **Output format is one entry per line.** Subdirectories are suffixed with `/`. No other metadata (size, permissions, timestamps). Entries are returned in `os.ReadDir` order (lexicographic by name).

5. **`call_agent` remains outside the registry.** Same as M2 — `call_agent` is special-cased at dispatch sites. `RegisterAll` does not register it.

6. **No new external dependencies.** Uses only Go stdlib (`os`, `path/filepath`, `strings`, `fmt`).

7. **Sub-agent tool resolution is wired up.** `ExecuteCallAgent` now calls `RegisterAll` on its registry and resolves `cfg.Tools` via `registry.Resolve`, appending the results to the sub-agent's `req.Tools`.

---

## 3. Requirements

### 3.1 `RegisterAll` Function

**Requirement 1.1:** Define an exported function `RegisterAll(r *Registry)` in `internal/tool/registry.go`.

**Requirement 1.2:** `RegisterAll` must call `r.Register(toolname.ListDirectory, ...)` with the `list_directory` tool entry.

**Requirement 1.3:** `RegisterAll` must import `internal/toolname` and use the `toolname.ListDirectory` constant for the tool name. The tool name string must not be hardcoded.

**Requirement 1.4:** `RegisterAll` must be safe to call multiple times on the same registry (idempotent — `Register` silently replaces duplicates).

**Requirement 1.5:** Future milestones (M4–M7) extend `RegisterAll` by adding additional `r.Register(...)` calls. No other wiring changes are needed at call sites.

### 3.2 `validatePath` Function

**Requirement 2.1:** Define a package-private function `validatePath(workdir, relPath string) (string, error)` in a file within `internal/tool/`.

**Requirement 2.2:** If `relPath` is empty, return an error with the message: `path is required`.

**Requirement 2.3:** If `relPath` is an absolute path (as determined by `filepath.IsAbs`), return an error with the message: `absolute paths are not allowed`.

**Requirement 2.4:** Compute the resolved path: `filepath.Clean(filepath.Join(workdir, relPath))`.

**Requirement 2.5:** Evaluate symlinks on the resolved path using `filepath.EvalSymlinks`. If `EvalSymlinks` returns an error (e.g., path does not exist), return that error directly — the caller handles nonexistent paths.

**Requirement 2.6:** Compute the clean workdir: `filepath.Clean(workdir)`. Evaluate symlinks on the workdir using `filepath.EvalSymlinks`. If this fails, return the error.

**Requirement 2.7:** Check that the evaluated resolved path starts with the evaluated clean workdir using `strings.HasPrefix`. If it does not, return an error with the message: `path escapes workdir`.

**Requirement 2.8:** The path `"."` must be valid and resolve to the workdir itself.

**Requirement 2.9:** `validatePath` must be reusable by M4–M7 tool implementations without modification.

### 3.3 `list_directory` Tool Definition

**Requirement 3.1:** Create a file `internal/tool/list_directory.go`.

**Requirement 3.2:** The tool definition returned by the `Definition` function must have:
- `Name`: the value of `toolname.ListDirectory` (i.e., `"list_directory"`)
- `Description`: a clear description for the LLM explaining the tool lists directory contents relative to the working directory
- `Parameters`: a single parameter `path` with:
  - `Type`: `"string"`
  - `Description`: a description stating it is a relative path to the directory to list, and that `"."` lists the working directory root
  - `Required`: `true`

**Requirement 3.3:** The tool name in the definition must use `toolname.ListDirectory`, not a hardcoded string.

### 3.4 `list_directory` Tool Executor

**Requirement 4.1:** The `Execute` function must have the signature: `func(ctx context.Context, call provider.ToolCall, ec ExecContext) provider.ToolResult`.

**Requirement 4.2:** Extract the `path` argument from `call.Arguments["path"]`.

**Requirement 4.3:** Call `validatePath(ec.Workdir, path)` to get the resolved absolute path. If `validatePath` returns an error, return a `ToolResult` with `CallID: call.ID`, `Content: error.Error()`, `IsError: true`.

**Requirement 4.4:** Call `os.ReadDir(resolvedPath)` to read directory entries. If `os.ReadDir` returns an error (e.g., path is a file, not a directory, or permission denied), return a `ToolResult` with `CallID: call.ID`, `Content: error.Error()`, `IsError: true`.

**Requirement 4.5:** Build the output string: one entry per line. For each entry, use `entry.Name()`. If `entry.IsDir()` is true, append `/` to the name.

**Requirement 4.6:** Return a `ToolResult` with `CallID: call.ID`, `Content: the formatted listing`, `IsError: false`.

**Requirement 4.7:** An empty directory must return a `ToolResult` with `Content: ""` (empty string) and `IsError: false`.

**Requirement 4.8:** The listing must not include `"."` or `".."` entries. (`os.ReadDir` does not return these.)

### 3.5 Registration in Registry

**Requirement 5.1:** The `list_directory` tool entry must be registered via `RegisterAll`, not at individual call sites.

**Requirement 5.2:** The `ToolEntry` must have both `Definition` and `Execute` set to non-nil functions.

### 3.6 Wiring: `cmd/run.go`

**Requirement 6.1:** After `registry := tool.NewRegistry()` (line 214), call `tool.RegisterAll(registry)`.

**Requirement 6.2:** No other changes to `cmd/run.go`. The existing `registry.Resolve(cfg.Tools)` call (line 223) now succeeds for `"list_directory"` because it is registered.

### 3.7 Wiring: `internal/tool/tool.go` `ExecuteCallAgent`

**Requirement 7.1:** Replace `NewRegistry()` on line 245 with a registry created via `NewRegistry()` followed by `RegisterAll(registry)`.

**Requirement 7.2:** After the existing `call_agent` tool injection (lines 228–232), add tool resolution for sub-agents: if `len(cfg.Tools) > 0`, call `registry.Resolve(cfg.Tools)` and append the resulting `[]provider.Tool` to `req.Tools`.

**Requirement 7.3:** If `registry.Resolve` returns an error for a sub-agent's tools, return an error `ToolResult` with `CallID: call.ID`, `Content` containing the error message, and `IsError: true`.

**Requirement 7.4:** Tool injection order for sub-agents: `call_agent` first (if applicable), then resolved `cfg.Tools`.

**Requirement 7.5:** The registry with registered tools must be passed to `runConversationLoop` so that dispatch works for sub-agent tool calls.

---

## 4. Project Structure

After completion, the following files will be added or modified:

```
axe/
├── cmd/
│   └── run.go                          # MODIFIED: add RegisterAll call after NewRegistry
├── internal/
│   └── tool/
│       ├── list_directory.go           # NEW: Definition, Execute, helper entry func
│       ├── list_directory_test.go      # NEW: table-driven tests
│       ├── path_validation.go          # NEW: validatePath function
│       ├── path_validation_test.go     # NEW: validatePath tests
│       ├── registry.go                 # MODIFIED: add RegisterAll function
│       ├── registry_test.go            # UNCHANGED
│       ├── tool.go                     # MODIFIED: RegisterAll + sub-agent tool resolution in ExecuteCallAgent
│       └── tool_test.go                # UNCHANGED (existing tests must still pass)
├── go.mod                              # UNCHANGED
├── go.sum                              # UNCHANGED
└── ...                                 # all other files UNCHANGED
```

---

## 5. Edge Cases

### 5.1 Path Validation

| Scenario | Input `relPath` | Behavior |
|----------|----------------|----------|
| Empty path | `""` | Error: `path is required` |
| Dot path | `"."` | Resolves to workdir. Valid. |
| Simple relative | `"src"` | Resolves to `workdir/src`. Valid if within workdir. |
| Nested relative | `"src/pkg/util"` | Resolves to `workdir/src/pkg/util`. Valid if within workdir. |
| Absolute path | `"/etc"` | Error: `absolute paths are not allowed` |
| Absolute with tilde | `"~/Documents"` | Not absolute per `filepath.IsAbs` on Linux. Resolves to `workdir/~/Documents`. Likely nonexistent. Error from `EvalSymlinks` or `ReadDir`. |
| Parent traversal within workdir | `"src/../lib"` | Resolves to `workdir/lib`. Valid (still within workdir). |
| Parent traversal escaping workdir | `"../../etc"` | After clean + eval, does not start with workdir. Error: `path escapes workdir` |
| Deep parent traversal | `"a/b/../../../../etc"` | After clean + eval, escapes workdir. Error: `path escapes workdir` |
| Symlink within workdir | Symlink `workdir/link` → `workdir/real` | `EvalSymlinks` resolves to `workdir/real`. HasPrefix check passes. Valid. |
| Symlink escaping workdir | Symlink `workdir/link` → `/tmp/outside` | `EvalSymlinks` resolves to `/tmp/outside`. HasPrefix check fails. Error: `path escapes workdir` |
| Nonexistent path | `"no_such_dir"` | `EvalSymlinks` returns error (ENOENT). Error propagated to caller. |
| Path with trailing slash | `"src/"` | `filepath.Clean` removes trailing slash. Resolves normally. |

### 5.2 Directory Listing

| Scenario | Behavior |
|----------|----------|
| Directory with files and subdirs | Returns entries: files without suffix, dirs with `/` suffix, one per line |
| Empty directory | Returns `ToolResult{Content: "", IsError: false}` |
| Nonexistent directory | `validatePath` fails (EvalSymlinks error) → `IsError: true` |
| Path points to a file, not a directory | `os.ReadDir` returns error → `IsError: true` |
| Permission denied | `os.ReadDir` returns error → `IsError: true` |
| Directory with hidden files (`.gitignore`) | Included in listing. No filtering. |
| Large directory (thousands of entries) | All entries returned. No truncation. No pagination. |

### 5.3 Argument Handling

| Scenario | Behavior |
|----------|----------|
| `path` argument present | Normal operation |
| `path` argument missing from `call.Arguments` | `call.Arguments["path"]` returns `""` → `validatePath` returns error: `path is required` |
| `path` argument is whitespace-only | `filepath.IsAbs` returns false, `filepath.Join` handles it, `EvalSymlinks` may fail. Not explicitly trimmed. |

### 5.4 Sub-Agent Tool Resolution

| Scenario | Behavior |
|----------|----------|
| Sub-agent has `tools = ["list_directory"]` | `RegisterAll` registers it. `Resolve` succeeds. Tool definition appended to `req.Tools`. |
| Sub-agent has `tools = ["list_directory"]` and `sub_agents = ["helper"]` | `req.Tools` has `call_agent` + `list_directory` definition. Both dispatch correctly. |
| Sub-agent has `tools = ["read_file"]` (not yet implemented) | `Resolve` fails: `unknown tool "read_file"`. Error `ToolResult` returned. |
| Sub-agent has `tools = []` | No resolution needed. No tools appended from `cfg.Tools`. |

---

## 6. Constraints

**Constraint 1:** No new external dependencies. `go.mod` must remain unchanged.

**Constraint 2:** No changes to `internal/provider/`, `internal/agent/`, or `internal/toolname/` packages.

**Constraint 3:** `call_agent` must NOT be registered in the registry. It remains special-cased.

**Constraint 4:** `validatePath` must not follow symlinks during the initial `filepath.Join`/`filepath.Clean` step — only `filepath.EvalSymlinks` does symlink resolution, and only after the path is constructed.

**Constraint 5:** The tool must not recurse into subdirectories. It lists a single directory level.

**Constraint 6:** No filtering of entries. Hidden files, dotfiles, and all other entries are included.

**Constraint 7:** No output truncation for large directories.

**Constraint 8:** The tool must not write to the filesystem.

**Constraint 9:** Cross-platform compatibility: must build and run on Linux, macOS, and Windows. Path separator handling must use `filepath` (not `path`).

**Constraint 10:** `ToolCall.Arguments` is `map[string]string`. All parameter values are strings parsed from the LLM's JSON tool call output. No type conversions needed for `list_directory` (its only parameter is a string).

---

## 7. Testing Requirements

### 7.1 Test Conventions

Tests must follow the patterns established in prior milestones:

- **Package-level tests:** Tests live in the same package (`package tool`)
- **Standard library only:** Use `testing` package. No test frameworks.
- **Temp directories:** Use `t.TempDir()` for filesystem isolation.
- **Table-driven tests:** Where multiple cases share structure.
- **Descriptive names:** `TestFunctionName_Scenario` with underscores.
- **Test real code, not mocks.** Tests must call actual functions with real filesystem operations. Each test must fail if the code under test is deleted.
- **Red/green TDD:** Write failing tests first, then implement code to make them pass.
- **Run tests with:** `make test`

### 7.2 `internal/tool/path_validation_test.go` Tests

**Test: `TestValidatePath_ValidRelativePath`** — Create a tmpdir with a subdirectory. Call `validatePath(tmpdir, "subdir")`. Verify the returned path equals the expected absolute path and error is nil.

**Test: `TestValidatePath_DotPath`** — Call `validatePath(tmpdir, ".")`. Verify the returned path equals `tmpdir` (cleaned) and error is nil.

**Test: `TestValidatePath_NestedPath`** — Create `tmpdir/a/b/c`. Call `validatePath(tmpdir, "a/b/c")`. Verify success.

**Test: `TestValidatePath_EmptyPath`** — Call `validatePath(tmpdir, "")`. Verify error message contains `path is required`.

**Test: `TestValidatePath_AbsolutePath`** — Call `validatePath(tmpdir, "/etc")`. Verify error message contains `absolute paths are not allowed`.

**Test: `TestValidatePath_ParentTraversalEscape`** — Call `validatePath(tmpdir, "../../etc")`. Verify error message contains `path escapes workdir`.

**Test: `TestValidatePath_ParentTraversalWithinWorkdir`** — Create `tmpdir/a/b` and call `validatePath(tmpdir, "a/b/../b")`. Verify success (path stays within workdir).

**Test: `TestValidatePath_SymlinkWithinWorkdir`** — Create a symlink inside tmpdir pointing to another location inside tmpdir. Call `validatePath`. Verify success.

**Test: `TestValidatePath_SymlinkEscapingWorkdir`** — Create a symlink inside tmpdir pointing to a location outside tmpdir (e.g., `os.TempDir()`). Call `validatePath`. Verify error message contains `path escapes workdir`.

**Test: `TestValidatePath_NonexistentPath`** — Call `validatePath(tmpdir, "no_such_dir")`. Verify an error is returned (from `EvalSymlinks`).

### 7.3 `internal/tool/list_directory_test.go` Tests

**Test: `TestListDirectory_ExistingDir`** — Create a tmpdir with two files (`a.txt`, `b.txt`) and one subdirectory (`sub/`). Call the `Execute` function with `Arguments: {"path": "."}`. Verify `IsError` is false and `Content` contains `a.txt`, `b.txt`, and `sub/` (with `/` suffix), one per line.

**Test: `TestListDirectory_NestedPath`** — Create `tmpdir/sub/` with a file inside. Call with `Arguments: {"path": "sub"}`. Verify only the nested directory's contents are listed.

**Test: `TestListDirectory_EmptyDir`** — Create an empty tmpdir. Call with `Arguments: {"path": "."}`. Verify `IsError` is false and `Content` is `""`.

**Test: `TestListDirectory_NonexistentPath`** — Call with `Arguments: {"path": "no_such_dir"}`. Verify `IsError` is true.

**Test: `TestListDirectory_AbsolutePathRejected`** — Call with `Arguments: {"path": "/etc"}`. Verify `IsError` is true and content mentions absolute paths.

**Test: `TestListDirectory_ParentTraversalRejected`** — Call with `Arguments: {"path": "../../etc"}`. Verify `IsError` is true and content mentions path escaping.

**Test: `TestListDirectory_SymlinkEscapeRejected`** — Create a symlink inside tmpdir pointing outside. Call with the symlink as path. Verify `IsError` is true.

**Test: `TestListDirectory_MissingPathArgument`** — Call with empty `Arguments` map. Verify `IsError` is true and content mentions path required.

**Test: `TestListDirectory_CallIDPassthrough`** — Call with a specific `call.ID`. Verify the returned `ToolResult.CallID` matches.

### 7.4 `RegisterAll` Tests

**Test: `TestRegisterAll_RegistersListDirectory`** — Call `NewRegistry()`, then `RegisterAll(r)`. Verify `r.Has(toolname.ListDirectory)` returns true.

**Test: `TestRegisterAll_ResolvesListDirectory`** — Call `RegisterAll(r)`, then `r.Resolve([]string{toolname.ListDirectory})`. Verify the returned tool has `Name` equal to `toolname.ListDirectory` and has a `path` parameter.

**Test: `TestRegisterAll_Idempotent`** — Call `RegisterAll(r)` twice on the same registry. Verify no panic and `Has` still returns true.

### 7.5 Existing Tests

All existing tests must continue to pass without modification:

- `internal/tool/tool_test.go` — All existing tests must pass.
- `internal/tool/registry_test.go` — All existing tests must pass.
- All other test files in the project.

### 7.6 Running Tests

All tests must pass when run with:

```bash
make test
```

---

## 8. Acceptance Criteria

| Criterion | Verification |
|-----------|-------------|
| `internal/tool/list_directory.go` exists and compiles | `go build ./internal/tool/` succeeds |
| `internal/tool/path_validation.go` exists and compiles | `go build ./internal/tool/` succeeds |
| `RegisterAll` registers `list_directory` | `TestRegisterAll_RegistersListDirectory` passes |
| `RegisterAll` is called in `cmd/run.go` | Agent with `tools = ["list_directory"]` resolves successfully |
| `RegisterAll` is called in `ExecuteCallAgent` | Sub-agent with `tools = ["list_directory"]` resolves successfully |
| Sub-agent `cfg.Tools` resolved and injected | `req.Tools` includes `list_directory` definition for sub-agents |
| `validatePath` rejects empty path | `TestValidatePath_EmptyPath` passes |
| `validatePath` rejects absolute paths | `TestValidatePath_AbsolutePath` passes |
| `validatePath` rejects `..` traversal | `TestValidatePath_ParentTraversalEscape` passes |
| `validatePath` rejects symlink escape | `TestValidatePath_SymlinkEscapingWorkdir` passes |
| `validatePath` allows `"."` | `TestValidatePath_DotPath` passes |
| `validatePath` allows valid relative paths | `TestValidatePath_ValidRelativePath` passes |
| `list_directory` lists files and subdirs | `TestListDirectory_ExistingDir` passes |
| `list_directory` handles empty dirs | `TestListDirectory_EmptyDir` passes |
| `list_directory` returns errors for invalid paths | Error test cases pass |
| `CallID` is propagated correctly | `TestListDirectory_CallIDPassthrough` passes |
| All existing tests pass | `make test` exits 0 |

---

## 9. Out of Scope

The following items are explicitly **not** included in this milestone:

1. Other tool executors (`read_file`, `write_file`, `edit_file`, `run_command`) — M4–M7 scope
2. Recursive directory listing
3. Filtering entries (hidden files, patterns, etc.)
4. Output truncation or pagination for large directories
5. File metadata in output (size, permissions, modification time)
6. Changes to `--dry-run`, `--json`, or `--verbose` output — M8 scope
7. Changes to `internal/agent/`, `internal/provider/`, or `internal/toolname/` packages
8. Thread safety for `Registry` writes (registration happens at startup before concurrent dispatch)

---

## 10. References

- Milestone Definition: `docs/plans/000_tool_call_milestones.md` (M3 section, lines 47–56)
- M2 Spec: `docs/plans/013_tool_registry_spec.md`
- M2 Implementation: `docs/plans/013_tool_registry_implement.md`
- Current `NewRegistry()` in `cmd/run.go`: line 214
- Current `NewRegistry()` in `ExecuteCallAgent`: `internal/tool/tool.go` line 245
- Sub-agent tool injection: `internal/tool/tool.go` lines 228–232
- `provider.Tool` type: `internal/provider/provider.go:34–38`
- `provider.ToolCall` type: `internal/provider/provider.go:41–45`
- `provider.ToolResult` type: `internal/provider/provider.go:48–52`
- `toolname.ListDirectory` constant: `internal/toolname/toolname.go:8`
- `Registry.Register`: `internal/tool/registry.go:40`
- `Registry.Resolve`: `internal/tool/registry.go:54`
- `Registry.Dispatch`: `internal/tool/registry.go:76`

---

## 11. Notes

- **`validatePath` calls `EvalSymlinks` on both the resolved path and the workdir.** This is necessary because if the workdir itself is behind a symlink, the HasPrefix check would fail even for valid paths. Both sides must be evaluated to the same "reality" before comparison.
- **`EvalSymlinks` on nonexistent paths returns an error.** This means `validatePath` cannot distinguish between "path escapes workdir" and "path does not exist" when the path happens to be both traversal and nonexistent. This is acceptable — both are error conditions, and the error message from `EvalSymlinks` is descriptive enough.
- **The `RegisterAll` pattern scales.** M4 adds one line (`r.Register(toolname.ReadFile, ...)`). M5–M7 do the same. No call-site changes needed after M3.
- **Sub-agent tool resolution error handling mirrors top-level.** In `cmd/run.go`, a resolve error returns `ExitError{Code: 1}`. In `ExecuteCallAgent`, it returns an error `ToolResult` — because sub-agents communicate results, not exit codes.
- **`os.ReadDir` returns entries sorted by name.** This is guaranteed by the Go stdlib. The output order is deterministic.
