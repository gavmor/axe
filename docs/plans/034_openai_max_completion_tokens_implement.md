<!-- spec: docs/plans/034_openai_max_completion_tokens_spec.md -->

# Implementation Guide 034: Use `max_completion_tokens` Instead of `max_tokens` for OpenAI Provider

**Spec:** `docs/plans/034_openai_max_completion_tokens_spec.md`  
**Issue:** [#34](https://github.com/jrswab/axe/issues/34)  
**Date:** 2026-03-15

---

## Section 1: Context Summary

OpenAI has deprecated the `max_tokens` request body parameter for newer models (gpt-5-nano, o-series). These models reject requests containing `max_tokens` with a 400 error and require `max_completion_tokens` instead. The `max_completion_tokens` parameter is backwards-compatible with all current OpenAI models, so a single JSON tag change in the `openaiRequest` struct fixes the bug universally without conditional logic. Only the OpenAI provider wire format changes — the Go field name, provider-agnostic `Request` struct, TOML config, CLI output, and all other providers remain untouched.

---

## Section 2: Implementation Checklist

### Red Phase — Update Tests to Expect New Field Name

- [x] **Update `TestOpenAI_Send_OmitsZeroMaxTokens` assertion** — `internal/provider/openai_test.go`: `TestOpenAI_Send_OmitsZeroMaxTokens()` (line 263). Change `raw["max_tokens"]` to `raw["max_completion_tokens"]`. Update the error message string on line 283 from `"expected max_tokens to be omitted when 0"` to `"expected max_completion_tokens to be omitted when 0"`.

- [x] **Update `TestOpenAI_Send_IncludesMaxTokens` assertion** — `internal/provider/openai_test.go`: `TestOpenAI_Send_IncludesMaxTokens()` (line 294). Change `raw["max_tokens"]` to `raw["max_completion_tokens"]`. Update the error message string on line 314 from `"expected max_tokens to be present"` to `"expected max_completion_tokens to be present"`.

- [x] **Run tests — confirm red** — Execute `go test ./internal/provider/ -run "TestOpenAI_Send_OmitsZeroMaxTokens|TestOpenAI_Send_IncludesMaxTokens" -v`. Both tests MUST fail. `TestOpenAI_Send_IncludesMaxTokens` should fail because `raw["max_completion_tokens"]` is not found (the struct still emits `max_tokens`). `TestOpenAI_Send_OmitsZeroMaxTokens` should still pass (the key is absent either way) — this is expected; only `IncludesMaxTokens` needs to go red.

### Green Phase — Fix the Struct Tag

- [x] **Change JSON struct tag on `openaiRequest.MaxTokens`** — `internal/provider/openai.go`: `openaiRequest` struct (line 64). Change the struct tag from `` `json:"max_tokens,omitempty"` `` to `` `json:"max_completion_tokens,omitempty"` ``. Do NOT rename the Go field — it remains `MaxTokens`.

- [x] **Run tests — confirm green** — Execute `go test ./internal/provider/ -run "TestOpenAI_Send_OmitsZeroMaxTokens|TestOpenAI_Send_IncludesMaxTokens" -v`. Both tests MUST pass.

### Full Suite Verification

- [x] **Run full OpenAI provider test suite** — Execute `go test ./internal/provider/ -v`. All tests MUST pass. No other test inspects the `max_tokens` / `max_completion_tokens` JSON key, so no other failures are expected.

- [x] **Run full project test suite** — Execute `go test ./...`. All tests MUST pass. Specifically verify that `cmd/gc_test.go` tests pass — the GC tests (line 368) check for `max_tokens` in the request body, but those tests use the opencode provider path (Anthropic format), not the OpenAI provider, so they are unaffected.
