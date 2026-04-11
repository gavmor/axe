# Specification: Tool Call M2 — Plugin Registry

**Status:** Draft
**Version:** 1.0
**Created:** 2026-03-01
**Scope:** Central registry that maps tool names to implementations and executors, following the microkernel registry pattern.

---

## 1. Purpose

Create a `Registry` type in `internal/tool/` that maps tool names to their definitions (for the core system) and executors (for runtime dispatch). This implements the microkernel registry pattern, replacing hardcoded dispatch sites with a dynamic, metadata-driven system.

M1 (012) added the `Tools []string` field to `AgentConfig`. M2 provides the infrastructure to resolve those names into Shared Protocol definitions and dispatch calls to the correct tool executor.

This milestone does NOT implement any specific tool components (M3–M7). It provides the registry mechanism and refactors existing dispatch sites to use it.

---

## 2. Design Decisions

The following decisions were made during planning and are binding for implementation:

1. **`call_agent` stays outside the registry.** The `call_agent` logic requires runtime state for recursive core invocation that is fundamentally different from generic tool components. The registry does NOT register `call_agent`.

2. **`NewRegistry()` returns an empty registry.** No tool components are pre-loaded. M3–M7 milestones will each register their component into the registry when implemented.

3. **`Dispatch` returns `(ToolResult, error)`.** Infrastructure errors (unknown tool, nil executor) are returned as `error`. Component execution results are returned as `ToolResult`.

4. **`ExecContext` is minimal.** It contains only the fields needed by generic tool executors: `Workdir`, `Stderr`, `Verbose`. It does NOT contain recursive core invocation state.

5. **Registry is passed explicitly.** No global registry. `NewRegistry()` is called at each entry point following the project's "no global state" principle.

6. **No new external dependencies.** This milestone uses only Go standard library and existing dependencies.

7. **Workdir for ExecContext comes from `resolve.Workdir()`.** These resolved values are used when constructing `ExecContext`.

---

## 3. Requirements

### 3.1 `ExecContext` Type

**Requirement 1.1:** Define an exported `ExecContext` struct in `internal/tool/registry.go`:

| Go Field | Go Type | Description |
|----------|---------|-------------|
| `Workdir` | `string` | Resolved working directory for the tool to operate in |
| `Stderr` | `io.Writer` | Writer for debug/verbose output |
| `Verbose` | `bool` | Whether verbose logging is enabled |

**Requirement 1.2:** `ExecContext` must NOT contain depth tracking, `GlobalConfig`, `AllowedAgents`, or any other `call_agent`-specific fields.

### 3.2 `ToolEntry` Type

**Requirement 2.1:** Define an exported `ToolEntry` struct in `internal/tool/registry.go`:

| Go Field | Go Type | Description |
|----------|---------|-------------|
| `Definition` | `func() provider.Tool` | Returns the tool definition sent to the LLM |
| `Execute` | `func(ctx context.Context, call provider.ToolCall, ec ExecContext) provider.ToolResult` | Executes the tool and returns a result |

**Requirement 2.2:** Both fields are function types. `Definition` is called at resolve time. `Execute` is called at dispatch time.

### 3.3 `Registry` Type

**Requirement 3.1:** Define an exported `Registry` struct in `internal/tool/registry.go` containing an unexported `entries` field of type `map[string]ToolEntry`.

**Requirement 3.2:** The `entries` map must NOT be directly accessible from outside the package.

### 3.4 `NewRegistry()` Constructor

**Requirement 4.1:** Define an exported function `NewRegistry() *Registry` that returns a new `Registry` with an empty, initialized `entries` map.

**Requirement 4.2:** The returned registry must have zero entries. No tools are pre-loaded.

### 3.5 `Register` Method

**Requirement 5.1:** Define an exported method `Register(name string, entry ToolEntry)` on `*Registry`.

**Requirement 5.2:** `Register` adds the entry to the registry's internal map, keyed by `name`.

**Requirement 5.3:** If a tool with the same name is already registered, the new entry silently replaces the old one.

### 3.6 `Has` Method

**Requirement 6.1:** Define an exported method `Has(name string) bool` on `*Registry`.

**Requirement 6.2:** `Has` returns `true` if a tool with the given name exists in the registry, `false` otherwise.

### 3.7 `Resolve` Method

**Requirement 7.1:** Define an exported method `Resolve(names []string) ([]provider.Tool, error)` on `*Registry`.

**Requirement 7.2:** For each name in `names`, `Resolve` must look up the entry in the registry and call `entry.Definition()` to get the `provider.Tool`.

**Requirement 7.3:** If any name is not found in the registry, `Resolve` must return an error with the message:

```
unknown tool "<name>"
```

Where `<name>` is the first unrecognized tool name.

**Requirement 7.4:** If `names` is nil or empty, `Resolve` must return an empty (non-nil) slice and nil error.

**Requirement 7.5:** The returned `[]provider.Tool` must be in the same order as the input `names`.

### 3.8 `Dispatch` Method

**Requirement 8.1:** Define an exported method `Dispatch(ctx context.Context, call provider.ToolCall, ec ExecContext) (provider.ToolResult, error)` on `*Registry`.

**Requirement 8.2:** `Dispatch` looks up `call.Name` in the registry. If not found, it returns a zero `ToolResult` and an error with the message:

```
unknown tool "<name>"
```

**Requirement 8.3:** If the entry's `Execute` field is nil, `Dispatch` returns a zero `ToolResult` and an error with the message:

```
tool "<name>" has no executor
```

**Requirement 8.4:** If the entry is found and `Execute` is non-nil, `Dispatch` calls `entry.Execute(ctx, call, ec)` and returns the resulting `ToolResult` with nil error.

**Requirement 8.5:** `Dispatch` must pass the `ctx`, `call`, and `ec` arguments through to the executor unchanged.

### 3.9 Refactor: `cmd/run.go` Tool Injection (line 213–222)

**Requirement 9.1:** After loading the agent config and resolving the workdir, create a registry via `tool.NewRegistry()`.

**Requirement 9.2:** If `cfg.Tools` is non-empty, call `registry.Resolve(cfg.Tools)` to get tool definitions. Append the resulting `[]provider.Tool` to `req.Tools`.

**Requirement 9.3:** If `Resolve` returns an error, `runAgent` must return an `ExitError` with code 1 and the resolve error wrapped with the message:

```
failed to resolve tools: <original error>
```

**Requirement 9.4:** The existing `call_agent` injection logic must remain unchanged: if `len(cfg.SubAgents) > 0 && depth < effectiveMaxDepth`, append `tool.CallAgentTool(cfg.SubAgents)` to `req.Tools`.

**Requirement 9.5:** Tool injection order: configured tools from `cfg.Tools` are appended first, then `call_agent` (if applicable). The result is a single `req.Tools` slice containing both.

**Requirement 9.6:** The registry must be passed to `executeToolCalls()` so it is available for dispatch.

### 3.10 Refactor: `cmd/run.go` `executeToolCalls()` (line 467–525)

**Requirement 10.1:** Add a `registry *tool.Registry` parameter to the `executeToolCalls` function signature.

**Requirement 10.2:** The function signature must become:

```go
func executeToolCalls(ctx context.Context, toolCalls []provider.ToolCall, cfg *agent.AgentConfig, globalCfg *config.GlobalConfig, registry *tool.Registry, depth, maxDepth int, parallel, verbose bool, stderr io.Writer) []provider.ToolResult
```

**Requirement 10.3:** In both the sequential and parallel dispatch branches, the dispatch logic must change from:

```
if call_agent → ExecuteCallAgent
else → "Unknown tool" error
```

To:

```
if call_agent → ExecuteCallAgent (unchanged)
else → registry.Dispatch(ctx, call, execContext)
  if dispatch error → ToolResult with IsError=true, Content=error message
  else → use returned ToolResult
```

**Requirement 10.4:** The `ExecContext` passed to `registry.Dispatch` must use:
- `Workdir`: the resolved workdir (passed into the function or derived from `cfg.Workdir` via `resolve.Workdir`)
- `Stderr`: the `stderr` parameter
- `Verbose`: the `verbose` parameter

**Requirement 10.5:** Add a `workdir string` parameter to `executeToolCalls` to pass the resolved workdir from `runAgent`.

**Requirement 10.6:** The call site in `runAgent` (line 332) must be updated to pass both the `registry` and the resolved `workdir`.

**Requirement 10.7:** When `registry.Dispatch` returns an error, the error `ToolResult` must have:
- `CallID`: set to `tc.ID` (or `call.ID` in the parallel goroutine)
- `Content`: the error's `.Error()` string
- `IsError`: `true`

### 3.11 Refactor: `internal/tool/tool.go` `runConversationLoop()` (line 286)

**Requirement 11.1:** Add a `registry *Registry` parameter to `runConversationLoop`. The new signature must be:

```go
func runConversationLoop(ctx context.Context, prov provider.Provider, req *provider.Request, cfg *agent.AgentConfig, registry *Registry, depth int, opts ExecuteOptions) (*provider.Response, error)
```

**Requirement 11.2:** The `registry` parameter must be placed after `cfg` and before `depth` in the parameter list.

**Requirement 11.3:** In the tool dispatch loop (lines 313–332), the dispatch logic must change from:

```
if call_agent → ExecuteCallAgent
else → "Unknown tool" error
```

To:

```
if call_agent → ExecuteCallAgent (unchanged, with existing subOpts construction)
else → registry.Dispatch(ctx, tc, execContext)
  if dispatch error → ToolResult with CallID=tc.ID, Content=error message, IsError=true
  else → use returned ToolResult
```

**Requirement 11.4:** The `ExecContext` passed to `registry.Dispatch` in `runConversationLoop` must use:
- `Workdir`: resolved from `cfg.Workdir` using `resolve.Workdir("", cfg.Workdir)` (sub-agents don't have flag overrides)
- `Stderr`: `opts.Stderr`
- `Verbose`: `opts.Verbose`

**Requirement 11.5:** The existing `call_agent` special-case branch (constructing `subOpts` with depth tracking, `AllowedAgents`, etc.) must remain unchanged.

### 3.12 Refactor: `internal/tool/tool.go` `ExecuteCallAgent()` (line 245)

**Requirement 12.1:** `ExecuteCallAgent` must create a `Registry` via `NewRegistry()` for the sub-agent's tool dispatch.

**Requirement 12.2:** The call to `runConversationLoop` (line 245) must pass the newly created registry.

**Requirement 12.3:** The existing tool injection logic for sub-agents (lines 228–232) must remain unchanged. The registry created here has no tools registered — it exists to satisfy the function signature and to allow future M3–M7 tools to be registered for sub-agent use.

**Requirement 12.4:** When M3–M7 milestones are implemented, they will register their tools into this registry before the `runConversationLoop` call. This is future work and NOT part of M2.

---

## 4. Project Structure

After completion, the following files will be added or modified:

```
axe/
├── cmd/
│   └── run.go                          # MODIFIED: registry creation, tool injection, executeToolCalls signature
├── internal/
│   └── tool/
│       ├── registry.go                 # NEW: ExecContext, ToolEntry, Registry, NewRegistry, Register, Has, Resolve, Dispatch
│       ├── registry_test.go            # NEW: tests for all registry functionality
│       ├── tool.go                     # MODIFIED: runConversationLoop and ExecuteCallAgent signatures
│       └── tool_test.go                # UNCHANGED (existing tests must still pass)
├── go.mod                              # UNCHANGED
├── go.sum                              # UNCHANGED
└── ...                                 # all other files UNCHANGED
```

---

## 5. Edge Cases

### 5.1 Registry Operations

| Scenario | Behavior |
|----------|----------|
| `NewRegistry()` | Returns non-nil `*Registry` with empty entries map |
| `Register` same name twice | Second entry replaces first silently |
| `Has` on empty registry | Returns `false` for any name |
| `Has` after `Register` | Returns `true` for registered name, `false` for others |
| `Resolve(nil)` | Returns empty `[]provider.Tool`, nil error |
| `Resolve([]string{})` | Returns empty `[]provider.Tool`, nil error |
| `Resolve` with all known tools | Returns definitions in input order |
| `Resolve` with one unknown tool | Returns nil, error for the unknown tool |
| `Resolve` with mix of known and unknown | Returns nil, error for the first unknown tool encountered |
| `Dispatch` on empty registry | Returns zero `ToolResult`, error |
| `Dispatch` for registered tool with nil `Execute` | Returns zero `ToolResult`, error |
| `Dispatch` for registered tool with valid `Execute` | Calls `Execute`, returns its `ToolResult`, nil error |

### 5.2 Tool Injection (`cmd/run.go`)

| Scenario | Behavior |
|----------|----------|
| Agent has `tools = []` and no `sub_agents` | `req.Tools` is nil/empty. Single-shot path (no conversation loop). |
| Agent has `tools = ["read_file"]` and no `sub_agents` | `req.Tools` has 1 entry from registry resolve. Conversation loop enabled. |
| Agent has no `tools` and `sub_agents = ["helper"]` | `req.Tools` has 1 entry (`call_agent`). Same as current behavior. |
| Agent has `tools = ["read_file"]` and `sub_agents = ["helper"]` | `req.Tools` has 2 entries: `read_file` definition first, then `call_agent`. |
| Agent has `tools = ["nonexistent"]` | `runAgent` returns `ExitError{Code: 1}` with resolve error. Never reaches LLM call. |
| Agent has `tools` but all M3–M7 tools are unimplemented | `Resolve` returns error (empty registry has no entries). `runAgent` returns error. |

**Important note on M2 alone:** Since `NewRegistry()` returns an empty registry and no M3–M7 tools register themselves yet, any agent with `tools = ["read_file"]` will fail at resolve time with `unknown tool "read_file"`. This is correct and expected — the registry is empty until M3–M7 implementations register their tools. Users should not configure `tools` until the corresponding milestone is complete.

### 5.3 Dispatch at Runtime

| Scenario | Behavior |
|----------|----------|
| LLM calls `call_agent` | Dispatched via existing `ExecuteCallAgent` special case. Registry not involved. |
| LLM calls a registered tool | `registry.Dispatch` routes to `entry.Execute`. Result returned to LLM. |
| LLM calls an unknown tool name | `registry.Dispatch` returns error. Caller wraps as error `ToolResult` for LLM. |
| LLM calls a tool registered with nil executor | `registry.Dispatch` returns error. Caller wraps as error `ToolResult` for LLM. |

### 5.4 Sub-Agent Tool Resolution

| Scenario | Behavior |
|----------|----------|
| Sub-agent has `tools = ["read_file"]` in its config | Not resolved in M2. Sub-agent tool injection (line 228–232) only injects `call_agent`. Sub-agent `cfg.Tools` is ignored until M3–M7 register tools. |
| Sub-agent has `sub_agents` and depth allows | `call_agent` injected into sub-agent's `req.Tools`. Same as current behavior. |

---

## 6. Constraints

**Constraint 1:** No new external dependencies. `go.mod` must remain unchanged.

**Constraint 2:** No tool executor implementations. The registry provides the framework; M3–M7 provide the executors.

**Constraint 3:** `call_agent` must NOT be registered in the registry. It remains special-cased at all three dispatch sites.

**Constraint 4:** No changes to the `Provider` interface or provider implementations.

**Constraint 5:** No changes to `--dry-run`, `--json`, or `--verbose` output. Those are M8 scope.

**Constraint 6:** No changes to `internal/agent/`, `internal/provider/`, or `internal/toolname/` packages.

**Constraint 7:** The `Registry` type must have no global instances. It is constructed and passed explicitly.

**Constraint 8:** Cross-platform compatibility: must build and run on Linux, macOS, and Windows.

**Constraint 9:** Sub-agent tool injection (`ExecuteCallAgent` lines 228–232) is NOT refactored to resolve `cfg.Tools` in this milestone. That wiring is deferred until M3–M7 tools exist and can be registered. M2 only passes an empty registry to `runConversationLoop`.

---

## 7. Testing Requirements

### 7.1 Test Conventions

Tests must follow the patterns established in prior milestones:

- **Package-level tests:** Tests live in the same package (`package tool`)
- **Standard library only:** Use `testing` package. No test frameworks.
- **Temp directories:** Use `t.TempDir()` for filesystem isolation.
- **Env overrides:** Use `t.Setenv` for XDG path control.
- **Descriptive names:** `TestFunctionName_Scenario` with underscores.
- **Test real code, not mocks.** Tests must call actual `Registry` methods. Each test must fail if the code under test is deleted.
- **Red/green TDD:** Write failing tests first, then implement code to make them pass.
- **Run tests with:** `make test`

### 7.2 `internal/tool/registry_test.go` Tests

**Test: `TestNewRegistry`** — Call `NewRegistry()`, verify the returned value is non-nil.

**Test: `TestRegistry_Register_And_Has`** — Register a tool entry. Verify `Has` returns `true` for the registered name and `false` for an unregistered name. Verify `Has` returns `false` before registration.

**Test: `TestRegistry_Resolve_KnownTools`** — Register two tools with known `Definition` functions. Call `Resolve` with both names. Verify the returned slice has 2 entries with correct `Name` fields in input order.

**Test: `TestRegistry_Resolve_UnknownTool`** — Register one tool. Call `Resolve` with that tool plus an unknown name. Verify an error is returned.

**Test: `TestRegistry_Resolve_Empty`** — Call `Resolve` with an empty slice. Verify it returns an empty slice and nil error. Repeat with nil. Same result.

**Test: `TestRegistry_Dispatch_KnownTool`** — Register a tool with a working `Execute` function that reads from `call.Arguments` and returns a `ToolResult`. Call `Dispatch` with a matching `ToolCall`. Verify the returned `ToolResult` has correct `CallID` and `Content`, and error is nil.

**Test: `TestRegistry_Dispatch_UnknownTool`** — Call `Dispatch` on an empty registry. Verify an error is returned (non-nil).

**Test: `TestRegistry_Dispatch_NilExecutor`** — Register a tool with `Execute: nil`. Call `Dispatch`. Verify an error is returned.

**Test: `TestRegistry_Dispatch_PassesExecContext`** — Register a tool whose `Execute` captures the `ExecContext` it receives. Call `Dispatch` with specific `ExecContext` values. Verify the executor received the correct `Workdir` and `Verbose` values.

### 7.3 Existing Tests

All existing tests must continue to pass without modification:

- `internal/tool/tool_test.go` — All 17+ tests must pass. The only change to `tool.go` is function signatures (`runConversationLoop`), and all tests call `ExecuteCallAgent` which internally calls `runConversationLoop` with the new signature.
- `cmd/run_test.go` (if it exists) — Must pass with the updated `executeToolCalls` signature.
- All other test files in the project.

### 7.4 Running Tests

All tests must pass when run with:

```bash
make test
```

---

## 8. Acceptance Criteria

| Criterion | Verification |
|-----------|-------------|
| `internal/tool/registry.go` exists | `go build ./internal/tool/` succeeds |
| `NewRegistry()` returns non-nil empty registry | `TestNewRegistry` passes |
| `Register` + `Has` work correctly | `TestRegistry_Register_And_Has` passes |
| `Resolve` returns definitions for known tools | `TestRegistry_Resolve_KnownTools` passes |
| `Resolve` errors on unknown tools | `TestRegistry_Resolve_UnknownTool` passes |
| `Resolve` handles empty/nil input | `TestRegistry_Resolve_Empty` passes |
| `Dispatch` routes to correct executor | `TestRegistry_Dispatch_KnownTool` passes |
| `Dispatch` errors on unknown tool | `TestRegistry_Dispatch_UnknownTool` passes |
| `Dispatch` errors on nil executor | `TestRegistry_Dispatch_NilExecutor` passes |
| `Dispatch` passes `ExecContext` through | `TestRegistry_Dispatch_PassesExecContext` passes |
| `cmd/run.go` resolves `cfg.Tools` via registry | Agent with `tools` config has definitions in `req.Tools` |
| `cmd/run.go` errors on invalid tool names | Agent with unknown tool returns `ExitError{Code: 1}` |
| `cmd/run.go` `executeToolCalls` uses registry dispatch | Non-`call_agent` calls go through `registry.Dispatch` |
| `internal/tool/tool.go` `runConversationLoop` uses registry | Non-`call_agent` calls go through `registry.Dispatch` |
| `ExecuteCallAgent` passes registry to conversation loop | `runConversationLoop` receives a `*Registry` |
| `call_agent` dispatch unchanged | All existing `tool_test.go` tests pass |
| All existing tests pass | `make test` exits 0 |

---

## 9. Out of Scope

The following items are explicitly **not** included in this milestone:

1. Tool executor implementations (`list_directory`, `read_file`, `write_file`, `edit_file`, `run_command`) — M3–M7 scope
2. Registering M3–M7 tools in `NewRegistry()` — each milestone registers its own tool
3. Resolving `cfg.Tools` for sub-agents in `ExecuteCallAgent` — deferred until M3–M7 tools exist
4. Changes to `--dry-run`, `--json`, or `--verbose` output — M8 scope
5. Changes to `internal/agent/`, `internal/provider/`, or `internal/toolname/` packages
6. Global registry instances or package-level init functions
7. Thread safety for `Registry` (tool registration happens at startup before concurrent dispatch)
8. Validation that `cfg.Tools` entries correspond to registered tools at config load time (validation only checks against `toolname.ValidNames()`, which was established in M1)

---

## 10. References

- Milestone Definition: `docs/plans/000_tool_call_milestones.md` (M2 section, lines 26–40)
- M1 Spec: `docs/plans/012_tools_config_spec.md`
- M1 Implementation: `docs/plans/012_tools_config_implement.md`
- Current `executeToolCalls`: `cmd/run.go:467–525`
- Current tool injection: `cmd/run.go:213–222`
- Current `runConversationLoop`: `internal/tool/tool.go:286–344`
- Current `ExecuteCallAgent`: `internal/tool/tool.go:69–282`
- `provider.Tool` type: `internal/provider/provider.go:34–38`
- `provider.ToolCall` type: `internal/provider/provider.go:41–45`
- `provider.ToolResult` type: `internal/provider/provider.go:48–52`
- `toolname` constants: `internal/toolname/toolname.go:6–13`
- `AgentConfig.Tools` field: `internal/agent/agent.go:45`

---

## 11. Notes

- **Empty registry is intentional for M2.** Since no tool executors exist yet (M3–M7), the registry will be empty at runtime. Any agent that configures `tools = [...]` will fail at resolve time in `cmd/run.go`. This is the correct behavior — users should not configure tools before the corresponding milestones are implemented. The infrastructure is in place for M3 to simply call `registry.Register(...)` and everything works.
- **`call_agent` cannot use the registry** because its `Definition` function requires runtime `allowedAgents` (which varies per agent invocation), and its `Execute` function requires `ExecuteOptions` (depth tracking, provider creation, global config) which is fundamentally different from the generic `ExecContext`. Forcing `call_agent` through the registry would either bloat `ExecContext` or require type assertions, both of which violate the project's simplicity principle.
- **The `resolve` package import** is needed in `internal/tool/tool.go` for `runConversationLoop`'s `ExecContext.Workdir` construction (calling `resolve.Workdir("", cfg.Workdir)`). Check if `resolve` is already imported; if not, add it. Since `tool.go` already imports `resolve` (line 14), no new import is needed.
- **Three dispatch sites, one pattern.** All three dispatch sites (`cmd/run.go` sequential, `cmd/run.go` parallel, `internal/tool/tool.go` conversation loop) follow the same pattern: `if call_agent → special case; else → registry.Dispatch; if dispatch error → error ToolResult`.
