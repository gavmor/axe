# Specification: Integration Test Infrastructure (Phase 1)

**Status:** Draft
**Version:** 1.0
**Created:** 2026-03-01
**Scope:** Fixture agents, stub skills, and shared test helpers for CLI-level and integration testing

---

## 1. Purpose

Establish the test infrastructure that all subsequent integration and end-to-end test phases depend on. This includes fixture agent configurations covering every agent shape, stub skill files for those fixtures, a helper that compiles the axe binary into a temp directory for CLI-level tests, and a helper that isolates tests from the real user configuration by overriding XDG directories.

This phase produces no new user-facing behavior. It produces test fixtures and test helpers consumed by future test phases (Phases 2-6 of the integration testing milestones).

---

## 2. Design Decisions

The following decisions were made during planning and are binding for implementation:

1. **Fixture location:** All fixture files live under `cmd/testdata/`. This extends the existing convention (`cmd/testdata/skills/sample/SKILL.md` already exists). Agent fixtures go in `cmd/testdata/agents/`. Skill stubs go in `cmd/testdata/skills/`.

2. **Helper location:** Shared test helpers live in a new package `internal/testutil/`. This package is importable by both `cmd/` and `internal/` test files. It is not part of the production binary.

3. **Binary build caching:** The `BuildBinary` helper compiles the axe binary once per test process using `sync.Once`. Subsequent calls return the cached path. This avoids redundant `go build` invocations across subtests.

4. **Parameter-based fixture access:** The `SeedFixtureAgents` and `SeedFixtureSkills` helpers accept a source directory path as a parameter. The caller provides the path to its `testdata/` directory. This avoids `embed.FS` constraints (Go embed only works within the same package subtree). `internal/` tests that need agent configs continue using inline TOML strings (the established pattern in `agent_test.go`).

5. **Standard library only:** No test frameworks. All helpers use the `testing` package and `t.Helper()`. This matches the project convention established in M1-M7.

6. **No mock servers in this phase.** Mock HTTP servers for LLM providers are deferred to Phase 2.

7. **XDG isolation uses `t.Setenv`.** The `SetupXDGDirs` helper sets `XDG_CONFIG_HOME` and `XDG_DATA_HOME` via `t.Setenv()`, which automatically restores the original values when the test completes. No manual cleanup required.

---

## 3. Requirements

### 3.1 Fixture Agent Configurations (`cmd/testdata/agents/`)

**Requirement 1.1:** Create a directory `cmd/testdata/agents/` containing five TOML files. Each file represents a distinct agent configuration shape. All files must pass `agent.Validate()`.

**Requirement 1.2:** `basic.toml` -- Minimal valid agent with only required fields:

| Field | Value |
|-------|-------|
| `name` | `"basic"` |
| `model` | `"openai/gpt-4o"` |

No optional fields. No sections. This is the smallest valid agent config.

**Requirement 1.3:** `with_skill.toml` -- Agent that references a skill file:

| Field | Value |
|-------|-------|
| `name` | `"with-skill"` |
| `model` | `"openai/gpt-4o"` |
| `skill` | `"skills/stub/SKILL.md"` |

The `skill` path is relative to `$XDG_CONFIG_HOME/axe/` (the config directory). It references the stub skill created in Requirement 2.1.

**Requirement 1.4:** `with_files.toml` -- Agent that includes context files:

| Field | Value |
|-------|-------|
| `name` | `"with-files"` |
| `model` | `"openai/gpt-4o"` |
| `files` | `["README.md", "docs/**/*.md"]` |

**Requirement 1.5:** `with_memory.toml` -- Agent with memory enabled:

| Field | Value |
|-------|-------|
| `name` | `"with-memory"` |
| `model` | `"openai/gpt-4o"` |
| `[memory]` | |
| `enabled` | `true` |
| `last_n` | `5` |
| `max_entries` | `50` |

The `path` field is intentionally omitted. When omitted, the memory system resolves the path to `$XDG_DATA_HOME/axe/memory/with-memory.md` (the default).

**Requirement 1.6:** `with_subagents.toml` -- Agent with sub-agent orchestration:

| Field | Value |
|-------|-------|
| `name` | `"with-subagents"` |
| `model` | `"anthropic/claude-sonnet-4-20250514"` |
| `sub_agents` | `["basic", "with-skill"]` |
| `[sub_agents_config]` | |
| `max_depth` | `3` |
| `parallel` | `true` |
| `timeout` | `60` |

The `sub_agents` values reference other fixture agent names. In actual integration tests (Phase 2+), the corresponding `.toml` files must be seeded into the test config directory for sub-agent resolution to work.

**Requirement 1.7:** Each fixture file must contain only the fields listed for its shape. No extra fields, no comments, no whitespace padding beyond what TOML requires for readability. The files are fixtures, not documentation.

### 3.2 Stub Skill Files (`cmd/testdata/skills/`)

**Requirement 2.1:** Create a stub skill file at `cmd/testdata/skills/stub/SKILL.md` with the following content:

```markdown
# Stub Skill

## Purpose

Test fixture skill for integration tests.

## Instructions

1. Respond with "stub skill active".

## Output Format

Plain text, single line.
```

This file is referenced by `with_skill.toml` (Requirement 1.3). It follows the same section structure as the existing sample skill (`skills/sample/SKILL.md`): title, Purpose, Instructions, Output Format.

**Requirement 2.2:** The existing `cmd/testdata/skills/sample/SKILL.md` must not be modified or removed. It is used by `cmd/config_test.go`.

### 3.3 XDG Directory Isolation Helper (`internal/testutil/`)

**Requirement 3.1:** Create a new Go package at `internal/testutil/testutil.go` with package name `testutil`.

**Requirement 3.2:** Implement `SetupXDGDirs`:

```go
func SetupXDGDirs(t *testing.T) (configDir, dataDir string)
```

Behavior:
- Call `t.Helper()`.
- Create a temp directory using `t.TempDir()`. This is the XDG root.
- Create `<tempdir>/config` and `<tempdir>/data` subdirectories.
- Call `t.Setenv("XDG_CONFIG_HOME", "<tempdir>/config")`.
- Call `t.Setenv("XDG_DATA_HOME", "<tempdir>/data")`.
- Create the `axe/agents/` directory tree under `<tempdir>/config/`.
- Create the `axe/skills/` directory tree under `<tempdir>/config/`.
- Create the `axe/` directory under `<tempdir>/data/`.
- Return the axe-level config path (`<tempdir>/config/axe`) as `configDir`.
- Return the axe-level data path (`<tempdir>/data/axe`) as `dataDir`.

Post-conditions:
- `xdg.GetConfigDir()` returns `configDir`.
- `xdg.GetDataDir()` returns `dataDir`.
- `agent.Load()` looks for TOML files in `configDir/agents/`.
- After the test completes, `t.Setenv` restores original env vars. `t.TempDir` removes the temp directory.

**Requirement 3.3:** `SetupXDGDirs` must not create any files, only directories. The caller is responsible for seeding agent configs, skill files, memory files, or global config as needed.

### 3.4 Fixture Seeding Helpers (`internal/testutil/`)

**Requirement 4.1:** Implement `SeedFixtureAgents`:

```go
func SeedFixtureAgents(t *testing.T, srcDir, dstAgentsDir string)
```

Behavior:
- Call `t.Helper()`.
- Read all `*.toml` files from `srcDir`.
- Copy each file to `dstAgentsDir/<filename>`.
- Call `t.Fatal` if any read or write operation fails.

`srcDir` is the caller-provided path to the fixture agents directory (e.g., `"testdata/agents"`).
`dstAgentsDir` is the target agents directory inside the isolated XDG config (e.g., `configDir + "/agents"`).

**Requirement 4.2:** `SeedFixtureAgents` must copy files byte-for-byte. No parsing, no validation, no transformation. The raw TOML content is preserved exactly.

**Requirement 4.3:** Implement `SeedFixtureSkills`:

```go
func SeedFixtureSkills(t *testing.T, srcDir, dstSkillsDir string)
```

Behavior:
- Call `t.Helper()`.
- Recursively copy the entire directory tree from `srcDir` to `dstSkillsDir`.
- Preserve directory structure. For example, `srcDir/stub/SKILL.md` becomes `dstSkillsDir/stub/SKILL.md`.
- Call `t.Fatal` if any operation fails.

**Requirement 4.4:** `SeedFixtureSkills` must handle nested directories (skill files live in `skills/<skillname>/SKILL.md`). It must create intermediate directories as needed.

### 3.5 Binary Build Helper (`internal/testutil/`)

**Requirement 5.1:** Implement `BuildBinary`:

```go
func BuildBinary(t *testing.T) string
```

Behavior:
- Call `t.Helper()`.
- On first invocation: run `go build -o <dir>/axe .` where `<dir>` is a directory created for caching the binary. The build command must target the module root (the directory containing `go.mod`).
- Store the binary path and any build error in package-level variables protected by `sync.Once`.
- On subsequent invocations: return the cached binary path without rebuilding.
- If the build failed (on first or subsequent call), call `t.Fatal` with the build error and stderr output.
- Return the absolute path to the compiled binary.

**Requirement 5.2:** The binary cache directory must be created using `os.MkdirTemp("", "axe-test-bin-*")`. It is NOT created with `t.TempDir()` because `t.TempDir()` is scoped to a single test and would be cleaned up too early. The cache directory persists for the entire test process.

**Requirement 5.3:** Provide a cleanup function for the binary cache:

```go
func CleanupBinary()
```

This removes the cache directory. It is intended to be called from `TestMain` in the consuming test package:

```go
func TestMain(m *testing.M) {
    code := m.Run()
    testutil.CleanupBinary()
    os.Exit(code)
}
```

**Requirement 5.4:** The `go build` command must be invoked with its working directory set to the module root. The module root is determined by walking up from the `testutil` package directory to find `go.mod`. If `go.mod` cannot be found, `BuildBinary` calls `t.Fatal`.

**Requirement 5.5:** The binary name must be `axe` on Unix systems and `axe.exe` on Windows. Use `runtime.GOOS` to determine the suffix.

### 3.6 Global Config Seeding Helper (`internal/testutil/`)

**Requirement 6.1:** Implement `SeedGlobalConfig`:

```go
func SeedGlobalConfig(t *testing.T, configDir, content string)
```

Behavior:
- Call `t.Helper()`.
- Write `content` to `configDir/config.toml`.
- Call `t.Fatal` if the write fails.

This helper allows tests to set up provider API keys and base URLs (e.g., pointing to a mock server) without touching the real config. The `content` parameter is a raw TOML string provided by the caller.

---

## 4. Project Structure

After this spec is implemented, the following files will be added:

```
axe/
+-- cmd/
|   +-- testdata/
|   |   +-- agents/
|   |   |   +-- basic.toml               # NEW
|   |   |   +-- with_skill.toml          # NEW
|   |   |   +-- with_files.toml          # NEW
|   |   |   +-- with_memory.toml         # NEW
|   |   |   +-- with_subagents.toml      # NEW
|   |   +-- skills/
|   |       +-- sample/
|   |       |   +-- SKILL.md             # UNCHANGED (existing)
|   |       +-- stub/
|   |           +-- SKILL.md             # NEW
+-- internal/
|   +-- testutil/
|       +-- testutil.go                  # NEW: SetupXDGDirs, SeedFixtureAgents,
|       |                                #       SeedFixtureSkills, SeedGlobalConfig,
|       |                                #       BuildBinary, CleanupBinary
|       +-- testutil_test.go             # NEW: tests for all helpers
+-- go.mod                               # UNCHANGED (no new dependencies)
+-- go.sum                               # UNCHANGED
```

No existing files are modified. No new dependencies are introduced.

---

## 5. Edge Cases

### 5.1 SetupXDGDirs

| Scenario | Behavior |
|----------|----------|
| `XDG_CONFIG_HOME` was already set before the test | `t.Setenv` captures the original value and restores it after the test completes. |
| `XDG_CONFIG_HOME` was not set before the test | `t.Setenv` unsets it after the test completes (restores to empty). |
| Test calls `SetupXDGDirs` twice | Second call creates a new temp directory and overwrites the env vars. The first temp directory is still cleaned up by `t.TempDir`. Both are valid. But this is not a recommended usage pattern. |
| Concurrent subtests call `SetupXDGDirs` | Each subtest gets its own temp directory and env vars. Since `t.Setenv` is per-test, concurrent tests with `t.Parallel()` must NOT call `SetupXDGDirs` (env vars are process-global). This is a known Go limitation. |

### 5.2 SeedFixtureAgents / SeedFixtureSkills

| Scenario | Behavior |
|----------|----------|
| `srcDir` does not exist | `t.Fatal` with a clear message ("source directory does not exist: ..."). |
| `srcDir` is empty (no `.toml` files) | No files are copied. No error. This is a valid (if useless) invocation. |
| `dstAgentsDir` does not exist | `t.Fatal`. The caller must create the directory first (typically via `SetupXDGDirs`). |
| A file in `srcDir` is not readable | `t.Fatal` with the underlying OS error. |
| `dstAgentsDir` is not writable | `t.Fatal` with the underlying OS error. |
| `SeedFixtureSkills` encounters a nested directory (e.g., `stub/SKILL.md`) | Creates the `stub/` subdirectory under `dstSkillsDir`, then copies the file. |
| Symlinks in `srcDir` | Not followed. Only regular files and directories are processed. |

### 5.3 BuildBinary

| Scenario | Behavior |
|----------|----------|
| `go build` succeeds | Binary path is cached. All subsequent calls return the same path. |
| `go build` fails (syntax error, missing dependency) | `t.Fatal` is called with the build error and stderr output. Every subsequent call to `BuildBinary` also calls `t.Fatal` with the same cached error (the build is not retried). |
| `go.mod` not found when walking up directories | `t.Fatal` with message "could not find module root (go.mod)". |
| Binary already exists at the cache path (from a previous test run) | Overwritten by `go build -o`. Not an issue since the cache dir has a unique name. |
| `CleanupBinary` called but `BuildBinary` was never called | No-op. The cache directory was never created. |
| `CleanupBinary` called and cache dir already removed | No-op. `os.RemoveAll` on a non-existent path returns nil. |
| Test on Windows | Binary name is `axe.exe`. Build command adjusts accordingly. |

### 5.4 SeedGlobalConfig

| Scenario | Behavior |
|----------|----------|
| `configDir` does not exist | `t.Fatal`. The caller must call `SetupXDGDirs` first. |
| `config.toml` already exists at the path | Overwritten. Tests that call `SeedGlobalConfig` multiple times get the last value. |
| `content` is empty string | Writes an empty file. `config.Load()` returns a valid but empty `GlobalConfig`. |
| `content` is invalid TOML | The file is written as-is. The error surfaces when `config.Load()` is called, not during seeding. The helper does not validate content. |

### 5.5 Fixture Agent Configs

| Scenario | Behavior |
|----------|----------|
| `with_subagents.toml` references `basic` and `with-skill` but those agents are not seeded | Agent loading succeeds (sub-agents are names, not validated at load time). Sub-agent resolution fails at runtime during `axe run`, which is Phase 2+ behavior. |
| `with_skill.toml` references `skills/stub/SKILL.md` but the skill is not seeded | Skill resolution fails at runtime during `resolve.Skill()`, not at agent load time. Tests that exercise skill resolution must call `SeedFixtureSkills` first. |
| `with_memory.toml` has no `path` field | `memory.FilePath("with-memory", "")` resolves to the default path: `$XDG_DATA_HOME/axe/memory/with-memory.md`. |
| Loading `with_files.toml` with glob patterns but no matching files | `resolve.Files()` returns an empty slice. No error. This is expected behavior. |

---

## 6. Constraints

**Constraint 1:** No new external dependencies. `go.mod` must still contain only `spf13/cobra` and `BurntSushi/toml` as direct dependencies.

**Constraint 2:** The `internal/testutil/` package must contain only test support code. It must not be imported by any production (non-test) Go file.

**Constraint 3:** Fixture TOML files are static. They do not contain template variables, environment variable references, or any form of dynamic content.

**Constraint 4:** `BuildBinary` compiles via `go build`, not `go install`. The binary is placed in a temp directory, not in `$GOPATH/bin` or `$GOBIN`.

**Constraint 5:** No mock HTTP servers in this phase. Phase 2 will add them.

**Constraint 6:** All helpers must call `t.Helper()` as their first statement so that test failure messages report the caller's location, not the helper's.

**Constraint 7:** `SetupXDGDirs` must not be used with `t.Parallel()`. Environment variables are process-global. Parallel tests that need XDG isolation must use separate test binaries or accept serialized execution.

**Constraint 8:** The `testutil` package must not import any `cmd/` package. It may import `internal/` packages only if needed for type definitions (e.g., none are needed for this phase).

**Constraint 9:** Cross-platform compatibility: all helpers must work on Linux, macOS, and Windows. Use `filepath.Join` for path construction, `runtime.GOOS` for platform detection, and `os.MkdirAll` for directory creation.

---

## 7. Testing Requirements

### 7.1 Test Conventions

Tests must follow the patterns established in M1-M7:

- **Package-level tests:** Tests live in the same package (`package testutil`).
- **Standard library only:** Use `testing` package. No test frameworks.
- **Temp directories:** Use `t.TempDir()` for filesystem isolation.
- **Env overrides:** Use `t.Setenv()` for environment variable control.
- **Descriptive names:** `TestFunctionName_Scenario` with underscores.
- **Test real code, not mocks.** Tests call the actual helper functions and verify real filesystem state.
- **Run tests with:** `make test`
- **Red/green TDD:** Write failing tests first, then implement code to make them pass.

### 7.2 `internal/testutil/testutil_test.go` Tests

**Test: `TestSetupXDGDirs_CreatesDirectoryStructure`** -- Call `SetupXDGDirs`. Verify that the returned `configDir` exists and contains `agents/` and `skills/` subdirectories. Verify that the returned `dataDir` exists. Verify that `os.Getenv("XDG_CONFIG_HOME")` points to the parent of `configDir` (i.e., `configDir` is `$XDG_CONFIG_HOME/axe`). Verify that `os.Getenv("XDG_DATA_HOME")` points to the parent of `dataDir`.

**Test: `TestSetupXDGDirs_EnvVarsSet`** -- Call `SetupXDGDirs`. Verify that `XDG_CONFIG_HOME` and `XDG_DATA_HOME` are set to the expected temp directory paths. Verify these are absolute paths.

**Test: `TestSetupXDGDirs_NoFilesCreated`** -- Call `SetupXDGDirs`. Walk the entire returned `configDir` and `dataDir` trees. Verify that no regular files exist (only directories).

**Test: `TestSeedFixtureAgents_CopiesAllTomlFiles`** -- Call `SetupXDGDirs`. Create a temp source directory with 3 `.toml` files and 1 `.txt` file. Call `SeedFixtureAgents` with the source dir and `configDir + "/agents"`. Verify that exactly 3 `.toml` files exist in the destination. Verify the `.txt` file was not copied. Verify file contents are byte-identical to the source.

**Test: `TestSeedFixtureAgents_SrcDirNotExist`** -- Call `SeedFixtureAgents` with a non-existent source directory. The test must verify that `t.Fatal` would be called. Use a pattern that detects the fatal call (e.g., run in a subprocess or verify the behavior through the test structure).

**Test: `TestSeedFixtureSkills_CopiesRecursively`** -- Call `SetupXDGDirs`. Create a temp source directory with nested structure: `stub/SKILL.md` and `advanced/SKILL.md`. Call `SeedFixtureSkills`. Verify both files exist at the correct paths under the destination with correct content.

**Test: `TestSeedGlobalConfig_WritesConfigToml`** -- Call `SetupXDGDirs`. Call `SeedGlobalConfig` with a TOML string containing a provider section. Verify that `configDir/config.toml` exists and its content matches the input string exactly.

**Test: `TestSeedGlobalConfig_OverwritesExisting`** -- Call `SetupXDGDirs`. Call `SeedGlobalConfig` twice with different content. Verify that the file contains only the second content.

**Test: `TestBuildBinary_ProducesBinary`** -- Call `BuildBinary`. Verify the returned path points to an existing file. Verify the file is executable (has execute permission on Unix). Run the binary with `version` argument and verify it produces output containing `"axe version"`.

**Test: `TestBuildBinary_ReturnsSamePathOnSecondCall`** -- Call `BuildBinary` twice. Verify both calls return the same path.

**Test: `TestCleanupBinary_RemovesCacheDir`** -- Call `BuildBinary` to ensure the cache dir exists. Call `CleanupBinary`. Verify the cache directory no longer exists.

### 7.3 Fixture Validation Tests

**Test: `TestFixtureAgents_AllParseAndValidate`** -- For each `.toml` file in `cmd/testdata/agents/`: read the file, parse it with `toml.Decode` into an `agent.AgentConfig`, and call `agent.Validate()`. Verify no errors. This is a data-driven test (loop over files).

**Test: `TestFixtureAgents_BasicHasOnlyRequiredFields`** -- Parse `basic.toml`. Verify `Name` is `"basic"` and `Model` is `"openai/gpt-4o"`. Verify all optional fields are zero-valued: `Description` is `""`, `Skill` is `""`, `Files` is `nil`, `SubAgents` is `nil`, `Memory.Enabled` is `false`.

**Test: `TestFixtureAgents_WithSkillReferencesStubSkill`** -- Parse `with_skill.toml`. Verify `Skill` is `"skills/stub/SKILL.md"`.

**Test: `TestFixtureAgents_WithMemoryConfig`** -- Parse `with_memory.toml`. Verify `Memory.Enabled` is `true`, `Memory.LastN` is `5`, `Memory.MaxEntries` is `50`, `Memory.Path` is `""` (empty, uses default).

**Test: `TestFixtureAgents_WithSubagentsConfig`** -- Parse `with_subagents.toml`. Verify `SubAgents` is `["basic", "with-skill"]`. Verify `SubAgentsConf.MaxDepth` is `3`, `SubAgentsConf.Parallel` is non-nil and `true`, `SubAgentsConf.Timeout` is `60`.

**Test: `TestFixtureSkills_StubSkillExists`** -- Read `cmd/testdata/skills/stub/SKILL.md`. Verify it contains the `# Stub Skill` header and all required sections (Purpose, Instructions, Output Format).

---

## 8. Acceptance Criteria

The milestone is complete when all of the following are true:

1. `make test` passes with zero failures.
2. Five fixture agent TOML files exist in `cmd/testdata/agents/` covering: basic, with-skill, with-files, with-memory, and with-subagents shapes.
3. A stub skill file exists at `cmd/testdata/skills/stub/SKILL.md` with valid skill structure.
4. `testutil.SetupXDGDirs` creates an isolated XDG directory structure and sets env vars.
5. `testutil.SeedFixtureAgents` copies `.toml` files from a source directory to a target agents directory.
6. `testutil.SeedFixtureSkills` recursively copies skill directories from a source to a target skills directory.
7. `testutil.SeedGlobalConfig` writes a `config.toml` to the isolated config directory.
8. `testutil.BuildBinary` compiles the axe binary once per test process and returns a cached path.
9. `testutil.CleanupBinary` removes the cached binary directory.
10. All fixture agents pass `agent.Validate()`.
11. No existing tests are broken.
12. No new external dependencies are introduced.
13. The `internal/testutil/` package is not imported by any production code.

---

## 9. Out of Scope

The following items are explicitly **not** included in this spec:

1. Mock HTTP servers for LLM providers (Phase 2)
2. `axe run` integration tests (Phase 2)
3. CLI smoke tests against the compiled binary (Phase 3)
4. Golden file comparison infrastructure (Phase 4)
5. GitHub Actions CI configuration (Phase 5)
6. Live provider tests (Phase 6)
7. Refactoring existing test helpers in `cmd/` test files (they continue working as-is)
8. Test coverage metrics or reporting
9. Benchmarks

---

## 10. References

- Integration Testing Milestones: `docs/plans/000_i9n_milestones.md` (Phase 1)
- Agent Config Schema: `docs/design/agent-config-schema.md`
- Existing XDG implementation: `internal/xdg/xdg.go`
- Existing agent config struct: `internal/agent/agent.go` (lines 14-48)
- Existing fixture pattern: `cmd/testdata/skills/sample/SKILL.md`
- Existing test helper pattern: `cmd/run_test.go` (`setupAgentsDir`, `writeAgentFile`)
