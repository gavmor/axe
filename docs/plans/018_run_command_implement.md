# Implementation Checklist: Tool Call M7 — `run_command` Tool

**Spec:** `docs/plans/018_run_command_spec.md`
**Status:** Complete

---

## Phase 1: Tests (Red)

Write all tests first. They must fail (compile errors or test failures) until the implementation exists.

- [x] Create `internal/tool/run_command_test.go` with `package tool` and required imports (`context`, `path/filepath`, `strings`, `testing`, `time`, `github.com/jrswab/axe/internal/provider`)
- [x] `TestRunCommand_Success` — command `"echo hello"`, verify IsError false, Content equals `"hello\n"`, CallID matches input call.ID
- [x] `TestRunCommand_FailingCommand` — command `"exit 42"`, verify IsError true, Content contains `"exit code 42"`
- [x] `TestRunCommand_OutputCapture` — command `"echo stdout; echo stderr >&2"`, verify IsError false, Content contains `"stdout"` and `"stderr"`
- [x] `TestRunCommand_Timeout` — context with 50ms timeout, command `"sleep 60"`, verify IsError true (killed by context cancellation)
- [x] `TestRunCommand_LargeOutputTruncation` — command generating >100KB output (e.g., `dd if=/dev/zero bs=1024 count=200 2>/dev/null | tr '\0' 'A'`), verify IsError false, Content contains `"[output truncated, exceeded 100KB]"`, Content length > 102400 but significantly < 200KB
- [x] `TestRunCommand_MissingCommand` — empty Arguments map `{}`, verify IsError true, Content contains `"command is required"`
- [x] `TestRunCommand_EmptyCommand` — command `""`, verify IsError true, Content contains `"command is required"`
- [x] `TestRunCommand_WorkdirRespected` — command `"pwd"`, verify IsError false, Content contains tmpdir path (use `filepath.EvalSymlinks` on tmpdir before comparison)
- [x] `TestRunCommand_CallIDPassthrough` — call.ID `"rc-unique-77"`, command `"true"`, verify ToolResult.CallID equals `"rc-unique-77"`
- [x] `TestRunCommand_FailingCommandWithOutput` — command `"echo before-fail; exit 1"`, verify IsError true, Content contains `"exit code 1"` and `"before-fail"`
- [x] Add `TestRegisterAll_RegistersRunCommand` to `internal/tool/registry_test.go` — verify `r.Has(toolname.RunCommand)` after `RegisterAll`
- [x] Add `TestRegisterAll_ResolvesRunCommand` to `internal/tool/registry_test.go` — verify resolved tool has Name `toolname.RunCommand` and `command` parameter
- [x] Run `make test` — confirm new tests fail (compile errors expected since `runCommandEntry` does not exist yet)

## Phase 2: Implementation (Green)

Minimal code to make all tests pass.

- [x] Create `internal/tool/run_command.go` with `package tool`
- [x] Implement `runCommandEntry() ToolEntry` returning `ToolEntry{Definition: runCommandDefinition, Execute: runCommandExecute}`
- [x] Implement `runCommandDefinition() provider.Tool` with Name `toolname.RunCommand`, Description, and 1 Parameter (`command`, required)
- [x] Implement `runCommandExecute(ctx, call, ec) provider.ToolResult`:
  - [x] Extract `command` from `call.Arguments["command"]`, return error ToolResult `"command is required"` if empty (Req 2.2, 2.3)
  - [x] Create command via `exec.CommandContext(ctx, "sh", "-c", command)`, set `cmd.Dir = ec.Workdir` (Req 2.4)
  - [x] Call `cmd.CombinedOutput()` to execute and capture combined stdout+stderr (Req 2.5)
  - [x] Truncate output if length > 102400 bytes, append `"\n... [output truncated, exceeded 100KB]"` (Req 2.6)
  - [x] On nil error: return success ToolResult with Content `string(output)`, IsError false (Req 2.7)
  - [x] On error that is `*exec.ExitError` (via `errors.As`): return ToolResult with Content `fmt.Sprintf("exit code %d\n%s", exitErr.ExitCode(), output)`, IsError true (Req 2.8)
  - [x] On error that is NOT `*exec.ExitError`: return ToolResult with Content `err.Error()`, IsError true (Req 2.9)
- [x] Add `r.Register(toolname.RunCommand, runCommandEntry())` to `RegisterAll` in `internal/tool/registry.go` (Req 3.1)
- [x] Run `make test` — confirm all new tests pass and all existing tests still pass

## Phase 3: Verification

- [x] Run `go build ./internal/tool/` — confirm compilation
- [x] Run `make test` — confirm all tests pass (exit 0)
- [x] Verify no changes to: `go.mod`, `go.sum`, `internal/provider/`, `internal/agent/`, `internal/toolname/`, `cmd/run.go`, `internal/tool/tool.go`, `internal/tool/path_validation.go`
