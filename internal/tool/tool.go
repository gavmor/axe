package tool

import (
	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/pkg/kernel"
)

// CallAgentTool returns the call_agent tool definition for LLM tool calling.
func CallAgentTool(allowedAgents []string) provider.Tool {
	return kernel.CallAgentTool(allowedAgents)
}

// EffectiveAllowedHosts returns the effective allowed hosts for a sub-agent.
// If subAgent is non-nil (even if empty), it is used as-is.
// If subAgent is nil, parent is inherited.
func EffectiveAllowedHosts(subAgent, parent []string) []string {
	if subAgent == nil {
		return parent
	}
	return subAgent
}
