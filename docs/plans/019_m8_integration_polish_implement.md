# 019 — M8 Integration & Polish Implementation Checklist

Spec: `docs/plans/019_m8_integration_polish_spec.md`
Status: **In Progress**

---

## Phase 1: `--dry-run` Tools Display (Spec §1)

Production change first, then update golden files to match.

- [x] Add `--- Tools ---` section to `printDryRun()` in `cmd/run.go` — insert between Memory/Stdin block and Sub-Agents block. Print `cfg.Tools` as comma-space-separated list or `(none)` if empty. Always printed (not conditional).
- [x] Run existing tests (`go test ./cmd/ -run TestGolden`) — expect failures on all 5 dry-run golden files due to new section. Confirm the failures are only the missing `--- Tools ---` section.

---

## Phase 2: Update Existing Golden Dry-Run Files (Spec §3d)

Update each file to include the new `--- Tools ---` / `(none)` section in the correct position.

- [x] Update `cmd/testdata/golden/dry-run/basic.txt` — add `--- Tools ---` / `(none)` between Stdin and Sub-Agents sections.
- [x] Update `cmd/testdata/golden/dry-run/with_skill.txt` — add `--- Tools ---` / `(none)`.
- [x] Update `cmd/testdata/golden/dry-run/with_files.txt` — add `--- Tools ---` / `(none)`.
- [x] Update `cmd/testdata/golden/dry-run/with_memory.txt` — add `--- Tools ---` / `(none)`.
- [x] Update `cmd/testdata/golden/dry-run/with_subagents.txt` — add `--- Tools ---` / `(none)`.
- [x] Run `go test ./cmd/ -run TestGolden/dry-run` — all 5 existing dry-run golden tests pass.

---

## Phase 3: Per-Tool Verbose Logging (Spec §2)

Create shared helper, then wire into each tool executor.

- [x] Create `internal/tool/verbose.go` — implement `toolVerboseLog(ec ExecContext, toolName string, result provider.ToolResult, summary string)`. Guard on `ec.Verbose && ec.Stderr != nil`. Format: `[tool] <toolName>: <summary> (success)\n` or `(error)\n` based on `result.IsError`.
- [x] Create `internal/tool/verbose_test.go` — tests for `toolVerboseLog`: success format, error format, nil stderr does not panic, verbose=false emits nothing.
- [x] Add verbose log to `listDirectoryExecute` in `internal/tool/list_directory.go` — summary: `path "<path>"` on success, `path "<path>": <error>` on error. Use single-return refactoring to ensure all paths log.
- [x] Add verbose log to `readFileExecute` in `internal/tool/read_file.go` — summary: `path "<path>" (<N> lines)` on success, `path "<path>": <error>` on error. `<N>` is line count from returned content.
- [x] Add verbose log to `writeFileExecute` in `internal/tool/write_file.go` — summary: `path "<path>" (<N> bytes)` on success, `path "<path>": <error>` on error. `<N>` is byte count written.
- [x] Add verbose log to `editFileExecute` in `internal/tool/edit_file.go` — summary: `path "<path>" (<N> occurrence(s))` on success, `path "<path>": <error>` on error. `<N>` is replacement count.
- [x] Add verbose log to `runCommandExecute` in `internal/tool/run_command.go` — summary: `"<command>" (exit 0)` on success, `"<command>" (<error detail>)` on error. Truncate command to 60 chars + `...` if longer.
- [x] Run all existing tool unit tests (`go test ./internal/tool/`) — zero regressions. Verbose logging is a side effect; existing tests pass `Verbose: false` or don't inspect stderr.

---

## Phase 4: Golden Test Matrix Extension (Spec §3a, §3b, §3c)

Add `with_tools` agent to the golden test matrix.

- [x] Create `cmd/testdata/golden/dry-run/with_tools.txt` — golden file showing `read_file, list_directory` in the Tools section. Match exact format from spec §3b.
- [x] Add `"with_tools"` to the `agents` slice in `TestGolden` in `cmd/golden_test.go`.
- [x] Add `case "with_tools"` to the JSON mock response switch in `TestGolden` — script a tool call conversation (e.g., `list_directory` then final response). Must produce deterministic output; use `--workdir` pointing to a controlled temp directory, or use `run_command` with `echo` for deterministic output.
- [x] Create `cmd/testdata/golden/json/with_tools.json` — golden file with correct cumulative token counts and `tool_calls: 1`. Adjust token values to match actual mock helper output (`input_tokens=20, output_tokens=25` for 2-turn OpenAI conversation).
- [x] Run `go test ./cmd/ -run TestGolden` — all 12 subtests pass (6 agents × 2 modes).

---

## Phase 5: Integration Test — Read-Only Tools (Spec §4)

- [x] Write `TestIntegration_Tools_ReadOnly` in `cmd/run_integration_test.go` — agent with `tools = ["read_file", "list_directory"]`, Anthropic provider, mock server with 2-response queue (tool_use + final). Create temp workdir with `hello.txt`. Assert: no error, stdout matches final response, request count is 2, request bodies contain tool definitions and tool results.
- [x] Run `go test ./cmd/ -run TestIntegration_Tools_ReadOnly` — passes.

---

## Phase 6: Integration Test — Mutation Tools (Spec §5)

- [x] Write `TestIntegration_Tools_Mutation` in `cmd/run_integration_test.go` — agent with `tools = ["write_file", "edit_file"]`, mock server with 3-response queue (write_file tool_use, edit_file tool_use, final). Create temp workdir. Assert: no error, file `output.txt` contains `"final draft"`, request count is 3, tool result messages in request bodies match expected success strings.
- [x] Run `go test ./cmd/ -run TestIntegration_Tools_Mutation` — passes.

---

## Phase 7: Integration Test — run_command (Spec §6)

- [x] Write `TestIntegration_Tools_RunCommand` in `cmd/run_integration_test.go` — agent with `tools = ["run_command"]`, mock server with 2-response queue (run_command tool_use for `echo hello`, final). Assert: no error, request count is 2, tool result in request body contains `"hello\n"`.
- [x] Run `go test ./cmd/ -run TestIntegration_Tools_RunCommand` — passes.

---

## Phase 8: Integration Tests — Mixed Tools + Sub-Agents (Spec §7)

- [x] Write `TestIntegration_Tools_MixedWithSubAgents_Sequential` in `cmd/run_integration_test.go` — parent agent with `tools = ["read_file"]` and `sub_agents = ["helper"]`. Mock server with 4-response queue: read_file tool_use, call_agent tool_use, sub-agent response, parent final. Two agent configs (parent + helper). Temp workdir with `data.txt`. Assert: no error, request count is 4, stdout is final response, request bodies contain correct tool results at correct positions.
- [x] Write `TestIntegration_Tools_MixedWithSubAgents_Parallel` in `cmd/run_integration_test.go` — parent agent with `tools = ["read_file"]` and `sub_agents = ["helper"]`, parallel enabled (default). Mock server with 3-response queue: tool_use with TWO calls (read_file + call_agent), sub-agent response, parent final. Assert: no error, request count is 3, final request body contains both tool results. If flaky due to parallel ordering, fall back to `parallel = false` per spec §7b note.
- [x] Run `go test ./cmd/ -run TestIntegration_Tools_MixedWithSubAgents` — both pass.

---

## Phase 9: Integration Test — Multi-Turn Conversation (Spec §8)

- [x] Write `TestIntegration_Tools_MultiTurnConversation` in `cmd/run_integration_test.go` — agent with `tools = ["list_directory", "read_file", "run_command"]`. Mock server with 4-response queue: list_directory tool_use, read_file tool_use, run_command tool_use, final. Temp workdir with `readme.txt`. Assert: no error, request count is 4, stdout is final response, each subsequent request body contains the correct tool result from the prior turn.
- [x] Run `go test ./cmd/ -run TestIntegration_Tools_MultiTurnConversation` — passes.

---

## Phase 10: Integration Test — JSON Output with Tool Calls (Spec §9)

- [x] Write `TestIntegration_JSONOutput_WithToolCalls` in `cmd/run_integration_test.go` — agent with `tools = ["read_file", "list_directory", "run_command"]`, `--json` flag. Mock server with 3-response queue: 2 tool calls in turn 1, 1 tool call in turn 2, final. Temp workdir with test files. Parse stdout as JSON. Assert: `tool_calls` == 3, `content` == final text, `model` present, `input_tokens` and `output_tokens` are cumulative sums.
- [x] Run `go test ./cmd/ -run TestIntegration_JSONOutput_WithToolCalls` — passes.

---

## Phase 11: Integration Test — Verbose Tool Execution (Spec §10)

- [x] Write `TestIntegration_Verbose_ToolExecution` in `cmd/run_integration_test.go` — agent with `tools = ["read_file"]`, `--verbose` flag. Mock server with 2-response queue: read_file tool_use, final. Temp workdir with `test.txt`. Assert: no error, stdout is final response, stderr contains `[turn 1]`, stderr contains `[tool] read_file:`, stderr contains `(success)`, stderr contains `Duration:`.
- [x] Run `go test ./cmd/ -run TestIntegration_Verbose_ToolExecution` — passes.

---

## Phase 12: Integration Test — Dry-Run Shows Tools (Spec §11)

- [x] Write `TestIntegration_DryRun_ShowsTools` in `cmd/run_integration_test.go` — agent with `tools = ["write_file", "run_command"]`, `--dry-run` flag. No API key needed. Assert: no error, stdout contains `--- Tools ---`, stdout contains `write_file, run_command`, stdout does NOT contain `read_file` or `list_directory`.
- [x] Run `go test ./cmd/ -run TestIntegration_DryRun_ShowsTools` — passes.

---

## Phase 13: Full Test Suite (Spec §12)

- [x] Run `make test` — all tests pass with zero failures across all packages.
