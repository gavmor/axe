# Specification: CLI Smoke Tests (Phase 3)

**Status:** Draft
**Version:** 1.0
**Created:** 2026-03-01
**Scope:** End-to-end smoke tests of the compiled `axe` binary via `exec.Command`

---

## 1. Purpose

Verify that the compiled `axe` binary behaves correctly when invoked as an external process. Phase 2 tested the full `axe run` flow in-process via `rootCmd.Execute()`. Phase 3 tests the actual compiled binary via `os/exec.Command`, validating the real entry point (`main.go`), process exit codes, stdout/stderr output, filesystem side effects, and stdin piping.

This is the first phase that exercises `testutil.BuildBinary`. Every test invokes the compiled binary as a child process — no in-process cobra execution.

---

## 2. Design Decisions

The following decisions were made during planning and are binding for implementation:

1. **Tests live in `cmd/smoke_test.go`.** All Phase 3 smoke tests are in a single file in the `cmd` package. This is consistent with Phase 2's `cmd/run_integration_test.go` placement.

2. **`TestMain` for binary cleanup.** A `TestMain` function is added to the `cmd` package (in `cmd/smoke_test.go`) that calls `m.Run()` then `testutil.CleanupBinary()`. Since `BuildBinary` uses `sync.Once`, the actual compilation happens on the first test that calls `BuildBinary(t)`. No `TestMain` currently exists in the `cmd` package.

3. **No build tags.** Smoke tests run unconditionally with `go test ./...` (and `make test`). This aligns with the Phase 5 milestone which plans `go test ./...` for unit + integration together.

4. **Out-of-process environment isolation.** Unlike Phase 2 which used `t.Setenv()`, these tests pass `XDG_CONFIG_HOME` and `XDG_DATA_HOME` via `exec.Cmd.Env`. The test helper constructs a clean environment that inherits from `os.Environ()` but overrides specific variables.

5. **API key stripping.** Tests that assert missing-API-key behavior must explicitly remove `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, and any other provider key env vars from the child process environment to avoid inheriting the host machine's real keys.

6. **Fixture seeding via `testutil.SeedFixtureAgents` and `testutil.SeedFixtureSkills`.** Tests that need agent configs use the Phase 1 seeders to copy fixture TOML files from `cmd/testdata/agents/` and `cmd/testdata/skills/` into the test's isolated XDG config directory.

7. **Standard library only.** No test frameworks. Continues the project convention.

8. **Dry-run tests use only the `basic` fixture agent.** More complex fixture agent coverage (with_skill, with_subagents) is deferred to Phase 4 (golden file tests).

9. **Config init is tested for idempotency.** The test runs `axe config init` twice and verifies the second invocation exits 0 without overwriting files created by the first.

---

## 3. Requirements

### 3.1 Test Helper: `runAxe`

**Requirement 1.1:** Add a file-local helper function in `cmd/smoke_test.go`:

```go
func runAxe(t *testing.T, env map[string]string, stdinData string, args ...string) (stdout, stderr string, exitCode int)
```

Behavior:
- Call `t.Helper()`.
- Call `testutil.BuildBinary(t)` to obtain the compiled binary path.
- Create `exec.Command(binPath, args...)`.
- Construct the child process environment:
  - Start with `os.Environ()`.
  - Override or add every key-value pair from the `env` map.
  - This approach means the child inherits the host's full environment except where explicitly overridden.
- If `stdinData` is non-empty, set `cmd.Stdin` to `strings.NewReader(stdinData)`.
- Capture stdout and stderr via `bytes.Buffer` on `cmd.Stdout` and `cmd.Stderr`.
- Call `cmd.Run()`.
- Extract the exit code:
  - If `cmd.Run()` returns `nil`, exit code is `0`.
  - If the error is `*exec.ExitError`, extract via `exitError.ExitCode()`.
  - For any other error, call `t.Fatalf` (unexpected failure to launch the process).
- Return `stdout.String()`, `stderr.String()`, and the exit code.

**Requirement 1.2:** Add a file-local helper function in `cmd/smoke_test.go`:

```go
func setupSmokeEnv(t *testing.T) (configDir, dataDir string, env map[string]string)
```

Behavior:
- Call `t.Helper()`.
- Create a temp directory root via `t.TempDir()`.
- Compute `configHome = root/config` and `dataHome = root/data`.
- Compute `configDir = configHome/axe` and `dataDir = dataHome/axe`.
- Create directories: `configDir/agents/`, `configDir/skills/`, `dataDir/`.
- Return `configDir`, `dataDir`, and an `env` map containing:
  - `"XDG_CONFIG_HOME"`: configHome
  - `"XDG_DATA_HOME"`: dataHome

This parallels `testutil.SetupXDGDirs` but returns an env map instead of calling `t.Setenv()`, because the environment must be passed to the child process, not set in the test process.

**Requirement 1.3:** Add a file-local helper function in `cmd/smoke_test.go`:

```go
func stripAPIKeys(env map[string]string) map[string]string
```

Behavior:
- Call this to add entries that explicitly blank out provider API key env vars in the child process.
- Set `"OPENAI_API_KEY"`, `"ANTHROPIC_API_KEY"`, and `"OLLAMA_API_KEY"` to `""` in the env map.
- Return the (mutated) env map for chaining convenience.

This ensures the child process does not inherit real API keys from the host. Tests that need a specific key set it after calling `stripAPIKeys`.

### 3.2 `TestMain`

**Requirement 2.1:** Add a `TestMain` function in `cmd/smoke_test.go`:

```go
func TestMain(m *testing.M) {
    code := m.Run()
    testutil.CleanupBinary()
    os.Exit(code)
}
```

This ensures the compiled binary temp directory is cleaned up after all tests in the `cmd` package finish, regardless of pass/fail. `BuildBinary` is not called here — it is called lazily by the first smoke test via `runAxe`.

### 3.3 Smoke Test: `axe version`

**Requirement 3.1 — Test: `TestSmoke_Version`**

Setup:
- No XDG isolation needed. No agent configs needed.
- Call `runAxe(t, nil, "", "version")`.

Assertions:
- Exit code is `0`.
- stdout contains the string `"axe version "` (note trailing space before version number).
- stdout is non-empty and ends with a newline.
- stderr is empty.

### 3.4 Smoke Test: `axe config path`

**Requirement 4.1 — Test: `TestSmoke_ConfigPath`**

Setup:
- Call `setupSmokeEnv(t)` to get isolated XDG dirs and env map.
- Call `runAxe(t, env, "", "config", "path")`.

Assertions:
- Exit code is `0`.
- stdout (trimmed) is a non-empty string.
- stdout (trimmed) ends with `/axe` (the axe config directory under `XDG_CONFIG_HOME`).
- stdout (trimmed) equals `configDir` (the path returned by `setupSmokeEnv`).
- stderr is empty.

### 3.5 Smoke Test: `axe config init`

**Requirement 5.1 — Test: `TestSmoke_ConfigInit`**

Setup:
- Call `setupSmokeEnv(t)` to get isolated XDG dirs and env map.
- First invocation: call `runAxe(t, env, "", "config", "init")`.

Assertions (first invocation):
- Exit code is `0`.
- stdout (trimmed) equals `configDir`.
- The following paths exist on the filesystem:
  - `configDir/agents/` (directory)
  - `configDir/skills/sample/SKILL.md` (file, non-empty)
  - `configDir/config.toml` (file, non-empty)
- stderr is empty.

Setup (idempotency check):
- Read `configDir/config.toml` content and save it.
- Read `configDir/skills/sample/SKILL.md` content and save it.
- Second invocation: call `runAxe(t, env, "", "config", "init")`.

Assertions (second invocation):
- Exit code is `0`.
- `configDir/config.toml` content is byte-identical to the saved content (not overwritten).
- `configDir/skills/sample/SKILL.md` content is byte-identical to the saved content (not overwritten).

### 3.6 Smoke Test: `axe run nonexistent-agent`

**Requirement 6.1 — Test: `TestSmoke_RunNonexistentAgent`**

Setup:
- Call `setupSmokeEnv(t)` to get isolated XDG dirs and env map.
- Do NOT seed any fixture agents (the agents directory is empty).
- Call `stripAPIKeys(env)`.
- Call `runAxe(t, env, "", "run", "nonexistent-agent")`.

Assertions:
- Exit code is `2`.
- stderr is non-empty.
- stderr contains the string `"nonexistent-agent"` (the error message references the agent name).
- stdout is empty.

### 3.7 Smoke Test: `axe run <fixture> --dry-run`

**Requirement 7.1 — Test: `TestSmoke_RunDryRun`**

Setup:
- Call `setupSmokeEnv(t)` to get isolated XDG dirs and env map.
- Seed fixture agents: call `testutil.SeedFixtureAgents(t, "testdata/agents", filepath.Join(configDir, "agents"))`. The source path is relative to the `cmd/` package test directory.
- Call `stripAPIKeys(env)`. (Dry-run does not require an API key.)
- Call `runAxe(t, env, "", "run", "basic", "--dry-run")`.

Assertions:
- Exit code is `0`.
- stdout contains `"=== Dry Run ==="`.
- stdout contains `"openai/gpt-4o"` (the model from `basic.toml`; the actual format is `"Model:    openai/gpt-4o"` with alignment padding).
- stdout contains `"--- System Prompt ---"`.
- stdout contains `"--- Stdin ---"`.
- stdout contains `"(none)"` in the Stdin section (no stdin was piped).
- stderr is empty.

### 3.8 Smoke Test: Bad `--model` format

**Requirement 8.1 — Test: `TestSmoke_BadModelFormat`**

Setup:
- Call `setupSmokeEnv(t)` to get isolated XDG dirs and env map.
- Seed fixture agents (need a valid agent to load before model parsing).
- Call `stripAPIKeys(env)`.
- Call `runAxe(t, env, "", "run", "basic", "--model", "no-slash-here")`.

Assertions:
- Exit code is `1`.
- stderr contains `"invalid model format"`.
- stdout is empty.

### 3.9 Smoke Test: Missing API key

**Requirement 9.1 — Test: `TestSmoke_MissingAPIKey`**

Setup:
- Call `setupSmokeEnv(t)` to get isolated XDG dirs and env map.
- Seed fixture agents.
- Call `stripAPIKeys(env)` to ensure no API keys are inherited.
- Do NOT set any API key env var.
- Do NOT write any API key to `config.toml`.
- Call `runAxe(t, env, "", "run", "basic")`.

The `basic.toml` agent uses `model = "openai/gpt-4o"`, so the binary will look for `OPENAI_API_KEY`.

Assertions:
- Exit code is `3`.
- stderr contains `"API key"`.
- stderr contains `"OPENAI_API_KEY"` (the env var hint in the error message).
- stdout is empty.

### 3.10 Smoke Test: Piped stdin in dry-run

**Requirement 10.1 — Test: `TestSmoke_PipedStdinInDryRun`**

Setup:
- Call `setupSmokeEnv(t)` to get isolated XDG dirs and env map.
- Seed fixture agents.
- Call `stripAPIKeys(env)`.
- Call `runAxe(t, env, "custom user input from stdin", "run", "basic", "--dry-run")`.

Assertions:
- Exit code is `0`.
- stdout contains `"=== Dry Run ==="`.
- stdout contains `"--- Stdin ---"`.
- stdout contains `"custom user input from stdin"`.
- stdout does NOT contain `"(none)"` in the Stdin section. (The presence of `"custom user input from stdin"` implicitly verifies this, but the test should also verify the `"(none)"` string does not appear after `"--- Stdin ---"` and before the next `"---"` section delimiter.)
- stderr is empty.

---

## 4. Project Structure

After this spec is implemented, the following files will be added or modified:

```
axe/
+-- cmd/
|   +-- smoke_test.go                   # NEW: TestMain, runAxe helper, all 8 smoke tests
+-- docs/
|   +-- plans/
|       +-- 000_i9n_milestones.md       # MODIFIED: Phase 2 items marked [x]
```

No other files are added or modified.

---

## 5. Edge Cases

### 5.1 Environment Handling

| Scenario | Behavior |
|----------|----------|
| Host has `OPENAI_API_KEY` set | `stripAPIKeys` sets it to `""` in the child env, overriding the host value. Tests asserting missing-key behavior are not affected by the host's real keys. |
| Host has `AXE_OPENAI_BASE_URL` set | Not stripped. This only matters for tests that actually call the LLM (none in Phase 3 — all tests use `--dry-run`, error paths, or commands that don't call providers). |
| `env` map is `nil` | `runAxe` inherits the full host environment with no overrides. Used only by `TestSmoke_Version` which needs no isolation. |
| `env` map has a key that doesn't exist in `os.Environ()` | The key is added to the child environment. Standard `exec.Cmd.Env` behavior. |

### 5.2 Binary Compilation

| Scenario | Behavior |
|----------|----------|
| `go build` fails | `BuildBinary` calls `t.Fatalf`. All smoke tests fail immediately. |
| Multiple smoke tests call `BuildBinary` | `sync.Once` ensures compilation runs exactly once. All tests share the same binary path. |
| `CleanupBinary` called in `TestMain` | Removes the temp directory containing the binary. Called after `m.Run()` regardless of test results. |
| Non-smoke tests in `cmd/` package (Phase 2, unit tests) | They continue to work. They do not call `BuildBinary`. `TestMain` only adds `CleanupBinary()` after `m.Run()`, which is a no-op if `BuildBinary` was never called. |

### 5.3 Exit Code Extraction

| Scenario | Behavior |
|----------|----------|
| Binary exits with code 0 | `cmd.Run()` returns `nil`. `runAxe` returns `exitCode = 0`. |
| Binary exits with non-zero code (1, 2, 3) | `cmd.Run()` returns `*exec.ExitError`. `runAxe` extracts the code via `ExitCode()`. |
| Binary crashes (signal, panic) | `cmd.Run()` returns `*exec.ExitError` with platform-dependent code (typically -1 or 128+signal). Tests do not assert on crash exit codes. |
| Binary not found at path | `cmd.Run()` returns a non-ExitError (e.g., `*os.PathError`). `runAxe` calls `t.Fatalf`. |

### 5.4 Stdin Piping

| Scenario | Behavior |
|----------|----------|
| `stdinData` is empty string | `cmd.Stdin` is not set. The child process's stdin is `os.DevNull` (default for `exec.Cmd`). The binary's `resolve.Stdin()` detects this is not a pipe (or is empty) and returns `""`. |
| `stdinData` is non-empty | `cmd.Stdin` is `strings.NewReader(stdinData)`. The binary reads it as piped stdin content. |
| `stdinData` contains newlines | Handled correctly. `io.ReadAll` in the binary reads the entire content including newlines. |

### 5.5 Config Init Idempotency

| Scenario | Behavior |
|----------|----------|
| First `config init` run | Creates `agents/`, `skills/sample/SKILL.md`, and `config.toml`. Exits 0. |
| Second `config init` run | `agents/` already exists — `MkdirAll` is a no-op. `SKILL.md` exists — `os.Stat` check skips write. `config.toml` exists — `os.Stat` check skips write. Exits 0. |
| `config.toml` modified between runs | Second run does not overwrite it. The `os.IsNotExist` check only writes when the file is absent. |

### 5.6 Fixture Agent Path Resolution

| Scenario | Behavior |
|----------|----------|
| `testdata/agents/` path relative to test file | Go test runner sets the working directory to the package directory (`cmd/`). Relative paths like `"testdata/agents"` resolve correctly. |
| `SeedFixtureAgents` source dir missing | `t.Fatalf` is called inside the helper. Test fails immediately. |

### 5.7 Parallel Test Safety

| Scenario | Behavior |
|----------|----------|
| Smoke tests with `t.Parallel()` | Safe for smoke tests because each test runs an external process with its own isolated environment and temp directories. Unlike Phase 2 integration tests, smoke tests do NOT mutate global cobra command state. Smoke tests MAY use `t.Parallel()`. |
| Smoke tests running alongside Phase 2 integration tests | Phase 2 tests mutate global cobra state and use `t.Setenv()`. Within `go test ./cmd/`, all tests in the package run in a single binary. If Phase 2 tests do NOT use `t.Parallel()`, and smoke tests use `t.Parallel()`, the two groups must not interleave. Go's test runner runs parallel tests concurrently but runs non-parallel tests sequentially. This is safe: non-parallel Phase 2 tests run sequentially, and parallel smoke tests run concurrently with each other but not concurrently with non-parallel tests. |

**Decision:** Smoke tests SHOULD use `t.Parallel()` where possible. Each smoke test creates its own isolated temp directories and runs the binary as a child process — there is no shared mutable state. Using `t.Parallel()` reduces total test time since the binary compilation cost is paid once and the tests themselves are I/O-bound.

---

## 6. Constraints

**Constraint 1:** No new external dependencies. `go.mod` must not change.

**Constraint 2:** Standard library only for all test code. No test frameworks.

**Constraint 3:** Existing tests in `cmd/run_test.go`, `cmd/run_integration_test.go`, and other files must not be modified.

**Constraint 4:** All Phase 3 tests must pass with zero real network calls. Every test either uses `--dry-run`, an error path that exits before making a provider call, or a command that does not involve providers.

**Constraint 5:** The `internal/testutil/` package must not be imported by any production (non-test) Go file.

**Constraint 6:** All helpers must call `t.Helper()` as their first statement.

**Constraint 7:** Cross-platform compatibility. Use `filepath.Join` for path construction. Use `runtime.GOOS` checks only where unavoidable.

**Constraint 8:** No `t.Setenv()` in smoke tests. Environment is passed via `exec.Cmd.Env` to the child process.

**Constraint 9:** The `TestMain` function must not break existing tests in the `cmd` package. `CleanupBinary()` is a no-op if `BuildBinary` was never called.

---

## 7. Testing Requirements

### 7.1 Test Conventions

- **Package-level tests:** Tests live in `package cmd`.
- **Standard library only:** Use `testing` package. No test frameworks.
- **Temp directories:** Use `t.TempDir()` for filesystem isolation.
- **Descriptive names:** `TestSmoke_<Command>_<Scenario>` or `TestSmoke_<Scenario>`.
- **Test real code, not mocks.** Every test invokes the real compiled binary. No mocks, no in-process cobra execution.
- **Red/green TDD:** Write failing tests first, then verify they pass against the existing binary (all tests should pass without production code changes since they test existing behavior).
- **Run tests with:** `make test`
- **Parallel execution:** Smoke tests should use `t.Parallel()`.

### 7.2 Test Summary

| # | Test Name | Command | Expected Exit | Key Assertions |
|---|-----------|---------|---------------|----------------|
| 1 | `TestSmoke_Version` | `axe version` | 0 | stdout contains `"axe version "` |
| 2 | `TestSmoke_ConfigPath` | `axe config path` | 0 | stdout equals `configDir` |
| 3 | `TestSmoke_ConfigInit` | `axe config init` (x2) | 0 | files created; idempotent |
| 4 | `TestSmoke_RunNonexistentAgent` | `axe run nonexistent-agent` | 2 | stderr contains agent name |
| 5 | `TestSmoke_RunDryRun` | `axe run basic --dry-run` | 0 | stdout contains dry-run output |
| 6 | `TestSmoke_BadModelFormat` | `axe run basic --model no-slash` | 1 | stderr contains `"invalid model format"` |
| 7 | `TestSmoke_MissingAPIKey` | `axe run basic` | 3 | stderr contains `"API key"` |
| 8 | `TestSmoke_PipedStdinInDryRun` | `axe run basic --dry-run` (with stdin) | 0 | stdout contains piped content |

---

## 8. Acceptance Criteria

The milestone is complete when all of the following are true:

1. `make test` passes with zero failures.
2. `cmd/smoke_test.go` exists with `TestMain`, `runAxe`, `setupSmokeEnv`, `stripAPIKeys`, and all 8 smoke tests.
3. Every smoke test invokes the compiled binary via `exec.Command` (no in-process cobra execution).
4. Zero network calls are made during smoke test execution.
5. No existing tests are broken.
6. No existing production files are modified.
7. No new external dependencies are introduced.
8. Phase 2 items in `docs/plans/000_i9n_milestones.md` are marked `[x]`.

---

## 9. Out of Scope

The following items are explicitly **not** included in this spec:

1. Golden file comparison tests (Phase 4)
2. GitHub Actions CI configuration (Phase 5)
3. Live provider tests (Phase 6)
4. Dry-run tests for complex fixture agents (with_skill, with_subagents) — deferred to Phase 4 golden files
5. Tests that require a running mock server (Phase 2 covers those in-process)
6. `axe agents` or `axe gc` command smoke tests (not in the Phase 3 milestone)
7. Windows-specific behavior testing
8. Test coverage metrics or reporting
9. Benchmarks
10. Refactoring `testutil.SetupXDGDirs` to support out-of-process usage (we create a parallel `setupSmokeEnv` helper instead)

---

## 10. References

- Integration Testing Milestones: `docs/plans/000_i9n_milestones.md` (Phase 3)
- Phase 1 Spec: `docs/plans/008_i9n_test_infrastructure_spec.md`
- Phase 2 Spec: `docs/plans/009_mock_provider_integration_spec.md`
- Version command: `cmd/version.go` (lines 9-17)
- Config commands: `cmd/config.go` (lines 26-106)
- Run command: `cmd/run.go` (lines 29-541)
- Dry-run output: `cmd/run.go` (lines 394-465)
- Model parsing: `cmd/run.go` (lines 51-68)
- API key check: `cmd/run.go` (lines 182-190)
- Exit codes: `cmd/exit.go` (lines 6-9), `cmd/root.go` (lines 29-42)
- Binary builder: `internal/testutil/testutil.go` (lines 152-198)
- XDG helper: `internal/testutil/testutil.go` (lines 18-45)
- Fixture seeders: `internal/testutil/testutil.go` (lines 49-125)
- Fixture agents: `cmd/testdata/agents/*.toml`
- Fixture skills: `cmd/testdata/skills/`
