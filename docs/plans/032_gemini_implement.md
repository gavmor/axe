# Implementation Guide: Google Gemini Provider Support

## 1. Context Summary

**Associated Milestone:** `docs/plans/000_milestones.md` — M4 (Multi-Provider Support)

Axe's multi-provider infrastructure (interface, registry, config resolution) is complete and stable. This implementation adds a Google Gemini provider to that existing infrastructure. The provider name is `"google"` (matching models.dev format), the API key env var is `GEMINI_API_KEY` (a non-standard mapping requiring explicit registration), and the Gemini REST API uses a fundamentally different wire format from OpenAI/Anthropic (contents/parts model, system_instruction as top-level field, model name in URL path, no tool call IDs). The implementation is stdlib-only, follows the same standalone-file pattern as Ollama, and requires zero changes to the frozen `Provider` interface, existing providers, `cmd/run.go`, or `go.mod`.

## 2. Implementation Checklist

### Phase 1: Test Infrastructure (Mock Server Helpers)

- [x] Add `GeminiResponse(text string) MockLLMResponse` to `internal/testutil/mockserver.go`: returns a valid Gemini `generateContent` JSON response with the given text, 200 status, usage metadata (`promptTokenCount: 10`, `candidatesTokenCount: 5`), `finishReason: "STOP"`, and `role: "model"`.
- [x] Add `GeminiToolCallResponse(text string, toolCalls []MockToolCall) MockLLMResponse` to `internal/testutil/mockserver.go`: returns a Gemini response with both `text` parts and `functionCall` parts in `candidates[0].content.parts[]`. Each `MockToolCall.Input` becomes `functionCall.args`. Status 200.
- [x] Add `GeminiErrorResponse(statusCode int, message string) MockLLMResponse` to `internal/testutil/mockserver.go`: returns a Gemini error shape `{"error": {"code": <statusCode>, "message": "<message>", "status": "ERROR"}}` with the given status code.

### Phase 2: Config Integration

- [x] Add `"google": "GEMINI_API_KEY"` entry to the `knownAPIKeyEnvVars` map in `internal/config/config.go`.
- [x] Add test `TestAPIKeyEnvVar_Google` in `internal/config/config_test.go`: assert `APIKeyEnvVar("google")` returns `"GEMINI_API_KEY"`.
- [x] Add test `TestResolveAPIKey_Google` in `internal/config/config_test.go`: verify env var `GEMINI_API_KEY` takes precedence over `providers.google.api_key` in config; verify config fallback when env var is empty; verify empty string when neither is set.

### Phase 3: Gemini Provider — Constructor and Wire Types

- [x] Create `internal/provider/gemini.go` with: named constant `defaultGeminiBaseURL = "https://generativelanguage.googleapis.com"`, functional option type `GeminiOption func(*Gemini)`, option function `WithGeminiBaseURL(url string) GeminiOption`, unexported struct `Gemini` with fields `apiKey string`, `baseURL string`, `client *http.Client`.
- [x] Implement `NewGemini(apiKey string, opts ...GeminiOption) (*Gemini, error)` in `internal/provider/gemini.go`: return error `"API key is required"` if apiKey is empty; initialize `http.Client` with `CheckRedirect` returning `http.ErrUseLastResponse`; apply functional options.
- [x] Define all unexported Gemini wire types in `internal/provider/gemini.go`: `geminiRequest`, `geminiContent`, `geminiPart`, `geminiFunctionCall`, `geminiFunctionResponse`, `geminiFunctionResponsePayload`, `geminiSystemInstruction`, `geminiToolDef`, `geminiFunctionDeclaration`, `geminiGenerationConfig`, `geminiResponse`, `geminiCandidate`, `geminiUsageMetadata`, `geminiErrorResponse`, `geminiErrorDetail`. Use `json:"...,omitempty"` tags to ensure optional fields are omitted when zero-valued.
- [x] Add test `TestNewGemini_EmptyAPIKey` in `internal/provider/gemini_test.go`: verify empty API key returns error containing `"API key is required"`.
- [x] Add test `TestNewGemini_ValidAPIKey` in `internal/provider/gemini_test.go`: verify non-empty API key returns `*Gemini` with no error.

### Phase 4: Gemini Provider — Message Conversion

- [x] Implement `convertToGeminiContents(msgs []Message) []geminiContent` in `internal/provider/gemini.go`: map `"assistant"` role to `"model"`, map `"user"` role to `"user"`, convert text messages to `parts[].text`, convert assistant messages with `ToolCalls` to `parts[].functionCall`, convert tool-result messages (`role == "tool"`) to `role: "user"` with `parts[].functionResponse`. Build a `callID→toolName` map from preceding assistant tool calls to resolve tool result names; fall back to `CallID` as name if unresolved.
- [x] Implement `convertToGeminiTools(tools []Tool) []geminiToolDef` in `internal/provider/gemini.go`: map each `Tool` to a `geminiFunctionDeclaration` with `name`, `description`, and `parameters` (JSON Schema object with `type: "object"`, `properties`, `required`).

### Phase 5: Gemini Provider — Send Method

- [x] Implement `(g *Gemini) Send(ctx context.Context, req *Request) (*Response, error)` in `internal/provider/gemini.go`: build `geminiRequest` from `*Request` (system_instruction when System non-empty, contents from Messages, tools when Tools non-empty, generationConfig when Temperature or MaxTokens non-zero), marshal to JSON, POST to `{baseURL}/v1beta/models/{model}:generateContent`, set headers `x-goog-api-key` and `Content-Type: application/json`.
- [x] Implement context cancellation handling in `Send`: if `client.Do` returns error and `ctx.Err() != nil`, return `ProviderError{Category: ErrCategoryTimeout}`.
- [x] Implement HTTP error handling in `Send` via `(g *Gemini) handleErrorResponse(status int, body []byte) *ProviderError`: parse Gemini error JSON for message, fall back to `http.StatusText(status)`.
- [x] Implement `(g *Gemini) mapStatusToCategory(status int) ErrorCategory` in `internal/provider/gemini.go`: 401/403→`ErrCategoryAuth`, 400/404→`ErrCategoryBadRequest`, 429→`ErrCategoryRateLimit`, 500/502/503→`ErrCategoryServer`, 529→`ErrCategoryOverloaded`, all other→`ErrCategoryServer`.
- [x] Implement response parsing in `Send`: check for empty candidates (return `ProviderError{Category: ErrCategoryServer, Message: "response contains no candidates"}`), concatenate text parts into `Response.Content`, parse `functionCall` parts into `Response.ToolCalls` with synthetic IDs `gemini_0`, `gemini_1`, etc., set `Response.Model` from `req.Model`, map usage metadata and finish reason.

### Phase 6: Gemini Provider — Tests

- [x] Add test `TestGemini_Send_Success` in `internal/provider/gemini_test.go`: use `testutil.NewMockLLMServer` with `GeminiResponse`; verify `Response.Content`, `Response.Model` (set from request), `Response.InputTokens`, `Response.OutputTokens`, `Response.StopReason`.
- [x] Add test `TestGemini_Send_RequestFormat` in `internal/provider/gemini_test.go`: inspect captured request method (POST), URL path (`/v1beta/models/<model>:generateContent`), headers (`x-goog-api-key`, `Content-Type`), and body JSON structure.
- [x] Add test `TestGemini_Send_SystemInstruction` in `internal/provider/gemini_test.go`: verify system prompt appears as `system_instruction.parts[0].text` in request body, not in `contents`.
- [x] Add test `TestGemini_Send_OmitsEmptySystem` in `internal/provider/gemini_test.go`: verify `system_instruction` key absent from request body when `Request.System` is empty.
- [x] Add test `TestGemini_Send_OmitsGenerationConfig` in `internal/provider/gemini_test.go`: verify `generationConfig` key absent when both `Temperature` and `MaxTokens` are 0.
- [x] Add test `TestGemini_Send_OmitsZeroTemperature` in `internal/provider/gemini_test.go`: verify `temperature` absent from `generationConfig` when `Temperature` is 0 but `MaxTokens` is non-zero.
- [x] Add test `TestGemini_Send_IncludesMaxTokens` in `internal/provider/gemini_test.go`: verify `generationConfig.maxOutputTokens` present when `MaxTokens` non-zero.
- [x] Add test `TestGemini_Send_IncludesTemperature` in `internal/provider/gemini_test.go`: verify `generationConfig.temperature` present when `Temperature` non-zero.
- [x] Add test `TestGemini_Send_RoleMapping` in `internal/provider/gemini_test.go`: verify assistant messages sent as `role: "model"` in request body.
- [x] Add test `TestGemini_Send_ToolCallResponse` in `internal/provider/gemini_test.go`: use `GeminiToolCallResponse`; verify `Response.ToolCalls` parsed correctly with synthetic IDs (`gemini_0`, `gemini_1`).
- [x] Add test `TestGemini_Send_ToolDefinitions` in `internal/provider/gemini_test.go`: verify `tools[0].functionDeclarations` in request body matches `Request.Tools`.
- [x] Add test `TestGemini_Send_ToolResults` in `internal/provider/gemini_test.go`: send messages with tool results; verify request body contains `functionResponse` parts with `role: "user"` and correct tool names resolved from preceding assistant tool calls.
- [x] Add test `TestGemini_Send_MixedContentAndToolCalls` in `internal/provider/gemini_test.go`: response with both text and functionCall parts; verify `Response.Content` has concatenated text and `Response.ToolCalls` has parsed tool calls.
- [x] Add test `TestGemini_Send_AuthError` in `internal/provider/gemini_test.go`: 401 status → `ProviderError` with `ErrCategoryAuth`.
- [x] Add test `TestGemini_Send_ForbiddenError` in `internal/provider/gemini_test.go`: 403 status → `ProviderError` with `ErrCategoryAuth`.
- [x] Add test `TestGemini_Send_NotFoundError` in `internal/provider/gemini_test.go`: 404 status → `ProviderError` with `ErrCategoryBadRequest`.
- [x] Add test `TestGemini_Send_RateLimitError` in `internal/provider/gemini_test.go`: 429 status → `ProviderError` with `ErrCategoryRateLimit`.
- [x] Add test `TestGemini_Send_ServerError` in `internal/provider/gemini_test.go`: 500 status → `ProviderError` with `ErrCategoryServer`.
- [x] Add test `TestGemini_Send_Timeout` in `internal/provider/gemini_test.go`: use cancelled context; verify `ProviderError` with `ErrCategoryTimeout`.
- [x] Add test `TestGemini_Send_EmptyCandidates` in `internal/provider/gemini_test.go`: response with empty `candidates` array → `ProviderError` with `ErrCategoryServer` and message `"response contains no candidates"`.
- [x] Add test `TestGemini_Send_ErrorResponseParsing` in `internal/provider/gemini_test.go`: use `GeminiErrorResponse`; verify `ProviderError.Message` contains the error message from JSON.
- [x] Add test `TestGemini_Send_UnparseableErrorBody` in `internal/provider/gemini_test.go`: non-JSON error body → `ProviderError.Message` falls back to HTTP status text.
- [x] Add test `TestGemini_Send_ZeroTokenCounts` in `internal/provider/gemini_test.go`: response with missing/zero `usageMetadata` → `Response.InputTokens` and `Response.OutputTokens` are 0, no error.

### Phase 7: Registry Integration

- [x] Add `"google": true` to the `supportedProviders` map in `internal/provider/registry.go`.
- [x] Add `case "google":` to the `New()` switch in `internal/provider/registry.go`: construct `[]GeminiOption`, append `WithGeminiBaseURL(baseURL)` when baseURL non-empty, call `NewGemini(apiKey, opts...)`.
- [x] Update the default error message in `New()` in `internal/provider/registry.go` to include `google` in the list: `"anthropic, openai, ollama, opencode, google"`.
- [x] Add test `TestNew_Google` in `internal/provider/registry_test.go`: `New("google", "test-key", "")` returns non-nil provider, no error.
- [x] Add test `TestNew_GoogleWithBaseURL` in `internal/provider/registry_test.go`: `New("google", "test-key", "http://custom:8080")` succeeds.
- [x] Add test `TestNew_GoogleMissingAPIKey` in `internal/provider/registry_test.go`: `New("google", "", "")` returns error containing `"API key is required"`.
- [x] Add test `TestSupported_Google` in `internal/provider/registry_test.go`: `Supported("google")` returns `true`.
- [x] Update `TestSupported_KnownProviders` in `internal/provider/registry_test.go`: add `"google"` to the provider name list.
- [x] Update `TestNew_UnsupportedProvider` in `internal/provider/registry_test.go`: change assertion to check for `"anthropic, openai, ollama, opencode, google"`.
- [x] Update `TestNew_UnsupportedProvider_ErrorMessage` in `internal/provider/registry_test.go`: add `"google"` to the list of names that must appear in the error message.

### Phase 8: Final Verification

- [x] Run `make test` — all tests pass, no new entries in `go.mod`, no real HTTP requests.
