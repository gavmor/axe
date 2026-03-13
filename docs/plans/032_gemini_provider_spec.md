# Specification: Google Gemini Provider Support

**Status:** Draft
**Version:** 1.0
**Created:** 2026-03-12
**Scope:** New Gemini provider implementation, registry integration, config integration
**Issue:** https://github.com/jrswab/axe/issues/23

---

## 1. Context & Constraints

### 1.1 Associated Milestone

From `docs/plans/000_milestones.md`, M4 (Multi-Provider Support):

> Support any provider/model from models.dev.
>
> - [X] Provider abstraction interface
> - [X] Anthropic provider
> - [X] OpenAI provider
> - [X] Ollama / local provider
> - [X] API key config (env vars, config file, or both)

Gemini is a new provider being added to the existing M4 infrastructure. All M4 infrastructure (provider interface, registry, config resolution) is complete and stable.

### 1.2 Codebase Structure

The provider system consists of:

- **`internal/provider/provider.go`** — `Provider` interface (`Send(ctx, *Request) (*Response, error)`), shared types (`Request`, `Response`, `Message`, `ToolCall`, `ToolResult`, `ProviderError`, `ErrorCategory`). These are frozen — no modifications permitted.
- **`internal/provider/registry.go`** — `supportedProviders` map and `New(providerName, apiKey, baseURL)` factory function with a switch statement dispatching to provider constructors.
- **`internal/provider/anthropic.go`**, **`openai.go`**, **`ollama.go`**, **`opencode.go`** — Each provider is a standalone file with its own unexported wire types, message conversion functions, `Send` method, and error mapping. No shared code between providers.
- **`internal/config/config.go`** — `knownAPIKeyEnvVars` map for canonical env var names, `ResolveAPIKey()` and `ResolveBaseURL()` methods.
- **`internal/testutil/mockserver.go`** — `MockLLMServer` with per-provider response helpers (`AnthropicResponse`, `OpenAIResponse`, etc.).
- **`cmd/run.go`** — Uses the factory generically; no provider-specific code. Adding a new provider to the registry and config requires zero changes to `cmd/run.go`.

### 1.3 Decisions Already Made

These decisions were confirmed during planning and are binding:

1. **Provider name is `"google"`** — Matches the models.dev format (`google/gemini-2.0-flash`). The alternative `"gemini"` was rejected because it would produce the redundant model string `gemini/gemini-2.0-flash`.
2. **API key env var is `GEMINI_API_KEY`** — As specified in the issue. This is a non-standard mapping (provider name `"google"` does not match the env var prefix `GEMINI`), so it must be added to the `knownAPIKeyEnvVars` map explicitly.
3. **API version is `v1beta`** — The current stable REST endpoint used in all official Gemini documentation. The `v1` endpoint exists but does not fully support function calling.
4. **Authentication uses the `x-goog-api-key` header** — Consistent with how other providers use headers for auth. The query parameter alternative (`key=`) was rejected because it leaks the API key in server access logs.
5. **Default base URL is `https://generativelanguage.googleapis.com`** — The official Gemini API host. The API version and model path are appended by the provider, not included in the base URL.
6. **No new dependencies** — Pure stdlib (`net/http`, `encoding/json`). Consistent with all existing providers.
7. **Separate implementation** — No shared code with OpenAI or other providers, consistent with the Ollama precedent and the project's design philosophy.

### 1.4 Approaches Ruled Out

- **Reusing the OpenAI provider with a custom base URL** — The Gemini REST API has a fundamentally different request/response format (contents/parts model, system_instruction as a top-level field, functionCall/functionResponse in parts, model name in URL path). It cannot be adapted to the OpenAI Chat Completions wire format.
- **Using the Gemini OpenAI-compatible endpoint** — Google offers an OpenAI-compatible endpoint, but it has limitations with tool calling and does not expose Gemini-specific features. Users who want this can already point the OpenAI provider at it via `base_url` override.
- **Using a Go SDK** — The project constraint is stdlib-only HTTP. No LLM SDK dependencies.

### 1.5 Gemini API Reference

**Endpoint:** `POST {baseURL}/v1beta/models/{model}:generateContent`

**Authentication:** `x-goog-api-key: {api_key}` header

**Request format:**
```json
{
  "system_instruction": {
    "parts": [{"text": "system prompt"}]
  },
  "contents": [
    {
      "role": "user",
      "parts": [{"text": "user message"}]
    },
    {
      "role": "model",
      "parts": [{"text": "assistant response"}]
    }
  ],
  "tools": [
    {
      "functionDeclarations": [
        {
          "name": "tool_name",
          "description": "tool description",
          "parameters": {
            "type": "object",
            "properties": { ... },
            "required": [...]
          }
        }
      ]
    }
  ],
  "generationConfig": {
    "temperature": 0.7,
    "maxOutputTokens": 4096
  }
}
```

**Response format:**
```json
{
  "candidates": [
    {
      "content": {
        "role": "model",
        "parts": [
          {"text": "response text"},
          {"functionCall": {"name": "tool_name", "args": {...}}}
        ]
      },
      "finishReason": "STOP"
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 10,
    "candidatesTokenCount": 5,
    "totalTokenCount": 15
  }
}
```

**Error response format:**
```json
{
  "error": {
    "code": 400,
    "message": "error description",
    "status": "INVALID_ARGUMENT"
  }
}
```

**Key differences from OpenAI/Anthropic:**
- Roles are `"user"` and `"model"` (not `"assistant"`)
- System prompt is a top-level `system_instruction` field, not a message in `contents`
- Messages use `contents[].parts[]` structure (not flat `content` string)
- Tool calls appear as `functionCall` parts within `candidates[].content.parts[]`
- Tool results are sent as `functionResponse` parts with `role: "user"`
- The model name is part of the URL path, not a body field
- No explicit tool call IDs in the API; IDs must be synthesized
- The `generationConfig` object wraps temperature and token limits

### 1.6 Constraints

- **`Provider` interface is frozen.** `Send(ctx context.Context, req *Request) (*Response, error)` — no modifications.
- **`Request`, `Response`, `Message`, `ProviderError`, `ErrorCategory` types are frozen.** New providers map their API-specific formats to/from these existing types.
- **No new entries in `go.mod`.** Only stdlib packages (`net/http`, `encoding/json`, `bytes`, `context`, `fmt`, `io`).
- **No retry logic.** Failed requests fail immediately.
- **No streaming.** Complete response received before returning.
- **No caching** of responses, configs, or provider instances.
- **HTTP client must not follow redirects.** Use `CheckRedirect` returning `http.ErrUseLastResponse`.
- **Provider name matching is case-sensitive.** `"Google"` is not valid; only `"google"`.
- **All tests use `httptest.NewServer`.** No real API calls.
- **Tests live in the same package** (e.g., `package provider`).

---

## 2. Requirements

### 2.1 Gemini Provider

**Requirement 1.1:** A new provider named `"google"` must be added that satisfies the existing `Provider` interface.

**Requirement 1.2:** The provider must require a non-empty API key. If the API key is empty, the constructor must return an error: `API key is required`.

**Requirement 1.3:** The provider must have a configurable base URL with a default of `https://generativelanguage.googleapis.com`. A functional option must allow overriding the base URL (used for testing with `httptest`).

**Requirement 1.4:** The `Send` method must make an HTTP POST request to `{baseURL}/v1beta/models/{model}:generateContent`, where `{model}` is the model name from `Request.Model`.

**Requirement 1.5:** The following HTTP headers must be set on every request:

| Header | Value |
|--------|-------|
| `x-goog-api-key` | `{apiKey}` |
| `Content-Type` | `application/json` |

**Requirement 1.6:** System prompt handling: If `Request.System` is non-empty, it must be sent as a top-level `system_instruction` field with the structure `{"parts": [{"text": "<system_prompt>"}]}`. If `Request.System` is empty, the `system_instruction` field must be omitted from the request body entirely.

**Requirement 1.7:** Message role mapping:

| Provider Role | Gemini Role |
|---------------|-------------|
| `"user"` | `"user"` |
| `"assistant"` | `"model"` |

Messages with `role: "user"` are sent as `role: "user"`. Messages with `role: "assistant"` are sent as `role: "model"`.

**Requirement 1.8:** Standard text messages must be sent as `contents[].parts[].text`.

**Requirement 1.9:** Assistant messages containing tool calls (`Message.ToolCalls` is non-empty) must be sent as `role: "model"` with `functionCall` parts. Each tool call becomes a part with `{"functionCall": {"name": "<name>", "args": {<arguments>}}}`.

**Requirement 1.10:** Tool result messages (`Message.Role == "tool"` with `Message.ToolResults` non-empty) must be sent as `role: "user"` with `functionResponse` parts. Each tool result becomes a part with `{"functionResponse": {"name": "<name>", "response": {"result": "<content>"}}}`. The tool result's `CallID` is not sent to the Gemini API (it does not support tool call IDs), but the tool name must be resolved. Since the Gemini API does not provide tool call IDs, the provider must track the mapping from `CallID` to tool name using the preceding assistant message's tool calls in the conversation history.

**Requirement 1.11:** Tool definitions must be sent as `tools[].functionDeclarations[]`. Each tool becomes a function declaration with `name`, `description`, and `parameters` (JSON Schema object with `type`, `properties`, and `required`). If `Request.Tools` is nil or empty, the `tools` field must be omitted from the request body.

**Requirement 1.12:** The `generationConfig` object must be included when `Request.Temperature` is non-zero OR `Request.MaxTokens` is non-zero:
- `temperature` is included only when `Request.Temperature != 0`
- `maxOutputTokens` is included only when `Request.MaxTokens != 0`
- If both are zero, the entire `generationConfig` field must be omitted.

**Requirement 1.13:** The model name must NOT be included in the request body. It is only used in the URL path.

**Requirement 1.14:** Response parsing must extract:

| Gemini Response Field | Maps To |
|-----------------------|---------|
| `candidates[0].content.parts[].text` (concatenated) | `Response.Content` |
| `candidates[0].content.parts[].functionCall` | `Response.ToolCalls` |
| `usageMetadata.promptTokenCount` | `Response.InputTokens` |
| `usageMetadata.candidatesTokenCount` | `Response.OutputTokens` |
| `candidates[0].finishReason` | `Response.StopReason` |

The model name in the response is not returned by the Gemini API in the response body. `Response.Model` must be set to the model name from the request (`Request.Model`).

**Requirement 1.15:** Tool call ID synthesis: The Gemini API does not return tool call IDs. The provider must generate synthetic IDs using the pattern `gemini_0`, `gemini_1`, etc. (sequential, zero-indexed per response). This is consistent with how the Ollama provider generates `ollama_0`, `ollama_1`.

**Requirement 1.16:** If the response `candidates` array is empty or missing, return a `ProviderError` with category `ErrCategoryServer` and message: `response contains no candidates`.

**Requirement 1.17:** HTTP error mapping:

| HTTP Status | Error Category |
|-------------|---------------|
| 401 | `ErrCategoryAuth` |
| 403 | `ErrCategoryAuth` |
| 400 | `ErrCategoryBadRequest` |
| 404 | `ErrCategoryBadRequest` |
| 429 | `ErrCategoryRateLimit` |
| 500, 502, 503 | `ErrCategoryServer` |
| 529 | `ErrCategoryOverloaded` |
| All other non-2xx | `ErrCategoryServer` |

**Requirement 1.18:** For non-2xx responses, the provider must attempt to parse the Gemini error response body:
```json
{"error": {"code": <int>, "message": "<string>", "status": "<string>"}}
```
If the body parses successfully and `error.message` is non-empty, use it in `ProviderError.Message`. If parsing fails, use the raw HTTP status text.

**Requirement 1.19:** The `Send` method must respect the `context.Context` passed to it. If the context is cancelled or its deadline is exceeded, the HTTP request must be aborted and a `ProviderError` with category `ErrCategoryTimeout` must be returned.

**Requirement 1.20:** The provider must not retry failed requests.

**Requirement 1.21:** The HTTP client must not follow redirects. Use a custom `http.Client` with `CheckRedirect` set to return `http.ErrUseLastResponse`.

**Requirement 1.22:** The default base URL must be a named constant, not a magic string.

### 2.2 Provider Registry Integration

**Requirement 2.1:** The `"google"` provider must be added to the `supportedProviders` map so that `Supported("google")` returns `true`.

**Requirement 2.2:** The `New` factory function must dispatch `"google"` to the Gemini constructor, passing the API key and base URL option (when non-empty).

**Requirement 2.3:** The error message for unsupported providers must be updated to include `"google"` in the list of supported providers.

### 2.3 Config Integration

**Requirement 3.1:** The `knownAPIKeyEnvVars` map must include `"google": "GEMINI_API_KEY"`. This is a non-standard mapping — the provider name `"google"` does not match the env var prefix `"GEMINI"` — so it must be explicitly registered.

**Requirement 3.2:** The base URL env var follows the existing convention: `AXE_GOOGLE_BASE_URL`. No code changes are needed for this — the `ResolveBaseURL` method already constructs `AXE_<PROVIDER_UPPER>_BASE_URL` dynamically.

**Requirement 3.3:** The `ResolveAPIKey` method must work correctly for the `"google"` provider: check `GEMINI_API_KEY` env var first, then fall back to `providers.google.api_key` in `config.toml`, then return empty string.

### 2.4 Mock Server Helpers

**Requirement 4.1:** A `GeminiResponse(text string) MockLLMResponse` helper must be added to `testutil/mockserver.go` that returns a valid Gemini `generateContent` response with the given text.

**Requirement 4.2:** A `GeminiToolCallResponse(text string, toolCalls []MockToolCall) MockLLMResponse` helper must be added that returns a Gemini response containing both text and `functionCall` parts.

**Requirement 4.3:** A `GeminiErrorResponse(statusCode int, message string) MockLLMResponse` helper must be added that returns a Gemini error response shape.

---

## 3. Edge Cases

### 3.1 Gemini Provider

| Scenario | Behavior |
|----------|----------|
| Empty API key | Constructor returns error: `API key is required` |
| Empty `Request.System` | `system_instruction` field omitted from request body |
| Empty `Request.Messages` | `contents` is an empty array (sent as-is; API will return an error) |
| `Request.Temperature` is 0 and `Request.MaxTokens` is 0 | `generationConfig` field omitted entirely |
| `Request.Temperature` is non-zero, `Request.MaxTokens` is 0 | `generationConfig` present with only `temperature` |
| `Request.MaxTokens` is non-zero, `Request.Temperature` is 0 | `generationConfig` present with only `maxOutputTokens` |
| Response has empty `candidates` array | `ProviderError` with `ErrCategoryServer`, message: `response contains no candidates` |
| Response has `candidates` but no `content.parts` | `Response.Content` is empty string, `Response.ToolCalls` is nil (not an error — the model may return empty content) |
| Response has mixed text and functionCall parts | Text parts concatenated into `Response.Content`, functionCall parts become `Response.ToolCalls` |
| Response has only functionCall parts (no text) | `Response.Content` is empty string, `Response.ToolCalls` populated |
| `usageMetadata` missing or zero | `Response.InputTokens` and `Response.OutputTokens` are 0 (not an error) |
| `finishReason` missing | `Response.StopReason` is empty string |
| Model name contains special characters | Sent as-is in URL path; URL encoding is the caller's responsibility if needed |
| Non-JSON error response body | `ProviderError.Message` falls back to HTTP status text |
| Context cancelled before response | `ProviderError` with `ErrCategoryTimeout` |
| Context deadline exceeded | `ProviderError` with `ErrCategoryTimeout` |
| Custom base URL (e.g., for proxy) | Fully supported via `base_url` config or `AXE_GOOGLE_BASE_URL` env var |
| Tool results without matching tool call in history | The `functionResponse.name` field must still be populated; if the name cannot be resolved from the conversation history, use the `CallID` as a fallback name |

### 3.2 Registry

| Scenario | Behavior |
|----------|----------|
| `New("google", "key", "")` | Returns valid Gemini provider, no error |
| `New("google", "", "")` | Returns error: `API key is required` |
| `New("google", "key", "http://custom:8080")` | Returns Gemini provider with custom base URL |
| `Supported("google")` | Returns `true` |
| `Supported("Google")` | Returns `false` (case-sensitive) |
| `Supported("gemini")` | Returns `false` (not a valid provider name) |
| Unsupported provider error message | Must list `google` alongside `anthropic`, `openai`, `ollama`, `opencode` |

### 3.3 Config

| Scenario | Behavior |
|----------|----------|
| `GEMINI_API_KEY` env var set | Used as API key for `"google"` provider |
| `GEMINI_API_KEY` not set, `providers.google.api_key` in config | Config value used |
| Neither set | Empty string returned; `cmd/run.go` returns exit code 3 with message about `GEMINI_API_KEY` |
| `AXE_GOOGLE_BASE_URL` env var set | Used as base URL override |
| `APIKeyEnvVar("google")` | Returns `"GEMINI_API_KEY"` (from `knownAPIKeyEnvVars`, not the default convention which would produce `GOOGLE_API_KEY`) |

---

## 4. Testing Requirements

### 4.1 Test Conventions

Tests must follow the patterns established in the existing codebase:

- **Package-level tests:** Tests live in the same package (`package provider`, `package config`)
- **Standard library only:** Use `testing` package. No test frameworks.
- **HTTP tests:** Use `httptest.NewServer` for all HTTP interactions. No real API calls.
- **Descriptive names:** `TestFunctionName_Scenario` with underscores.
- **Test real code, not mocks.** Tests must call actual functions with real I/O.
- **Red/green TDD:** Write failing test first, then implement.

### 4.2 Gemini Provider Tests

| Test Name | Description |
|-----------|-------------|
| `TestNewGemini_EmptyAPIKey` | Empty string returns error containing `API key is required` |
| `TestNewGemini_ValidAPIKey` | Non-empty string returns `*Gemini` with no error |
| `TestGemini_Send_Success` | Valid response; verify all `Response` fields (content, model, tokens, stop reason) |
| `TestGemini_Send_RequestFormat` | Inspect method (POST), URL path (`/v1beta/models/{model}:generateContent`), headers (`x-goog-api-key`, `Content-Type`), body JSON structure |
| `TestGemini_Send_SystemInstruction` | System prompt sent as `system_instruction.parts[0].text`, not in `contents` |
| `TestGemini_Send_OmitsEmptySystem` | Empty system -> no `system_instruction` field in body |
| `TestGemini_Send_OmitsZeroTemperature` | Temperature 0 -> no `temperature` in `generationConfig` |
| `TestGemini_Send_OmitsGenerationConfig` | Both temperature and max_tokens are 0 -> no `generationConfig` field |
| `TestGemini_Send_IncludesMaxTokens` | MaxTokens non-zero -> `generationConfig.maxOutputTokens` present |
| `TestGemini_Send_IncludesTemperature` | Temperature non-zero -> `generationConfig.temperature` present |
| `TestGemini_Send_RoleMapping` | Assistant messages sent as `role: "model"` |
| `TestGemini_Send_ToolCallResponse` | Response with `functionCall` parts; verify `ToolCall` parsing and synthetic IDs |
| `TestGemini_Send_ToolDefinitions` | Verify tool definitions sent as `functionDeclarations` |
| `TestGemini_Send_ToolResults` | Verify function results sent as `functionResponse` parts with `role: "user"` |
| `TestGemini_Send_AuthError` | 401 -> `ErrCategoryAuth` |
| `TestGemini_Send_ForbiddenError` | 403 -> `ErrCategoryAuth` |
| `TestGemini_Send_NotFoundError` | 404 -> `ErrCategoryBadRequest` |
| `TestGemini_Send_RateLimitError` | 429 -> `ErrCategoryRateLimit` |
| `TestGemini_Send_ServerError` | 500 -> `ErrCategoryServer` |
| `TestGemini_Send_Timeout` | Context deadline exceeded -> `ErrCategoryTimeout` |
| `TestGemini_Send_EmptyCandidates` | Empty `candidates` array -> `ProviderError` with `ErrCategoryServer` |
| `TestGemini_Send_ErrorResponseParsing` | Parse Gemini error JSON; verify `ProviderError.Message` |
| `TestGemini_Send_UnparseableErrorBody` | Non-JSON error body -> fallback to HTTP status text |
| `TestGemini_Send_MixedContentAndToolCalls` | Response with both text and functionCall parts |
| `TestGemini_Send_ZeroTokenCounts` | Missing or zero usage metadata -> tokens are 0, no error |

### 4.3 Registry Tests

| Test Name | Description |
|-----------|-------------|
| `TestNew_Google` | `New("google", "test-key", "")` returns non-nil provider, no error |
| `TestNew_GoogleWithBaseURL` | `New("google", "test-key", "http://custom:8080")` succeeds |
| `TestNew_GoogleMissingAPIKey` | `New("google", "", "")` returns error about missing API key |
| `TestSupported_Google` | `Supported("google")` returns `true` |
| Update `TestSupported_KnownProviders` | Include `"google"` in the list |
| Update `TestNew_UnsupportedProvider` | Error message must include `"google"` |
| Update `TestNew_UnsupportedProvider_ErrorMessage` | Error message must list all providers including `"google"` |

### 4.4 Config Tests

| Test Name | Description |
|-----------|-------------|
| `TestAPIKeyEnvVar_Google` | `APIKeyEnvVar("google")` returns `"GEMINI_API_KEY"` |
| `TestResolveAPIKey_Google` | Env var `GEMINI_API_KEY` takes precedence; falls back to config; returns empty when neither set |

### 4.5 Running Tests

All tests must pass when run with:

```bash
make test
```

No test may make real HTTP requests to external APIs.

---

## 5. Files Affected

| File | Action | Description |
|------|--------|-------------|
| `internal/provider/gemini.go` | CREATE | Gemini provider implementation |
| `internal/provider/gemini_test.go` | CREATE | Gemini provider tests |
| `internal/provider/registry.go` | MODIFY | Add `"google"` to `supportedProviders` and `New()` switch |
| `internal/provider/registry_test.go` | MODIFY | Add Google tests, update error message assertions |
| `internal/config/config.go` | MODIFY | Add `"google": "GEMINI_API_KEY"` to `knownAPIKeyEnvVars` |
| `internal/config/config_test.go` | MODIFY | Add Google-specific API key resolution tests |
| `internal/testutil/mockserver.go` | MODIFY | Add `GeminiResponse`, `GeminiToolCallResponse`, `GeminiErrorResponse` helpers |

Files NOT modified:
- `internal/provider/provider.go` — interface and types are frozen
- `internal/provider/anthropic.go`, `openai.go`, `ollama.go`, `opencode.go` — existing providers unchanged
- `cmd/run.go` — generic factory usage requires no changes
- `go.mod` — no new dependencies

---

## 6. Acceptance Criteria

| Criterion | Verification |
|-----------|-------------|
| `provider.New("google", key, "")` returns a working Gemini provider | Registry test |
| `provider.Supported("google")` returns `true` | Registry test |
| `Gemini.Send` makes correct HTTP POST to `/v1beta/models/{model}:generateContent` | Request format test |
| `x-goog-api-key` header is sent with the API key | Request format test |
| System prompt sent as `system_instruction`, not in `contents` | System instruction test |
| System prompt omitted when empty | Omit empty system test |
| Messages use `role: "model"` for assistant messages | Role mapping test |
| Tool definitions sent as `functionDeclarations` | Tool definitions test |
| Tool call responses parsed with synthetic IDs (`gemini_0`, etc.) | Tool call response test |
| Tool results sent as `functionResponse` parts | Tool results test |
| `generationConfig` omitted when both temperature and max_tokens are 0 | Generation config test |
| HTTP 401 -> `ErrCategoryAuth` | Auth error test |
| HTTP 403 -> `ErrCategoryAuth` | Forbidden error test |
| HTTP 404 -> `ErrCategoryBadRequest` | Not found error test |
| HTTP 429 -> `ErrCategoryRateLimit` | Rate limit error test |
| HTTP 500 -> `ErrCategoryServer` | Server error test |
| Context cancellation -> `ErrCategoryTimeout` | Timeout test |
| Empty candidates -> `ProviderError` | Empty candidates test |
| `APIKeyEnvVar("google")` returns `"GEMINI_API_KEY"` | Config test |
| `GEMINI_API_KEY` env var resolved correctly | Config resolution test |
| No new entries in `go.mod` | Manual verification |
| All tests pass with `make test` | CI |

---

## 7. Out of Scope

1. Streaming output (`streamGenerateContent` endpoint)
2. Retry logic or exponential backoff
3. Response caching
4. Multi-modal content (images, audio, video)
5. Gemini-specific features (grounding, code execution, Google Search tool)
6. Token cost estimation
7. Model validation (checking if a model name exists)
8. Updating `axe config init` scaffold to include `[providers.google]` section
9. Integration tests against the real Gemini API
10. The `v1` API endpoint (only `v1beta` is supported)
