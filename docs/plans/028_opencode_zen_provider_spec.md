# Specification: OpenCode Zen Provider

**Status:** Draft
**Version:** 1.0
**Created:** 2026-03-08
**Scope:** New `opencode` provider for OpenCode Zen API key support

---

## 1. Purpose

Add an `opencode` provider to Axe so that users can authenticate with a single OpenCode Zen API key
to access Claude, GPT, Gemini, and OpenAI-compatible models (Kimi, MiniMax, GLM, Qwen, etc.).

OpenCode Zen is an AI gateway at `https://opencode.ai/zen` that proxies requests to multiple
underlying providers. It exposes three distinct wire formats depending on the model family, and
uses a single Bearer token for authentication across all of them.

After this milestone, users configure agents as:

```toml
# $XDG_CONFIG_HOME/axe/agents/myagent.toml
model = "opencode/claude-sonnet-4-6"
```

and supply one API key:

```toml
# $XDG_CONFIG_HOME/axe/config.toml
[providers.opencode]
api_key = "zen-key-here"
```

or via environment variable: `OPENCODE_API_KEY=zen-key-here axe run myagent`

---

## 2. Background and Design Decisions

The following decisions are binding for implementation:

1. **No new external dependencies.** All HTTP calls use stdlib `net/http` and `encoding/json`, consistent
   with Anthropic, OpenAI, and Ollama providers. `go.mod` must not gain new direct dependencies.

2. **No shared code with existing providers.** The `opencode` provider is a fully independent
   implementation in `internal/provider/opencode.go`. It does not call `NewAnthropic`,
   `NewOpenAI`, or any other existing provider constructor.

3. **Endpoint routing by model name prefix.** The provider determines which Zen endpoint and wire
   format to use by inspecting the model name (post-slash portion of the agent `model` field):
   - `claude-*` → Anthropic Messages format at `<baseURL>/v1/messages`
   - `gpt-*` → OpenAI Responses format at `<baseURL>/v1/responses`
   - everything else → OpenAI Chat Completions format at `<baseURL>/v1/chat/completions`

   Prefix matching is case-sensitive. Only exact lowercase prefix matches apply.

4. **The Gemini Google API format is out of scope.** The Gemini endpoint (`/zen/v1/models/<model-id>`)
   uses the Google AI SDK wire format, which is structurally different from the three other families.
   Gemini model support requires a future milestone. If a user specifies a `gemini-*` model, the
   provider falls through to the OpenAI Chat Completions format. This will produce a Zen API error
   at runtime. This is explicitly documented as a known limitation.

5. **The `Provider` interface is not modified.** The new provider implements:
   ```go
   type Provider interface {
       Send(ctx context.Context, req *Request) (*Response, error)
   }
   ```

6. **The `Request`, `Response`, `Message`, `ProviderError`, and `ErrorCategory` types are not
   modified.** The new provider maps Zen-specific formats to and from these existing types.

7. **No retry logic.** Failed requests fail immediately, consistent with all other providers.

8. **No redirect following.** The HTTP client uses `CheckRedirect: func(...) error { return http.ErrUseLastResponse }`.

---

## 3. Zen Endpoint Reference

The following is the authoritative mapping of Zen model IDs to endpoints and wire formats
as of the spec creation date. Model IDs are used verbatim in the HTTP request body `"model"` field.

| Model Family         | Zen Model ID Examples                                  | Endpoint                               | Wire Format          |
|---------------------|-------------------------------------------------------|----------------------------------------|----------------------|
| Claude (Anthropic)  | `claude-sonnet-4-6`, `claude-opus-4-6`, `claude-haiku-4-5`, `claude-3-5-haiku` | `<baseURL>/v1/messages`    | Anthropic Messages   |
| GPT (OpenAI)        | `gpt-5`, `gpt-5-codex`, `gpt-5.3-codex`, `gpt-5.4-pro` | `<baseURL>/v1/responses`  | OpenAI Responses     |
| OpenAI-Compatible   | `kimi-k2`, `kimi-k2.5`, `minimax-m2.5`, `glm-5`, `glm-4.7`, `qwen3-coder`, `big-pickle` | `<baseURL>/v1/chat/completions` | OpenAI Chat Completions |
| Gemini (Google)     | `gemini-3-pro`, `gemini-3-flash`, `gemini-3.1-pro`    | `<baseURL>/v1/models/<model-id>` | Google AI (out of scope) |

The Zen API base URL is `https://opencode.ai/zen`. All endpoints are relative to this base.

Authentication for all endpoints uses: `Authorization: Bearer <zen-api-key>`

---

## 4. Requirements

### 4.1 Config Package (`internal/config/config.go`)

**Requirement 1.1:** Add `"opencode"` to the `knownAPIKeyEnvVars` map:

```go
var knownAPIKeyEnvVars = map[string]string{
    "anthropic": "ANTHROPIC_API_KEY",
    "openai":    "OPENAI_API_KEY",
    "opencode":  "OPENCODE_API_KEY",
}
```

This causes `ResolveAPIKey("opencode")` to check `OPENCODE_API_KEY` before falling back to
the config file, and causes `APIKeyEnvVar("opencode")` to return `"OPENCODE_API_KEY"`.

**Requirement 1.2:** The base URL env var for the `opencode` provider follows the existing convention
automatically (no code change required): `AXE_OPENCODE_BASE_URL`.

### 4.2 Provider Registry (`internal/provider/registry.go`)

**Requirement 2.1:** Add `"opencode": true` to the `supportedProviders` map:

```go
var supportedProviders = map[string]bool{
    "anthropic": true,
    "openai":    true,
    "ollama":    true,
    "opencode":  true,
}
```

**Requirement 2.2:** Add a `case "opencode"` branch in the `New()` factory function that
calls `NewOpenCode(apiKey, opts...)`. If `baseURL` is non-empty, pass it as a functional
option using `WithOpenCodeBaseURL(baseURL)`.

**Requirement 2.3:** Update the error message for the `default` case in `New()` to include
`opencode` in the list of supported providers:

```
unsupported provider "<name>": supported providers are anthropic, openai, ollama, opencode
```

**Requirement 2.4:** The `opencode` provider requires an API key. The existing API key check
in `cmd/run.go` (which skips the check only for `"ollama"`) already enforces this without
any modification to `cmd/run.go`.

### 4.3 OpenCode Provider (`internal/provider/opencode.go`)

**Requirement 3.1:** Implement an `OpenCode` struct that satisfies the `Provider` interface.

**Requirement 3.2:** Define the `OpenCode` struct:

| Field      | Go Type        | Default                         | Description              |
|------------|----------------|---------------------------------|--------------------------|
| `apiKey`   | `string`       | *(required)*                    | Zen API key              |
| `baseURL`  | `string`       | `"https://opencode.ai/zen"`     | Zen gateway base URL     |
| `client`   | `*http.Client` | Client with `CheckRedirect` set | HTTP client              |

The default base URL `"https://opencode.ai/zen"` must be defined as a named constant:

```go
const defaultOpenCodeBaseURL = "https://opencode.ai/zen"
```

**Requirement 3.3:** Constructor function signature:

```go
func NewOpenCode(apiKey string, opts ...OpenCodeOption) (*OpenCode, error)
```

The constructor must return an error if `apiKey` is an empty string: `API key is required`.

**Requirement 3.4:** Functional option for base URL override:

```go
type OpenCodeOption func(*OpenCode)
func WithOpenCodeBaseURL(url string) OpenCodeOption
```

**Requirement 3.5:** The HTTP client on the `OpenCode` struct must not follow redirects. Use:

```go
&http.Client{
    CheckRedirect: func(req *http.Request, via []*http.Request) error {
        return http.ErrUseLastResponse
    },
}
```

**Requirement 3.6:** The `Send` method must inspect the `req.Model` field (the post-slash model name
as received from the caller) to determine which endpoint and wire format to use:

| Condition                         | Endpoint                          | Wire Format              |
|----------------------------------|-----------------------------------|--------------------------|
| `req.Model` has prefix `claude-` | `<baseURL>/v1/messages`           | Anthropic Messages       |
| `req.Model` has prefix `gpt-`    | `<baseURL>/v1/responses`          | OpenAI Responses         |
| All other model names            | `<baseURL>/v1/chat/completions`   | OpenAI Chat Completions  |

Prefix matching uses `strings.HasPrefix`. Matching is case-sensitive. Only the lowercase prefixes
`"claude-"` and `"gpt-"` are matched.

**Requirement 3.7:** Authentication header for all three endpoints:

```
Authorization: Bearer <apiKey>
```

Additionally, for Anthropic Messages endpoint only, send:

```
anthropic-version: 2023-06-01
```

**Requirement 3.8:** `Content-Type: application/json` must be set on all requests.

---

#### 4.3.1 Anthropic Messages Format (`claude-*` models)

**Requirement 3.9:** The request JSON body for the Anthropic Messages format must conform to:

```json
{
  "model": "<req.Model>",
  "max_tokens": <max_tokens>,
  "messages": [ ... ],
  "system": "<req.System>",
  "temperature": <temperature>
}
```

Field-by-field rules:
- `"model"`: the value of `req.Model`, sent verbatim.
- `"max_tokens"`: if `req.MaxTokens` is 0, use `4096` as the default. The Anthropic API requires
  this field to be present and greater than zero.
- `"messages"`: see Requirement 3.10 for message conversion.
- `"system"`: omit the field entirely when `req.System` is empty (use `omitempty`).
- `"temperature"`: omit the field entirely when `req.Temperature` is `0`.
- `"tools"`: include the field when `req.Tools` is non-empty; omit otherwise.

**Requirement 3.10:** Message conversion for the Anthropic Messages format follows the same rules
as the existing `internal/provider/anthropic.go` `convertToAnthropicMessages` function:

- Standard text messages: `{"role": "<role>", "content": "<content>"}`.
- `role == "tool"` messages with `ToolResults`: converted to `role = "user"` with an array of
  `tool_result` content blocks (each block contains `type`, `tool_use_id`, `content`, `is_error`).
- `role == "assistant"` messages with `ToolCalls`: content is an array of content blocks;
  a `text` block is included if `msg.Content` is non-empty, followed by `tool_use` blocks
  for each tool call (each block contains `type`, `id`, `name`, `input`).

**Requirement 3.11:** Tool definition conversion for the Anthropic Messages format follows the same
rules as the existing `convertToAnthropicTools` function:

```json
{
  "name": "<tool.Name>",
  "description": "<tool.Description>",
  "input_schema": {
    "type": "object",
    "properties": { ... },
    "required": [ ... ]
  }
}
```

The `"required"` field is omitted from `input_schema` when no parameters are required.

**Requirement 3.12:** Response parsing for the Anthropic Messages format:

| Anthropic Response Field          | Maps To                   |
|----------------------------------|---------------------------|
| `content[].text` (type = "text") | `Response.Content` (concatenated) |
| `content[]` (type = "tool_use")  | `Response.ToolCalls`      |
| `model`                          | `Response.Model`          |
| `usage.input_tokens`             | `Response.InputTokens`    |
| `usage.output_tokens`            | `Response.OutputTokens`   |
| `stop_reason`                    | `Response.StopReason`     |

**Requirement 3.13:** If the Anthropic-format response `content` array is empty, return a
`ProviderError` with category `ErrCategoryServer` and message: `response contains no content`.

**Requirement 3.14:** HTTP error mapping for the Anthropic Messages endpoint:

| HTTP Status   | Error Category       |
|--------------|----------------------|
| 401           | `ErrCategoryAuth`    |
| 400           | `ErrCategoryBadRequest` |
| 429           | `ErrCategoryRateLimit`  |
| 529           | `ErrCategoryOverloaded` |
| 500, 502, 503 | `ErrCategoryServer`     |
| All others    | `ErrCategoryServer`     |

Attempt to parse the error body as:
```json
{"type": "error", "error": {"type": "...", "message": "..."}}
```
If successful and `error.message` is non-empty, use it as `ProviderError.Message`. Otherwise
fall back to the HTTP status text (`http.StatusText(status)`).

---

#### 4.3.2 OpenAI Responses Format (`gpt-*` models)

**Requirement 3.15:** The request JSON body for the OpenAI Responses format must conform to:

```json
{
  "model": "<req.Model>",
  "input": [ ... ],
  "temperature": <temperature>,
  "max_output_tokens": <max_tokens>
}
```

Field-by-field rules:
- `"model"`: the value of `req.Model`, sent verbatim.
- `"input"`: the message array. See Requirement 3.16 for conversion.
- `"temperature"`: omit the field entirely when `req.Temperature` is `0`.
- `"max_output_tokens"`: omit the field entirely when `req.MaxTokens` is `0`. The Responses
  API does not require this field.
- `"tools"`: include when `req.Tools` is non-empty; omit otherwise. Tool definitions follow
  the OpenAI function calling schema (see Requirement 3.18).

**Requirement 3.16:** Message conversion for the OpenAI Responses format:

- If `req.System` is non-empty, prepend a message: `{"role": "system", "content": "<req.System>"}`.
- Standard user and assistant text messages: `{"role": "<role>", "content": "<content>"}`.
- `role == "tool"` messages with `ToolResults`: each ToolResult becomes a separate message:
  `{"role": "tool", "tool_call_id": "<tr.CallID>", "content": "<tr.Content>"}`.
- `role == "assistant"` messages with `ToolCalls`: same representation as OpenAI Chat
  Completions (see Requirement 3.20 below).
- If `req.System` is empty, do not include a system message.

**Requirement 3.17:** Response parsing for the OpenAI Responses format.

The Responses API (`/v1/responses`) returns a different JSON shape from Chat Completions.
The response body is:

```json
{
  "model": "<model>",
  "output": [ ... ],
  "usage": {
    "input_tokens": <n>,
    "output_tokens": <n>
  },
  "status": "completed"
}
```

Each element of `output` has a `"type"` field. Parse as follows:

| `output[].type`  | Action                                                              |
|-----------------|---------------------------------------------------------------------|
| `"message"`     | Iterate `output[].content[]`. For each item with `type = "output_text"`, append `text` to `Response.Content`. For each item with `type = "tool_use"`, append to `Response.ToolCalls`. |
| `"function_call"` | Append to `Response.ToolCalls` using `output[].id`, `output[].name`, and `output[].arguments` (JSON-decoded map). |
| All others      | Ignore.                                                             |

Mappings:

| Responses API Field       | Maps To                  |
|--------------------------|--------------------------|
| Accumulated text content | `Response.Content`       |
| `model`                  | `Response.Model`         |
| `usage.input_tokens`     | `Response.InputTokens`   |
| `usage.output_tokens`    | `Response.OutputTokens`  |
| `status`                 | `Response.StopReason`    |

**Requirement 3.18:** Tool definitions for both the Responses format and the Chat Completions format
use the same OpenAI function calling schema:

```json
{
  "type": "function",
  "function": {
    "name": "<tool.Name>",
    "description": "<tool.Description>",
    "parameters": {
      "type": "object",
      "properties": { ... },
      "required": [ ... ]
    }
  }
}
```

The `"required"` field is omitted when no parameters are required.

**Requirement 3.19:** If the OpenAI Responses format `output` array is empty, return a
`ProviderError` with category `ErrCategoryServer` and message: `response contains no output`.

**Requirement 3.20:** HTTP error mapping for the OpenAI Responses endpoint:

| HTTP Status   | Error Category          |
|--------------|-------------------------|
| 401, 403      | `ErrCategoryAuth`        |
| 400, 404      | `ErrCategoryBadRequest`  |
| 429           | `ErrCategoryRateLimit`   |
| 500, 502, 503 | `ErrCategoryServer`      |
| All others    | `ErrCategoryServer`      |

Attempt to parse the error body as:
```json
{"error": {"message": "...", "type": "...", "code": "..."}}
```
If successful and `error.message` is non-empty, use it as `ProviderError.Message`. Otherwise
fall back to `http.StatusText(status)`.

---

#### 4.3.3 OpenAI Chat Completions Format (all other models)

**Requirement 3.21:** The request JSON body for the OpenAI Chat Completions format must conform to:

```json
{
  "model": "<req.Model>",
  "messages": [ ... ],
  "temperature": <temperature>,
  "max_tokens": <max_tokens>
}
```

Field-by-field rules:
- `"model"`: the value of `req.Model`, sent verbatim.
- `"messages"`: see Requirement 3.22 for conversion.
- `"temperature"`: omit the field entirely when `req.Temperature` is `0`.
- `"max_tokens"`: omit the field entirely when `req.MaxTokens` is `0`.
- `"tools"`: include when `req.Tools` is non-empty; omit otherwise. See Requirement 3.18.

**Requirement 3.22:** Message conversion for the OpenAI Chat Completions format follows the same
rules as the existing `internal/provider/openai.go` `convertToOpenAIMessages` function:

- If `req.System` is non-empty, prepend: `{"role": "system", "content": "<req.System>"}`.
- Standard messages: `{"role": "<role>", "content": "<content>"}`. The `content` field is
  a pointer-to-string (nullable) in the wire format; assistant messages with tool calls send
  `null` for content.
- `role == "tool"` messages with `ToolResults`: each ToolResult becomes a separate `"tool"` role
  message with `"tool_call_id"` and `"content"` fields.
- `role == "assistant"` messages with `ToolCalls`: send `"tool_calls"` array where each item has
  `"id"`, `"type": "function"`, and `"function": {"name": "...", "arguments": "<json-string>"}`.
  Arguments are JSON-encoded as a string (not an object).

**Requirement 3.23:** Response parsing for the OpenAI Chat Completions format:

| Chat Completions Response Field           | Maps To                  |
|------------------------------------------|--------------------------|
| `choices[0].message.content`             | `Response.Content`       |
| `choices[0].message.tool_calls`          | `Response.ToolCalls`     |
| `model`                                  | `Response.Model`         |
| `usage.prompt_tokens`                    | `Response.InputTokens`   |
| `usage.completion_tokens`                | `Response.OutputTokens`  |
| `choices[0].finish_reason`               | `Response.StopReason`    |

`content` may be `null` when the response contains tool calls. When null, `Response.Content`
is empty string.

**Requirement 3.24:** If the Chat Completions response `choices` array is empty, return a
`ProviderError` with category `ErrCategoryServer` and message: `response contains no choices`.

**Requirement 3.25:** HTTP error mapping for the Chat Completions endpoint is identical to
the Responses endpoint mapping (Requirement 3.20).

---

#### 4.3.4 Shared Error Handling

**Requirement 3.26:** Context cancellation and deadline exceeded: if `ctx.Err()` is non-nil after
the HTTP client returns an error, return a `ProviderError` with category `ErrCategoryTimeout`
and `Message` set to `ctx.Err().Error()`. This applies to all three endpoint paths.

**Requirement 3.27:** If the HTTP client itself returns an error (not a non-2xx status, but a
transport-level error) and the context is not expired, return a `ProviderError` with category
`ErrCategoryServer` and `Message` set to `err.Error()`. This applies to all three endpoint paths.

**Requirement 3.28:** Failed requests must not be retried. Return the error immediately.

---

## 5. Project Structure

The following files will be added or modified:

```
axe/
├── cmd/
│   └── run.go                    UNCHANGED
├── internal/
│   ├── config/
│   │   ├── config.go             MODIFIED: add "opencode" to knownAPIKeyEnvVars
│   │   └── config_test.go        MODIFIED: add opencode key resolution tests
│   └── provider/
│       ├── registry.go           MODIFIED: add "opencode" to supportedProviders + New() case
│       ├── registry_test.go      MODIFIED: add opencode factory tests
│       ├── opencode.go           NEW: OpenCode provider implementation
│       └── opencode_test.go      NEW: OpenCode provider tests
```

All other files are unchanged.

---

## 6. Edge Cases

### 6.1 Model Routing

| Scenario                                    | Behavior                                                                           |
|--------------------------------------------|------------------------------------------------------------------------------------|
| `req.Model = "claude-sonnet-4-6"`          | Routes to Anthropic Messages endpoint                                              |
| `req.Model = "claude-opus-4-6"`            | Routes to Anthropic Messages endpoint                                              |
| `req.Model = "claude-3-5-haiku"`           | Routes to Anthropic Messages endpoint (prefix `"claude-"` matched)                |
| `req.Model = "gpt-5"`                      | Routes to OpenAI Responses endpoint                                                |
| `req.Model = "gpt-5.3-codex"`             | Routes to OpenAI Responses endpoint                                                |
| `req.Model = "gpt-5-nano"`                 | Routes to OpenAI Responses endpoint                                                |
| `req.Model = "kimi-k2"`                    | Routes to Chat Completions endpoint                                                |
| `req.Model = "minimax-m2.5"`               | Routes to Chat Completions endpoint                                                |
| `req.Model = "big-pickle"`                 | Routes to Chat Completions endpoint                                                |
| `req.Model = "gemini-3-pro"`               | Routes to Chat Completions endpoint (Gemini out of scope; Zen will return error)  |
| `req.Model = "CLAUDE-sonnet-4-6"`          | Routes to Chat Completions endpoint (case-sensitive: `"CLAUDE-"` is not `"claude-"`) |
| `req.Model = "GPT-5"`                      | Routes to Chat Completions endpoint (case-sensitive: `"GPT-"` is not `"gpt-"`)   |
| `req.Model = ""`                           | Routes to Chat Completions endpoint; Zen API will return a 400 error              |
| `req.Model = "claude"` (no hyphen)         | Routes to Chat Completions endpoint (prefix `"claude-"` not matched)              |

### 6.2 API Key Resolution

| Scenario                                            | Behavior                                                                           |
|----------------------------------------------------|------------------------------------------------------------------------------------|
| `OPENCODE_API_KEY` set, config file also has key   | Env var wins                                                                       |
| `OPENCODE_API_KEY` not set, config file has key    | Config file value used                                                             |
| Neither env var nor config file                    | `ResolveAPIKey` returns `""` → `cmd/run.go` returns `ExitError` code 3 with message: `API key for provider "opencode" is not configured (set OPENCODE_API_KEY or add to config.toml)` |
| `OPENCODE_API_KEY` set to empty string             | Treated as unset; falls through to config file                                     |

### 6.3 Base URL

| Scenario                                    | Behavior                                                                                |
|--------------------------------------------|-----------------------------------------------------------------------------------------|
| `AXE_OPENCODE_BASE_URL` not set, no config | Provider uses `"https://opencode.ai/zen"` as default                                   |
| `AXE_OPENCODE_BASE_URL` set                | Provider uses the env var value as base URL                                             |
| `base_url` set in config, no env var        | Provider uses the config file value                                                     |
| `AXE_OPENCODE_BASE_URL` set and config set | Env var wins                                                                            |
| Base URL has trailing slash (e.g. `"https://opencode.ai/zen/"`) | The base URL is stored as-is; paths are appended as `<baseURL>/v1/messages`, resulting in a double slash (`/zen//v1/messages`). This is not automatically normalized. Users must not include a trailing slash. |

### 6.4 Request and Response Edge Cases

| Scenario                                                      | Behavior                                                                        |
|--------------------------------------------------------------|---------------------------------------------------------------------------------|
| `req.System` is empty string                                  | System field/message is omitted entirely from the request body                 |
| `req.Temperature` is `0`                                      | Temperature field omitted from all three wire formats                          |
| `req.MaxTokens` is `0`, Claude model                          | `max_tokens` defaults to `4096` in Anthropic Messages format                  |
| `req.MaxTokens` is `0`, GPT model                             | `max_output_tokens` omitted from Responses format                              |
| `req.MaxTokens` is `0`, other model                           | `max_tokens` omitted from Chat Completions format                              |
| `req.Tools` is nil or empty                                   | Tools field omitted from all three wire formats                                |
| Response body is not valid JSON                               | `ProviderError` with `ErrCategoryServer`, message: `failed to parse response: <err>` |
| Anthropic-format response with zero content blocks            | `ProviderError` with `ErrCategoryServer`, message: `response contains no content` |
| Responses-format response with empty `output` array           | `ProviderError` with `ErrCategoryServer`, message: `response contains no output` |
| Chat Completions response with empty `choices` array          | `ProviderError` with `ErrCategoryServer`, message: `response contains no choices` |
| Non-2xx status with unparseable error body                    | `ProviderError.Message` falls back to `http.StatusText(status)`                |
| Non-2xx status with parseable error body but empty message    | `ProviderError.Message` falls back to `http.StatusText(status)`                |
| Context cancelled before request sent                         | `ProviderError` with `ErrCategoryTimeout`                                      |
| Context deadline exceeded mid-request                         | `ProviderError` with `ErrCategoryTimeout`                                      |

### 6.5 Provider Registry

| Scenario                                       | Behavior                                                                              |
|-----------------------------------------------|---------------------------------------------------------------------------------------|
| `provider.New("opencode", "key", "")`         | Returns `*OpenCode` with default base URL                                             |
| `provider.New("opencode", "key", "http://x")` | Returns `*OpenCode` with custom base URL                                              |
| `provider.New("opencode", "", "")`            | Error: `API key is required`                                                          |
| `provider.Supported("opencode")`              | Returns `true`                                                                        |
| `provider.New("OpenCode", "key", "")`         | Error: `unsupported provider "OpenCode": supported providers are ...` (case-sensitive) |

---

## 7. Testing Requirements

### 7.1 Test Conventions

Tests must follow the patterns established in the existing provider tests:

- **Package-level tests:** Tests live in `package provider`.
- **Standard library only:** Use `testing` package. No third-party test frameworks.
- **Table-driven tests:** Prefer `tt := range tests` where appropriate.
- **HTTP tests:** Use `httptest.NewServer` for all HTTP interactions. No real API calls.
- **Env overrides:** Use `t.Setenv()` for environment variable control.
- **Descriptive names:** `TestFunctionName_Scenario` format.
- **Test real code, not mocks.** Every test must fail if the code under test is deleted.
- **Run tests with:** `make test`

### 7.2 `internal/config/config_test.go` Additions

**Test: `TestResolveAPIKey_OpenCode`** — Verify that `ResolveAPIKey("opencode")` checks the
`OPENCODE_API_KEY` env var. Set the env var and confirm the value is returned. Unset it and
confirm the config file value is returned.

**Test: `TestAPIKeyEnvVar_OpenCode`** — Call `APIKeyEnvVar("opencode")` and verify it returns
`"OPENCODE_API_KEY"`.

### 7.3 `internal/provider/registry_test.go` Additions

**Test: `TestNew_OpenCode`** — Call `New("opencode", "test-key", "")`. Verify returned provider
is non-nil and error is nil.

**Test: `TestNew_OpenCodeWithBaseURL`** — Call `New("opencode", "test-key", "http://custom:8080")`.
Verify returned provider is non-nil and error is nil.

**Test: `TestNew_OpenCodeMissingAPIKey`** — Call `New("opencode", "", "")`. Verify error message
contains `API key is required`.

**Test: `TestSupported_OpenCode`** — Call `Supported("opencode")`. Verify it returns `true`.

**Test: `TestNew_UnsupportedProvider_ErrorMessage`** — Call `New("groq", "key", "")`. Verify
the error message contains all four supported providers: `anthropic`, `openai`, `ollama`, `opencode`.

### 7.4 `internal/provider/opencode_test.go` (New File)

**Test: `TestNewOpenCode_EmptyAPIKey`** — Empty string returns error containing `API key is required`.

**Test: `TestNewOpenCode_ValidAPIKey`** — Non-empty string returns `*OpenCode` with no error,
`baseURL` set to `defaultOpenCodeBaseURL`.

**Test: `TestNewOpenCode_WithBaseURL`** — Verify `WithOpenCodeBaseURL` option overrides the default
base URL. The test should confirm the provider makes requests to the custom base URL.

---

**Anthropic Messages Format Tests (`claude-*` models):**

**Test: `TestOpenCode_Send_Claude_Success`** — Use `httptest.NewServer` returning a valid Anthropic
Messages API response with a text content block. Verify:
- `Response.Content` is populated from `content[0].text`.
- `Response.Model`, `Response.InputTokens`, `Response.OutputTokens`, `Response.StopReason` are
  populated correctly.
- `Response.ToolCalls` is empty.

**Test: `TestOpenCode_Send_Claude_RequestFormat`** — Use `httptest.NewServer` that inspects the
incoming request. Verify:
- HTTP method is `POST`.
- URL path is `/v1/messages`.
- `Authorization` header is `Bearer <key>`.
- `anthropic-version` header is `2023-06-01`.
- `Content-Type` header is `application/json`.
- Request body contains `"model": "claude-sonnet-4-6"` and `"max_tokens": 4096`.

**Test: `TestOpenCode_Send_Claude_SystemPrompt`** — Verify `req.System` non-empty → `"system"`
field present in request body. Verify `req.System` empty → `"system"` field absent.

**Test: `TestOpenCode_Send_Claude_DefaultMaxTokens`** — `req.MaxTokens = 0` → request body contains
`"max_tokens": 4096`.

**Test: `TestOpenCode_Send_Claude_CustomMaxTokens`** — `req.MaxTokens = 1000` → request body contains
`"max_tokens": 1000`.

**Test: `TestOpenCode_Send_Claude_OmitsZeroTemperature`** — `req.Temperature = 0` → `"temperature"`
key absent from request body.

**Test: `TestOpenCode_Send_Claude_EmptyContent`** — Server returns Anthropic response with empty
`content` array. Verify `ProviderError` with `ErrCategoryServer`, message contains `no content`.

**Test: `TestOpenCode_Send_Claude_AuthError`** — Server returns HTTP 401. Verify `ProviderError`
with `ErrCategoryAuth`.

**Test: `TestOpenCode_Send_Claude_RateLimit`** — Server returns HTTP 429. Verify `ProviderError`
with `ErrCategoryRateLimit`.

**Test: `TestOpenCode_Send_Claude_Overloaded`** — Server returns HTTP 529. Verify `ProviderError`
with `ErrCategoryOverloaded`.

**Test: `TestOpenCode_Send_Claude_ServerError`** — Server returns HTTP 500. Verify `ProviderError`
with `ErrCategoryServer`.

**Test: `TestOpenCode_Send_Claude_ErrorBodyParsed`** — Server returns HTTP 400 with Anthropic-style
error JSON. Verify `ProviderError.Message` comes from `error.message` field.

**Test: `TestOpenCode_Send_Claude_ErrorBodyUnparseable`** — Server returns HTTP 400 with non-JSON
body. Verify `ProviderError.Message` falls back to HTTP status text.

---

**OpenAI Responses Format Tests (`gpt-*` models):**

**Test: `TestOpenCode_Send_GPT_Success`** — Use `httptest.NewServer` returning a valid Responses
API response with a `"message"` output containing an `"output_text"` content block. Verify:
- `Response.Content` is populated.
- `Response.Model`, `Response.InputTokens`, `Response.OutputTokens`, `Response.StopReason`
  are populated correctly.

**Test: `TestOpenCode_Send_GPT_RequestFormat`** — Use `httptest.NewServer` that inspects the
incoming request. Verify:
- HTTP method is `POST`.
- URL path is `/v1/responses`.
- `Authorization` header is `Bearer <key>`.
- No `anthropic-version` header.
- `Content-Type` header is `application/json`.
- Request body contains `"model": "gpt-5"` and `"input"` array (not `"messages"`).

**Test: `TestOpenCode_Send_GPT_SystemPrompt`** — Verify `req.System` non-empty → system message
prepended to `"input"` array. Verify `req.System` empty → no system message.

**Test: `TestOpenCode_Send_GPT_OmitsZeroMaxOutputTokens`** — `req.MaxTokens = 0` → `"max_output_tokens"`
absent from request body.

**Test: `TestOpenCode_Send_GPT_IncludesMaxOutputTokens`** — `req.MaxTokens = 500` → `"max_output_tokens": 500`
present in request body.

**Test: `TestOpenCode_Send_GPT_EmptyOutput`** — Server returns Responses API response with empty
`output` array. Verify `ProviderError` with `ErrCategoryServer`, message contains `no output`.

**Test: `TestOpenCode_Send_GPT_AuthError`** — Server returns HTTP 401. Verify `ProviderError`
with `ErrCategoryAuth`.

**Test: `TestOpenCode_Send_GPT_ForbiddenError`** — Server returns HTTP 403. Verify `ProviderError`
with `ErrCategoryAuth`.

**Test: `TestOpenCode_Send_GPT_NotFoundError`** — Server returns HTTP 404. Verify `ProviderError`
with `ErrCategoryBadRequest`.

---

**OpenAI Chat Completions Format Tests (all other models):**

**Test: `TestOpenCode_Send_ChatCompletions_Success`** — Use `httptest.NewServer` returning a valid
Chat Completions response. Verify `Response` fields are correctly populated.

**Test: `TestOpenCode_Send_ChatCompletions_RequestFormat`** — Use `httptest.NewServer` that inspects
the request. Verify:
- HTTP method is `POST`.
- URL path is `/v1/chat/completions`.
- `Authorization` header is `Bearer <key>`.
- No `anthropic-version` header.
- `Content-Type` header is `application/json`.
- Request body contains `"model": "kimi-k2"` and `"messages"` array.

**Test: `TestOpenCode_Send_ChatCompletions_EmptyChoices`** — Server returns Chat Completions
response with empty `choices`. Verify `ProviderError` with `ErrCategoryServer`, message contains
`no choices`.

---

**Routing Tests:**

**Test: `TestOpenCode_Send_RoutesClaudeToMessages`** — Create an `OpenCode` provider with a custom
base URL pointing to an `httptest.NewServer`. Send a request with `req.Model = "claude-sonnet-4-6"`.
Verify the server received a POST to `/v1/messages`.

**Test: `TestOpenCode_Send_RoutesGPTToResponses`** — Send a request with `req.Model = "gpt-5"`.
Verify the server received a POST to `/v1/responses`.

**Test: `TestOpenCode_Send_RoutesOtherToChatCompletions`** — Send a request with `req.Model = "kimi-k2"`.
Verify the server received a POST to `/v1/chat/completions`.

**Test: `TestOpenCode_Send_RoutesGeminiToChatCompletions`** — Send a request with `req.Model = "gemini-3-pro"`.
Verify the server received a POST to `/v1/chat/completions` (not a Gemini-specific path).

**Test: `TestOpenCode_Send_CaseSensitiveRouting_ClaudeUppercase`** — Send with `req.Model = "CLAUDE-sonnet-4-6"`.
Verify server received POST to `/v1/chat/completions` (uppercase not matched).

**Test: `TestOpenCode_Send_CaseSensitiveRouting_GPTUppercase`** — Send with `req.Model = "GPT-5"`.
Verify server received POST to `/v1/chat/completions` (uppercase not matched).

---

**Shared Behavior Tests:**

**Test: `TestOpenCode_Send_ContextTimeout`** — For each of the three routing paths, use an
`httptest.NewServer` that blocks until the context deadline passes. Verify each returns a
`ProviderError` with `ErrCategoryTimeout`.

**Test: `TestOpenCode_Send_NoRedirectFollowing`** — Use an `httptest.NewServer` that returns
HTTP 302 with a `Location` header. Verify the provider does not follow the redirect and returns
a non-nil error (the redirect response itself is treated as a non-2xx error).

---

## 8. Exit Codes

No new exit codes are introduced. The existing exit code table is unchanged:

| Code | Meaning                                                                              |
|------|--------------------------------------------------------------------------------------|
| 0    | Success                                                                              |
| 1    | Agent/general error (unsupported provider, bad request from provider)                |
| 2    | Config error (malformed `config.toml`, agent not found)                              |
| 3    | API error (auth failure, rate limit, timeout, server error, missing API key)         |

---

## 9. Constraints

**Constraint 1:** No new external dependencies. `go.mod` must still contain only `spf13/cobra`
and `BurntSushi/toml` as direct dependencies.

**Constraint 2:** `cmd/run.go` must not be modified. The existing API key required check
(`provName != "ollama"`) correctly enforces that `opencode` requires a key.

**Constraint 3:** No existing provider files (`anthropic.go`, `openai.go`, `ollama.go`) may be
modified. No code from those files may be called by `opencode.go`.

**Constraint 4:** The `Provider` interface must not be modified.

**Constraint 5:** The `Request`, `Response`, `Message`, `ProviderError`, and `ErrorCategory` types
must not be modified.

**Constraint 6:** The HTTP client must not follow redirects.

**Constraint 7:** No streaming. The complete response is buffered before returning.

**Constraint 8:** No retry logic.

**Constraint 9:** Provider name matching in the registry is case-sensitive. `"OpenCode"` is not
a valid provider name.

---

## 10. Out of Scope

1. Gemini (`gemini-*`) models. The Google AI wire format is structurally different and requires
   a separate milestone.
2. Dynamic model discovery via `GET https://opencode.ai/zen/v1/models`.
3. Credit balance checking or pre-flight cost estimation.
4. Cached read/write pricing differentiation.
5. Team workspace management or multi-key routing.
6. Streaming output.
7. Retry logic or exponential backoff.
8. Any change to existing provider implementations.

---

## 11. Acceptance Criteria

| Criterion                                              | Verified By                                               |
|-------------------------------------------------------|-----------------------------------------------------------|
| `provider.New("opencode", key, "")` returns valid provider | `TestNew_OpenCode`                                   |
| `provider.New("opencode", "", "")` returns error       | `TestNew_OpenCodeMissingAPIKey`                           |
| `provider.Supported("opencode")` returns `true`       | `TestSupported_OpenCode`                                  |
| `claude-*` routes to `/v1/messages`                   | `TestOpenCode_Send_RoutesClaudeToMessages`                |
| `gpt-*` routes to `/v1/responses`                     | `TestOpenCode_Send_RoutesGPTToResponses`                  |
| All other models route to `/v1/chat/completions`      | `TestOpenCode_Send_RoutesOtherToChatCompletions`          |
| Gemini routes to `/v1/chat/completions`               | `TestOpenCode_Send_RoutesGeminiToChatCompletions`         |
| Routing is case-sensitive                             | `TestOpenCode_Send_CaseSensitiveRouting_*`                |
| Bearer token sent on all requests                     | `TestOpenCode_Send_Claude_RequestFormat`, `_GPT_`, `_ChatCompletions_` |
| `anthropic-version` header on Claude only             | `TestOpenCode_Send_Claude_RequestFormat`; absent in GPT/chat tests |
| Claude: default `max_tokens = 4096`                   | `TestOpenCode_Send_Claude_DefaultMaxTokens`               |
| GPT: `max_output_tokens` omitted when zero            | `TestOpenCode_Send_GPT_OmitsZeroMaxOutputTokens`          |
| Chat: `max_tokens` omitted when zero                  | `TestOpenCode_Send_ChatCompletions_RequestFormat`         |
| `OPENCODE_API_KEY` env var resolves correctly         | `TestResolveAPIKey_OpenCode`                              |
| Missing key → exit code 3, helpful message            | Integration via `cmd/run.go` existing behavior            |
| No new `go.mod` direct dependencies                   | `go mod tidy && git diff go.mod`                          |
| All tests pass                                        | `make test`                                               |

---

## 12. References

- OpenCode Zen docs: https://opencode.ai/docs/zen
- Existing multi-provider spec: `docs/plans/004_multi_provider_support_spec.md`
- Provider interface: `internal/provider/provider.go`
- Anthropic provider (wire format reference): `internal/provider/anthropic.go`
- OpenAI provider (wire format reference): `internal/provider/openai.go`
- Config package: `internal/config/config.go`
- Provider registry: `internal/provider/registry.go`
- Anthropic Messages API: https://docs.anthropic.com/en/api/messages
- OpenAI Responses API: https://platform.openai.com/docs/api-reference/responses
- OpenAI Chat Completions API: https://platform.openai.com/docs/api-reference/chat/create
