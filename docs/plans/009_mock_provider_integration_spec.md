# Specification: Mock Provider Integration Tests (Phase 2)

**Status:** Draft
**Version:** 1.0
**Created:** 2026-03-01
**Scope:** Reusable mock LLM servers, full `axe run` integration tests with mocked providers

---

## 1. Purpose

Test the full `axe run` flow — from cobra command entry through agent config loading, context resolution, provider dispatch, conversation loop, output formatting, and memory append — without hitting real LLM APIs. All provider HTTP traffic is intercepted by `httptest` mock servers that return canned responses in the correct wire format.

This phase validates that the real code paths work correctly end-to-end. Every test calls the actual `runAgent` function through cobra's `rootCmd.Execute()`. The only thing replaced is the remote HTTP endpoint.

---

## 2. Design Decisions

The following decisions were made during planning and are binding for implementation:

1. **Reusable mock server in `internal/testutil/`.** A new `MockLLMServer` type is created in `internal/testutil/` that can serve both OpenAI and Anthropic response shapes. This replaces the pattern of per-test inline `httptest.NewServer` calls for new Phase 2 tests. Existing tests in `cmd/run_test.go` are not modified.

2. **Integration test file location:** All Phase 2 integration tests live in a new file `cmd/run_integration_test.go` (package `cmd`). This file imports `internal/testutil` for the mock server and XDG helpers.

3. **Provider scope: OpenAI and Anthropic only.** Ollama mock support is deferred to a future phase. The milestone explicitly lists only OpenAI and Anthropic.

4. **Mock server is configurable via response queue.** The `MockLLMServer` accepts a sequence of canned responses. Each `Send` request pops the next response from the queue. This supports multi-turn conversation loop testing without closures or mutable counter variables.

5. **Tests exercise real code paths.** No mocking of internal Go functions. The `provider.Send()` implementation, HTTP serialization, error mapping, conversation loop, tool dispatch, memory append, and JSON output are all exercised through the actual code. The mock server is the only fake.

6. **Standard library only.** No test frameworks. Continues the project convention.

7. **Follow actual exit code mapping.** The code maps: auth (401) → exit 3, rate limit (429) → exit 3, server error (500) → exit 3, bad request (400) → exit 1. Tests assert against these actual mappings.

8. **Test isolation via `testutil.SetupXDGDirs`.** Every integration test uses the Phase 1 `SetupXDGDirs` helper for XDG isolation. Agent configs are written inline in each test (not using `SeedFixtureAgents`) to keep each test self-documenting and independent.

---

## 3. Requirements

### 3.1 Reusable Mock LLM Server (`internal/testutil/`)

**Requirement 1.1:** Add a new file `internal/testutil/mockserver.go` containing the `MockLLMServer` type. This file is in the existing `testutil` package.

**Requirement 1.2:** Define `MockLLMResponse`:

```go
type MockLLMResponse struct {
    StatusCode int
    Body       string
}
```

`StatusCode` is the HTTP status code to return (e.g., 200, 401, 429, 500). `Body` is the raw JSON response body string. The caller is responsible for providing valid JSON in the correct provider wire format.

**Requirement 1.3:** Define `MockLLMServer`:

```go
type MockLLMServer struct {
    Server    *httptest.Server
    Requests  []MockLLMRequest
    mu        sync.Mutex
    responses []MockLLMResponse
    callIndex int
}
```

`Server` is the underlying `httptest.Server`. `Requests` is a slice that captures every request received (method, path, headers, body). `responses` is the queue of canned responses. `callIndex` tracks which response to serve next.

**Requirement 1.4:** Define `MockLLMRequest`:

```go
type MockLLMRequest struct {
    Method  string
    Path    string
    Headers http.Header
    Body    string
}
```

Every HTTP request received by the mock server is recorded into the `Requests` slice. `Body` is the raw request body as a string.

**Requirement 1.5:** Implement `NewMockLLMServer`:

```go
func NewMockLLMServer(t *testing.T, responses []MockLLMResponse) *MockLLMServer
```

Behavior:
- Call `t.Helper()`.
- Create a `MockLLMServer` with the provided response queue.
- Start an `httptest.NewServer` with a handler that:
  - Reads the full request body.
  - Records the request into `Requests` (protected by `mu`).
  - Pops the next `MockLLMResponse` from the queue (by incrementing `callIndex`).
  - Sets `Content-Type: application/json` header.
  - Writes the response `StatusCode` and `Body`.
  - If `callIndex` exceeds the queue length, calls `t.Fatalf("mock server received unexpected request #%d (only %d responses queued)")`.
- Register `t.Cleanup(server.Close)` so the server is automatically shut down when the test ends.
- Return the `MockLLMServer`.

**Requirement 1.6:** Implement `URL`:

```go
func (m *MockLLMServer) URL() string
```

Returns `m.Server.URL`. Convenience accessor so callers write `mock.URL()` instead of `mock.Server.URL`.

**Requirement 1.7:** Implement `RequestCount`:

```go
func (m *MockLLMServer) RequestCount() int
```

Returns the current `callIndex` (number of requests served). Thread-safe via `mu`.

**Requirement 1.8:** The mock server must handle requests to any path. It does not validate that the path matches `/v1/chat/completions` or `/v1/messages`. The tests use the mock for both OpenAI and Anthropic by simply changing the response body format. Path validation is the provider implementation's responsibility, not the mock's.

### 3.2 Anthropic Response Helpers (`internal/testutil/mockserver.go`)

**Requirement 2.1:** Implement `AnthropicResponse`:

```go
func AnthropicResponse(text string) MockLLMResponse
```

Returns a `MockLLMResponse` with `StatusCode: 200` and a `Body` containing a valid Anthropic messages API response:

```json
{
  "id": "msg_mock",
  "type": "message",
  "role": "assistant",
  "content": [{"type": "text", "text": "<text>"}],
  "model": "claude-sonnet-4-20250514",
  "stop_reason": "end_turn",
  "usage": {"input_tokens": 10, "output_tokens": 5}
}
```

The `<text>` placeholder is replaced with the `text` parameter. Token counts are fixed placeholder values. The model is hardcoded to `claude-sonnet-4-20250514`.

**Requirement 2.2:** Implement `AnthropicToolUseResponse`:

```go
func AnthropicToolUseResponse(text string, toolCalls []MockToolCall) MockLLMResponse
```

Where `MockToolCall` is:

```go
type MockToolCall struct {
    ID    string
    Name  string
    Input map[string]string
}
```

Returns a `MockLLMResponse` with `StatusCode: 200` and a body containing an Anthropic response with both a text content block and one or more `tool_use` content blocks. `stop_reason` is `"tool_use"`. Example body for one tool call:

```json
{
  "id": "msg_mock",
  "type": "message",
  "role": "assistant",
  "content": [
    {"type": "text", "text": "<text>"},
    {"type": "tool_use", "id": "<toolCalls[0].ID>", "name": "<toolCalls[0].Name>", "input": {"agent": "helper", "task": "do work"}}
  ],
  "model": "claude-sonnet-4-20250514",
  "stop_reason": "tool_use",
  "usage": {"input_tokens": 10, "output_tokens": 20}
}
```

**Requirement 2.3:** Implement `AnthropicErrorResponse`:

```go
func AnthropicErrorResponse(statusCode int, errType, message string) MockLLMResponse
```

Returns a `MockLLMResponse` with the given `StatusCode` and a body matching the Anthropic error shape:

```json
{
  "type": "error",
  "error": {
    "type": "<errType>",
    "message": "<message>"
  }
}
```

### 3.3 OpenAI Response Helpers (`internal/testutil/mockserver.go`)

**Requirement 3.1:** Implement `OpenAIResponse`:

```go
func OpenAIResponse(text string) MockLLMResponse
```

Returns a `MockLLMResponse` with `StatusCode: 200` and a body containing a valid OpenAI chat completions response:

```json
{
  "model": "gpt-4o",
  "choices": [
    {
      "message": {"content": "<text>"},
      "finish_reason": "stop"
    }
  ],
  "usage": {"prompt_tokens": 10, "completion_tokens": 5}
}
```

**Requirement 3.2:** Implement `OpenAIToolCallResponse`:

```go
func OpenAIToolCallResponse(text string, toolCalls []MockToolCall) MockLLMResponse
```

Returns a `MockLLMResponse` with `StatusCode: 200` and a body containing an OpenAI response with tool calls. The `message.content` is `null` (or the provided text), and `tool_calls` contains function call objects. `finish_reason` is `"tool_calls"`. Each tool call's `arguments` is a JSON-encoded string of the `Input` map (matching OpenAI's wire format where arguments is a JSON string, not an object).

```json
{
  "model": "gpt-4o",
  "choices": [
    {
      "message": {
        "content": null,
        "tool_calls": [
          {
            "id": "<toolCalls[0].ID>",
            "type": "function",
            "function": {
              "name": "<toolCalls[0].Name>",
              "arguments": "{\"agent\":\"helper\",\"task\":\"do work\"}"
            }
          }
        ]
      },
      "finish_reason": "tool_calls"
    }
  ],
  "usage": {"prompt_tokens": 10, "completion_tokens": 20}
}
```

**Requirement 3.3:** Implement `OpenAIErrorResponse`:

```go
func OpenAIErrorResponse(statusCode int, errType, message string) MockLLMResponse
```

Returns a `MockLLMResponse` with the given `StatusCode` and a body matching the OpenAI error shape:

```json
{
  "error": {
    "message": "<message>",
    "type": "<errType>",
    "code": "<errType>"
  }
}
```

### 3.4 Slow Response Helper (`internal/testutil/mockserver.go`)

**Requirement 4.1:** Implement `SlowResponse`:

```go
func SlowResponse(delay time.Duration) MockLLMResponse
```

Returns a `MockLLMResponse` with a special sentinel `StatusCode` of `-1` and the delay encoded in the `Body` field as a duration string. When the mock server handler encounters `StatusCode == -1`, it sleeps for the parsed delay duration before returning a 200 response with an empty valid Anthropic response body. This is used for timeout testing.

The handler must check `StatusCode == -1` before writing headers. The sleep must be interruptible by context cancellation (use `time.After` with `select` on `r.Context().Done()` or simply `time.Sleep` since the HTTP server will close the connection on client context cancel).

---

## 4. Integration Tests

All tests live in `cmd/run_integration_test.go`, package `cmd`.

### 4.1 Test Setup Pattern

Every test in this file follows this pattern:

1. Call `resetRunCmd(t)` to reset cobra flags.
2. Create a `MockLLMServer` with the appropriate response queue.
3. Call `testutil.SetupXDGDirs(t)` to create isolated XDG directories.
4. Write agent TOML config to `configDir/agents/<name>.toml`.
5. Write global config to `configDir/config.toml` if needed (e.g., for API key via config file tests), or set env vars via `t.Setenv`.
6. Set `AXE_<PROVIDER>_BASE_URL` to `mock.URL()` via `t.Setenv`.
7. Set API key via `t.Setenv` (e.g., `ANTHROPIC_API_KEY=test-key`).
8. Capture stdout/stderr via `bytes.Buffer` on `rootCmd.SetOut`/`rootCmd.SetErr`.
9. Set args via `rootCmd.SetArgs` and execute via `rootCmd.Execute()`.
10. Assert on: exit error / exit code, stdout content, stderr content, `mock.RequestCount()`, `mock.Requests` contents, filesystem state (memory files, etc.).

**Requirement 5.0:** Create a test-file-local helper:

```go
func writeAgentConfig(t *testing.T, configDir, name, toml string)
```

Behavior: calls `t.Helper()`, writes `toml` content to `configDir/agents/<name>.toml`. Calls `t.Fatal` on error. This avoids repeating `os.WriteFile` with path construction in every test.

### 4.2 Single-Shot Run Tests (No Tools)

**Requirement 5.1 — Test: `TestIntegration_SingleShot_Anthropic`**

Setup:
- Mock server queued with one `AnthropicResponse("Hello from Anthropic")`.
- Agent config: `name = "single-anthropic"`, `model = "anthropic/claude-sonnet-4-20250514"`.
- Env: `ANTHROPIC_API_KEY=test-key`, `AXE_ANTHROPIC_BASE_URL=<mock.URL()>`.

Assertions:
- `rootCmd.Execute()` returns `nil` (success, exit 0).
- stdout contains exactly `"Hello from Anthropic"`.
- `mock.RequestCount()` is `1`.
- `mock.Requests[0].Path` is `"/v1/messages"`.
- `mock.Requests[0].Method` is `"POST"`.
- Request body contains `"claude-sonnet-4-20250514"` as the model.
- Request body contains `"Execute the task described in your instructions."` as the user message content (the default when no stdin is piped).

**Requirement 5.2 — Test: `TestIntegration_SingleShot_OpenAI`**

Setup:
- Mock server queued with one `OpenAIResponse("Hello from OpenAI")`.
- Agent config: `name = "single-openai"`, `model = "openai/gpt-4o"`.
- Env: `OPENAI_API_KEY=test-key`, `AXE_OPENAI_BASE_URL=<mock.URL()>`.

Assertions:
- `rootCmd.Execute()` returns `nil`.
- stdout contains exactly `"Hello from OpenAI"`.
- `mock.RequestCount()` is `1`.
- `mock.Requests[0].Path` is `"/v1/chat/completions"`.
- Request body contains `"gpt-4o"` as the model.

**Requirement 5.3 — Test: `TestIntegration_SingleShot_StdinPiped`**

Setup:
- Mock server queued with one `AnthropicResponse("Processed your input")`.
- Agent config: basic anthropic agent.
- Override cobra stdin: `rootCmd.SetIn(strings.NewReader("user-provided input"))`.

Assertions:
- stdout contains `"Processed your input"`.
- Request body contains `"user-provided input"` as the user message (not the default message).

### 4.3 Conversation Loop Tests (Tool Calls)

**Requirement 5.4 — Test: `TestIntegration_ConversationLoop_SingleToolCall`**

Setup:
- Parent agent config: `sub_agents = ["helper-agent"]`, model = anthropic.
- Helper agent config: basic anthropic agent (no sub_agents).
- Mock server queued with 3 responses (both parent and helper use the same provider/base URL):
  1. `AnthropicToolUseResponse("Delegating.", [{ID: "toolu_1", Name: "call_agent", Input: {"agent": "helper-agent", "task": "say hello"}}])`
  2. `AnthropicResponse("hello from helper")` — served to the helper sub-agent.
  3. `AnthropicResponse("The helper said: hello from helper")` — served to the parent on the second turn.

Assertions:
- `rootCmd.Execute()` returns `nil`.
- stdout contains `"The helper said: hello from helper"`.
- `mock.RequestCount()` is `3`.
- The second request (index 1) body contains `"say hello"` as the user task for the helper.
- The third request (index 2) body contains the tool result from the helper.

**Requirement 5.5 — Test: `TestIntegration_ConversationLoop_MultipleRoundTrips`**

Setup:
- Agent config: with sub_agents.
- Mock server queued with 5 responses: tool call → helper response → tool call → helper response → final text response.

Assertions:
- `rootCmd.Execute()` returns `nil`.
- `mock.RequestCount()` is `5`.
- stdout contains the final response text.

**Requirement 5.6 — Test: `TestIntegration_ConversationLoop_MaxTurnsExceeded`**

Setup:
- Agent config: with sub_agents.
- Mock server queued with 51 responses, all returning tool calls (never a final text response).

Assertions:
- `rootCmd.Execute()` returns an error.
- Error is `ExitError` with `Code == 1`.
- Error message contains `"exceeded maximum conversation turns"`.

### 4.4 Sub-Agent Orchestration Tests

**Requirement 5.7 — Test: `TestIntegration_SubAgent_DepthLimit`**

Setup:
- Agent "depth-parent" with `sub_agents = ["depth-child"]`, `max_depth = 1`.
- Agent "depth-child" with `sub_agents = ["depth-grandchild"]`.
- Agent "depth-grandchild" basic agent.
- Mock server queued with:
  1. Parent response: tool call to "depth-child".
  2. Child response: tool call to "depth-grandchild" (this should fail because depth 1 is the max — the child runs at depth 1, which equals max_depth 1, so tool injection is skipped).
  3. Parent's second turn: final text response.

Actually, because `max_depth = 1`, the child agent runs at `depth = 1` which equals `effectiveMaxDepth`, so no tools are injected for the child. The child agent runs as a single-shot call. The mock server needs:
  1. Parent response: tool call to "depth-child".
  2. Child response: `AnthropicResponse("child result")` — child has no tools, so this is its only call.
  3. Parent second turn: `AnthropicResponse("Got: child result")`.

Assertions:
- `rootCmd.Execute()` returns `nil`.
- stdout contains `"Got: child result"`.
- `mock.RequestCount()` is `3`.
- The second request (child's request) body does NOT contain `"tools"` (no tools injected at depth 1).

**Requirement 5.8 — Test: `TestIntegration_SubAgent_ParallelExecution`**

Setup:
- Agent config with `sub_agents = ["worker-a", "worker-b"]`, `parallel = true` (or omitted, since default is `true`).
- Worker-a and worker-b are basic anthropic agents.
- Mock server queued with:
  1. Parent response: two simultaneous tool calls (one to worker-a, one to worker-b).
  2 & 3. Worker responses (order may vary due to parallelism): two `AnthropicResponse` calls.
  4. Parent second turn: final text.

Assertions:
- `rootCmd.Execute()` returns `nil`.
- `mock.RequestCount()` is `4`.
- stdout contains the final response text.

**Requirement 5.9 — Test: `TestIntegration_SubAgent_SequentialExecution`**

Setup:
- Agent config with `sub_agents = ["worker-a", "worker-b"]`, explicitly `parallel = false`.
- Mock server queued with 4 responses (same as parallel test).

Assertions:
- `rootCmd.Execute()` returns `nil`.
- `mock.RequestCount()` is `4`.
- The worker requests arrive in a deterministic order: worker-a request body appears before worker-b request body in `mock.Requests`. (In sequential mode, goroutines are not used, so request order matches tool call order.)

### 4.5 Memory Tests

**Requirement 5.10 — Test: `TestIntegration_MemoryAppend_AfterSuccessfulRun`**

Setup:
- Agent config: `name = "mem-agent"`, model = anthropic, `[memory]` with `enabled = true`.
- Mock server queued with one `AnthropicResponse("task completed")`.
- No pre-existing memory file.

Assertions:
- `rootCmd.Execute()` returns `nil`.
- Memory file exists at `<dataDir>/memory/mem-agent.md`.
- Memory file content contains `"## "` (an entry header with a timestamp).
- Memory file content contains `"**Task:**"`.
- Memory file content contains `"**Result:** task completed"`.

**Requirement 5.11 — Test: `TestIntegration_MemoryAppend_NotOnError`**

Setup:
- Agent config: memory enabled.
- Mock server queued with one `AnthropicErrorResponse(500, "server_error", "boom")`.

Assertions:
- `rootCmd.Execute()` returns an error (exit code 3).
- Memory file does NOT exist at the expected path. No memory was appended because the LLM call failed.

**Requirement 5.12 — Test: `TestIntegration_MemoryLoad_IntoSystemPrompt`**

Setup:
- Agent config: memory enabled, `last_n = 2`.
- Pre-seed memory file at `<dataDir>/memory/mem-load.md` with 3 entries.
- Mock server queued with one `AnthropicResponse("ok")`. Use a capture mechanism: inspect `mock.Requests[0].Body` after the test.

Assertions:
- `rootCmd.Execute()` returns `nil`.
- The request body sent to the mock contains the last 2 memory entries (verify by checking for entry timestamps or task text from the seeded memory file).
- The request body does NOT contain the first (oldest) memory entry.

### 4.6 JSON Output Tests

**Requirement 5.13 — Test: `TestIntegration_JSONOutput_Structure`**

Setup:
- Agent config: basic anthropic agent.
- Mock server queued with one `AnthropicResponse("json test output")`.
- Args include `--json`.

Assertions:
- `rootCmd.Execute()` returns `nil`.
- stdout is valid JSON.
- Parsed JSON contains all required fields: `"model"` (string), `"content"` (string), `"input_tokens"` (number), `"output_tokens"` (number), `"stop_reason"` (string), `"duration_ms"` (number), `"tool_calls"` (number).
- `content` equals `"json test output"`.
- `stop_reason` equals `"end_turn"`.
- `tool_calls` equals `0` (no tool calls in single-shot).
- `duration_ms` is `>= 0`.
- `input_tokens` equals `10` (from mock response).
- `output_tokens` equals `5` (from mock response).

**Requirement 5.14 — Test: `TestIntegration_JSONOutput_WithToolCalls`**

Setup:
- Agent config: with sub_agents.
- Mock server queued with: tool call response → helper response → final response.
- Args include `--json`.

Assertions:
- Parsed JSON `tool_calls` equals `1` (one tool call was executed).
- `input_tokens` is the sum across all turns (e.g., 10 + 5 + 10 = 25 if those are the mock values).
- `output_tokens` is the sum across all turns.

### 4.7 Timeout Handling Tests

**Requirement 5.15 — Test: `TestIntegration_Timeout_ContextDeadlineExceeded`**

Setup:
- Agent config: basic anthropic agent.
- Mock server queued with one `SlowResponse(5 * time.Second)`.
- Args include `--timeout 1` (1 second timeout).

Assertions:
- `rootCmd.Execute()` returns an error.
- Error is `ExitError` with `Code == 3`.
- Error message or error chain contains a reference to timeout or context deadline.

### 4.8 Error Mapping Tests

**Requirement 5.16 — Test: `TestIntegration_ErrorMapping_Auth401`**

Setup:
- Agent config: basic anthropic agent.
- Mock server queued with one `AnthropicErrorResponse(401, "authentication_error", "invalid api key")`.

Assertions:
- Error is `ExitError` with `Code == 3`.

**Requirement 5.17 — Test: `TestIntegration_ErrorMapping_RateLimit429`**

Setup:
- Agent config: basic anthropic agent.
- Mock server queued with one `AnthropicErrorResponse(429, "rate_limit_error", "too many requests")`.

Assertions:
- Error is `ExitError` with `Code == 3`.

**Requirement 5.18 — Test: `TestIntegration_ErrorMapping_Server500`**

Setup:
- Agent config: basic anthropic agent.
- Mock server queued with one `AnthropicErrorResponse(500, "server_error", "internal error")`.

Assertions:
- Error is `ExitError` with `Code == 3`.

**Requirement 5.19 — Test: `TestIntegration_ErrorMapping_OpenAI401`**

Setup:
- Agent config: basic openai agent.
- Mock server queued with one `OpenAIErrorResponse(401, "invalid_api_key", "invalid api key")`.

Assertions:
- Error is `ExitError` with `Code == 3`.

**Requirement 5.20 — Test: `TestIntegration_ErrorMapping_OpenAI429`**

Setup:
- Agent config: basic openai agent.
- Mock server queued with one `OpenAIErrorResponse(429, "rate_limit_exceeded", "rate limit")`.

Assertions:
- Error is `ExitError` with `Code == 3`.

**Requirement 5.21 — Test: `TestIntegration_ErrorMapping_OpenAI500`**

Setup:
- Agent config: basic openai agent.
- Mock server queued with one `OpenAIErrorResponse(500, "server_error", "internal error")`.

Assertions:
- Error is `ExitError` with `Code == 3`.

---

## 5. Project Structure

After this spec is implemented, the following files will be added:

```
axe/
+-- cmd/
|   +-- run_integration_test.go          # NEW: Phase 2 integration tests
+-- internal/
|   +-- testutil/
|       +-- mockserver.go                # NEW: MockLLMServer, response helpers
|       +-- mockserver_test.go           # NEW: tests for mock server
|       +-- testutil.go                  # UNCHANGED (Phase 1)
|       +-- testutil_test.go             # UNCHANGED (Phase 1)
```

No existing files are modified.

---

## 6. Edge Cases

### 6.1 MockLLMServer

| Scenario | Behavior |
|----------|----------|
| More requests than queued responses | `t.Fatalf` is called with a message identifying the request index and queue size. |
| Fewer requests than queued responses | Not an error. Unused responses are silently ignored. Tests assert `mock.RequestCount()` for verification. |
| Concurrent requests (parallel tool calls) | `mu` protects `Requests` append and `callIndex` increment. Each concurrent request gets the next sequential response. Order of response assignment to concurrent requests is non-deterministic. |
| Request body is empty | Recorded as empty string in `MockLLMRequest.Body`. Not an error. |
| SlowResponse interrupted by client disconnect | The mock handler's sleep may be interrupted when the HTTP server closes the connection. The response may or may not be written. The client receives a context deadline exceeded error. |
| `NewMockLLMServer` called with empty response slice | First request triggers `t.Fatalf`. |
| `NewMockLLMServer` called with `nil` response slice | Same as empty slice. First request triggers `t.Fatalf`. |

### 6.2 Integration Tests

| Scenario | Behavior |
|----------|----------|
| Sub-agent uses same `AXE_<PROVIDER>_BASE_URL` as parent | Both parent and sub-agent hit the same mock server. The response queue must include responses for both. Request order in the queue must match the expected call order. |
| Sub-agent uses different provider than parent | Not tested in this phase. All fixtures use the same provider for parent and sub-agents. |
| Memory file directory does not exist before append | The `memory.AppendEntry` function creates the directory via `os.MkdirAll`. Tests rely on this behavior. |
| Memory file content contains special characters | Not tested. Phase 2 uses simple ASCII strings in memory entries. |
| JSON output with no tool calls | `tool_calls` field is `0`, not absent. |
| JSON output: `duration_ms` precision | Value depends on wall-clock timing. Tests assert `>= 0`, not an exact value. |
| `--timeout 0` | Creates a context with zero-duration deadline that expires immediately. Not tested — this is a degenerate case. |
| Mock returns 200 with invalid JSON body | Provider `Send` returns an error (JSON unmarshal failure). Mapped to `ExitError{Code: 1}`. Not explicitly tested in this phase. |

### 6.3 Parallel Test Safety

| Scenario | Behavior |
|----------|----------|
| Integration tests run with `t.Parallel()` | NOT supported. Integration tests call `resetRunCmd(t)` which mutates the global `runCmd` cobra command. They also use `t.Setenv` which is process-global. Tests in `run_integration_test.go` must NOT use `t.Parallel()`. |
| Integration tests run alongside `cmd/run_test.go` tests | Safe. `go test ./cmd/` runs all test files in the `cmd` package sequentially within a single binary. Tests in different files share the same process but run sequentially (no `t.Parallel()`). |

---

## 7. Constraints

**Constraint 1:** No new external dependencies. `go.mod` must not change.

**Constraint 2:** The `internal/testutil/` package must not be imported by any production (non-test) Go file.

**Constraint 3:** Existing tests in `cmd/run_test.go` and other files must not be modified. Phase 2 tests are additive only.

**Constraint 4:** All Phase 2 tests must pass with zero real network calls. Every provider HTTP request hits an `httptest.Server`.

**Constraint 5:** No `t.Parallel()` in integration tests. Environment variables and global cobra command state prevent parallel execution.

**Constraint 6:** The `MockLLMServer` must be usable by future phases (Phase 3 CLI smoke tests, Phase 4 golden file tests). Its API must not be specific to Phase 2 test scenarios.

**Constraint 7:** Response helper functions (`AnthropicResponse`, `OpenAIResponse`, etc.) must produce valid JSON that the real provider implementations can parse without error. If a helper produces malformed JSON, it is a bug in the helper, not a test for the production code.

**Constraint 8:** Standard library only for all test code. No test frameworks.

**Constraint 9:** All helpers must call `t.Helper()` as their first statement.

**Constraint 10:** Cross-platform compatibility. Use `filepath.Join` for path construction.

---

## 8. Testing Requirements

### 8.1 Test Conventions

Follows the patterns established in Phases 1-8:

- **Package-level tests:** Tests live in the same package (`package testutil` for mock server tests, `package cmd` for integration tests).
- **Standard library only:** Use `testing` package. No test frameworks.
- **Temp directories:** Use `t.TempDir()` for filesystem isolation.
- **Env overrides:** Use `t.Setenv()` for environment variable control.
- **Descriptive names:** `TestFunctionName_Scenario` with underscores.
- **Test real code, not mocks.** Integration tests call the real `rootCmd.Execute()` which runs real `runAgent` code. The only fake is the HTTP endpoint.
- **Red/green TDD:** Write failing tests first, then implement code to make them pass.
- **Run tests with:** `make test`

### 8.2 Mock Server Tests (`internal/testutil/mockserver_test.go`)

**Test: `TestNewMockLLMServer_ServesResponses`** — Create mock with 2 responses. Send 2 HTTP requests. Verify each returns the correct status code and body.

**Test: `TestNewMockLLMServer_CapturesRequests`** — Create mock with 1 response. Send a POST with a JSON body. Verify `mock.Requests[0]` has correct Method, Path, Body.

**Test: `TestNewMockLLMServer_RequestCount`** — Create mock with 3 responses. Send 2 requests. Verify `mock.RequestCount()` is `2`.

**Test: `TestAnthropicResponse_ValidJSON`** — Call `AnthropicResponse("test")`. Parse the `Body` as JSON. Verify all required fields are present with correct types and the `text` content block contains `"test"`.

**Test: `TestAnthropicToolUseResponse_ValidJSON`** — Call `AnthropicToolUseResponse` with a tool call. Parse body. Verify `stop_reason` is `"tool_use"` and the tool_use content block is present with correct ID, name, and input.

**Test: `TestAnthropicErrorResponse_ValidJSON`** — Call `AnthropicErrorResponse(401, "authentication_error", "bad key")`. Verify status code is 401 and body parses to correct error structure.

**Test: `TestOpenAIResponse_ValidJSON`** — Call `OpenAIResponse("test")`. Parse body. Verify choices[0].message.content is `"test"` and finish_reason is `"stop"`.

**Test: `TestOpenAIToolCallResponse_ValidJSON`** — Call `OpenAIToolCallResponse` with a tool call. Parse body. Verify the function call structure and that arguments is a JSON string (not an object).

**Test: `TestOpenAIErrorResponse_ValidJSON`** — Call `OpenAIErrorResponse(500, "server_error", "boom")`. Verify status code and body structure.

**Test: `TestSlowResponse_Delays`** — Create mock with one `SlowResponse(500ms)`. Send request with a 2-second timeout. Verify response arrives after ~500ms (within tolerance). Verify response is a valid 200.

**Test: `TestSlowResponse_InterruptedByCancel`** — Create mock with one `SlowResponse(5s)`. Send request with a 100ms context timeout. Verify the client receives a context deadline exceeded error before 1 second.

### 8.3 Integration Tests (`cmd/run_integration_test.go`)

All tests listed in Section 4 (Requirements 5.1 through 5.21) are the integration test requirements. See Section 4 for full descriptions of each test's setup and assertions.

---

## 9. Acceptance Criteria

The milestone is complete when all of the following are true:

1. `make test` passes with zero failures.
2. `internal/testutil/mockserver.go` exists with `MockLLMServer`, `MockLLMResponse`, `MockLLMRequest`, `MockToolCall` types and all response helper functions.
3. `internal/testutil/mockserver_test.go` exists with tests for all mock server functionality.
4. `cmd/run_integration_test.go` exists with all 21 integration tests (Requirements 5.0 through 5.21).
5. Every integration test exercises the real `runAgent` code path through `rootCmd.Execute()`.
6. Zero network calls are made during test execution (all provider traffic hits `httptest.Server`).
7. No existing tests are broken.
8. No existing files are modified.
9. No new external dependencies are introduced.
10. The `internal/testutil/` package is not imported by any production code.

---

## 10. Out of Scope

The following items are explicitly **not** included in this spec:

1. Ollama mock server support (may be added in a future phase)
2. CLI smoke tests against the compiled binary (Phase 3)
3. Golden file comparison infrastructure (Phase 4)
4. GitHub Actions CI configuration (Phase 5)
5. Live provider tests (Phase 6)
6. Refactoring existing mock helpers in `cmd/run_test.go` (they remain as-is)
7. Test coverage metrics or reporting
8. Benchmarks
9. Testing invalid JSON responses from mock (provider-level unit tests cover this)
10. Cross-provider sub-agent tests (parent on Anthropic, child on OpenAI)

---

## 11. References

- Integration Testing Milestones: `docs/plans/000_i9n_milestones.md` (Phase 2)
- Phase 1 Spec: `docs/plans/008_i9n_test_infrastructure_spec.md`
- Provider interface: `internal/provider/provider.go` (lines 82-85)
- Provider error categories: `internal/provider/provider.go` (lines 9-24)
- Anthropic wire format: `internal/provider/anthropic.go` (request lines 65-72, response lines 102-116, error lines 119-125)
- OpenAI wire format: `internal/provider/openai.go` (request lines 58-64, response lines 101-114, error lines 117-123)
- Run command: `cmd/run.go` (lines 29-541)
- Error mapping: `cmd/run.go` (lines 528-541)
- Exit codes: `cmd/exit.go` (lines 6-22)
- JSON output envelope: `cmd/run.go` (lines 359-373)
- Conversation loop: `cmd/run.go` (lines 289-354)
- Sub-agent execution: `internal/tool/tool.go` (lines 65-281)
- Memory append: `internal/memory/memory.go` (lines 34-76)
- Existing mock helpers: `cmd/run_test.go` (lines 47-62, 615-641, 1690-1709)
- Phase 1 test helpers: `internal/testutil/testutil.go`
