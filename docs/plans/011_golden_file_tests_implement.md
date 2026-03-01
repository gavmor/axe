# 011 — Golden File Tests Implementation Checklist

Spec: `docs/plans/011_golden_file_tests_spec.md`

---

## Phase A — Directory Structure & Update Flag

- [x] Create directory `cmd/testdata/golden/dry-run/`
- [x] Create directory `cmd/testdata/golden/json/`
- [x] Create `cmd/golden_test.go` with package declaration, imports, and the `-update-golden` test flag (via `flag.Bool` in an `init()` or `TestMain`); also support `UPDATE_GOLDEN=1` env var as a fallback

## Phase B — Masking Helpers

- [x] Implement `maskDryRunOutput(output string, workdir string) string` — replaces the absolute workdir path after `Workdir:  ` with `{{WORKDIR}}`; accepts an extensible rule list
- [x] Implement `maskJSONOutput(output string) string` — parses JSON, replaces `duration_ms` value with `"{{DURATION_MS}}"`, re-serializes with `json.MarshalIndent` (2-space indent)
- [x] Write unit tests for `maskDryRunOutput` (verify workdir replaced, other content untouched)
- [x] Write unit tests for `maskJSONOutput` (verify `duration_ms` masked, other fields preserved, output is pretty-printed)

## Phase C — Golden File I/O Helpers

- [x] Implement `readGoldenFile(t *testing.T, path string) string` — reads file; fails test if missing (message: "golden file not found: <path>. Run with -update-golden to generate."); fails if empty (message: "golden file is empty: <path>. Run with -update-golden to regenerate.")
- [x] Implement `writeGoldenFile(t *testing.T, path string, content string)` — creates parent dirs with `os.MkdirAll`, writes content; fails test on error
- [x] Implement `diffStrings(expected, actual string) string` — returns a human-readable line-by-line diff; no external dependency required (use a simple loop or a lightweight approach)

## Phase D — Dry-Run Golden Tests

- [x] Implement `TestGolden/dry-run/basic` — runs `axe run basic --dry-run` via `runAxe`, seeds agents with `SeedFixtureAgents`, strips API keys, masks output, compares or updates golden file `cmd/testdata/golden/dry-run/basic.txt`
- [x] Implement `TestGolden/dry-run/with_skill` — same pattern; must also call `SeedFixtureSkills` so the skill file resolves
- [x] Implement `TestGolden/dry-run/with_files` — same pattern; file globs resolve to `(none)` in isolated env (no files seeded)
- [x] Implement `TestGolden/dry-run/with_memory` — same pattern; memory enabled but no entries → shows `(none)` under `--- Memory ---`
- [x] Implement `TestGolden/dry-run/with_subagents` — same pattern; all 5 agents seeded so sub-agent names resolve
- [x] Run dry-run tests with `-update-golden` to generate the 5 `.txt` golden files
- [x] Run dry-run tests in compare mode to confirm they pass against the generated golden files

## Phase E — JSON Golden Tests

- [x] Implement `TestGolden/json/basic` — starts `NewMockLLMServer` with a single `OpenAIResponse("Hello from mock.")`, sets `AXE_OPENAI_BASE_URL` + `OPENAI_API_KEY=test-key` in env, runs `axe run basic --json` via `runAxe`, masks output, compares or updates `cmd/testdata/golden/json/basic.json`
- [x] Implement `TestGolden/json/with_skill` — same mock setup (OpenAI provider per agent config), single-shot response
- [x] Implement `TestGolden/json/with_files` — same mock setup, single-shot response
- [x] Implement `TestGolden/json/with_memory` — same mock setup, single-shot response
- [x] Implement `TestGolden/json/with_subagents` — uses `AnthropicToolUseResponse` + child responses + parent final response; enqueue 4 mock responses (parent tool_use → 2 child responses → parent final); set `AXE_ANTHROPIC_BASE_URL` + `ANTHROPIC_API_KEY=test-key` and `AXE_OPENAI_BASE_URL` + `OPENAI_API_KEY=test-key` (children use OpenAI)
- [x] Run JSON tests with `-update-golden` to generate the 5 `.json` golden files
- [x] Run JSON tests in compare mode to confirm they pass against the generated golden files

## Phase F — Full Validation

- [x] Run `make test` to verify all golden file tests pass alongside existing tests
- [x] Verify diff output: temporarily alter a golden file, run tests in compare mode, confirm failure message includes the golden file path, a readable diff, and `-update-golden` instructions
- [x] Verify empty golden file detection: truncate a golden file to 0 bytes, confirm the test fails with the expected staleness message
- [x] Verify missing golden file detection: delete a golden file, confirm the test fails with the expected "not found" message
- [x] Restore all golden files to correct state after manual checks

## Phase G — Milestone Bookkeeping

- [x] Update `docs/plans/000_i9n_milestones.md` — check off all 4 Phase 4 items (`[x]`)
