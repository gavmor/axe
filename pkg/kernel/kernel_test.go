package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jrswab/axe/internal/agent"
	"github.com/jrswab/axe/internal/budget"
	"github.com/jrswab/axe/internal/config"
	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/toolname"
)

func setupKernelTestAgentsDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	agentsDir := filepath.Join(tmpDir, "axe", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}
	return agentsDir
}

func writeKernelTestAgent(t *testing.T, agentsDir, name, content string) {
	t.Helper()
	path := filepath.Join(agentsDir, name+".toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}
}

func TestExecuteCallAgent_EmptyAgentName(t *testing.T) {
	k := &Kernel{
		Config: &agent.AgentConfig{SubAgents: []string{"helper"}},
	}
	call := provider.ToolCall{
		ID:        "test-1",
		Name:      toolname.CallAgent,
		Arguments: map[string]string{"agent": "", "task": "do something"},
	}
	result := k.ExecuteCallAgent(context.Background(), call, 0, 3)
	if !result.IsError {
		t.Fatal("expected IsError=true for empty agent name")
	}
	want := `call_agent error: "agent" argument is required`
	if result.Content != want {
		t.Errorf("Content = %q, want %q", result.Content, want)
	}
}

func TestExecuteCallAgent_EmptyTask(t *testing.T) {
	k := &Kernel{
		Config: &agent.AgentConfig{SubAgents: []string{"helper"}},
	}
	call := provider.ToolCall{
		ID:        "test-2",
		Name:      toolname.CallAgent,
		Arguments: map[string]string{"agent": "helper", "task": ""},
	}
	result := k.ExecuteCallAgent(context.Background(), call, 0, 3)
	if !result.IsError {
		t.Fatal("expected IsError=true for empty task")
	}
	want := `call_agent error: "task" argument is required`
	if result.Content != want {
		t.Errorf("Content = %q, want %q", result.Content, want)
	}
}

func TestExecuteCallAgent_AgentNotAllowed(t *testing.T) {
	k := &Kernel{
		Config: &agent.AgentConfig{SubAgents: []string{"helper"}},
	}
	call := provider.ToolCall{
		ID:        "test-3",
		Name:      toolname.CallAgent,
		Arguments: map[string]string{"agent": "unknown", "task": "do something"},
	}
	result := k.ExecuteCallAgent(context.Background(), call, 0, 3)
	if !result.IsError {
		t.Fatal("expected IsError=true for unknown agent")
	}
	want := `call_agent error: agent "unknown" is not in this agent's sub_agents list`
	if result.Content != want {
		t.Errorf("Content = %q, want %q", result.Content, want)
	}
}

func TestExecuteCallAgent_DepthLimitReached(t *testing.T) {
	k := &Kernel{
		Config: &agent.AgentConfig{SubAgents: []string{"helper"}},
	}
	call := provider.ToolCall{
		ID:        "test-4",
		Name:      toolname.CallAgent,
		Arguments: map[string]string{"agent": "helper", "task": "do something"},
	}
	result := k.ExecuteCallAgent(context.Background(), call, 3, 3)
	if !result.IsError {
		t.Fatal("expected IsError=true for depth limit")
	}
	want := fmt.Sprintf("call_agent error: maximum sub-agent depth (%d) reached", 3)
	if result.Content != want {
		t.Errorf("Content = %q, want %q", result.Content, want)
	}
}

func TestExecuteCallAgent_Success(t *testing.T) {
	agentsDir := setupKernelTestAgentsDir(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id":    "msg_123",
			"type":  "message",
			"model": "claude-3-sonnet-20240229",
			"role":  "assistant",
			"content": []map[string]interface{}{
				{"type": "text", "text": "Sub-agent result here"},
			},
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":  10,
				"output_tokens": 5,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	toml := `name = "helper"
model = "anthropic/claude-3-sonnet-20240229"
system_prompt = "You are a helper."
`
	writeKernelTestAgent(t, agentsDir, "helper", toml)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	k := &Kernel{
		Config:        &agent.AgentConfig{SubAgents: []string{"helper"}},
		GlobalCfg:     &config.GlobalConfig{},
		BudgetTracker: budget.New(0),
	}

	call := provider.ToolCall{
		ID:        "test-6",
		Name:      toolname.CallAgent,
		Arguments: map[string]string{"agent": "helper", "task": "say hello"},
	}
	result := k.ExecuteCallAgent(context.Background(), call, 0, 3)
	if result.IsError {
		t.Fatalf("expected IsError=false, got error: %s", result.Content)
	}
	if result.Content != "Sub-agent result here" {
		t.Errorf("Content = %q, want %q", result.Content, "Sub-agent result here")
	}
}
