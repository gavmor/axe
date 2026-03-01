package testutil

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Phase 2: MockLLMServer Core Tests

func TestNewMockLLMServer_ServesResponses(t *testing.T) {
	responses := []MockLLMResponse{
		{StatusCode: 200, Body: `{"response":"first"}`},
		{StatusCode: 201, Body: `{"response":"second"}`},
	}
	mock := NewMockLLMServer(t, responses)

	// First request
	resp1, err := http.Post(mock.URL()+"/v1/messages", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("unexpected error on first request: %v", err)
	}
	defer resp1.Body.Close()

	if resp1.StatusCode != 200 {
		t.Errorf("first response status: got %d, want 200", resp1.StatusCode)
	}
	body1, _ := io.ReadAll(resp1.Body)
	if string(body1) != `{"response":"first"}` {
		t.Errorf("first response body: got %q, want %q", string(body1), `{"response":"first"}`)
	}

	// Second request
	resp2, err := http.Post(mock.URL()+"/v1/messages", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("unexpected error on second request: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != 201 {
		t.Errorf("second response status: got %d, want 201", resp2.StatusCode)
	}
	body2, _ := io.ReadAll(resp2.Body)
	if string(body2) != `{"response":"second"}` {
		t.Errorf("second response body: got %q, want %q", string(body2), `{"response":"second"}`)
	}
}

func TestNewMockLLMServer_CapturesRequests(t *testing.T) {
	mock := NewMockLLMServer(t, []MockLLMResponse{
		{StatusCode: 200, Body: `{}`},
	})

	reqBody := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`
	resp, err := http.Post(mock.URL()+"/v1/messages", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if len(mock.Requests) != 1 {
		t.Fatalf("expected 1 captured request, got %d", len(mock.Requests))
	}

	captured := mock.Requests[0]
	if captured.Method != "POST" {
		t.Errorf("captured method: got %q, want %q", captured.Method, "POST")
	}
	if captured.Path != "/v1/messages" {
		t.Errorf("captured path: got %q, want %q", captured.Path, "/v1/messages")
	}
	if captured.Body != reqBody {
		t.Errorf("captured body: got %q, want %q", captured.Body, reqBody)
	}
}

func TestNewMockLLMServer_RequestCount(t *testing.T) {
	mock := NewMockLLMServer(t, []MockLLMResponse{
		{StatusCode: 200, Body: `{}`},
		{StatusCode: 200, Body: `{}`},
		{StatusCode: 200, Body: `{}`},
	})

	// Send 2 requests
	for i := 0; i < 2; i++ {
		resp, err := http.Post(mock.URL()+"/test", "application/json", strings.NewReader(`{}`))
		if err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
		resp.Body.Close()
	}

	if got := mock.RequestCount(); got != 2 {
		t.Errorf("RequestCount: got %d, want 2", got)
	}
}

// Phase 5: Anthropic Helper Tests

func TestAnthropicResponse_ValidJSON(t *testing.T) {
	resp := AnthropicResponse("test output")

	if resp.StatusCode != 200 {
		t.Errorf("StatusCode: got %d, want 200", resp.StatusCode)
	}

	var parsed struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Role    string `json:"role"`
		Model   string `json:"model"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal([]byte(resp.Body), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if parsed.ID != "msg_mock" {
		t.Errorf("id: got %q, want %q", parsed.ID, "msg_mock")
	}
	if parsed.Type != "message" {
		t.Errorf("type: got %q, want %q", parsed.Type, "message")
	}
	if parsed.Role != "assistant" {
		t.Errorf("role: got %q, want %q", parsed.Role, "assistant")
	}
	if parsed.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model: got %q, want %q", parsed.Model, "claude-sonnet-4-20250514")
	}
	if parsed.StopReason != "end_turn" {
		t.Errorf("stop_reason: got %q, want %q", parsed.StopReason, "end_turn")
	}
	if len(parsed.Content) != 1 {
		t.Fatalf("content length: got %d, want 1", len(parsed.Content))
	}
	if parsed.Content[0].Type != "text" {
		t.Errorf("content[0].type: got %q, want %q", parsed.Content[0].Type, "text")
	}
	if parsed.Content[0].Text != "test output" {
		t.Errorf("content[0].text: got %q, want %q", parsed.Content[0].Text, "test output")
	}
	if parsed.Usage.InputTokens != 10 {
		t.Errorf("input_tokens: got %d, want 10", parsed.Usage.InputTokens)
	}
	if parsed.Usage.OutputTokens != 5 {
		t.Errorf("output_tokens: got %d, want 5", parsed.Usage.OutputTokens)
	}
}

func TestAnthropicToolUseResponse_ValidJSON(t *testing.T) {
	toolCalls := []MockToolCall{
		{ID: "toolu_1", Name: "call_agent", Input: map[string]string{"agent": "helper", "task": "do work"}},
	}
	resp := AnthropicToolUseResponse("Delegating.", toolCalls)

	if resp.StatusCode != 200 {
		t.Errorf("StatusCode: got %d, want 200", resp.StatusCode)
	}

	var parsed struct {
		Content []struct {
			Type  string            `json:"type"`
			Text  string            `json:"text"`
			ID    string            `json:"id"`
			Name  string            `json:"name"`
			Input map[string]string `json:"input"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
	}

	if err := json.Unmarshal([]byte(resp.Body), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if parsed.StopReason != "tool_use" {
		t.Errorf("stop_reason: got %q, want %q", parsed.StopReason, "tool_use")
	}
	if len(parsed.Content) != 2 {
		t.Fatalf("content length: got %d, want 2", len(parsed.Content))
	}

	// Text block
	if parsed.Content[0].Type != "text" {
		t.Errorf("content[0].type: got %q, want %q", parsed.Content[0].Type, "text")
	}
	if parsed.Content[0].Text != "Delegating." {
		t.Errorf("content[0].text: got %q, want %q", parsed.Content[0].Text, "Delegating.")
	}

	// Tool use block
	if parsed.Content[1].Type != "tool_use" {
		t.Errorf("content[1].type: got %q, want %q", parsed.Content[1].Type, "tool_use")
	}
	if parsed.Content[1].ID != "toolu_1" {
		t.Errorf("content[1].id: got %q, want %q", parsed.Content[1].ID, "toolu_1")
	}
	if parsed.Content[1].Name != "call_agent" {
		t.Errorf("content[1].name: got %q, want %q", parsed.Content[1].Name, "call_agent")
	}
	if parsed.Content[1].Input["agent"] != "helper" {
		t.Errorf("content[1].input.agent: got %q, want %q", parsed.Content[1].Input["agent"], "helper")
	}
	if parsed.Content[1].Input["task"] != "do work" {
		t.Errorf("content[1].input.task: got %q, want %q", parsed.Content[1].Input["task"], "do work")
	}
}

func TestAnthropicErrorResponse_ValidJSON(t *testing.T) {
	resp := AnthropicErrorResponse(401, "authentication_error", "bad key")

	if resp.StatusCode != 401 {
		t.Errorf("StatusCode: got %d, want 401", resp.StatusCode)
	}

	var parsed struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(resp.Body), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if parsed.Type != "error" {
		t.Errorf("type: got %q, want %q", parsed.Type, "error")
	}
	if parsed.Error.Type != "authentication_error" {
		t.Errorf("error.type: got %q, want %q", parsed.Error.Type, "authentication_error")
	}
	if parsed.Error.Message != "bad key" {
		t.Errorf("error.message: got %q, want %q", parsed.Error.Message, "bad key")
	}
}

// Phase 7: OpenAI Helper Tests

func TestOpenAIResponse_ValidJSON(t *testing.T) {
	resp := OpenAIResponse("test output")

	if resp.StatusCode != 200 {
		t.Errorf("StatusCode: got %d, want 200", resp.StatusCode)
	}

	var parsed struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal([]byte(resp.Body), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if parsed.Model != "gpt-4o" {
		t.Errorf("model: got %q, want %q", parsed.Model, "gpt-4o")
	}
	if len(parsed.Choices) != 1 {
		t.Fatalf("choices length: got %d, want 1", len(parsed.Choices))
	}
	if parsed.Choices[0].Message.Content != "test output" {
		t.Errorf("content: got %q, want %q", parsed.Choices[0].Message.Content, "test output")
	}
	if parsed.Choices[0].FinishReason != "stop" {
		t.Errorf("finish_reason: got %q, want %q", parsed.Choices[0].FinishReason, "stop")
	}
	if parsed.Usage.PromptTokens != 10 {
		t.Errorf("prompt_tokens: got %d, want 10", parsed.Usage.PromptTokens)
	}
	if parsed.Usage.CompletionTokens != 5 {
		t.Errorf("completion_tokens: got %d, want 5", parsed.Usage.CompletionTokens)
	}
}

func TestOpenAIToolCallResponse_ValidJSON(t *testing.T) {
	toolCalls := []MockToolCall{
		{ID: "call_1", Name: "call_agent", Input: map[string]string{"agent": "helper", "task": "do work"}},
	}
	resp := OpenAIToolCallResponse("", toolCalls)

	if resp.StatusCode != 200 {
		t.Errorf("StatusCode: got %d, want 200", resp.StatusCode)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content   *string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal([]byte(resp.Body), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(parsed.Choices) != 1 {
		t.Fatalf("choices length: got %d, want 1", len(parsed.Choices))
	}

	choice := parsed.Choices[0]

	// Content should be null when text is empty
	if choice.Message.Content != nil {
		t.Errorf("content: got %v, want nil", choice.Message.Content)
	}

	if choice.FinishReason != "tool_calls" {
		t.Errorf("finish_reason: got %q, want %q", choice.FinishReason, "tool_calls")
	}

	if len(choice.Message.ToolCalls) != 1 {
		t.Fatalf("tool_calls length: got %d, want 1", len(choice.Message.ToolCalls))
	}

	tc := choice.Message.ToolCalls[0]
	if tc.ID != "call_1" {
		t.Errorf("tool_call id: got %q, want %q", tc.ID, "call_1")
	}
	if tc.Type != "function" {
		t.Errorf("tool_call type: got %q, want %q", tc.Type, "function")
	}
	if tc.Function.Name != "call_agent" {
		t.Errorf("function name: got %q, want %q", tc.Function.Name, "call_agent")
	}

	// Arguments should be a JSON string that can be parsed into the input map
	var args map[string]string
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		t.Fatalf("failed to parse arguments JSON: %v", err)
	}
	if args["agent"] != "helper" {
		t.Errorf("arguments.agent: got %q, want %q", args["agent"], "helper")
	}
	if args["task"] != "do work" {
		t.Errorf("arguments.task: got %q, want %q", args["task"], "do work")
	}

	if parsed.Usage.PromptTokens != 10 {
		t.Errorf("prompt_tokens: got %d, want 10", parsed.Usage.PromptTokens)
	}
	if parsed.Usage.CompletionTokens != 20 {
		t.Errorf("completion_tokens: got %d, want 20", parsed.Usage.CompletionTokens)
	}
}

func TestOpenAIErrorResponse_ValidJSON(t *testing.T) {
	resp := OpenAIErrorResponse(500, "server_error", "boom")

	if resp.StatusCode != 500 {
		t.Errorf("StatusCode: got %d, want 500", resp.StatusCode)
	}

	var parsed struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(resp.Body), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if parsed.Error.Message != "boom" {
		t.Errorf("error.message: got %q, want %q", parsed.Error.Message, "boom")
	}
	if parsed.Error.Type != "server_error" {
		t.Errorf("error.type: got %q, want %q", parsed.Error.Type, "server_error")
	}
	if parsed.Error.Code != "server_error" {
		t.Errorf("error.code: got %q, want %q", parsed.Error.Code, "server_error")
	}
}

// Phase 9: SlowResponse Tests

func TestSlowResponse_Delays(t *testing.T) {
	mock := NewMockLLMServer(t, []MockLLMResponse{
		SlowResponse(500 * time.Millisecond),
	})

	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", mock.URL()+"/v1/messages", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	elapsed := time.Since(start)
	if elapsed < 400*time.Millisecond {
		t.Errorf("response arrived too fast: %v (expected >= 400ms)", elapsed)
	}

	if resp.StatusCode != 200 {
		t.Errorf("status code: got %d, want 200", resp.StatusCode)
	}

	// Verify the response body is valid JSON
	body, _ := io.ReadAll(resp.Body)
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Errorf("response body is not valid JSON: %v", err)
	}
}

func TestSlowResponse_InterruptedByCancel(t *testing.T) {
	mock := NewMockLLMServer(t, []MockLLMResponse{
		SlowResponse(5 * time.Second),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", mock.URL()+"/v1/messages", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	start := time.Now()
	_, err = http.DefaultClient.Do(req)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}

	if elapsed >= 1*time.Second {
		t.Errorf("cancel took too long: %v (expected < 1s)", elapsed)
	}
}
