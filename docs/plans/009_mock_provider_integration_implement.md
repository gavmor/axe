# Implementation Checklist: Mock Provider Integration Tests (Phase 2)

**Spec:** `docs/plans/009_mock_provider_integration_spec.md`
**Status:** In Progress

---

## 1. MockLLMServer Core Types & Constructor (`internal/testutil/mockserver.go`)

- [x] Create `internal/testutil/mockserver.go` with `package testutil` declaration and imports (`net/http`, `net/http/httptest`, `sync`, `testing`, `io`, `time`) (Req 1.1)
- [x] Define `MockLLMResponse` struct with `StatusCode int` and `Body string` (Req 1.2)
- [x] Define `MockLLMRequest` struct with `Method`, `Path`, `Headers`, `Body` fields (Req 1.4)
- [x] Define `MockLLMServer` struct with `Server`, `Requests`, `mu`, `responses`, `callIndex` fields (Req 1.3)
- [x] Implement `NewMockLLMServer(t *testing.T, responses []MockLLMResponse) *MockLLMServer` — creates httptest server, records requests, pops responses from queue, fatals on overflow, registers cleanup (Req 1.5)
- [x] Handle `StatusCode == -1` sentinel in the handler for slow responses — parse duration from Body, sleep with context awareness, return 200 with empty Anthropic response (Req 4.1 handler portion)
- [x] Implement `URL() string` convenience method (Req 1.6)
- [x] Implement `RequestCount() int` thread-safe method (Req 1.7)

## 2. MockLLMServer Unit Tests — Red Phase (`internal/testutil/mockserver_test.go`)

- [x] Create `internal/testutil/mockserver_test.go` with `package testutil` declaration
- [x] Write failing test `TestNewMockLLMServer_ServesResponses` — 2 responses, 2 requests, verify status and body (Req 8.2)
- [x] Write failing test `TestNewMockLLMServer_CapturesRequests` — 1 response, POST with JSON body, verify Requests[0] fields (Req 8.2)
- [x] Write failing test `TestNewMockLLMServer_RequestCount` — 3 responses, 2 requests, verify count is 2 (Req 8.2)

## 3. MockLLMServer Unit Tests — Green Phase

- [x] Verify all 3 mock server core tests pass with `make test`

## 4. Anthropic Response Helpers (`internal/testutil/mockserver.go`)

- [x] Define `MockToolCall` struct with `ID`, `Name`, `Input` fields (Req 2.2)
- [x] Implement `AnthropicResponse(text string) MockLLMResponse` — 200 with valid Anthropic message JSON (Req 2.1)
- [x] Implement `AnthropicToolUseResponse(text string, toolCalls []MockToolCall) MockLLMResponse` — 200 with text + tool_use blocks, stop_reason "tool_use" (Req 2.2)
- [x] Implement `AnthropicErrorResponse(statusCode int, errType, message string) MockLLMResponse` — error shape JSON (Req 2.3)

## 5. Anthropic Helper Unit Tests — Red then Green

- [x] Write failing test `TestAnthropicResponse_ValidJSON` — parse body, verify fields and text content (Req 8.2)
- [x] Write failing test `TestAnthropicToolUseResponse_ValidJSON` — parse body, verify stop_reason and tool_use block (Req 8.2)
- [x] Write failing test `TestAnthropicErrorResponse_ValidJSON` — verify status code and error body structure (Req 8.2)
- [x] Verify all Anthropic helper tests pass with `make test`

## 6. OpenAI Response Helpers (`internal/testutil/mockserver.go`)

- [x] Implement `OpenAIResponse(text string) MockLLMResponse` — 200 with valid OpenAI chat completions JSON (Req 3.1)
- [x] Implement `OpenAIToolCallResponse(text string, toolCalls []MockToolCall) MockLLMResponse` — 200 with tool_calls, arguments as JSON string (Req 3.2)
- [x] Implement `OpenAIErrorResponse(statusCode int, errType, message string) MockLLMResponse` — error shape JSON (Req 3.3)

## 7. OpenAI Helper Unit Tests — Red then Green

- [x] Write failing test `TestOpenAIResponse_ValidJSON` — parse body, verify choices and finish_reason (Req 8.2)
- [x] Write failing test `TestOpenAIToolCallResponse_ValidJSON` — parse body, verify function call structure, arguments is JSON string (Req 8.2)
- [x] Write failing test `TestOpenAIErrorResponse_ValidJSON` — verify status code and error body structure (Req 8.2)
- [x] Verify all OpenAI helper tests pass with `make test`

## 8. SlowResponse Helper (`internal/testutil/mockserver.go`)

- [x] Implement `SlowResponse(delay time.Duration) MockLLMResponse` — returns StatusCode -1 with duration in Body (Req 4.1)

## 9. SlowResponse Unit Tests — Red then Green

- [x] Write failing test `TestSlowResponse_Delays` — send request with 2s timeout, verify response arrives after ~500ms (Req 8.2)
- [x] Write failing test `TestSlowResponse_InterruptedByCancel` — send request with 100ms context timeout, verify context deadline error before 1s (Req 8.2)
- [x] Verify both SlowResponse tests pass with `make test`

## 10. Integration Test File Setup (`cmd/run_integration_test.go`)

- [ ] Create `cmd/run_integration_test.go` with `package cmd` declaration and imports
- [ ] Implement file-local `writeAgentConfig(t *testing.T, configDir, name, toml string)` helper (Req 5.0)

## 11. Single-Shot Run Tests — Red then Green

- [ ] Write failing test `TestIntegration_SingleShot_Anthropic` — verify stdout, request count, path, method, model, default user message (Req 5.1)
- [ ] Write failing test `TestIntegration_SingleShot_OpenAI` — verify stdout, request count, path, model (Req 5.2)
- [ ] Write failing test `TestIntegration_SingleShot_StdinPiped` — verify stdin override replaces default message (Req 5.3)
- [ ] Make all 3 single-shot tests pass with `make test`

## 12. Conversation Loop Tests — Red then Green

- [ ] Write failing test `TestIntegration_ConversationLoop_SingleToolCall` — 3-response queue, verify stdout, request count, sub-agent task, tool result (Req 5.4)
- [ ] Write failing test `TestIntegration_ConversationLoop_MultipleRoundTrips` — 5-response queue, verify request count (Req 5.5)
- [ ] Write failing test `TestIntegration_ConversationLoop_MaxTurnsExceeded` — 51 tool-call responses, verify ExitError Code 1 and error message (Req 5.6)
- [ ] Make all 3 conversation loop tests pass with `make test`

## 13. Sub-Agent Orchestration Tests — Red then Green

- [ ] Write failing test `TestIntegration_SubAgent_DepthLimit` — max_depth=1, child has no tools injected, verify request count and missing tools (Req 5.7)
- [ ] Write failing test `TestIntegration_SubAgent_ParallelExecution` — two simultaneous tool calls, verify request count (Req 5.8)
- [ ] Write failing test `TestIntegration_SubAgent_SequentialExecution` — parallel=false, verify deterministic request order (Req 5.9)
- [ ] Make all 3 sub-agent orchestration tests pass with `make test`

## 14. Memory Tests — Red then Green

- [ ] Write failing test `TestIntegration_MemoryAppend_AfterSuccessfulRun` — verify memory file created with entry header, task, result (Req 5.10)
- [ ] Write failing test `TestIntegration_MemoryAppend_NotOnError` — verify memory file does NOT exist after provider error (Req 5.11)
- [ ] Write failing test `TestIntegration_MemoryLoad_IntoSystemPrompt` — pre-seed memory, verify last_n entries appear in request body (Req 5.12)
- [ ] Make all 3 memory tests pass with `make test`

## 15. JSON Output Tests — Red then Green

- [ ] Write failing test `TestIntegration_JSONOutput_Structure` — verify valid JSON with all required fields, correct values (Req 5.13)
- [ ] Write failing test `TestIntegration_JSONOutput_WithToolCalls` — verify tool_calls count and token sums (Req 5.14)
- [ ] Make both JSON output tests pass with `make test`

## 16. Timeout Handling Test — Red then Green

- [ ] Write failing test `TestIntegration_Timeout_ContextDeadlineExceeded` — SlowResponse with --timeout 1, verify ExitError Code 3 (Req 5.15)
- [ ] Make timeout test pass with `make test`

## 17. Error Mapping Tests — Red then Green

- [ ] Write failing test `TestIntegration_ErrorMapping_Auth401` — Anthropic 401, verify ExitError Code 3 (Req 5.16)
- [ ] Write failing test `TestIntegration_ErrorMapping_RateLimit429` — Anthropic 429, verify ExitError Code 3 (Req 5.17)
- [ ] Write failing test `TestIntegration_ErrorMapping_Server500` — Anthropic 500, verify ExitError Code 3 (Req 5.18)
- [ ] Write failing test `TestIntegration_ErrorMapping_OpenAI401` — OpenAI 401, verify ExitError Code 3 (Req 5.19)
- [ ] Write failing test `TestIntegration_ErrorMapping_OpenAI429` — OpenAI 429, verify ExitError Code 3 (Req 5.20)
- [ ] Write failing test `TestIntegration_ErrorMapping_OpenAI500` — OpenAI 500, verify ExitError Code 3 (Req 5.21)
- [ ] Make all 6 error mapping tests pass with `make test`

## 18. Final Validation

- [ ] Run full `make test` — all tests pass, including pre-existing tests
- [ ] Verify no existing files were modified (only new files added)
- [ ] Verify `go.mod` is unchanged (no new dependencies)
- [ ] Verify `internal/testutil/` is not imported by any non-test file
