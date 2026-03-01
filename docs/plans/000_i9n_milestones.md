# Integration Testing Milestones

High-level phases for adding integration/e2e tests to axe.

Goal: fully automated test suite that runs before every release.

Status key: `[ ]` not started · `[-]` in progress · `[x]` done

---

## Phase 1 — Fixture Agents & Test Infrastructure

Set up the foundation everything else builds on.

- [x] Create `testdata/agents/` with minimal TOML configs covering common shapes (basic, with skill, with files, with memory, with sub-agents)
- [x] Create `testdata/skills/` with stub SKILL.md files referenced by fixture agents
- [x] Add helper to build the axe binary into a temp dir for CLI-level tests
- [x] Add helper to override XDG config/data dirs so tests never touch the real user config

---

## Phase 2 — Mock Provider Integration Tests

Test the full `axe run` flow without hitting real APIs.

- [ ] Implement a reusable `httptest` mock server that speaks the OpenAI chat completions shape
- [ ] Extend mock to support Anthropic messages shape
- [ ] Test: single-shot run (no tools) → correct stdout output
- [ ] Test: conversation loop with tool calls → correct round-trips and final output
- [ ] Test: sub-agent orchestration (depth limits, parallel vs sequential)
- [ ] Test: memory append after successful run
- [ ] Test: `--json` output envelope structure and fields
- [ ] Test: timeout handling (slow mock → context deadline exceeded)
- [ ] Test: error mapping (mock returns 401/429/500 → correct exit codes 1/3)

---

## Phase 3 — CLI Smoke Tests

Test the compiled binary end-to-end via shell invocation.

- [ ] `axe version` → prints version string, exit 0
- [ ] `axe config path` → prints valid path, exit 0
- [ ] `axe config init` → creates expected files/dirs, exit 0
- [ ] `axe run nonexistent-agent` → exit 2, stderr contains meaningful error
- [ ] `axe run <fixture> --dry-run` → validates full resolution pipeline output
- [ ] Bad `--model` format → exit 1
- [ ] Missing API key → exit 3
- [ ] Piped stdin content arrives in dry-run output

---

## Phase 4 — Golden File Tests

Catch unintended output regressions.

- [ ] Store expected `--dry-run` output as golden files in `testdata/golden/`
- [ ] Store expected `--json` envelopes as golden files
- [ ] Add test runner that compares actual vs golden, with `-update` flag to refresh
- [ ] Cover at least 3 fixture agents (basic, with skill+files, with sub-agents)

---

## Phase 5 — GitHub Actions CI

Automate the full suite on every push/PR.

- [ ] Add workflow that builds axe and runs `go test ./...` (unit + integration)
- [ ] Run CLI smoke tests as a separate step
- [ ] Fail the pipeline on any test failure
- [ ] Cache Go modules for speed

---

## Phase 6 — Live Provider Tests (Optional)

Guarded by env vars / build tag so they only run when explicitly enabled.

- [ ] Add `//go:build live` tag for live tests
- [ ] Test against at least one real provider (OpenAI or Anthropic)
- [ ] Keep assertions loose (response is non-empty, valid JSON, correct stop reason) to tolerate model variability
- [ ] Skip gracefully with clear message when API key is missing or provider returns 5xx

---

## Notes

- Phases 1-4 must pass with **zero network calls** — all provider interaction is mocked or dry-run.
- Phase 6 is never required to pass for a release; it's a confidence check.
- Each phase can land as its own PR.
