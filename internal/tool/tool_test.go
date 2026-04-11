package tool

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/toolname"
)

func TestCallAgentTool_Definition(t *testing.T) {
	tool := CallAgentTool([]string{"helper", "runner"})

	if tool.Name != toolname.CallAgent {
		t.Errorf("Name = %q, want %q", tool.Name, toolname.CallAgent)
	}
	if tool.Name != "call_agent" {
		t.Errorf("Name = %q, want %q", tool.Name, "call_agent")
	}

	// Description must contain available agent names
	if !strings.Contains(tool.Description, "helper") {
		t.Errorf("Description missing agent name 'helper': %q", tool.Description)
	}
	if !strings.Contains(tool.Description, "runner") {
		t.Errorf("Description missing agent name 'runner': %q", tool.Description)
	}

	// Must have exactly three parameters
	if len(tool.Parameters) != 3 {
		t.Fatalf("Parameters count = %d, want 3", len(tool.Parameters))
	}

	// Check "agent" parameter
	agentParam, ok := tool.Parameters["agent"]
	if !ok {
		t.Fatal("missing 'agent' parameter")
	}
	if agentParam.Type != "string" {
		t.Errorf("agent.Type = %q, want %q", agentParam.Type, "string")
	}
	if !agentParam.Required {
		t.Error("agent.Required = false, want true")
	}
	if !strings.Contains(agentParam.Description, "helper") {
		t.Errorf("agent.Description missing agent name 'helper': %q", agentParam.Description)
	}

	// Check "task" parameter
	taskParam, ok := tool.Parameters["task"]
	if !ok {
		t.Fatal("missing 'task' parameter")
	}
	if taskParam.Type != "string" {
		t.Errorf("task.Type = %q, want %q", taskParam.Type, "string")
	}
	if !taskParam.Required {
		t.Error("task.Required = false, want true")
	}

	// Check "context" parameter
	contextParam, ok := tool.Parameters["context"]
	if !ok {
		t.Fatal("missing 'context' parameter")
	}
	if contextParam.Type != "string" {
		t.Errorf("context.Type = %q, want %q", contextParam.Type, "string")
	}
	if contextParam.Required {
		t.Error("context.Required = true, want false")
	}
}

func TestCallAgentTool_EmptyAgents(t *testing.T) {
	tool := CallAgentTool([]string{})

	if tool.Name != toolname.CallAgent {
		t.Errorf("Name = %q, want %q", tool.Name, toolname.CallAgent)
	}

	// Must still have valid structure with 3 parameters
	if len(tool.Parameters) != 3 {
		t.Fatalf("Parameters count = %d, want 3", len(tool.Parameters))
	}

	if _, ok := tool.Parameters["agent"]; !ok {
		t.Error("missing 'agent' parameter")
	}
	if _, ok := tool.Parameters["task"]; !ok {
		t.Error("missing 'task' parameter")
	}
	if _, ok := tool.Parameters["context"]; !ok {
		t.Error("missing 'context' parameter")
	}
}

func TestEffectiveAllowedHosts(t *testing.T) {
	tests := []struct {
		name      string
		subAgent  []string // cfg.AllowedHosts (nil or empty or populated)
		parent    []string // opts.AllowedHosts
		wantHosts []string // expected effective result (nil-aware via DeepEqual)
	}{
		{
			name:      "nil sub-agent inherits parent list",
			subAgent:  nil,
			parent:    []string{"api.example.com"},
			wantHosts: []string{"api.example.com"},
		},
		{
			name:      "empty sub-agent clears parent list",
			subAgent:  []string{},
			parent:    []string{"api.example.com"},
			wantHosts: []string{},
		},
		{
			name:      "populated sub-agent uses own list",
			subAgent:  []string{"docs.example.com"},
			parent:    []string{"api.example.com"},
			wantHosts: []string{"docs.example.com"},
		},
		{
			name:      "nil sub-agent with nil parent stays nil",
			subAgent:  nil,
			parent:    nil,
			wantHosts: nil,
		},
		{
			name:      "populated sub-agent with multiple hosts",
			subAgent:  []string{"a.example.com", "b.example.com"},
			parent:    []string{"api.example.com"},
			wantHosts: []string{"a.example.com", "b.example.com"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			effective := EffectiveAllowedHosts(tc.subAgent, tc.parent)

			if !reflect.DeepEqual(effective, tc.wantHosts) {
				t.Errorf("EffectiveAllowedHosts(%v, %v) = %v, want %v", tc.subAgent, tc.parent, effective, tc.wantHosts)
			}
		})
	}
}
