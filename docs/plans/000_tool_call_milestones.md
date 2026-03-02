# Tool Calling Milestones

LLM-driven command execution and file operations for axe v1.

Design decisions:
- Shell execution **and** file operation tools (full capability set)
- No interactive confirmation (non-interactive CLI; safety via agent config)
- `tools` field in agent TOML (explicit opt-in per agent)

Status key: `[ ]` not started · `[-]` in progress · `[x]` done

---

## M1 — Agent Config: `tools` field

Add a `Tools []string` field to `AgentConfig` so agents can opt into specific tools.

- [X] Add `Tools []string` with `toml:"tools"` tag to `AgentConfig`
- [X] Add validation: each entry must be in a known set of tool names (reject unknown)
- [X] Update `Scaffold` / init TOML template with commented `tools` example
- [X] Update `agents show` to display enabled tools
- [X] Tests for parsing, validation (known, unknown, empty), scaffold output

---

## M2 — Tool Registry

Central registry that maps tool names to definitions and executors. Replaces the hardcoded "Unknown tool" error at all three dispatch sites. `call_agent` stays special-cased outside the registry (it needs depth tracking, provider creation, and runtime state that generic tools don't).

- [x] Create `internal/tool/registry.go`
- [x] `ExecContext` struct: `Workdir string`, `Stderr io.Writer`, `Verbose bool` (minimal — no call_agent-specific fields)
- [x] `ToolEntry` struct: `Definition func() provider.Tool`, `Execute func(ctx, ToolCall, ExecContext) ToolResult`
- [x] `Registry` type with unexported `entries map[string]ToolEntry`
- [x] `NewRegistry()` returns empty registry (M3–M7 register their own tools)
- [x] `Register(name, entry)` — adds entry, silent replacement on duplicate
- [x] `Has(name) bool` — checks if tool is registered
- [x] `Resolve(names []string) ([]provider.Tool, error)` — validates names, returns definitions in input order
- [x] `Dispatch(ctx, ToolCall, ExecContext) (ToolResult, error)` — routes by name; errors for unknown tool or nil executor
- [x] Refactor `cmd/run.go`: create registry in `runAgent`, resolve `cfg.Tools` via `registry.Resolve()`, append to `req.Tools` before `call_agent`
- [x] Refactor `cmd/run.go:executeToolCalls()`: add `registry` and `workdir` params, dispatch non-`call_agent` calls via `registry.Dispatch`
- [x] Refactor `internal/tool/tool.go:runConversationLoop()`: add `registry` param, dispatch non-`call_agent` calls via `registry.Dispatch`
- [x] Refactor `internal/tool/tool.go:ExecuteCallAgent()`: pass `NewRegistry()` to `runConversationLoop`
- [x] Tests: `NewRegistry`, `Register`+`Has` (including silent replacement), `Resolve` (known, unknown, empty/nil, nil definition), `Dispatch` (known, unknown, nil executor, ExecContext passthrough)

---

## M3 — `list_directory` tool

Safest tool to start with. Read-only, no side effects. Also introduces `RegisterAll` (single registration point for all tools) and `validatePath` (shared path security for M4–M7).

- [x] Create `internal/tool/path_validation.go`: `validatePath(workdir, relPath)` with boundary-safe `isWithinDir` helper — rejects empty paths, absolute paths, `..` traversal, and symlink escapes via `filepath.EvalSymlinks`
- [x] Create `internal/tool/list_directory.go`: `listDirectoryEntry()` returning `ToolEntry` with `Definition` (name from `toolname.ListDirectory`, `path` parameter) and `Execute` (validates path, reads dir via `os.ReadDir`, formats entries with `/` suffix for subdirectories, one per line)
- [x] Create `RegisterAll(r *Registry)` in `registry.go` — single registration point; registers `list_directory` via `toolname.ListDirectory` constant. Future milestones add lines here, no call-site changes needed.
- [x] Wire `cmd/run.go`: call `tool.RegisterAll(registry)` after `tool.NewRegistry()` so top-level agents resolve `tools = ["list_directory"]`
- [x] Wire `internal/tool/tool.go` `ExecuteCallAgent`: create registry with `RegisterAll`, resolve `cfg.Tools` for sub-agents (error `ToolResult` on failure), pass populated registry to `runConversationLoop`. Injection order: `call_agent` first, then `cfg.Tools`.
- [x] Tests — `path_validation_test.go` (11 tests): valid relative, dot path, nested, empty, absolute, parent traversal escape, traversal within workdir, symlink within, symlink escape, nonexistent, sibling directory prefix overlap regression
- [x] Tests — `list_directory_test.go` (9 tests): existing dir with files+subdirs, nested path, empty dir, nonexistent, absolute path rejected, parent traversal rejected, symlink escape rejected, missing path argument, CallID passthrough
- [x] Tests — `registry_test.go` (3 new): `RegisterAll` registers list_directory, resolves with correct name and `path` parameter, idempotent (double call safe)

---

## M4 — `read_file` tool

Read-only file access for the LLM.

- [X] Create `internal/tool/read_file.go`
- [X] Parameters: `path` (required), `offset` (optional, 1-indexed line number), `limit` (optional, max lines to return)
- [X] Returns content prefixed with line numbers (e.g., `1: package main`)
- [X] Defaults: offset=1, limit=2000
- [X] Path validation: same rules as `list_directory` (relative to workdir, no escape)
- [X] Register in registry
- [X] Tests: read full file, read with offset/limit, nonexistent file error, binary file handling (reject or truncate), path traversal rejection

---

## M5 — `write_file` tool

File creation and overwrite.

- [x] Create `internal/tool/write_file.go`
- [x] Parameters: `path` (required), `content` (optional — missing/empty creates 0-byte file)
- [x] Creates parent directories if needed (`os.MkdirAll` with `0o755`)
- [x] Writes content to file (overwrite if exists, create if not) via `os.WriteFile` with `0o644`
- [x] Path validation: inline (not `validatePath`) — rejects empty, absolute, `..` traversal via `isWithinDir`, symlink escape check on parent via `filepath.EvalSymlinks`
- [x] Returns confirmation message: `"wrote %d bytes to %s"` (byte count, not rune count)
- [x] Register in registry
- [x] Tests: create new file, overwrite existing, create with nested dirs, path traversal rejection, absolute path rejection, empty content, missing content key, missing path, symlink escape rejection, CallID passthrough, UTF-8 byte count accuracy, registry registration + resolution

---

## M6 — `edit_file` tool

Find-and-replace within existing files.

- [x] Create `internal/tool/edit_file.go`
- [x] Parameters: `path` (required), `old_string` (required), `new_string` (required), `replace_all` (optional string parsed via `strconv.ParseBool`, default false)
- [x] Reads file, performs exact string replacement, writes back with `0o644`
- [x] Error if `old_string` not found: `"old_string not found in file"`
- [x] Error if `old_string` found multiple times and `replace_all` is false: `"old_string found %d times; set replace_all to true or provide a more unique string"`
- [x] Path validation: reuses `validatePath` (same as `read_file`) — rejects empty, absolute, `..` traversal, symlink escape, nonexistent files. Directory paths rejected via `os.Stat` + `IsDir()`.
- [x] Returns confirmation: `"replaced %d occurrence(s) in %s"` with replacement count and original relative path
- [x] Register in registry via `RegisterAll`
- [x] Tests: single replace, replace_all, not found error, multiple matches without replace_all error, path traversal rejection, absolute path rejection, missing path, missing old_string, empty new_string deletion, symlink escape rejection, CallID passthrough, nonexistent file, directory path rejection, invalid replace_all value, multiline match, registry registration + resolution

---

## M7 — `run_command` tool

Shell execution. Most powerful tool — implemented last intentionally.

- [x] Create `internal/tool/run_command.go`: `runCommandEntry()` returning `ToolEntry` with `Definition` (name from `toolname.RunCommand`, `command` parameter) and `Execute`
- [x] Parameters: `command` (required string) — single parameter, no type conversion needed (`map[string]string`)
- [x] Executes via `exec.CommandContext(ctx, "sh", "-c", command)` with `cmd.Dir` set to `ec.Workdir`
- [x] Captures combined stdout+stderr via `cmd.CombinedOutput()`
- [x] Inherits context timeout — `exec.CommandContext` sends SIGKILL on context cancellation
- [x] Non-zero exit code: `IsError: true`, content format `"exit code %d\n%s"` via `errors.As(*exec.ExitError)` with `exitErr.ExitCode()`
- [x] Zero exit code: `IsError: false`, content is raw command output as string
- [x] Non-exit errors (command not found, context cancelled): `IsError: true`, content is `err.Error()`
- [x] Truncate output exceeding 102400 bytes (100KB) with `"\n... [output truncated, exceeded 100KB]"` — byte-level truncation before exit code prefix
- [x] Empty/missing `command` argument: `IsError: true`, content `"command is required"`
- [x] Register in registry via `RegisterAll`
- [x] Tests — `run_command_test.go` (10 tests): success, failing command (exit 42), stdout+stderr capture, context timeout (50ms), large output truncation (>100KB), missing command, empty command, workdir respected (pwd + EvalSymlinks), CallID passthrough, failing command with output
- [x] Tests — `registry_test.go` (2 new): `RegisterAll` registers run_command, resolves with correct name and `command` parameter

---

## M8 — Integration & Polish

End-to-end wiring, integration tests, documentation.

- [x] Integration test: agent with `tools = ["read_file", "list_directory"]` can read files via LLM tool calls (mock provider)
- [x] Integration test: agent with `tools = ["run_command"]` executes commands via LLM tool calls (mock provider)
- [x] Integration test: agent with `tools = ["write_file", "edit_file"]` modifies files via LLM tool calls (mock provider)
- [x] Integration test: agent with tools AND sub_agents — both tool types work together (sequential + parallel)
- [x] Integration test: multi-turn conversation with mixed tool calls
- [x] Verify `--dry-run` shows enabled tools list — `--- Tools ---` section added to `printDryRun()`
- [x] Verify `--json` output includes tool call counts — `tool_calls` field verified with cumulative counts
- [x] Verify `--verbose` logs tool execution details — per-tool `[tool]` prefix lines via `toolVerboseLog` helper
- [x] Update golden file tests for new output format — `with_tools` added to matrix, all 5 existing dry-run golden files updated
- [x] All tests pass: `make test`

---

## Notes

- **Path security model:** All file tools resolve paths relative to the agent's `workdir`. Absolute paths and `..` traversal outside workdir are rejected. This is the trust boundary — the agent config determines what the LLM can access.
- **No new dependencies:** All tools use Go stdlib (`os`, `os/exec`, `path/filepath`, `strings`, `io`).
- **`ToolCall.Arguments` is `map[string]string`:** Optional int/bool parameters (like `offset`, `limit`, `replace_all`) are parsed from string values within each tool executor.
- **`call_agent` remains special-cased:** It needs depth tracking, provider creation, and the full sub-agent pipeline. Other tools are simpler and go through the generic registry dispatch.
- **Order matters:** M1-M2 are infrastructure. M3-M4 are read-only (safe to test patterns). M5-M6 are mutations. M7 is shell access. M8 ties it all together.
