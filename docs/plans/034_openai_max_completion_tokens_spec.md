<!-- issue: https://github.com/jrswab/axe/issues/34 -->

# Spec 034: Use `max_completion_tokens` Instead of `max_tokens` for OpenAI Provider

**Issue:** [#34](https://github.com/jrswab/axe/issues/34) — `max_tokens` is not supported in newer OpenAI models  
**Date:** 2026-03-15

---

## Section 1: Context & Constraints

### Problem

When an agent is configured with `model = "openai/gpt-5-nano"` (or any o-series model) and `max_tokens` is set in the agent's `[params]`, the OpenAI API returns a `400 Bad Request`:

```
bad_request: Unsupported parameter: 'max_tokens' is not supported with this model. Use 'max_completion_tokens' instead.
```

OpenAI has deprecated the `max_tokens` request body parameter for newer models. The replacement is `max_completion_tokens`. The `max_completion_tokens` parameter is backwards-compatible — older OpenAI models that previously accepted `max_tokens` also accept `max_completion_tokens`.

### Codebase Structure

The OpenAI provider is self-contained within `internal/provider/`:

| File | Role |
|------|------|
| `internal/provider/openai.go` | OpenAI provider implementation. Contains `openaiRequest` struct (line 60) with the JSON wire format. |
| `internal/provider/openai_test.go` (994 lines) | Comprehensive test suite with httptest-based tests. |
| `internal/provider/provider.go` | Provider-agnostic `Request` struct with `MaxTokens int` field. |

The wire format struct that serializes to JSON for the OpenAI API:

```go
// openai.go, lines 60-66
type openaiRequest struct {
    Model       string          `json:"model"`
    Messages    []openaiMessage `json:"messages"`
    Temperature *float64        `json:"temperature,omitempty"`
    MaxTokens   *int            `json:"max_tokens,omitempty"`    // ← BUG: deprecated parameter name
    Tools       []openaiToolDef `json:"tools,omitempty"`
}
```

The `MaxTokens` field is populated at lines 235-238:

```go
if req.MaxTokens != 0 {
    mt := req.MaxTokens
    body.MaxTokens = &mt
}
```

### Existing Tests Affected

Two tests directly assert on the JSON field name `max_tokens`:

1. **`TestOpenAI_Send_OmitsZeroMaxTokens`** (line 256) — Checks that `raw["max_tokens"]` is absent when `MaxTokens` is 0.
2. **`TestOpenAI_Send_IncludesMaxTokens`** (line 287) — Checks that `raw["max_tokens"]` is present when `MaxTokens` is 1024.

Both tests inspect the raw JSON body sent to the httptest server. They must be updated to look for `max_completion_tokens` instead of `max_tokens`.

### Decisions Already Made

| Decision | Rationale |
|----------|-----------|
| **Change only the JSON struct tag, not the Go field name** | The Go field `MaxTokens` is internal to the provider package. Renaming it would be cosmetic churn with no behavioral benefit. Only the wire format (JSON tag) matters. |
| **Do NOT change `opencode.go`** | The opencode provider (`ocChatRequest` at line 583) also has `json:"max_tokens,omitempty"`, but it is a separate provider with its own API contract. The user explicitly decided to leave it as-is. |
| **Do NOT change `anthropic.go`** | Anthropic's API still requires `max_tokens`. No change needed. |
| **Do NOT change the TOML config field name** | `max_tokens` in agent TOML files (`internal/agent/agent.go:27`) is a user-facing config key, not an API wire format. It maps to the provider-agnostic `Request.MaxTokens` field. No change needed. |
| **Do NOT change CLI debug output** | `cmd/run.go` prints `max_tokens=%d` in verbose mode. This is a display string, not an API parameter. No change needed. |

### Approaches Ruled Out — Do Not Re-Evaluate

| Approach | Why Rejected |
|----------|-------------|
| **Conditional logic based on model name** (send `max_tokens` for old models, `max_completion_tokens` for new) | Unnecessary complexity. `max_completion_tokens` is backwards-compatible with all current OpenAI models. A single field name works universally. |
| **Updating `opencode.go` simultaneously** | Out of scope per user decision. The opencode provider is a separate concern. |

### Constraints

- **No new public API.** The change is entirely within the `openaiRequest` struct's JSON tag.
- **No new dependencies.** This is a one-line struct tag change.
- **Red/green TDD required.** Update test assertions first (red), then fix the struct tag (green).
- **All other tests must pass unchanged.** The remaining 20+ tests in `openai_test.go` do not inspect the `max_tokens` / `max_completion_tokens` field and are unaffected.

---

## Section 2: Requirements

### R1: OpenAI API Wire Format

When the OpenAI provider serializes a request to JSON for the Chat Completions API, the maximum output tokens parameter MUST be sent as `max_completion_tokens` (not `max_tokens`).

- If the agent config specifies a non-zero `max_tokens` value, the JSON body sent to OpenAI MUST contain the key `"max_completion_tokens"` with the configured integer value.
- If the agent config specifies zero or omits `max_tokens`, the JSON body MUST NOT contain the key `"max_completion_tokens"` (omitempty behavior, unchanged).
- The JSON body MUST NOT contain the key `"max_tokens"` under any circumstances.

### R2: Backwards Compatibility with Older OpenAI Models

The `max_completion_tokens` parameter MUST work with all OpenAI models currently supported by axe, including:

- `gpt-4o` and variants
- `gpt-5-nano` and newer models
- o-series models (o1, o3, etc.)

This is satisfied by the fact that OpenAI's API accepts `max_completion_tokens` as the canonical parameter for all current models.

### R3: No Change to Provider-Agnostic Interface

The provider-agnostic `Request` struct (`internal/provider/provider.go`) MUST NOT change. The field remains `MaxTokens int`. The mapping from `Request.MaxTokens` to the OpenAI-specific JSON field `max_completion_tokens` is an internal concern of the OpenAI provider.

### R4: No Change to User-Facing Config

The TOML agent config field `max_tokens` (under `[params]`) MUST NOT change. Users continue to write `max_tokens = 4096` in their agent TOML files. The translation to `max_completion_tokens` happens at the provider wire-format level only.

### R5: No Change to Other Providers

The Anthropic provider, opencode provider, Gemini provider, and Ollama provider MUST NOT be affected by this change. Each provider controls its own JSON serialization independently.

---

### Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| `max_tokens = 4096` in TOML, model is `openai/gpt-5-nano` | JSON body contains `"max_completion_tokens": 4096`. No error from OpenAI. |
| `max_tokens = 4096` in TOML, model is `openai/gpt-4o` | JSON body contains `"max_completion_tokens": 4096`. No error from OpenAI (backwards-compatible). |
| `max_tokens = 0` (or omitted) in TOML, any OpenAI model | JSON body does NOT contain `"max_completion_tokens"`. Omitempty behavior preserved. |
| `max_tokens = 4096` in TOML, model is `anthropic/claude-sonnet-4-20250514` | Anthropic provider sends `"max_tokens": 4096`. Unaffected by this change. |
| `max_tokens = 4096` in TOML, model is `opencode/...` | Opencode provider sends `"max_tokens": 4096`. Unaffected by this change. |
| GC command (`axe gc`) using an OpenAI model with `max_tokens` set | GC uses the same provider path. JSON body contains `"max_completion_tokens"`. No error. |
