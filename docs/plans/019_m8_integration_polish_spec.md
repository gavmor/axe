# 019 — M8 Integration & Polish Spec

Status: **Draft**
Depends on: M1–M7 (all complete), Phases 1–4 test infrastructure (complete)

---

## Goal

Wire the tool system end-to-end, add per-tool verbose logging, display tools in `--dry-run` output, extend the golden test matrix, and write integration tests proving that agents with tools work correctly through the full conversation loop using the mock provider.

---

## Non-Goals

- No new tool implementations (M3–M7 are complete).
- No changes to `call_agent` dispatch or sub-agent mechanics.
- No changes to the `Registry` API surface.
- No new CLI flags.
- No new external dependencies.

---

## Scope

Seven work areas:

1. **`--dry-run` tools display** — Add a `--- Tools ---` section to dry-run output.
2. **Per-tool verbose logging** — Each of the 5 built-in tool executors emits a stderr line when `ExecContext.Verbose` is true.
3. **Golden file updates** — Add `with_tools` to the golden test matrix; update existing golden files if format changes.
4. **Integration tests: read-only tools** — Agent with `tools = ["read_file", "list_directory"]`.
5. **Integration tests: mutation tools** — Agent with `tools = ["write_file", "edit_file"]`.
6. **Integration tests: run_command** — Agent with `tools = ["run_command"]`.
7. **Integration tests: mixed tools + sub-agents** — Agent with both `tools` and `sub_agents`.
8. **Integration test: multi-turn mixed tool calls** — Multiple tool call rounds in a single conversation.

---

## 1. `--dry-run` Tools Display

### Current State

`printDryRun()` in `cmd/run.go:406-477` displays sections for Model, Workdir, Timeout, Params, System Prompt, Skill, Files, Stdin, Memory (conditional), and Sub-Agents. It does NOT display the `cfg.Tools` field.

### Required Change

Add a `--- Tools ---` section to the dry-run output, placed **between** the `--- Stdin ---` / `--- Memory ---` block and the `--- Sub-Agents ---` block.

### Output Format

When `cfg.Tools` is non-empty:
```
--- Tools ---
read_file, list_directory
```

When `cfg.Tools` is empty:
```
--- Tools ---
(none)
```

### Rules

- Tool names are printed as a comma-space-separated list in the order they appear in the TOML config (`cfg.Tools` slice order).
- The section is always printed (not conditional like Memory).
- No additional metadata (descriptions, parameter lists) — just the names.

### Signature Impact

No change to `printDryRun` function signature. The `cfg *agent.AgentConfig` parameter already carries `cfg.Tools`.

### Affected Files

| File | Change |
|------|--------|
| `cmd/run.go` | Add `--- Tools ---` section in `printDryRun()` |
| `cmd/testdata/golden/dry-run/basic.txt` | Add `--- Tools ---` / `(none)` section |
| `cmd/testdata/golden/dry-run/with_skill.txt` | Add `--- Tools ---` / `(none)` section |
| `cmd/testdata/golden/dry-run/with_files.txt` | Add `--- Tools ---` / `(none)` section |
| `cmd/testdata/golden/dry-run/with_memory.txt` | Add `--- Tools ---` / `(none)` section |
| `cmd/testdata/golden/dry-run/with_subagents.txt` | Add `--- Tools ---` / `(none)` section |
| `cmd/testdata/golden/dry-run/with_tools.txt` | New golden file showing `read_file, list_directory` |

### Placement Rule

The section order in dry-run output after this change:

1. `=== Dry Run ===`
2. Model / Workdir / Timeout / Params
3. `--- System Prompt ---`
4. `--- Skill ---`
5. `--- Files (N) ---`
6. `--- Stdin ---`
7. `--- Memory ---` (only if `cfg.Memory.Enabled`)
8. **`--- Tools ---`** (always)
9. `--- Sub-Agents ---`

---

## 2. Per-Tool Verbose Logging

### Current State

`ExecContext` has `Verbose bool` and `Stderr io.Writer` fields. All 5 tool executors receive them via `registry.Dispatch()` but none emit any output. Verbose logging currently only exists at the conversation-loop level (turn summaries) and in `ExecuteCallAgent` (sub-agent start/complete/fail).

### Required Change

Each of the 5 built-in tool executors emits a single log line to `ec.Stderr` when `ec.Verbose` is true, immediately before returning the `ToolResult`.

### Implementation Approach

Tool executors have 4–13 return points each. Rather than inserting a verbose log at every return site, use a **single-return refactoring** pattern: create a shared `toolVerboseLog(ec ExecContext, toolName string, result provider.ToolResult, summary string)` helper in a new file `internal/tool/verbose.go`. Each executor calls this helper once before its single return point, or uses a deferred/collected-result pattern so that all exit paths flow through one log call. This avoids missing return paths and keeps changes minimal.

### Log Format

Success:
```
[tool] <tool_name>: <summary> (success)
```

Error:
```
[tool] <tool_name>: <summary> (error)
```

### Per-Tool Summary Content

| Tool | Success Summary | Error Summary |
|------|----------------|---------------|
| `list_directory` | `path "<path>"` | `path "<path>": <error message>` |
| `read_file` | `path "<path>" (<N> lines)` | `path "<path>": <error message>` |
| `write_file` | `path "<path>" (<N> bytes)` | `path "<path>": <error message>` |
| `edit_file` | `path "<path>" (<N> occurrence(s))` | `path "<path>": <error message>` |
| `run_command` | `"<command>" (exit 0)` | `"<command>" (<error detail>)` |

Where:
- `<path>` is the raw `path` argument from `call.Arguments["path"]` (not the resolved absolute path). If the argument is missing, use `""`.
- `<N> lines` for `read_file` is the count of lines in the returned content.
- `<N> bytes` for `write_file` is the byte count that was written (same as the count in the success message).
- `<N> occurrence(s)` for `edit_file` is the replacement count.
- `<command>` for `run_command` is truncated to 60 characters if longer, with `...` appended.
- `<error detail>` for `run_command` includes exit code if available (e.g., `exit 42`), otherwise the error message.

### Rules

- Verbose logging is a **side effect** only — it does not change the `ToolResult`.
- Use `fmt.Fprintf(ec.Stderr, ...)` — no trailing newline (use `Fprintln` or include `\n`).
- If `ec.Stderr` is nil and `ec.Verbose` is true, skip the log (do not panic).
- The `[tool]` prefix distinguishes these from `[sub-agent]` and `[turn N]` prefixes used elsewhere.

### Affected Files

| File | Change |
|------|--------|
| `internal/tool/verbose.go` | New: shared `toolVerboseLog` helper |
| `internal/tool/list_directory.go` | Add verbose log in `listDirectoryExecute` |
| `internal/tool/read_file.go` | Add verbose log in `readFileExecute` |
| `internal/tool/write_file.go` | Add verbose log in `writeFileExecute` |
| `internal/tool/edit_file.go` | Add verbose log in `editFileExecute` |
| `internal/tool/run_command.go` | Add verbose log in `runCommandExecute` |

---

## 3. Golden File Updates

### Current Golden Test Matrix

The `TestGolden` test in `cmd/golden_test.go:191` iterates over:
```go
agents := []string{"basic", "with_skill", "with_files", "with_memory", "with_subagents"}
```

Each agent runs in two modes: `dry-run` and `json`. Total: 10 subtests.

### Required Changes

#### 3a. Add `with_tools` to the agents list

Add `"with_tools"` to the `agents` slice. The fixture `cmd/testdata/agents/with_tools.toml` already exists:
```toml
name = "with_tools"
model = "openai/gpt-4o"
tools = ["read_file", "list_directory"]
```

#### 3b. Dry-run golden file for `with_tools`

Create `cmd/testdata/golden/dry-run/with_tools.txt`. Expected content after the `--- Tools ---` section is added:
```
=== Dry Run ===

Model:    openai/gpt-4o
Workdir:  {{WORKDIR}}
Timeout:  120s
Params:   temperature=0, max_tokens=0

--- System Prompt ---


--- Skill ---
(none)

--- Files (0) ---
(none)

--- Stdin ---
(none)

--- Tools ---
read_file, list_directory

--- Sub-Agents ---
(none)
```

#### 3c. JSON golden file for `with_tools`

Create `cmd/testdata/golden/json/with_tools.json`. The mock response queue for `with_tools` in JSON mode must simulate a tool-call conversation:

1. LLM returns a `tool_use` response requesting `list_directory` with `path: "."`.
2. The tool executes against the real filesystem (the agent's workdir).
3. LLM receives the directory listing and returns a final text response.

Because the tool call executes against the real filesystem and produces non-deterministic output (the directory listing depends on the testdata contents), the JSON golden test for `with_tools` requires a controlled workdir. The mock response queue:

```go
// Turn 1: LLM requests list_directory
testutil.OpenAIToolCallResponse("Let me list the directory.", []testutil.MockToolCall{
    {ID: "tc_1", Name: "list_directory", Input: map[string]string{"path": "."}},
}),
// Turn 2: LLM receives directory listing, returns final response
testutil.OpenAIResponse("Directory listed successfully."),
```

The golden JSON output:
```json
{
  "content": "Directory listed successfully.",
  "duration_ms": "{{DURATION_MS}}",
  "input_tokens": 15,
  "model": "gpt-4o",
  "output_tokens": 10,
  "stop_reason": "stop",
  "tool_calls": 1
}
```

Note: `input_tokens` and `output_tokens` are cumulative across 2 turns. The `OpenAIToolCallResponse` helper returns `prompt_tokens=10, completion_tokens=20`. The `OpenAIResponse` helper returns `prompt_tokens=10, completion_tokens=5`. So the total is `input_tokens=20, output_tokens=25`. Adjust the golden file to match the actual helper values.

**Important:** The golden JSON test for `with_tools` requires setting up a workdir that the `list_directory` tool can actually read. The test setup must create a temporary directory as the workdir and write the agent config with `workdir = "<tempdir>"` or pass `--workdir <tempdir>` via CLI args. If the compiled binary test harness does not support this cleanly, an alternative approach: use `run_command` instead (with `tools = ["run_command"]`) since its output is controlled by the command string in the mock response, not the filesystem. The implementer should choose whichever approach produces deterministic output.

#### 3d. Update existing dry-run golden files

All 5 existing dry-run golden files must be updated to include the `--- Tools ---` / `(none)` section.

#### 3e. Existing JSON golden files

No changes to existing JSON golden files — the JSON envelope fields are unchanged.

### Updated Golden Test Matrix

After this change:
```go
agents := []string{"basic", "with_skill", "with_files", "with_memory", "with_subagents", "with_tools"}
```

Total: 12 subtests (6 agents × 2 modes).

The golden test's `switch` statement for JSON mock responses needs a new case:
```go
case "with_tools":
    tc.mockResponses = []testutil.MockLLMResponse{
        // Tool call + final response
    }
```

### Affected Files

| File | Change |
|------|--------|
| `cmd/golden_test.go` | Add `"with_tools"` to agents list; add `case "with_tools"` for JSON mock responses |
| `cmd/testdata/golden/dry-run/with_tools.txt` | New file |
| `cmd/testdata/golden/json/with_tools.json` | New file |
| `cmd/testdata/golden/dry-run/basic.txt` | Add `--- Tools ---` section |
| `cmd/testdata/golden/dry-run/with_skill.txt` | Add `--- Tools ---` section |
| `cmd/testdata/golden/dry-run/with_files.txt` | Add `--- Tools ---` section |
| `cmd/testdata/golden/dry-run/with_memory.txt` | Add `--- Tools ---` section |
| `cmd/testdata/golden/dry-run/with_subagents.txt` | Add `--- Tools ---` section |

---

## 4. Integration Test: Read-Only Tools

### Test Name

`TestIntegration_Tools_ReadOnly`

### File

`cmd/run_integration_test.go`

### Purpose

Verify that an agent configured with `tools = ["read_file", "list_directory"]` can use those tools in a multi-turn conversation via the mock provider.

### Setup

1. Call `resetRunCmd(t)`.
2. Create a mock server with a 3-response queue:
   - Response 1: `AnthropicToolUseResponse` requesting `read_file` with `path: "hello.txt"`.
   - Response 2 (after tool result): `AnthropicResponse` with final text incorporating the file content.
3. Call `testutil.SetupXDGDirs(t)`.
4. Write agent config inline: `model = "anthropic/claude-sonnet-4-20250514"`, `tools = ["read_file", "list_directory"]`.
5. Create a temp workdir with `t.TempDir()`. Write a file `hello.txt` containing `"Hello, World!"` to it.
6. Set `rootCmd.SetArgs([]string{"run", "<agent>", "--workdir", "<tempdir>"})`.
7. Set env vars: `ANTHROPIC_API_KEY`, `AXE_ANTHROPIC_BASE_URL`.

### Assertions

- `rootCmd.Execute()` returns `nil`.
- Stdout contains the expected final response text.
- `mock.RequestCount()` is 2 (initial request + follow-up after tool result).
- `mock.Requests[0].Body` contains `"read_file"` in the tools array.
- `mock.Requests[0].Body` contains `"list_directory"` in the tools array.
- `mock.Requests[1].Body` contains the tool result with `"Hello, World!"`.

### Edge Cases

None — this is a happy-path integration test. Tool-level edge cases are covered by unit tests in `internal/tool/`.

---

## 5. Integration Test: Mutation Tools

### Test Name

`TestIntegration_Tools_Mutation`

### File

`cmd/run_integration_test.go`

### Purpose

Verify that an agent configured with `tools = ["write_file", "edit_file"]` can create and modify files in its workdir via the mock provider.

### Setup

1. Call `resetRunCmd(t)`.
2. Create a mock server with a 4-response queue:
   - Response 1: `AnthropicToolUseResponse` requesting `write_file` with `path: "output.txt"`, `content: "first draft"`.
   - Response 2 (after write result): `AnthropicToolUseResponse` requesting `edit_file` with `path: "output.txt"`, `old_string: "first"`, `new_string: "final"`.
   - Response 3 (after edit result): `AnthropicResponse` with final text.
3. Call `testutil.SetupXDGDirs(t)`.
4. Write agent config inline: `tools = ["write_file", "edit_file"]`.
5. Create a temp workdir with `t.TempDir()`.
6. Set `rootCmd.SetArgs` with `--workdir`.

### Assertions

- `rootCmd.Execute()` returns `nil`.
- File `<tempdir>/output.txt` exists and contains `"final draft"`.
- `mock.RequestCount()` is 3.
- Stdout contains the expected final response text.
- The tool result in `mock.Requests[1].Body` contains the `write_file` success message (`"wrote 11 bytes to output.txt"`).
- The tool result in `mock.Requests[2].Body` contains the `edit_file` success message (`"replaced 1 occurrence(s) in output.txt"`).

---

## 6. Integration Test: run_command

### Test Name

`TestIntegration_Tools_RunCommand`

### File

`cmd/run_integration_test.go`

### Purpose

Verify that an agent configured with `tools = ["run_command"]` can execute shell commands in its workdir.

### Setup

1. Call `resetRunCmd(t)`.
2. Create a mock server with a 2-response queue:
   - Response 1: `AnthropicToolUseResponse` requesting `run_command` with `command: "echo hello"`.
   - Response 2 (after tool result): `AnthropicResponse` with final text.
3. Call `testutil.SetupXDGDirs(t)`.
4. Write agent config inline: `tools = ["run_command"]`.
5. Create a temp workdir with `t.TempDir()`.

### Assertions

- `rootCmd.Execute()` returns `nil`.
- `mock.RequestCount()` is 2.
- `mock.Requests[1].Body` contains the tool result with `"hello\n"`.
- Stdout contains the expected final response text.

---

## 7. Integration Tests: Mixed Tools + Sub-Agents

### 7a. Sequential Mixed Calls

#### Test Name

`TestIntegration_Tools_MixedWithSubAgents_Sequential`

#### Purpose

Verify that an agent with both `tools = ["read_file"]` and `sub_agents = ["helper"]` can use both in alternating turns.

#### Setup

1. Call `resetRunCmd(t)`.
2. Create a mock server with a 4-response queue:
   - Response 1: `AnthropicToolUseResponse` requesting `read_file` with `path: "data.txt"`.
   - Response 2 (after read result): `AnthropicToolUseResponse` requesting `call_agent` with `agent: "helper"`, `task: "summarize this"`.
   - Response 3 (sub-agent response): `AnthropicResponse("summary result")`.
   - Response 4 (parent final): `AnthropicResponse("Done.")`.
3. Call `testutil.SetupXDGDirs(t)`.
4. Write two agent configs:
   - Parent: `tools = ["read_file"]`, `sub_agents = ["helper"]`.
   - Helper: a minimal agent with the same model (Anthropic).
5. Create temp workdir with `data.txt` containing test content.

#### Assertions

- `rootCmd.Execute()` returns `nil`.
- `mock.RequestCount()` is 4 (parent turn 1, parent turn 2, sub-agent, parent turn 3).
- Stdout is `"Done."`.
- `mock.Requests[0].Body` contains both `"read_file"` and `"call_agent"` in the tools array.
- `mock.Requests[1].Body` contains the `read_file` tool result.
- `mock.Requests[3].Body` contains the `call_agent` tool result (`"summary result"`).

### 7b. Parallel Mixed Calls

#### Test Name

`TestIntegration_Tools_MixedWithSubAgents_Parallel`

#### Purpose

Verify that an agent with both `tools = ["read_file"]` and `sub_agents = ["helper"]` can dispatch both tool types in the same turn when the LLM returns multiple tool calls simultaneously.

#### Setup

1. Call `resetRunCmd(t)`.
2. Create a mock server with a 3-response queue:
   - Response 1: `AnthropicToolUseResponse` with TWO tool calls: `read_file` (path: `"info.txt"`) AND `call_agent` (agent: `"helper"`, task: `"do work"`).
   - Response 2 (sub-agent response): `AnthropicResponse("sub-agent done")`.
   - Response 3 (parent final, after both tool results): `AnthropicResponse("All done.")`.
3. Call `testutil.SetupXDGDirs(t)`.
4. Write parent agent config: `tools = ["read_file"]`, `sub_agents = ["helper"]`. Ensure parallel is enabled (default).
5. Write helper agent config.
6. Create temp workdir with `info.txt`.

#### Assertions

- `rootCmd.Execute()` returns `nil`.
- `mock.RequestCount()` is 3.
- Stdout is `"All done."`.
- `mock.Requests[2].Body` contains TWO tool results (one for `read_file`, one for `call_agent`).

#### Note on Parallel Dispatch Ordering

Because `read_file` and `call_agent` run concurrently when parallel is enabled, the mock server receives the sub-agent request at an indeterminate position in the queue. The mock server is FIFO, so responses 2 and 3 must be ordered correctly. Since `read_file` completes synchronously (no HTTP call) and `call_agent` makes an HTTP call (response 2), the sub-agent HTTP request will be response 2 in the queue. However, this ordering is an implementation detail. If this causes flakiness, the implementer should either:
- Use sequential dispatch for this test (set `parallel = false` in the sub_agents_config), OR
- Accept the current ordering since `read_file` does not make HTTP calls.

---

## 8. Integration Test: Multi-Turn Mixed Tool Calls

### Test Name

`TestIntegration_Tools_MultiTurnConversation`

### Purpose

Verify that a conversation with multiple rounds of tool calls (3+ turns with tool use) works correctly end-to-end.

### Setup

1. Call `resetRunCmd(t)`.
2. Create a mock server with a 4-response queue:
   - Response 1: `AnthropicToolUseResponse` requesting `list_directory` with `path: "."`.
   - Response 2 (after listing result): `AnthropicToolUseResponse` requesting `read_file` with `path: "readme.txt"`.
   - Response 3 (after read result): `AnthropicToolUseResponse` requesting `run_command` with `command: "echo done"`.
   - Response 4 (after command result): `AnthropicResponse("Completed all three steps.")`.
3. Call `testutil.SetupXDGDirs(t)`.
4. Write agent config: `tools = ["list_directory", "read_file", "run_command"]`.
5. Create temp workdir with `readme.txt` containing `"project readme"`.

### Assertions

- `rootCmd.Execute()` returns `nil`.
- `mock.RequestCount()` is 4.
- Stdout is `"Completed all three steps."`.
- Turn 1 tool result in request 2 body contains a directory listing that includes `readme.txt`.
- Turn 2 tool result in request 3 body contains `"project readme"`.
- Turn 3 tool result in request 4 body contains `"done\n"`.

---

## 9. Verify `--json` Output Includes Tool Call Counts

### Current State

The JSON envelope already includes `"tool_calls": N` (integer count, cumulative across all turns). This field was added during Phase 2. No code changes are needed.

### Test Coverage

The golden JSON test for `with_tools` (Section 3c) verifies the `tool_calls` field is present and correct. The existing `TestIntegration_JSONOutput_Structure` already verifies the JSON envelope structure for non-tool agents.

### Additional Integration Test

#### Test Name

`TestIntegration_JSONOutput_WithToolCalls`

#### Purpose

Verify that `--json` output correctly counts tool calls in a multi-turn tool conversation.

#### Setup

1. Call `resetRunCmd(t)`.
2. Create a mock server with a 3-response queue:
   - Response 1: `AnthropicToolUseResponse` with 2 tool calls (e.g., `read_file` + `list_directory`).
   - Response 2 (after results): `AnthropicToolUseResponse` with 1 tool call (`run_command`).
   - Response 3 (after result): `AnthropicResponse("final")`.
3. Agent config: `tools = ["read_file", "list_directory", "run_command"]`.
4. Pass `--json` flag.
5. Create temp workdir with test files.

#### Assertions

- `rootCmd.Execute()` returns `nil`.
- Parse stdout as JSON.
- `envelope["tool_calls"]` equals `3` (2 from turn 1 + 1 from turn 2).
- `envelope["content"]` equals `"final"`.
- `envelope["model"]` is present.
- `envelope["input_tokens"]` and `envelope["output_tokens"]` are cumulative (sum of all 3 responses).

---

## 10. Verify `--verbose` Logs Tool Execution Details

### Current State

The conversation loop emits `[turn N]` lines to stderr when verbose is true. Sub-agent calls emit `[sub-agent]` lines. Built-in tools emit nothing (Section 2 adds this).

### Integration Test

#### Test Name

`TestIntegration_Verbose_ToolExecution`

#### Purpose

Verify that `--verbose` emits per-tool log lines and turn summaries when tools are used.

#### Setup

1. Call `resetRunCmd(t)`.
2. Create a mock server with a 2-response queue:
   - Response 1: `AnthropicToolUseResponse` requesting `read_file` with `path: "test.txt"`.
   - Response 2: `AnthropicResponse("done")`.
3. Agent config: `tools = ["read_file"]`.
4. Pass `--verbose` flag.
5. Create temp workdir with `test.txt` containing `"line one\nline two"`.

#### Assertions

- `rootCmd.Execute()` returns `nil`.
- Stdout is `"done"`.
- Stderr contains `[turn 1]` (turn summary).
- Stderr contains `[tool] read_file:` (per-tool verbose log from Section 2).
- Stderr contains `(success)` (indicating the tool succeeded).
- Stderr contains `Duration:` (post-loop summary).

---

## 11. Verify `--dry-run` Shows Enabled Tools

### Test Coverage

The golden file test for `with_tools` in dry-run mode (Section 3b) is the primary verification. No additional integration test is needed — the golden file comparison is an exact match.

### Additional In-Process Test

#### Test Name

`TestIntegration_DryRun_ShowsTools`

#### Purpose

Verify `--dry-run` displays the `--- Tools ---` section with correct tool names for a tool-enabled agent.

#### Setup

1. Call `resetRunCmd(t)`.
2. Call `testutil.SetupXDGDirs(t)`.
3. Write agent config: `tools = ["write_file", "run_command"]`.
4. Set `rootCmd.SetArgs([]string{"run", "<agent>", "--dry-run"})`.
5. No API key needed (dry-run exits before provider creation).

#### Assertions

- `rootCmd.Execute()` returns `nil`.
- Stdout contains `--- Tools ---`.
- Stdout contains `write_file, run_command`.
- Stdout does NOT contain `read_file` or `list_directory` (only configured tools shown).

---

## 12. Full Test Pass

After all changes, `make test` must pass with zero failures. This includes:

- All existing unit tests in `internal/tool/` (M3–M7).
- All existing integration tests in `cmd/run_integration_test.go`.
- All existing golden file tests (updated golden files).
- All existing smoke tests.
- All new integration tests from Sections 4–10.

Run: `make test`

---

## Test Summary

| # | Test Name | File | Section |
|---|-----------|------|---------|
| 1 | `TestIntegration_Tools_ReadOnly` | `cmd/run_integration_test.go` | 4 |
| 2 | `TestIntegration_Tools_Mutation` | `cmd/run_integration_test.go` | 5 |
| 3 | `TestIntegration_Tools_RunCommand` | `cmd/run_integration_test.go` | 6 |
| 4 | `TestIntegration_Tools_MixedWithSubAgents_Sequential` | `cmd/run_integration_test.go` | 7a |
| 5 | `TestIntegration_Tools_MixedWithSubAgents_Parallel` | `cmd/run_integration_test.go` | 7b |
| 6 | `TestIntegration_Tools_MultiTurnConversation` | `cmd/run_integration_test.go` | 8 |
| 7 | `TestIntegration_JSONOutput_WithToolCalls` | `cmd/run_integration_test.go` | 9 |
| 8 | `TestIntegration_Verbose_ToolExecution` | `cmd/run_integration_test.go` | 10 |
| 9 | `TestIntegration_DryRun_ShowsTools` | `cmd/run_integration_test.go` | 11 |
| 10 | `TestGolden/dry-run/with_tools` | `cmd/golden_test.go` | 3b |
| 11 | `TestGolden/json/with_tools` | `cmd/golden_test.go` | 3c |

---

## File Change Summary

| File | Type | Description |
|------|------|-------------|
| `cmd/run.go` | Modify | Add `--- Tools ---` section to `printDryRun()` |
| `internal/tool/verbose.go` | Create | Shared `toolVerboseLog` helper |
| `internal/tool/list_directory.go` | Modify | Add verbose log call |
| `internal/tool/read_file.go` | Modify | Add verbose log call |
| `internal/tool/write_file.go` | Modify | Add verbose log call |
| `internal/tool/edit_file.go` | Modify | Add verbose log call |
| `internal/tool/run_command.go` | Modify | Add verbose log call |
| `cmd/run_integration_test.go` | Modify | Add 9 integration tests |
| `cmd/golden_test.go` | Modify | Add `with_tools` to agents list and JSON mock case |
| `cmd/testdata/golden/dry-run/with_tools.txt` | Create | New golden file |
| `cmd/testdata/golden/json/with_tools.json` | Create | New golden file |
| `cmd/testdata/golden/dry-run/basic.txt` | Modify | Add `--- Tools ---` section |
| `cmd/testdata/golden/dry-run/with_skill.txt` | Modify | Add `--- Tools ---` section |
| `cmd/testdata/golden/dry-run/with_files.txt` | Modify | Add `--- Tools ---` section |
| `cmd/testdata/golden/dry-run/with_memory.txt` | Modify | Add `--- Tools ---` section |
| `cmd/testdata/golden/dry-run/with_subagents.txt` | Modify | Add `--- Tools ---` section |

---

## Constraints

- No new external dependencies. All changes use Go stdlib and existing `testutil` helpers.
- No `t.Parallel()` in integration tests (global cobra state + `t.Setenv`).
- Agent configs in integration tests are written inline (not using fixture files).
- All test helpers call `t.Helper()`.
- Error assertions use `errors.As` with `*ExitError` where applicable.
- Tool tests call real code — no mocking of tool executors. The mock provider controls what tool calls the LLM requests; the actual tool execution happens against real filesystems / real shell.
- Verbose output goes to stderr only. Stdout remains clean and pipeable.

---

## Acceptance Criteria

1. `--dry-run` shows a `--- Tools ---` section with configured tool names (or `(none)`).
2. Each built-in tool emits a `[tool]` verbose log line to stderr when `Verbose` is true.
3. Golden files for all 6 agents pass in both dry-run and JSON modes.
4. Integration tests prove read-only tools, mutation tools, run_command, mixed tools+sub-agents (sequential and parallel), multi-turn conversations, JSON output with tool counts, verbose logging, and dry-run tools display all work correctly.
5. `make test` passes with zero failures.
