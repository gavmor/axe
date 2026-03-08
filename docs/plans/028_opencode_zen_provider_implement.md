# Implementation Checklist: OpenCode Zen Provider

**Spec:** `docs/plans/028_opencode_zen_provider_spec.md`
**Status:** Not started

---

## Phase 1: Config Package

**File:** `internal/config/config.go`

- [x] Add `"opencode": "OPENCODE_API_KEY"` entry to the `knownAPIKeyEnvVars` map (Req 1.1)

**File:** `internal/config/config_test.go`

- [x] Add `TestResolveAPIKey_OpenCode`: set `OPENCODE_API_KEY` env var, call `ResolveAPIKey("opencode")`, verify value returned; unset env var, verify config file value is used (Req 1.1, §7.2)
- [x] Add `TestAPIKeyEnvVar_OpenCode`: call `APIKeyEnvVar("opencode")`, verify it returns `"OPENCODE_API_KEY"` (§7.2)

---

## Phase 2: Provider Registry

**File:** `internal/provider/registry.go`

- [x] Add `"opencode": true` to the `supportedProviders` map (Req 2.1)
- [x] Add `case "opencode"` branch in `New()` that calls `NewOpenCode(apiKey, opts...)`, passing `WithOpenCodeBaseURL(baseURL)` when `baseURL` is non-empty (Req 2.2)
- [x] Update the `default` error message to include `opencode`: `unsupported provider "<name>": supported providers are anthropic, openai, ollama, opencode` (Req 2.3)

**File:** `internal/provider/registry_test.go`

- [x] Add `TestNew_OpenCode`: call `New("opencode", "test-key", "")`, verify non-nil provider and nil error (§7.3)
- [x] Add `TestNew_OpenCodeWithBaseURL`: call `New("opencode", "test-key", "http://custom:8080")`, verify non-nil provider and nil error (§7.3)
- [x] Add `TestNew_OpenCodeMissingAPIKey`: call `New("opencode", "", "")`, verify error message contains `API key is required` (§7.3)
- [x] Add `TestSupported_OpenCode`: call `Supported("opencode")`, verify it returns `true` (§7.3)
- [x] Add `TestNew_UnsupportedProvider_ErrorMessage`: call `New("groq", "key", "")`, verify error message contains all four provider names: `anthropic`, `openai`, `ollama`, `opencode` (§7.3)

---

## Phase 3: OpenCode Provider — Core Structure

**File:** `internal/provider/opencode.go` (new file)

- [x] Define `const defaultOpenCodeBaseURL = "https://opencode.ai/zen"` (Req 3.2)
- [x] Define `OpenCode` struct with fields: `apiKey string`, `baseURL string`, `client *http.Client` (Req 3.2)
- [x] Define `OpenCodeOption` type: `type OpenCodeOption func(*OpenCode)` (Req 3.4)
- [x] Implement `WithOpenCodeBaseURL(url string) OpenCodeOption` functional option (Req 3.4)
- [x] Implement `NewOpenCode(apiKey string, opts ...OpenCodeOption) (*OpenCode, error)` constructor (Req 3.3)
  - [x] Return error `"API key is required"` when `apiKey` is empty string (Req 3.3)
  - [x] Initialize `baseURL` to `defaultOpenCodeBaseURL` (Req 3.2)
  - [x] Initialize `client` with `CheckRedirect` set to return `http.ErrUseLastResponse` (Req 3.5)
  - [x] Apply all options before returning (Req 3.4)
- [x] Implement `Send(ctx context.Context, req *Request) (*Response, error)` method on `*OpenCode` (Req 3.1, 3.6)
  - [x] Route to Anthropic Messages format when `strings.HasPrefix(req.Model, "claude-")` (Req 3.6)
  - [x] Route to OpenAI Responses format when `strings.HasPrefix(req.Model, "gpt-")` (Req 3.6)
  - [x] Route to OpenAI Chat Completions format for all other model names (Req 3.6)

---

## Phase 4: Anthropic Messages Format (`claude-*`)

**File:** `internal/provider/opencode.go`

- [ ] Implement `sendClaude(ctx, req)` (or inline in `Send`): build Anthropic Messages request body struct with fields `Model`, `MaxTokens`, `Messages`, `System` (omitempty), `Temperature` (omitempty), `Tools` (omitempty) (Req 3.9)
  - [ ] Default `MaxTokens` to `4096` when `req.MaxTokens == 0` (Req 3.9)
- [ ] Implement message conversion for Anthropic format (Req 3.10):
  - [ ] Standard messages: `{"role": "<role>", "content": "<content>"}`
  - [ ] `role == "tool"` with `ToolResults`: convert to `role = "user"` with `tool_result` content block array (fields: `type`, `tool_use_id`, `content`, `is_error`)
  - [ ] `role == "assistant"` with `ToolCalls`: content is array of blocks — `text` block (if `msg.Content` non-empty) followed by `tool_use` blocks (fields: `type`, `id`, `name`, `input`)
- [ ] Implement tool definition conversion for Anthropic format (Req 3.11):
  - [ ] Format: `{"name", "description", "input_schema": {"type": "object", "properties": {...}, "required": [...]}}`
  - [ ] Omit `"required"` field when no parameters are required
- [ ] Set `Authorization: Bearer <apiKey>` header (Req 3.7)
- [ ] Set `anthropic-version: 2023-06-01` header (Req 3.7)
- [ ] Set `Content-Type: application/json` header (Req 3.8)
- [ ] POST to `<baseURL>/v1/messages` (Req 3.6)
- [ ] Handle transport-level errors: check `ctx.Err()` → `ErrCategoryTimeout`; otherwise `ErrCategoryServer` (Req 3.26, 3.27)
- [ ] Map non-2xx HTTP status codes to `ProviderError` categories (Req 3.14):
  - [ ] 401 → `ErrCategoryAuth`
  - [ ] 400 → `ErrCategoryBadRequest`
  - [ ] 429 → `ErrCategoryRateLimit`
  - [ ] 529 → `ErrCategoryOverloaded`
  - [ ] 500, 502, 503, all others → `ErrCategoryServer`
  - [ ] Parse error body as `{"type": "error", "error": {"type": "...", "message": "..."}}`, use `error.message` if non-empty, else fall back to `http.StatusText(status)` (Req 3.14)
- [ ] Parse successful Anthropic response body (Req 3.12):
  - [ ] Accumulate `content[].text` (type=`"text"`) into `Response.Content`
  - [ ] Collect `content[]` (type=`"tool_use"`) into `Response.ToolCalls`
  - [ ] Map `model` → `Response.Model`
  - [ ] Map `usage.input_tokens` → `Response.InputTokens`
  - [ ] Map `usage.output_tokens` → `Response.OutputTokens`
  - [ ] Map `stop_reason` → `Response.StopReason`
- [ ] Return `ProviderError{Category: ErrCategoryServer, Message: "response contains no content"}` when `content` array is empty (Req 3.13)
- [ ] Return `ProviderError{Category: ErrCategoryServer, Message: "failed to parse response: <err>"}` on JSON decode failure (§6.4)

---

## Phase 5: OpenAI Responses Format (`gpt-*`)

**File:** `internal/provider/opencode.go`

- [ ] Implement `sendGPT(ctx, req)` (or inline in `Send`): build OpenAI Responses request body struct with fields `Model`, `Input`, `Temperature` (omitempty), `MaxOutputTokens` (omitempty), `Tools` (omitempty) (Req 3.15)
  - [ ] Omit `MaxOutputTokens` entirely when `req.MaxTokens == 0` (Req 3.15)
- [ ] Implement message conversion for Responses format (Req 3.16):
  - [ ] If `req.System` is non-empty, prepend `{"role": "system", "content": "<req.System>"}`
  - [ ] Standard user/assistant text messages: `{"role": "<role>", "content": "<content>"}`
  - [ ] `role == "tool"` with `ToolResults`: each ToolResult → separate message `{"role": "tool", "tool_call_id": "<tr.CallID>", "content": "<tr.Content>"}`
  - [ ] `role == "assistant"` with `ToolCalls`: use OpenAI Chat Completions tool_calls format (same as Req 3.22)
- [ ] Use OpenAI function calling schema for tool definitions (shared with Chat Completions, Req 3.18):
  - [ ] Format: `{"type": "function", "function": {"name", "description", "parameters": {"type": "object", "properties": {...}, "required": [...]}}}`
  - [ ] Omit `"required"` when no parameters are required
- [ ] Set `Authorization: Bearer <apiKey>` header (Req 3.7)
- [ ] Set `Content-Type: application/json` header (Req 3.8)
- [ ] Do NOT set `anthropic-version` header (Req 3.7)
- [ ] POST to `<baseURL>/v1/responses` (Req 3.6)
- [ ] Handle transport-level errors: check `ctx.Err()` → `ErrCategoryTimeout`; otherwise `ErrCategoryServer` (Req 3.26, 3.27)
- [ ] Map non-2xx HTTP status codes to `ProviderError` categories (Req 3.20):
  - [ ] 401, 403 → `ErrCategoryAuth`
  - [ ] 400, 404 → `ErrCategoryBadRequest`
  - [ ] 429 → `ErrCategoryRateLimit`
  - [ ] 500, 502, 503, all others → `ErrCategoryServer`
  - [ ] Parse error body as `{"error": {"message": "...", "type": "...", "code": "..."}}`, use `error.message` if non-empty, else fall back to `http.StatusText(status)` (Req 3.20)
- [ ] Parse successful Responses API response body (Req 3.17):
  - [ ] For `output[].type == "message"`: iterate `output[].content[]`, append `output_text` items to `Response.Content`, append `tool_use` items to `Response.ToolCalls`
  - [ ] For `output[].type == "function_call"`: append to `Response.ToolCalls` using `output[].id`, `output[].name`, `output[].arguments` (JSON-decoded map)
  - [ ] Ignore all other `output[].type` values
  - [ ] Map `model` → `Response.Model`
  - [ ] Map `usage.input_tokens` → `Response.InputTokens`
  - [ ] Map `usage.output_tokens` → `Response.OutputTokens`
  - [ ] Map `status` → `Response.StopReason`
- [ ] Return `ProviderError{Category: ErrCategoryServer, Message: "response contains no output"}` when `output` array is empty (Req 3.19)
- [ ] Return `ProviderError{Category: ErrCategoryServer, Message: "failed to parse response: <err>"}` on JSON decode failure (§6.4)

---

## Phase 6: OpenAI Chat Completions Format (all other models)

**File:** `internal/provider/opencode.go`

- [ ] Implement `sendChatCompletions(ctx, req)` (or inline in `Send`): build Chat Completions request body struct with fields `Model`, `Messages`, `Temperature` (omitempty), `MaxTokens` (omitempty), `Tools` (omitempty) (Req 3.21)
  - [ ] Omit `MaxTokens` entirely when `req.MaxTokens == 0` (Req 3.21)
- [ ] Implement message conversion for Chat Completions format (Req 3.22):
  - [ ] If `req.System` is non-empty, prepend `{"role": "system", "content": "<req.System>"}`
  - [ ] Standard messages: `{"role": "<role>", "content": "<content>"}` where `content` is a pointer-to-string (nullable)
  - [ ] Assistant messages with `ToolCalls`: send `null` for content, include `"tool_calls"` array with items `{"id", "type": "function", "function": {"name", "arguments": "<json-string>"}}`; arguments JSON-encoded as a string
  - [ ] `role == "tool"` with `ToolResults`: each ToolResult → separate `"tool"` role message with `"tool_call_id"` and `"content"` fields
- [ ] Reuse OpenAI function calling schema for tool definitions (Req 3.18, shared with GPT format)
- [ ] Set `Authorization: Bearer <apiKey>` header (Req 3.7)
- [ ] Set `Content-Type: application/json` header (Req 3.8)
- [ ] Do NOT set `anthropic-version` header (Req 3.7)
- [ ] POST to `<baseURL>/v1/chat/completions` (Req 3.6)
- [ ] Handle transport-level errors: check `ctx.Err()` → `ErrCategoryTimeout`; otherwise `ErrCategoryServer` (Req 3.26, 3.27)
- [ ] Map non-2xx HTTP status codes to `ProviderError` categories (identical to Req 3.20, Req 3.25):
  - [ ] 401, 403 → `ErrCategoryAuth`
  - [ ] 400, 404 → `ErrCategoryBadRequest`
  - [ ] 429 → `ErrCategoryRateLimit`
  - [ ] 500, 502, 503, all others → `ErrCategoryServer`
  - [ ] Parse error body as `{"error": {"message": "...", "type": "...", "code": "..."}}`, use `error.message` if non-empty, else fall back to `http.StatusText(status)`
- [ ] Parse successful Chat Completions response body (Req 3.23):
  - [ ] Map `choices[0].message.content` → `Response.Content` (empty string when `null`)
  - [ ] Map `choices[0].message.tool_calls` → `Response.ToolCalls`
  - [ ] Map `model` → `Response.Model`
  - [ ] Map `usage.prompt_tokens` → `Response.InputTokens`
  - [ ] Map `usage.completion_tokens` → `Response.OutputTokens`
  - [ ] Map `choices[0].finish_reason` → `Response.StopReason`
- [ ] Return `ProviderError{Category: ErrCategoryServer, Message: "response contains no choices"}` when `choices` array is empty (Req 3.24)
- [ ] Return `ProviderError{Category: ErrCategoryServer, Message: "failed to parse response: <err>"}` on JSON decode failure (§6.4)

---

## Phase 7: OpenCode Provider Tests

**File:** `internal/provider/opencode_test.go` (new file)

### Constructor Tests

- [x] `TestNewOpenCode_EmptyAPIKey`: empty string returns error containing `"API key is required"` (§7.4)
- [x] `TestNewOpenCode_ValidAPIKey`: non-empty string returns `*OpenCode` with no error and `baseURL == defaultOpenCodeBaseURL` (§7.4)
- [x] `TestNewOpenCode_WithBaseURL`: `WithOpenCodeBaseURL` option overrides default; verify provider makes requests to custom base URL (§7.4)

### Anthropic Messages Format Tests

- [x] `TestOpenCode_Send_Claude_Success`: `httptest.NewServer` returning valid Anthropic response; verify `Response.Content`, `Response.Model`, `Response.InputTokens`, `Response.OutputTokens`, `Response.StopReason` populated; `Response.ToolCalls` empty (§7.4)
- [x] `TestOpenCode_Send_Claude_RequestFormat`: inspect incoming request — method=`POST`, path=`/v1/messages`, `Authorization: Bearer <key>`, `anthropic-version: 2023-06-01`, `Content-Type: application/json`, body contains `"model": "claude-sonnet-4-6"` and `"max_tokens": 4096` (§7.4)
- [x] `TestOpenCode_Send_Claude_SystemPrompt`: non-empty `req.System` → `"system"` field present; empty `req.System` → `"system"` field absent (§7.4)
- [x] `TestOpenCode_Send_Claude_DefaultMaxTokens`: `req.MaxTokens = 0` → body contains `"max_tokens": 4096` (§7.4)
- [x] `TestOpenCode_Send_Claude_CustomMaxTokens`: `req.MaxTokens = 1000` → body contains `"max_tokens": 1000` (§7.4)
- [x] `TestOpenCode_Send_Claude_OmitsZeroTemperature`: `req.Temperature = 0` → `"temperature"` key absent from body (§7.4)
- [x] `TestOpenCode_Send_Claude_EmptyContent`: server returns empty `content` array → `ProviderError` with `ErrCategoryServer`, message contains `"no content"` (§7.4)
- [x] `TestOpenCode_Send_Claude_AuthError`: server returns HTTP 401 → `ProviderError` with `ErrCategoryAuth` (§7.4)
- [x] `TestOpenCode_Send_Claude_RateLimit`: server returns HTTP 429 → `ProviderError` with `ErrCategoryRateLimit` (§7.4)
- [x] `TestOpenCode_Send_Claude_Overloaded`: server returns HTTP 529 → `ProviderError` with `ErrCategoryOverloaded` (§7.4)
- [x] `TestOpenCode_Send_Claude_ServerError`: server returns HTTP 500 → `ProviderError` with `ErrCategoryServer` (§7.4)
- [x] `TestOpenCode_Send_Claude_ErrorBodyParsed`: server returns HTTP 400 with Anthropic-style error JSON → `ProviderError.Message` from `error.message` field (§7.4)
- [x] `TestOpenCode_Send_Claude_ErrorBodyUnparseable`: server returns HTTP 400 with non-JSON body → `ProviderError.Message` falls back to HTTP status text (§7.4)

### OpenAI Responses Format Tests

- [x] `TestOpenCode_Send_GPT_Success`: `httptest.NewServer` returning valid Responses API response; verify `Response.Content`, `Response.Model`, `Response.InputTokens`, `Response.OutputTokens`, `Response.StopReason` populated (§7.4)
- [x] `TestOpenCode_Send_GPT_RequestFormat`: inspect incoming request — method=`POST`, path=`/v1/responses`, `Authorization: Bearer <key>`, no `anthropic-version` header, `Content-Type: application/json`, body contains `"model": "gpt-5"` and `"input"` array (not `"messages"`) (§7.4)
- [x] `TestOpenCode_Send_GPT_SystemPrompt`: non-empty `req.System` → system message prepended to `"input"` array; empty → no system message (§7.4)
- [x] `TestOpenCode_Send_GPT_OmitsZeroMaxOutputTokens`: `req.MaxTokens = 0` → `"max_output_tokens"` absent from body (§7.4)
- [x] `TestOpenCode_Send_GPT_IncludesMaxOutputTokens`: `req.MaxTokens = 500` → `"max_output_tokens": 500` present in body (§7.4)
- [x] `TestOpenCode_Send_GPT_EmptyOutput`: server returns empty `output` array → `ProviderError` with `ErrCategoryServer`, message contains `"no output"` (§7.4)
- [x] `TestOpenCode_Send_GPT_AuthError`: server returns HTTP 401 → `ProviderError` with `ErrCategoryAuth` (§7.4)
- [x] `TestOpenCode_Send_GPT_ForbiddenError`: server returns HTTP 403 → `ProviderError` with `ErrCategoryAuth` (§7.4)
- [x] `TestOpenCode_Send_GPT_NotFoundError`: server returns HTTP 404 → `ProviderError` with `ErrCategoryBadRequest` (§7.4)

### OpenAI Chat Completions Format Tests

- [x] `TestOpenCode_Send_ChatCompletions_Success`: `httptest.NewServer` returning valid Chat Completions response; verify `Response` fields populated correctly (§7.4)
- [x] `TestOpenCode_Send_ChatCompletions_RequestFormat`: inspect incoming request — method=`POST`, path=`/v1/chat/completions`, `Authorization: Bearer <key>`, no `anthropic-version` header, `Content-Type: application/json`, body contains `"model": "kimi-k2"` and `"messages"` array (§7.4)
- [x] `TestOpenCode_Send_ChatCompletions_EmptyChoices`: server returns empty `choices` → `ProviderError` with `ErrCategoryServer`, message contains `"no choices"` (§7.4)

### Routing Tests

- [x] `TestOpenCode_Send_RoutesClaudeToMessages`: `req.Model = "claude-sonnet-4-6"` → server receives POST to `/v1/messages` (§7.4)
- [x] `TestOpenCode_Send_RoutesGPTToResponses`: `req.Model = "gpt-5"` → server receives POST to `/v1/responses` (§7.4)
- [x] `TestOpenCode_Send_RoutesOtherToChatCompletions`: `req.Model = "kimi-k2"` → server receives POST to `/v1/chat/completions` (§7.4)
- [x] `TestOpenCode_Send_RoutesGeminiToChatCompletions`: `req.Model = "gemini-3-pro"` → server receives POST to `/v1/chat/completions` (not a Gemini-specific path) (§7.4)
- [x] `TestOpenCode_Send_CaseSensitiveRouting_ClaudeUppercase`: `req.Model = "CLAUDE-sonnet-4-6"` → server receives POST to `/v1/chat/completions` (§7.4)
- [x] `TestOpenCode_Send_CaseSensitiveRouting_GPTUppercase`: `req.Model = "GPT-5"` → server receives POST to `/v1/chat/completions` (§7.4)

### Shared Behavior Tests

- [x] `TestOpenCode_Send_ContextTimeout`: for each of the three routing paths, server blocks until context deadline; verify `ProviderError` with `ErrCategoryTimeout` (§7.4)
- [x] `TestOpenCode_Send_NoRedirectFollowing`: server returns HTTP 302 with `Location` header; verify provider does not follow redirect and returns non-nil error (§7.4)

---

## Phase 8: Verification

- [x] Run `make test` — all tests pass
- [x] Run `go mod tidy && git diff go.mod` — no new direct dependencies added
- [x] Confirm `cmd/run.go` is unchanged
- [x] Confirm `anthropic.go`, `openai.go`, `ollama.go` are unchanged
- [x] Confirm `provider.go` (Provider interface and core types) is unchanged
