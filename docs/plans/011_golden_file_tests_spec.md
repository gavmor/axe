# 011 — Golden File Tests Specification

Status: **Draft**
Phase: **Milestone 4** (from `000_i9n_milestones.md`)

---

## 1. Goal

Catch unintended output regressions in `axe run --dry-run` and `axe run --json`
by comparing actual CLI output against stored golden files. If the output changes
intentionally, a single flag regenerates the golden files.

## 2. Scope

### In Scope

- Golden files for `--dry-run` output (all 5 fixture agents).
- Golden files for `--json` envelope output (all 5 fixture agents).
- A test runner that diffs actual output against golden files.
- An `-update` flag to regenerate golden files from current output.
- Placeholder masking for non-deterministic values.

### Out of Scope

- Live provider tests (Phase 6).
- CI integration (Phase 5).
- New fixture agents — use the 5 that already exist.
- Changes to `--dry-run` or `--json` output formatting.

## 3. Constraints

- **Zero network calls.** All tests use `--dry-run` or mocked providers.
- **Subprocess execution.** Tests invoke the compiled `axe` binary via
  `exec.Command`, consistent with the existing smoke test pattern
  (`cmd/smoke_test.go`).
- **Isolated environment.** Every test uses `setupSmokeEnv` + `stripAPIKeys`
  so no real user config or API keys are touched.
- **`make test` runs everything.** No build tags, no separate targets. Golden
  file tests must pass under `go test ./...`.

## 4. Golden File Directory Layout

All golden files live under `cmd/testdata/golden/`. Two subdirectories split
the output types:

```
cmd/testdata/golden/
  dry-run/
    basic.txt
    with_skill.txt
    with_files.txt
    with_memory.txt
    with_subagents.txt
  json/
    basic.json
    with_skill.json
    with_files.json
    with_memory.json
    with_subagents.json
```

File names match the fixture agent TOML file names (without the `.toml` suffix).

### 4.1 Dry-Run Golden Files

Each `.txt` file contains the full stdout of `axe run <agent> --dry-run` after
placeholder masking has been applied (see Section 6).

### 4.2 JSON Golden Files

Each `.json` file contains the full stdout of `axe run <agent>` (with the mock
LLM server providing a deterministic response) after placeholder masking has
been applied (see Section 6). The file is stored as pretty-printed JSON for
readability and diffability.

## 5. Fixture Agents Under Test

All 5 existing fixture agents in `cmd/testdata/agents/`:

| Agent File         | `--dry-run` Golden | `--json` Golden | Notes                          |
| ------------------ | ------------------ | --------------- | ------------------------------ |
| `basic.toml`       | Yes                | Yes             | Minimal config, zero optionals |
| `with_skill.toml`  | Yes                | Yes             | Includes skill content         |
| `with_files.toml`  | Yes                | Yes             | Includes file globs            |
| `with_memory.toml` | Yes                | Yes             | Memory section enabled         |
| `with_subagents.toml` | Yes             | Yes             | Sub-agent orchestration        |

Each fixture agent produces one dry-run golden file and one JSON golden file,
totaling **10 golden files**.

## 6. Non-Deterministic Value Masking

Before comparing actual output to a golden file, the test runner must replace
known non-deterministic values with fixed placeholders. The same masking is
applied when generating golden files with `-update`.

### 6.1 Masking Rules

| Value               | Source             | Placeholder          | Applies To   |
| ------------------- | ------------------ | -------------------- | ------------ |
| Workdir path        | `Workdir:` line    | `{{WORKDIR}}`        | `--dry-run`  |
| `duration_ms` value | JSON field         | `{{DURATION_MS}}`    | `--json`     |

### 6.2 Masking Behavior

- **Workdir (`--dry-run`):** Replace the absolute path appearing after
  `Workdir:  ` (note: two spaces per the format string) with the literal
  string `{{WORKDIR}}`. The pattern to match is the full path from the start
  of the value to the end of the line.

- **`duration_ms` (`--json`):** After parsing the JSON, replace the
  `duration_ms` numeric value with the string `"{{DURATION_MS}}"`, then
  re-serialize. Alternatively, perform a regex replacement on the raw string
  `"duration_ms":\s*\d+` with `"duration_ms":"{{DURATION_MS}}"` before
  comparison.

### 6.3 Extensibility

The masking function must accept a list of masking rules so future
non-deterministic fields can be added without refactoring the comparison logic.

## 7. Test Runner Behavior

### 7.1 Test File

All golden file tests live in a single file: `cmd/golden_test.go`.

### 7.2 Update Flag

A test-level variable controlled by a custom flag or environment variable
determines whether tests run in **compare** mode (default) or **update** mode.

- **Compare mode (default):** Run the command, mask output, read the golden
  file, compare. Fail with a diff if they do not match. If the golden file
  does not exist, fail with a clear message instructing the user to run with
  the update flag.
- **Update mode:** Run the command, mask output, write the result to the
  golden file path. The test passes (it is a generation run, not a
  validation run). Log which files were written.

The update flag must be activated via:
```
go test ./cmd/ -run TestGolden -update-golden
```
or via the environment variable `UPDATE_GOLDEN=1`.

### 7.3 Test Structure

Use table-driven subtests. Each entry specifies:

- Agent name (matches the TOML filename without extension).
- Output mode (`--dry-run` or `--json`).
- Golden file path (derived from agent name + output mode).
- For `--json` tests: the mock LLM response(s) to enqueue and the mock
  server URL to pass via environment.

Example pseudostructure:
```
TestGolden/dry-run/basic
TestGolden/dry-run/with_skill
TestGolden/dry-run/with_files
TestGolden/dry-run/with_memory
TestGolden/dry-run/with_subagents
TestGolden/json/basic
TestGolden/json/with_skill
TestGolden/json/with_files
TestGolden/json/with_memory
TestGolden/json/with_subagents
```

### 7.4 Diff Output on Failure

When actual output does not match the golden file, the test failure message
must include:

1. The path to the golden file.
2. A unified diff between expected (golden) and actual (masked) output.
3. Instructions to run with `-update-golden` to regenerate.

### 7.5 Golden File Staleness

If a golden file exists but is empty (0 bytes), the test must fail with a
message indicating the file appears uninitialized.

## 8. JSON Golden File Details

### 8.1 Mock Server Requirement

`--json` tests require a real LLM response. Each `--json` test case must:

1. Start a `testutil.NewMockLLMServer` with a deterministic, canned response.
2. Pass the mock server URL to the axe binary via the appropriate environment
   variable (e.g., `OPENAI_BASE_URL` or `ANTHROPIC_BASE_URL` depending on
   the agent's configured model provider).
3. Provide a valid (but fake) API key in the environment so the binary does
   not exit with code 3.

### 8.2 Deterministic Responses

The mock server responses must be identical across runs. Use fixed values for:

- `content` (e.g., `"Hello from mock."`)
- `input_tokens` (e.g., `10`)
- `output_tokens` (e.g., `5`)
- `stop_reason` (e.g., `"end_turn"` / `"stop"`)
- `model` (e.g., matching the agent's configured model string)

### 8.3 JSON Normalization

Before comparison, the JSON output must be:

1. Unmarshaled from the raw stdout line.
2. Masked (see Section 6).
3. Re-marshaled with `json.MarshalIndent` using 2-space indentation.

This ensures formatting differences (whitespace, key ordering) do not cause
false failures. Golden files are stored in the same pretty-printed format.

### 8.4 Sub-Agent JSON Test

The `with_subagents` agent references `basic` and `with_skill` as sub-agents.
The mock server must enqueue enough responses for the full orchestration
(parent + child agent calls). The exact number depends on the orchestration
flow. The golden file captures the final aggregated JSON envelope.

## 9. Dry-Run Golden File Details

### 9.1 No Mock Server Needed

`--dry-run` exits before making any provider call. No mock server, no API key
required. API keys must be stripped via `stripAPIKeys` to confirm this.

### 9.2 Workdir Masking

The temp directory path injected by `setupSmokeEnv` varies per run. After
capturing stdout, replace the workdir path with `{{WORKDIR}}`. The path to
replace is known because `setupSmokeEnv` returns `configDir` from which the
data root can be derived.

### 9.3 File Content in Dry-Run

The `with_files` agent specifies `files = ["README.md", "docs/**/*.md"]`.
Dry-run output lists matched files. The golden file must reflect what actually
resolves from the test environment. If no files match (because the test runs
in an isolated temp dir with no such files), the golden file records `(none)`
or the empty-state output. If files are seeded into the test workdir, the
golden file records those paths (after masking).

The spec does not prescribe whether to seed files or accept `(none)`. The
implementation must be consistent: whatever the first `-update-golden` run
produces becomes the golden truth.

## 10. Test Helper Functions

The following helper functions are needed in `cmd/golden_test.go` (or a shared
helper file):

### 10.1 `maskDryRunOutput(output string, workdir string) string`

Applies all `--dry-run` masking rules. Accepts the raw stdout and the known
workdir path. Returns masked output.

### 10.2 `maskJSONOutput(output string) string`

Applies all `--json` masking rules. Accepts the raw stdout JSON line. Returns
masked, pretty-printed JSON.

### 10.3 `readGoldenFile(t *testing.T, path string) string`

Reads the golden file from disk. Fails the test with a descriptive message if
the file does not exist or is empty.

### 10.4 `writeGoldenFile(t *testing.T, path string, content string)`

Writes masked output to the golden file path, creating parent directories if
needed.

### 10.5 `diffStrings(expected, actual string) string`

Returns a unified diff string. Use a Go diffing library or implement a simple
line-by-line diff. The diff must be human-readable.

## 11. Edge Cases

| Case | Expected Behavior |
| ---- | ----------------- |
| Golden file missing, compare mode | Test fails. Message: "golden file not found: <path>. Run with -update-golden to generate." |
| Golden file empty (0 bytes), compare mode | Test fails. Message: "golden file is empty: <path>. Run with -update-golden to regenerate." |
| Actual output empty, compare mode | Test fails. The diff shows the golden content as entirely deleted. |
| Update mode, golden dir doesn't exist | Create the directory tree, then write the file. |
| Multiple masking rules overlap | Apply rules in order. Each rule operates on the output of the previous rule. Order is documented in the masking rules table. |
| Agent TOML not found during test | Test fails immediately (pre-condition). This is a test setup error, not a golden file error. |
| Mock server not responding (`--json` tests) | Test fails with timeout or connection error. This is a test infrastructure error. |
| `with_subagents` references agents not seeded | All 5 fixture agents must be seeded for every test case. `setupSmokeEnv` + `SeedFixtureAgents` handles this. |
| Trailing newline differences | Normalize both golden and actual output by trimming trailing whitespace/newlines before comparison. |

## 12. Acceptance Criteria

1. `testdata/golden/dry-run/` contains 5 golden files (one per fixture agent).
2. `testdata/golden/json/` contains 5 golden files (one per fixture agent).
3. `go test ./cmd/ -run TestGolden` passes in compare mode against the
   committed golden files.
4. `go test ./cmd/ -run TestGolden -update-golden` regenerates all 10 golden
   files and the test passes.
5. Intentionally changing `--dry-run` output formatting in `cmd/run.go` causes
   `TestGolden/dry-run/*` to fail with a readable diff.
6. Intentionally changing the `--json` envelope structure in `cmd/run.go`
   causes `TestGolden/json/*` to fail with a readable diff.
7. Non-deterministic values (`workdir`, `duration_ms`) never cause flaky
   failures.
8. `make test` passes with all golden file tests included.
9. Zero network calls — verified by stripping API keys and using mock servers.

## 13. Files to Create or Modify

| File | Action | Purpose |
| ---- | ------ | ------- |
| `cmd/golden_test.go` | Create | All golden file test code |
| `cmd/testdata/golden/dry-run/*.txt` | Create | 5 dry-run golden files |
| `cmd/testdata/golden/json/*.json` | Create | 5 JSON golden files |
| `docs/plans/000_i9n_milestones.md` | Modify | Check off Phase 4 items |
