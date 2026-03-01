# Implementation: CLI Smoke Tests (Phase 3)

**Spec:** `docs/plans/010_cli_smoke_tests_spec.md`
**File to create:** `cmd/smoke_test.go`

---

## Checklist

### Helpers & Infrastructure

- [x] Create `cmd/smoke_test.go` with package declaration and imports
- [x] Implement `TestMain` (Req 2.1): `m.Run()` then `testutil.CleanupBinary()` then `os.Exit(code)`
- [x] Implement `setupSmokeEnv` helper (Req 1.2): creates temp XDG dirs, returns `configDir`, `dataDir`, and `env` map with `XDG_CONFIG_HOME`/`XDG_DATA_HOME`
- [x] Implement `stripAPIKeys` helper (Req 1.3): blanks `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `OLLAMA_API_KEY` in env map
- [x] Implement `runAxe` helper (Req 1.1): builds binary, runs `exec.Command` with env/stdin, captures stdout/stderr, extracts exit code

### Smoke Tests

- [x] `TestSmoke_Version` (Req 3.1): `axe version` → exit 0, stdout contains `"axe version "`
- [x] `TestSmoke_ConfigPath` (Req 4.1): `axe config path` → exit 0, stdout equals `configDir`
- [x] `TestSmoke_ConfigInit` (Req 5.1): `axe config init` → exit 0, files created; second run is idempotent (files unchanged)
- [x] `TestSmoke_RunNonexistentAgent` (Req 6.1): `axe run nonexistent-agent` → exit 2, stderr contains `"nonexistent-agent"`
- [x] `TestSmoke_RunDryRun` (Req 7.1): `axe run basic --dry-run` → exit 0, stdout contains `"=== Dry Run ==="` and `"openai/gpt-4o"`
- [x] `TestSmoke_BadModelFormat` (Req 8.1): `axe run basic --model no-slash-here` → exit 1, stderr contains `"invalid model format"`
- [x] `TestSmoke_MissingAPIKey` (Req 9.1): `axe run basic` (no key) → exit 3, stderr contains `"API key"` and `"OPENAI_API_KEY"`
- [x] `TestSmoke_PipedStdinInDryRun` (Req 10.1): `axe run basic --dry-run` with stdin → exit 0, stdout contains piped content, Stdin section does not contain `"(none)"`

### Verification

- [x] Run `make test` — all tests pass (existing + new), zero failures
- [x] Verify no existing files were modified (only `cmd/smoke_test.go` added, `000_i9n_milestones.md` already updated)
