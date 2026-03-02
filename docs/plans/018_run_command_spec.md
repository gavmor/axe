# Specification: Tool Call M7 — `run_command` Tool

**Status:** Draft
**Version:** 1.0
**Created:** 2026-03-02
**Scope:** Shell command execution tool with combined output capture, exit code reporting, context-based timeout, output truncation, and workdir sandboxing

---

## 1. Purpose

Implement the `run_command` tool — a shell execution tool registered in the `Registry`. This tool runs a shell command via `sh -c` in the agent's workdir, captures combined stdout+stderr output, and reports success or failure based on the command's exit code.

This is the final tool in the M3–M7 tool series and the most powerful — shell access is intentionally implemented last. It builds on the registry infrastructure established in M2:

- **`RegisterAll`** — extended with one additional `r.Register(...)` call
- **`toolname.RunCommand`** — constant already declared in `internal/toolname/toolname.go`
- **`ExecContext`** — reused for `Workdir` (sets `cmd.Dir`)

This tool does NOT use `validatePath` or `isWithinDir`. The command string is passed directly to `sh -c`; the workdir is the only sandbox boundary (set via `cmd.Dir`). Path security is the agent config's responsibility, not the tool's.

---

## 2. Design Decisions

The following decisions were made during planning and are binding for implementation:

1. **Execution via `sh -c`.** The command string is passed to `exec.CommandContext(ctx, "sh", "-c", command)`. This allows the LLM to use shell features (pipes, redirects, chaining). No argument splitting or escaping is performed by the tool.

2. **`exec.CommandContext` is used, not `exec.Command`.** The `ctx` parameter from the executor signature is passed directly to `exec.CommandContext`. This means context cancellation (e.g., from a deadline/timeout) will kill the process. The tool does not impose its own timeout — it inherits whatever context the caller provides.

3. **Combined stdout+stderr capture.** The tool uses `cmd.CombinedOutput()` to capture both stdout and stderr into a single byte slice. Stdout and stderr are interleaved in the order the OS delivers them. There is no separation of stdout from stderr in the result.

4. **Exit code extraction via `exec.ExitError`.** On a non-zero exit, `cmd.CombinedOutput()` returns an error. If the error is an `*exec.ExitError` (checked via `errors.As`), the exit code is extracted from `exitErr.ExitCode()`. If the error is NOT an `*exec.ExitError` (e.g., command not found, permission denied, context cancelled), the error message from `err.Error()` is used directly.

5. **Success result format.** When the command exits with code 0, the tool returns `ToolResult{CallID: call.ID, Content: string(output), IsError: false}`. The content is the raw command output as a string. If the command produces no output, `Content` is an empty string.

6. **Failure result format for exit errors.** When the command exits with a non-zero code and the error is an `*exec.ExitError`, the tool returns `ToolResult{CallID: call.ID, Content: "exit code N\n" + string(output), IsError: true}` where `N` is the integer exit code and `output` is whatever was captured before/during the failure.

7. **Failure result format for non-exit errors.** When the error is NOT an `*exec.ExitError` (e.g., `exec: "sh": executable file not found`), the tool returns `ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}`. No output is prepended because `CombinedOutput` returns nil output on these errors.

8. **Output truncation at 100KB.** If the combined output exceeds 102400 bytes (100 * 1024), it is truncated to 102400 bytes and the string `"\n... [output truncated, exceeded 100KB]"` is appended. Truncation happens before the exit code prefix is prepended (for failure cases). This means the total content for a failing command could exceed 102400 bytes by the length of the prefix and truncation notice.

9. **`cmd.Dir` set to `ec.Workdir`.** The command executes with its working directory set to the agent's workdir from `ExecContext`. This is the only sandboxing the tool provides.

10. **Single parameter: `command`.** The tool has one required parameter named `command` of type `string`. There are no optional parameters. An empty or missing `command` is rejected before execution.

11. **`call_agent` remains outside the registry.** Same as M2–M6.

12. **No new external dependencies.** Uses only Go stdlib (`os/exec`, `context`, `errors`, `fmt`).

---

## 3. Requirements

### 3.1 `run_command` Tool Definition

**Requirement 1.1:** Create a file `internal/tool/run_command.go`.

**Requirement 1.2:** Define an unexported function `runCommandEntry() ToolEntry` that returns a `ToolEntry` with both `Definition` and `Execute` set to non-nil functions.

**Requirement 1.3:** The tool definition returned by the `Definition` function must have:
- `Name`: the value of `toolname.RunCommand` (i.e., `"run_command"`)
- `Description`: a clear description for the LLM explaining the tool executes a shell command in the agent's working directory and returns the combined stdout/stderr output
- `Parameters`: one parameter:
  - `command`:
    - `Type`: `"string"`
    - `Description`: a description stating it is the shell command to execute
    - `Required`: `true`

**Requirement 1.4:** The tool name in the definition must use `toolname.RunCommand`, not a hardcoded string.

**Requirement 1.5:** `ToolCall.Arguments` is `map[string]string`. The `command` parameter is a string used directly — no type conversion is needed.

### 3.2 `run_command` Tool Executor

**Requirement 2.1:** The `Execute` function must have the signature: `func(ctx context.Context, call provider.ToolCall, ec ExecContext) provider.ToolResult`.

**Requirement 2.2:** Extract the `command` argument from `call.Arguments["command"]`.

**Requirement 2.3 (Empty Command Check):** If `command` is an empty string (including when the key is absent from the map), return a `ToolResult` with `CallID: call.ID`, `Content: "command is required"`, `IsError: true`.

**Requirement 2.4 (Create Command):** Create the command using `exec.CommandContext(ctx, "sh", "-c", command)`. Set `cmd.Dir` to `ec.Workdir`.

**Requirement 2.5 (Execute and Capture):** Call `cmd.CombinedOutput()` to execute the command and capture combined stdout+stderr.

**Requirement 2.6 (Truncation):** If the output byte slice length exceeds 102400 bytes (100 * 1024), truncate the slice to 102400 bytes and append the string `"\n... [output truncated, exceeded 100KB]"` to the string representation.

**Requirement 2.7 (Success Case):** If `cmd.CombinedOutput()` returns a nil error (exit code 0), return a `ToolResult` with `CallID: call.ID`, `Content: string(output)` (after truncation if applicable), `IsError: false`.

**Requirement 2.8 (Failure — ExitError):** If `cmd.CombinedOutput()` returns a non-nil error and the error is an `*exec.ExitError` (checked via `errors.As`), return a `ToolResult` with `CallID: call.ID`, `Content` in the format `"exit code %d\n%s"` (where the first value is `exitErr.ExitCode()` and the second value is `string(output)` after truncation if applicable), `IsError: true`.

**Requirement 2.9 (Failure — Non-ExitError):** If `cmd.CombinedOutput()` returns a non-nil error and the error is NOT an `*exec.ExitError`, return a `ToolResult` with `CallID: call.ID`, `Content: err.Error()`, `IsError: true`.

### 3.3 Registration in Registry

**Requirement 3.1:** Add `r.Register(toolname.RunCommand, runCommandEntry())` to the `RegisterAll` function in `internal/tool/registry.go`.

**Requirement 3.2:** This is the only change to `registry.go`. No call-site changes are needed in `cmd/run.go` or `internal/tool/tool.go`.

### 3.4 Registry Tests

**Requirement 4.1:** Add tests to `internal/tool/registry_test.go` that verify `run_command` is registered by `RegisterAll`.

**Requirement 4.2:** Add a test that resolves `run_command` and verifies the tool definition has the correct name and the `command` parameter.

---

## 4. Project Structure

After completion, the following files will be added or modified:

```
axe/
├── internal/
│   └── tool/
│       ├── run_command.go              # NEW: Definition, Execute, helper entry func
│       ├── run_command_test.go         # NEW: tests
│       ├── registry.go                 # MODIFIED: add one line to RegisterAll
│       ├── registry_test.go            # MODIFIED: add run_command registration tests
│       ├── edit_file.go                # UNCHANGED
│       ├── edit_file_test.go           # UNCHANGED
│       ├── write_file.go              # UNCHANGED
│       ├── write_file_test.go         # UNCHANGED
│       ├── read_file.go               # UNCHANGED
│       ├── read_file_test.go          # UNCHANGED
│       ├── list_directory.go          # UNCHANGED
│       ├── list_directory_test.go     # UNCHANGED
│       ├── path_validation.go         # UNCHANGED
│       ├── path_validation_test.go    # UNCHANGED
│       ├── tool.go                    # UNCHANGED
│       └── tool_test.go              # UNCHANGED
├── go.mod                             # UNCHANGED
├── go.sum                             # UNCHANGED
└── ...                                # all other files UNCHANGED
```

---

## 5. Edge Cases

### 5.1 Command Argument

| Scenario | Input `command` | Behavior |
|----------|----------------|----------|
| Empty command | `""` | Error: `"command is required"` |
| Missing `command` key in Arguments map | Key absent | `call.Arguments["command"]` returns `""` → Error: `"command is required"` |
| Simple command | `"echo hello"` | Executes `sh -c "echo hello"`. Returns output `"hello\n"`. |
| Command with pipes | `"echo hello \| tr a-z A-Z"` | Executes via `sh -c`. Shell handles pipe. Returns `"HELLO\n"`. |
| Command with redirects | `"echo hello > /dev/null"` | Executes via `sh -c`. No stdout captured (redirected to /dev/null). Returns empty string. |
| Command with semicolons | `"echo a; echo b"` | Both commands run. Returns `"a\nb\n"`. |
| Command with shell expansion | `"echo $HOME"` | Shell expands `$HOME`. Returns the home directory path. |
| Whitespace-only command | `"   "` | Passed to `sh -c "   "`. Shell executes empty command, exits 0. Returns empty string. Not rejected by the empty check (the check is for empty string, not whitespace-only). |

### 5.2 Exit Code Handling

| Scenario | Command | Behavior |
|----------|---------|----------|
| Exit code 0 (success) | `"true"` or `"exit 0"` | `IsError: false`. Content is command output (empty for these commands). |
| Exit code 1 | `"exit 1"` | `IsError: true`. Content: `"exit code 1\n"`. |
| Exit code 42 (arbitrary) | `"exit 42"` | `IsError: true`. Content: `"exit code 42\n"`. |
| Exit code 127 (command not found, from shell) | `"nonexistent_command_xyz"` | Shell returns exit 127. `IsError: true`. Content: `"exit code 127\n<shell error message>"`. |
| Exit code 126 (permission denied, from shell) | `"sh -c 'exit 126'"` | `IsError: true`. Content: `"exit code 126\n"`. |
| Signal termination (e.g., SIGKILL) | Process killed by signal | `exec.ExitError` with platform-dependent exit code (typically -1 or 128+signal). `IsError: true`. Content includes exit code and any captured output. |

### 5.3 Output Capture

| Scenario | Command | Behavior |
|----------|---------|----------|
| stdout only | `"echo hello"` | Content: `"hello\n"`. |
| stderr only | `"echo error >&2"` | Content: `"error\n"`. (stderr captured by `CombinedOutput`.) |
| Both stdout and stderr | `"echo out; echo err >&2"` | Content contains both `"out\n"` and `"err\n"`. Order is OS-dependent but typically interleaved. |
| No output | `"true"` | Content: `""` (empty string). `IsError: false`. |
| No output on failure | `"exit 1"` | Content: `"exit code 1\n"`. |
| Output with trailing newline | `"echo hello"` | Content: `"hello\n"`. No stripping of trailing newlines. |
| Binary-like output | `"printf '\\x00\\x01\\x02'"` | Content contains raw bytes as string. No encoding or escaping. |

### 5.4 Output Truncation

| Scenario | Output size | Behavior |
|----------|------------|----------|
| Output < 100KB | < 102400 bytes | No truncation. Content is full output. |
| Output = 100KB exactly | 102400 bytes | No truncation. Threshold is "exceeds", not "equals". |
| Output > 100KB | > 102400 bytes | Truncated to 102400 bytes. `"\n... [output truncated, exceeded 100KB]"` appended. |
| Large output on success | > 100KB, exit 0 | Content: first 102400 bytes + truncation notice. `IsError: false`. |
| Large output on failure | > 100KB, exit 1 | Content: `"exit code 1\n"` + first 102400 bytes of output + truncation notice. `IsError: true`. |
| Truncation at multi-byte boundary | Output truncated mid-UTF-8 sequence | Truncation occurs at the byte level. The resulting string may contain an incomplete UTF-8 sequence. No rune-aware truncation is performed. |

### 5.5 Context and Timeout

| Scenario | Behavior |
|----------|----------|
| Context with no deadline | Command runs until completion. No timeout. |
| Context with deadline, command finishes before deadline | Normal result (success or failure based on exit code). |
| Context with deadline, command exceeds deadline | `exec.CommandContext` sends SIGKILL to the process. `CombinedOutput` returns an error. The error is an `*exec.ExitError` (killed process). `IsError: true`. Content includes whatever output was captured before kill and the exit code. |
| Context already cancelled before execution | `CombinedOutput` returns immediately with a context error. This is NOT an `*exec.ExitError` — falls through to Requirement 2.9. Content: `err.Error()` (e.g., `"context canceled"`). |

### 5.6 Workdir

| Scenario | Behavior |
|----------|----------|
| `ec.Workdir` is a valid directory | `cmd.Dir` set to workdir. Command sees this as its current directory. |
| `ec.Workdir` is empty string | `cmd.Dir` is `""`. Go's `exec` package uses the current process's working directory. |
| `ec.Workdir` does not exist | `CombinedOutput` returns an error (not an `*exec.ExitError`). Content: `err.Error()`. `IsError: true`. |
| Command accesses files outside workdir | Allowed. The tool does not restrict file access. Sandboxing is the agent config's responsibility. |

### 5.7 Argument Handling

| Scenario | Behavior |
|----------|----------|
| `command` argument present | Normal operation. |
| `command` argument missing from `call.Arguments` | `call.Arguments["command"]` returns `""` → Error: `"command is required"`. |
| `command` argument is empty string | Error: `"command is required"`. |

---

## 6. Constraints

**Constraint 1:** No new external dependencies. `go.mod` must remain unchanged.

**Constraint 2:** No changes to `internal/provider/`, `internal/agent/`, or `internal/toolname/` packages.

**Constraint 3:** No changes to `cmd/run.go` or `internal/tool/tool.go`. The only modifications outside the new files are adding one line to `RegisterAll` in `registry.go` and adding tests to `registry_test.go`.

**Constraint 4:** `call_agent` must NOT be registered in the registry. It remains special-cased.

**Constraint 5:** No path validation functions (`validatePath`, `isWithinDir`) are used by this tool. The command string is passed directly to `sh -c`.

**Constraint 6:** Cross-platform note: the tool uses `sh -c` which requires a POSIX-compatible shell. This works on Linux and macOS. On Windows, `sh` may not be available unless a POSIX environment is installed (e.g., Git Bash, WSL). This is acceptable — axe is a Unix-first CLI tool (see AGENTS.md: "Unix citizen").

**Constraint 7:** `ToolCall.Arguments` is `map[string]string`. The `command` parameter is a string used directly.

**Constraint 8:** No output transformation. Output is returned as-is from the command. No CRLF normalization, no encoding conversion, no trailing newline stripping.

**Constraint 9:** The truncation threshold is 102400 bytes (100 * 1024). This value is a constant within the implementation, not configurable.

**Constraint 10:** The tool does not inherit or set environment variables beyond what Go's `exec.Command` provides by default (the current process's environment).

---

## 7. Testing Requirements

### 7.1 Test Conventions

Tests must follow the patterns established in M3–M6:

- **Package-level tests:** Tests live in the same package (`package tool`)
- **Standard library only:** Use `testing` package. No test frameworks.
- **Temp directories:** Use `t.TempDir()` for filesystem isolation where workdir is needed.
- **Descriptive names:** `TestRunCommand_Scenario` with underscores.
- **Test real code, not mocks.** Tests must call actual functions with real shell commands. Each test must fail if the code under test is deleted.
- **Red/green TDD:** Write failing tests first, then implement code to make them pass.
- **Run tests with:** `make test`

### 7.2 `internal/tool/run_command_test.go` Tests

**Test: `TestRunCommand_Success`** — Create a tmpdir. Call `Execute` with `Arguments: {"command": "echo hello"}` and `ExecContext{Workdir: tmpdir}`. Verify `IsError` is false. Verify `Content` equals `"hello\n"`. Verify `CallID` matches the input `call.ID`.

**Test: `TestRunCommand_FailingCommand`** — Create a tmpdir. Call `Execute` with `Arguments: {"command": "exit 42"}` and `ExecContext{Workdir: tmpdir}`. Verify `IsError` is true. Verify `Content` contains `"exit code 42"`.

**Test: `TestRunCommand_OutputCapture`** — Create a tmpdir. Call `Execute` with `Arguments: {"command": "echo stdout; echo stderr >&2"}` and `ExecContext{Workdir: tmpdir}`. Verify `IsError` is false. Verify `Content` contains `"stdout"`. Verify `Content` contains `"stderr"`.

**Test: `TestRunCommand_Timeout`** — Create a tmpdir. Create a context with a short timeout (e.g., `context.WithTimeout(context.Background(), 50*time.Millisecond)`). Call `Execute` with `Arguments: {"command": "sleep 60"}` and `ExecContext{Workdir: tmpdir}`. Verify `IsError` is true. (The command should be killed by context cancellation before completing.)

**Test: `TestRunCommand_LargeOutputTruncation`** — Create a tmpdir. Call `Execute` with a command that generates more than 100KB of output (e.g., `Arguments: {"command": "dd if=/dev/zero bs=1024 count=200 2>/dev/null | tr '\\0' 'A'"}`). Verify `IsError` is false. Verify `Content` contains `"[output truncated, exceeded 100KB]"`. Verify the length of `Content` is greater than 102400 (it includes the truncation notice) but significantly less than 200KB (proving truncation occurred).

**Test: `TestRunCommand_MissingCommand`** — Create a tmpdir. Call `Execute` with empty `Arguments` map `{}` and `ExecContext{Workdir: tmpdir}`. Verify `IsError` is true. Verify `Content` contains `"command is required"`.

**Test: `TestRunCommand_EmptyCommand`** — Create a tmpdir. Call `Execute` with `Arguments: {"command": ""}` and `ExecContext{Workdir: tmpdir}`. Verify `IsError` is true. Verify `Content` contains `"command is required"`.

**Test: `TestRunCommand_WorkdirRespected`** — Create a tmpdir. Call `Execute` with `Arguments: {"command": "pwd"}` and `ExecContext{Workdir: tmpdir}`. Verify `IsError` is false. Verify `Content` contains the tmpdir path (after resolving symlinks, since `t.TempDir()` may involve symlinks on some platforms — use `filepath.EvalSymlinks` on tmpdir before comparison).

**Test: `TestRunCommand_CallIDPassthrough`** — Create a tmpdir. Call `Execute` with `call.ID` set to `"rc-unique-77"` and `Arguments: {"command": "true"}`. Verify the returned `ToolResult.CallID` equals `"rc-unique-77"`.

**Test: `TestRunCommand_FailingCommandWithOutput`** — Create a tmpdir. Call `Execute` with `Arguments: {"command": "echo before-fail; exit 1"}` and `ExecContext{Workdir: tmpdir}`. Verify `IsError` is true. Verify `Content` contains `"exit code 1"`. Verify `Content` contains `"before-fail"`.

### 7.3 `RegisterAll` Tests (additions to `registry_test.go`)

**Test: `TestRegisterAll_RegistersRunCommand`** — Call `NewRegistry()`, then `RegisterAll(r)`. Verify `r.Has(toolname.RunCommand)` returns true.

**Test: `TestRegisterAll_ResolvesRunCommand`** — Call `RegisterAll(r)`, then `r.Resolve([]string{toolname.RunCommand})`. Verify the returned tool has `Name` equal to `toolname.RunCommand` and has a `command` parameter.

### 7.4 Existing Tests

All existing tests must continue to pass without modification:

- `internal/tool/tool_test.go`
- `internal/tool/registry_test.go` (existing tests)
- `internal/tool/edit_file_test.go`
- `internal/tool/write_file_test.go`
- `internal/tool/read_file_test.go`
- `internal/tool/list_directory_test.go`
- `internal/tool/path_validation_test.go`
- All other test files in the project.

### 7.5 Running Tests

All tests must pass when run with:

```bash
make test
```

---

## 8. Acceptance Criteria

| Criterion | Verification |
|-----------|-------------|
| `internal/tool/run_command.go` exists and compiles | `go build ./internal/tool/` succeeds |
| `RegisterAll` registers `run_command` | `TestRegisterAll_RegistersRunCommand` passes |
| Tool definition has correct name | `toolname.RunCommand` constant used, not hardcoded string |
| Tool definition has 1 parameter | `command` (required) |
| Successful command returns output | `TestRunCommand_Success` passes |
| Failing command returns exit code + output | `TestRunCommand_FailingCommand` passes |
| Failing command with output includes both | `TestRunCommand_FailingCommandWithOutput` passes |
| Both stdout and stderr captured | `TestRunCommand_OutputCapture` passes |
| Context timeout kills command | `TestRunCommand_Timeout` passes |
| Large output truncated at 100KB | `TestRunCommand_LargeOutputTruncation` passes |
| Missing command argument rejected | `TestRunCommand_MissingCommand` passes |
| Empty command argument rejected | `TestRunCommand_EmptyCommand` passes |
| Workdir respected | `TestRunCommand_WorkdirRespected` passes |
| CallID propagated | `TestRunCommand_CallIDPassthrough` passes |
| Registry registers run_command | `TestRegisterAll_RegistersRunCommand` passes |
| Registry resolves run_command with correct params | `TestRegisterAll_ResolvesRunCommand` passes |
| All existing tests pass | `make test` exits 0 |

---

## 9. Out of Scope

The following items are explicitly **not** included in this milestone:

1. Integration tests combining multiple tools (e.g., `run_command` + `read_file`) — M8 scope
2. Timeout configuration per tool or per agent — the tool inherits the caller's context
3. Environment variable configuration (tool uses process environment as-is)
4. Shell selection (always `sh -c`; no user-configurable shell)
5. Working directory override via tool parameter (always uses `ExecContext.Workdir`)
6. Stdin piping to the command (the tool does not write to the command's stdin)
7. Separate stdout/stderr capture (combined only)
8. Output encoding detection or conversion
9. Command allowlists or blocklists
10. Resource limits (CPU, memory, file descriptors)
11. Changes to `--dry-run`, `--json`, or `--verbose` output — M8 scope
12. Changes to `internal/agent/`, `internal/provider/`, or `internal/toolname/` packages
13. Changes to `cmd/run.go` or `internal/tool/tool.go`
14. Windows `cmd.exe` support (POSIX `sh` only)
15. Rune-aware output truncation (byte-level only)

---

## 10. References

- Milestone Definition: `docs/plans/000_tool_call_milestones.md` (M7 section, lines 107–121)
- M6 Spec: `docs/plans/017_edit_file_spec.md`
- M5 Spec: `docs/plans/016_write_file_spec.md`
- M4 Spec: `docs/plans/015_read_file_spec.md`
- M3 Spec: `docs/plans/014_list_directory_spec.md`
- `RegisterAll` function: `internal/tool/registry.go:92`
- `toolname.RunCommand` constant: `internal/toolname/toolname.go:12`
- `provider.Tool` type: `internal/provider/provider.go:34`
- `provider.ToolCall` type: `internal/provider/provider.go:41`
- `provider.ToolResult` type: `internal/provider/provider.go:48`
- `ToolEntry` type: `internal/tool/registry.go:20`
- `ExecContext` type: `internal/tool/registry.go:13`

---

## 11. Notes

- **No path validation needed.** Unlike M3–M6 file tools, `run_command` does not operate on a file path parameter. The command string is opaque — it goes to `sh -c` as-is. The workdir (`cmd.Dir`) is the only boundary. The agent config's `tools` field controls which agents have shell access.
- **`CombinedOutput` vs `Output` + `StderrPipe`.** `CombinedOutput` is simpler and matches the spec ("captures combined stdout+stderr"). Separate capture would require goroutines to avoid deadlocks, adding complexity for no benefit.
- **Truncation before exit code prefix.** The output is truncated first, then the exit code prefix is prepended for failure cases. This ensures the LLM always sees the exit code even when output is truncated. The total `Content` length for a failing, truncated command is approximately 102400 + len("exit code N\n") + len("\n... [output truncated, exceeded 100KB]").
- **Whitespace-only commands are not rejected.** The empty check only rejects `""`. A command like `"   "` is passed to `sh -c` which executes it as a no-op. This is intentional — the tool should not second-guess what the shell considers valid.
- **Signal termination exit codes are platform-dependent.** On Linux, a process killed by SIGKILL has exit code -1 in Go's `ExitError.ExitCode()`. On macOS, behavior may differ. Tests should not assert specific exit codes for signal-killed processes; they should assert `IsError: true`.
- **The `RegisterAll` pattern is now complete.** M7 adds the final line. After this, `RegisterAll` registers all five tools: `list_directory`, `read_file`, `write_file`, `edit_file`, `run_command`. M8 integration tests will exercise them together.
- **Security model.** The `run_command` tool is intentionally unrestricted within the workdir. If an agent has `tools = ["run_command"]` in its TOML config, the LLM can execute any shell command. This is by design — agent configs are written by the user, not the LLM. The TOML `tools` field is the trust boundary.
