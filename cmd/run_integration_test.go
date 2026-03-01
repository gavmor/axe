package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jrswab/axe/internal/testutil"
)

// writeAgentConfig writes a TOML agent config file to configDir/agents/<name>.toml.
func writeAgentConfig(t *testing.T, configDir, name, toml string) {
	t.Helper()
	agentsDir := filepath.Join(configDir, "agents")
	if err := os.WriteFile(filepath.Join(agentsDir, name+".toml"), []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}
}

// --- Phase 11: Single-Shot Run Tests ---

func TestIntegration_SingleShot_Anthropic(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("Hello from Anthropic"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "single-anthropic", `name = "single-anthropic"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "single-anthropic"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	output := buf.String()
	if output != "Hello from Anthropic" {
		t.Errorf("expected stdout %q, got %q", "Hello from Anthropic", output)
	}

	if mock.RequestCount() != 1 {
		t.Errorf("expected 1 request, got %d", mock.RequestCount())
	}

	if mock.Requests[0].Path != "/v1/messages" {
		t.Errorf("expected path /v1/messages, got %q", mock.Requests[0].Path)
	}

	if mock.Requests[0].Method != "POST" {
		t.Errorf("expected method POST, got %q", mock.Requests[0].Method)
	}

	body := mock.Requests[0].Body
	if !strings.Contains(body, "claude-sonnet-4-20250514") {
		t.Errorf("expected request body to contain model name, got %q", body)
	}

	if !strings.Contains(body, defaultUserMessage) {
		t.Errorf("expected request body to contain default user message, got %q", body)
	}
}

func TestIntegration_SingleShot_OpenAI(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.OpenAIResponse("Hello from OpenAI"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "single-openai", `name = "single-openai"
model = "openai/gpt-4o"
`)

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("AXE_OPENAI_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "single-openai"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	output := buf.String()
	if output != "Hello from OpenAI" {
		t.Errorf("expected stdout %q, got %q", "Hello from OpenAI", output)
	}

	if mock.RequestCount() != 1 {
		t.Errorf("expected 1 request, got %d", mock.RequestCount())
	}

	if mock.Requests[0].Path != "/v1/chat/completions" {
		t.Errorf("expected path /v1/chat/completions, got %q", mock.Requests[0].Path)
	}

	body := mock.Requests[0].Body
	if !strings.Contains(body, "gpt-4o") {
		t.Errorf("expected request body to contain model name, got %q", body)
	}
}

func TestIntegration_SingleShot_StdinPiped(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("Processed your input"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "stdin-agent", `name = "stdin-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetIn(bytes.NewReader([]byte("user-provided input")))
	rootCmd.SetArgs([]string{"run", "stdin-agent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Processed your input") {
		t.Errorf("expected stdout to contain %q, got %q", "Processed your input", output)
	}

	body := mock.Requests[0].Body
	if !strings.Contains(body, "user-provided input") {
		t.Errorf("expected request body to contain user input, got %q", body)
	}

	if strings.Contains(body, defaultUserMessage) {
		t.Errorf("expected request body to NOT contain default message, got %q", body)
	}
}

// --- Phase 12: Conversation Loop Tests ---

func TestIntegration_ConversationLoop_SingleToolCall(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Turn 1: parent returns tool_use calling helper-agent
		testutil.AnthropicToolUseResponse("Delegating.", []testutil.MockToolCall{
			{ID: "toolu_1", Name: "call_agent", Input: map[string]string{"agent": "helper-agent", "task": "say hello"}},
		}),
		// Sub-agent call: helper returns a simple response
		testutil.AnthropicResponse("hello from helper"),
		// Turn 2: parent returns final text
		testutil.AnthropicResponse("The helper said: hello from helper"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "loop-parent", `name = "loop-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["helper-agent"]
`)
	writeAgentConfig(t, configDir, "helper-agent", `name = "helper-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "loop-parent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "The helper said: hello from helper") {
		t.Errorf("expected stdout to contain final response, got %q", output)
	}

	if mock.RequestCount() != 3 {
		t.Errorf("expected 3 requests, got %d", mock.RequestCount())
	}

	// Second request (index 1) is the sub-agent call - should contain the task
	if !strings.Contains(mock.Requests[1].Body, "say hello") {
		t.Errorf("expected sub-agent request to contain task 'say hello', got %q", mock.Requests[1].Body)
	}

	// Third request (index 2) should contain the tool result from the helper
	if !strings.Contains(mock.Requests[2].Body, "hello from helper") {
		t.Errorf("expected third request to contain tool result, got %q", mock.Requests[2].Body)
	}
}

func TestIntegration_ConversationLoop_MultipleRoundTrips(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Turn 1: tool call
		testutil.AnthropicToolUseResponse("First delegation.", []testutil.MockToolCall{
			{ID: "toolu_1", Name: "call_agent", Input: map[string]string{"agent": "multi-helper", "task": "step one"}},
		}),
		// Sub-agent 1 response
		testutil.AnthropicResponse("step one done"),
		// Turn 2: another tool call
		testutil.AnthropicToolUseResponse("Second delegation.", []testutil.MockToolCall{
			{ID: "toolu_2", Name: "call_agent", Input: map[string]string{"agent": "multi-helper", "task": "step two"}},
		}),
		// Sub-agent 2 response
		testutil.AnthropicResponse("step two done"),
		// Turn 3: final response
		testutil.AnthropicResponse("All steps completed"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "multi-parent", `name = "multi-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["multi-helper"]
`)
	writeAgentConfig(t, configDir, "multi-helper", `name = "multi-helper"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "multi-parent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	if mock.RequestCount() != 5 {
		t.Errorf("expected 5 requests, got %d", mock.RequestCount())
	}

	output := buf.String()
	if !strings.Contains(output, "All steps completed") {
		t.Errorf("expected stdout to contain final response, got %q", output)
	}
}

func TestIntegration_ConversationLoop_MaxTurnsExceeded(t *testing.T) {
	resetRunCmd(t)

	// Queue 100 responses: 50 parent tool_use + 50 sub-agent responses
	// Each turn: parent returns tool_use -> sub-agent returns text -> loop continues
	var responses []testutil.MockLLMResponse
	for i := 0; i < 50; i++ {
		responses = append(responses,
			testutil.AnthropicToolUseResponse("Delegating again.", []testutil.MockToolCall{
				{ID: "toolu_loop", Name: "call_agent", Input: map[string]string{"agent": "loop-helper", "task": "keep going"}},
			}),
			testutil.AnthropicResponse("still going"),
		)
	}

	mock := testutil.NewMockLLMServer(t, responses)

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "max-turns-parent", `name = "max-turns-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["loop-helper"]
`)
	writeAgentConfig(t, configDir, "loop-helper", `name = "loop-helper"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "max-turns-parent"})

	err := rootCmd.Execute()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 1 {
		t.Errorf("expected exit code 1, got %d", exitErr.Code)
	}
	if !strings.Contains(err.Error(), "exceeded maximum conversation turns") {
		t.Errorf("expected error about max turns, got %q", err.Error())
	}
}

// --- Phase 13: Sub-Agent Orchestration Tests ---

func TestIntegration_SubAgent_DepthLimit(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Parent: tool call to depth-child
		testutil.AnthropicToolUseResponse("Calling child.", []testutil.MockToolCall{
			{ID: "toolu_d1", Name: "call_agent", Input: map[string]string{"agent": "depth-child", "task": "do work"}},
		}),
		// Child runs at depth 1 = max_depth, so no tools injected. Single-shot response.
		testutil.AnthropicResponse("child result"),
		// Parent second turn: final response
		testutil.AnthropicResponse("Got: child result"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "depth-parent", `name = "depth-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["depth-child"]

[sub_agents_config]
max_depth = 1
`)
	writeAgentConfig(t, configDir, "depth-child", `name = "depth-child"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["depth-grandchild"]
`)
	writeAgentConfig(t, configDir, "depth-grandchild", `name = "depth-grandchild"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "depth-parent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Got: child result") {
		t.Errorf("expected stdout to contain final response, got %q", output)
	}

	if mock.RequestCount() != 3 {
		t.Errorf("expected 3 requests, got %d", mock.RequestCount())
	}

	// Second request (child's request at depth 1) should NOT contain tools
	childBody := mock.Requests[1].Body
	if strings.Contains(childBody, `"tools"`) {
		t.Errorf("expected child request to NOT contain tools (depth limit reached), got %q", childBody)
	}
}

func TestIntegration_SubAgent_ParallelExecution(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Parent: two simultaneous tool calls
		testutil.AnthropicToolUseResponse("Calling workers.", []testutil.MockToolCall{
			{ID: "toolu_a", Name: "call_agent", Input: map[string]string{"agent": "worker-a", "task": "task A"}},
			{ID: "toolu_b", Name: "call_agent", Input: map[string]string{"agent": "worker-b", "task": "task B"}},
		}),
		// Worker responses (order may vary due to parallelism)
		testutil.AnthropicResponse("result A"),
		testutil.AnthropicResponse("result B"),
		// Parent second turn: final response
		testutil.AnthropicResponse("Both workers done"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "par-parent", `name = "par-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["worker-a", "worker-b"]
`)
	writeAgentConfig(t, configDir, "worker-a", `name = "worker-a"
model = "anthropic/claude-sonnet-4-20250514"
`)
	writeAgentConfig(t, configDir, "worker-b", `name = "worker-b"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "par-parent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	if mock.RequestCount() != 4 {
		t.Errorf("expected 4 requests, got %d", mock.RequestCount())
	}

	output := buf.String()
	if !strings.Contains(output, "Both workers done") {
		t.Errorf("expected stdout to contain final response, got %q", output)
	}
}

func TestIntegration_SubAgent_SequentialExecution(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Parent: two tool calls
		testutil.AnthropicToolUseResponse("Calling workers sequentially.", []testutil.MockToolCall{
			{ID: "toolu_a", Name: "call_agent", Input: map[string]string{"agent": "seq-worker-a", "task": "task A"}},
			{ID: "toolu_b", Name: "call_agent", Input: map[string]string{"agent": "seq-worker-b", "task": "task B"}},
		}),
		// Worker A response (sequential: A always before B)
		testutil.AnthropicResponse("result A"),
		// Worker B response
		testutil.AnthropicResponse("result B"),
		// Parent second turn: final response
		testutil.AnthropicResponse("Sequential workers done"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "seq-parent", `name = "seq-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["seq-worker-a", "seq-worker-b"]

[sub_agents_config]
parallel = false
`)
	writeAgentConfig(t, configDir, "seq-worker-a", `name = "seq-worker-a"
model = "anthropic/claude-sonnet-4-20250514"
`)
	writeAgentConfig(t, configDir, "seq-worker-b", `name = "seq-worker-b"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "seq-parent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	if mock.RequestCount() != 4 {
		t.Errorf("expected 4 requests, got %d", mock.RequestCount())
	}

	output := buf.String()
	if !strings.Contains(output, "Sequential workers done") {
		t.Errorf("expected stdout to contain final response, got %q", output)
	}

	// In sequential mode, worker-a request must come before worker-b request.
	// Requests[0] = parent, Requests[1] = first worker, Requests[2] = second worker, Requests[3] = parent turn 2
	workerABody := mock.Requests[1].Body
	workerBBody := mock.Requests[2].Body
	if !strings.Contains(workerABody, "task A") {
		t.Errorf("expected first worker request to contain 'task A', got %q", workerABody)
	}
	if !strings.Contains(workerBBody, "task B") {
		t.Errorf("expected second worker request to contain 'task B', got %q", workerBBody)
	}
}

// --- Phase 14: Memory Tests ---

func TestIntegration_MemoryAppend_AfterSuccessfulRun(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("task completed"),
	})

	configDir, dataDir := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "mem-agent", `name = "mem-agent"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "mem-agent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	memoryFile := filepath.Join(dataDir, "memory", "mem-agent.md")
	data, readErr := os.ReadFile(memoryFile)
	if readErr != nil {
		t.Fatalf("expected memory file to exist at %s: %v", memoryFile, readErr)
	}

	content := string(data)
	if !strings.Contains(content, "## ") {
		t.Errorf("expected entry header in memory file, got %q", content)
	}
	if !strings.Contains(content, "**Task:**") {
		t.Errorf("expected Task line in memory file, got %q", content)
	}
	if !strings.Contains(content, "**Result:** task completed") {
		t.Errorf("expected Result line with 'task completed' in memory file, got %q", content)
	}
}

func TestIntegration_MemoryAppend_NotOnError(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicErrorResponse(500, "server_error", "boom"),
	})

	configDir, dataDir := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "mem-err-agent", `name = "mem-err-agent"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "mem-err-agent"})

	err := rootCmd.Execute()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}

	memoryFile := filepath.Join(dataDir, "memory", "mem-err-agent.md")
	if _, statErr := os.Stat(memoryFile); !os.IsNotExist(statErr) {
		t.Errorf("expected memory file to NOT exist after error, but got: %v", statErr)
	}
}

func TestIntegration_MemoryLoad_IntoSystemPrompt(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("ok"),
	})

	configDir, dataDir := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "mem-load", `name = "mem-load"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
last_n = 2
`)

	// Pre-seed memory file with 3 entries
	memoryDir := filepath.Join(dataDir, "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		t.Fatal(err)
	}
	memoryContent := `## 2026-01-01T10:00:00Z
**Task:** oldest task
**Result:** oldest result

## 2026-01-02T10:00:00Z
**Task:** middle task
**Result:** middle result

## 2026-01-03T10:00:00Z
**Task:** newest task
**Result:** newest result

`
	if err := os.WriteFile(filepath.Join(memoryDir, "mem-load.md"), []byte(memoryContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "mem-load"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	// Verify that the request body contains the last 2 entries but NOT the oldest
	body := mock.Requests[0].Body
	if !strings.Contains(body, "middle task") {
		t.Errorf("expected request body to contain 'middle task' (2nd entry), got body length %d", len(body))
	}
	if !strings.Contains(body, "newest task") {
		t.Errorf("expected request body to contain 'newest task' (3rd entry), got body length %d", len(body))
	}
	if strings.Contains(body, "oldest task") {
		t.Errorf("expected request body to NOT contain 'oldest task' (trimmed by last_n=2)")
	}
}

// --- Phase 15: JSON Output Tests ---

func TestIntegration_JSONOutput_Structure(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("json test output"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "json-agent", `name = "json-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "json-agent", "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	var result map[string]interface{}
	if jsonErr := json.Unmarshal(buf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\nraw: %q", jsonErr, buf.String())
	}

	// Verify required fields
	requiredFields := []string{"model", "content", "input_tokens", "output_tokens", "stop_reason", "duration_ms", "tool_calls"}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("expected JSON field %q to be present", field)
		}
	}

	if content, ok := result["content"].(string); !ok || content != "json test output" {
		t.Errorf("expected content %q, got %v", "json test output", result["content"])
	}

	if stopReason, ok := result["stop_reason"].(string); !ok || stopReason != "end_turn" {
		t.Errorf("expected stop_reason %q, got %v", "end_turn", result["stop_reason"])
	}

	if toolCalls, ok := result["tool_calls"].(float64); !ok || toolCalls != 0 {
		t.Errorf("expected tool_calls 0, got %v", result["tool_calls"])
	}

	if durationMs, ok := result["duration_ms"].(float64); !ok || durationMs < 0 {
		t.Errorf("expected duration_ms >= 0, got %v", result["duration_ms"])
	}

	if inputTokens, ok := result["input_tokens"].(float64); !ok || inputTokens != 10 {
		t.Errorf("expected input_tokens 10, got %v", result["input_tokens"])
	}

	if outputTokens, ok := result["output_tokens"].(float64); !ok || outputTokens != 5 {
		t.Errorf("expected output_tokens 5, got %v", result["output_tokens"])
	}
}

func TestIntegration_JSONOutput_WithToolCalls(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Parent: tool call (input_tokens: 10, output_tokens: 20)
		testutil.AnthropicToolUseResponse("Delegating.", []testutil.MockToolCall{
			{ID: "toolu_j1", Name: "call_agent", Input: map[string]string{"agent": "json-helper", "task": "do work"}},
		}),
		// Sub-agent response (input_tokens: 10, output_tokens: 5)
		testutil.AnthropicResponse("helper done"),
		// Parent final (input_tokens: 10, output_tokens: 5)
		testutil.AnthropicResponse("all done"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "json-tool-parent", `name = "json-tool-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["json-helper"]
`)
	writeAgentConfig(t, configDir, "json-helper", `name = "json-helper"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "json-tool-parent", "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	var result map[string]interface{}
	if jsonErr := json.Unmarshal(buf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\nraw: %q", jsonErr, buf.String())
	}

	if toolCalls, ok := result["tool_calls"].(float64); !ok || toolCalls != 1 {
		t.Errorf("expected tool_calls 1, got %v", result["tool_calls"])
	}

	// Token sums: turn 1 (10+20) + turn 2 (10+5) = input 20, output 25
	if inputTokens, ok := result["input_tokens"].(float64); !ok || inputTokens != 20 {
		t.Errorf("expected input_tokens 20 (sum of 2 parent turns), got %v", result["input_tokens"])
	}

	if outputTokens, ok := result["output_tokens"].(float64); !ok || outputTokens != 25 {
		t.Errorf("expected output_tokens 25 (sum of 2 parent turns), got %v", result["output_tokens"])
	}
}

// --- Phase 16: Timeout Handling Test ---

func TestIntegration_Timeout_ContextDeadlineExceeded(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.SlowResponse(5 * time.Second),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "timeout-agent", `name = "timeout-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "timeout-agent", "--timeout", "1"})

	err := rootCmd.Execute()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}
}

// --- Error Mapping Tests (Phase 17) ---

func TestIntegration_ErrorMapping_Auth401(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicErrorResponse(401, "authentication_error", "invalid api key"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "err-auth", `name = "err-auth"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "err-auth"})

	err := rootCmd.Execute()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}
}

func TestIntegration_ErrorMapping_RateLimit429(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicErrorResponse(429, "rate_limit_error", "too many requests"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "err-rate", `name = "err-rate"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "err-rate"})

	err := rootCmd.Execute()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}
}

func TestIntegration_ErrorMapping_Server500(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicErrorResponse(500, "server_error", "internal error"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "err-server", `name = "err-server"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "err-server"})

	err := rootCmd.Execute()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}
}

func TestIntegration_ErrorMapping_OpenAI401(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.OpenAIErrorResponse(401, "invalid_api_key", "invalid api key"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "err-oai-auth", `name = "err-oai-auth"
model = "openai/gpt-4o"
`)

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("AXE_OPENAI_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "err-oai-auth"})

	err := rootCmd.Execute()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}
}

func TestIntegration_ErrorMapping_OpenAI429(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.OpenAIErrorResponse(429, "rate_limit_exceeded", "rate limit"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "err-oai-rate", `name = "err-oai-rate"
model = "openai/gpt-4o"
`)

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("AXE_OPENAI_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "err-oai-rate"})

	err := rootCmd.Execute()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}
}

func TestIntegration_ErrorMapping_OpenAI500(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.OpenAIErrorResponse(500, "server_error", "internal error"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "err-oai-server", `name = "err-oai-server"
model = "openai/gpt-4o"
`)

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("AXE_OPENAI_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "err-oai-server"})

	err := rootCmd.Execute()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}
}
