package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jrswab/axe/internal/agent"
	"github.com/jrswab/axe/internal/artifact"
	"github.com/jrswab/axe/internal/budget"
	"github.com/jrswab/axe/internal/config"
	"github.com/jrswab/axe/internal/mcpclient"
	"github.com/jrswab/axe/internal/memory"
	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/resolve"
	"github.com/jrswab/axe/internal/toolname"
	"github.com/jrswab/axe/internal/wasmloader"
	"github.com/jrswab/axe/internal/xdg"
	"github.com/jrswab/axe/pkg/protocol"
)

// ExecContext holds the context needed by tool executors.
type ExecContext struct {
	Workdir         string
	Stderr          io.Writer
	Verbose         bool
	AllowedHosts    []string
	ArtifactDir     string
	ArtifactTracker *artifact.Tracker
}

// ToolEntry holds a tool's definition and executor functions.
type ToolEntry struct {
	Definition func() protocol.ToolDefinition
	Execute    func(ctx context.Context, call protocol.ToolCall, ec ExecContext) protocol.ToolResult
}

// Registry interface defines the operations for tool registration and dispatch.
type Registry interface {
	Register(name string, t protocol.Tool)
	Has(name string) bool
	Resolve(names []string) ([]protocol.ToolDefinition, error)
	Dispatch(ctx context.Context, call protocol.ToolCall, ec ExecContext) (protocol.ToolResult, error)
	SetLoader(l *wasmloader.Loader)
	LoadPlugins(ctx context.Context, dir string) error
}

// Kernel handles the core orchestration of an agent run.
type Kernel struct {
	Config          *agent.AgentConfig
	GlobalCfg       *config.GlobalConfig
	AgentName       string
	Workdir         string
	AgentsDir       string
	PluginsDir      string
	Verbose         bool
	JsonOutput      bool
	Stderr          io.Writer
	Stdout          io.Writer
	MaxTokens       int
	ArtifactDir     string
	KeepArtifacts   bool
	ArtifactTracker *artifact.Tracker
	BudgetTracker   *budget.BudgetTracker
	WasmLoader      *wasmloader.Loader
	// RegisterTools is a callback to register all tools in a registry.
	RegisterTools func(r Registry)
}

const maxConversationTurns = 50
const maxToolOutputBytes = 1024

type ToolCallDetail struct {
	Name       string            `json:"name"`
	Input      map[string]string `json:"input"`
	Output     string            `json:"output"`
	IsError    bool              `json:"is_error"`
	Turn       int               `json:"turn"`
	DurationMs int64             `json:"duration_ms"`
}

type ToolExecResult struct {
	Result   protocol.ToolResult
	Duration time.Duration
}

// Run executes the agent conversation loop.
func (k *Kernel) Run(ctx context.Context, prov protocol.Provider, req *protocol.Request, registry Registry, mcpRouter *mcpclient.Router, streamEnabled bool) (*protocol.Response, []ToolCallDetail, int, int, int, bool, error) {
	// Load WASM plugins if a loader and directory are provided
	if k.WasmLoader != nil && k.PluginsDir != "" {
		registry.SetLoader(k.WasmLoader)
		if err := registry.LoadPlugins(ctx, k.PluginsDir); err != nil {
			if k.Verbose {
				_, _ = fmt.Fprintf(k.Stderr, "Warning: failed to load plugins from %s: %v\n", k.PluginsDir, err)
			}
		}
	}

	start := time.Now()

	// Determine parallel execution setting.
	parallel := true
	if k.Config.SubAgentsConf.Parallel != nil {
		parallel = *k.Config.SubAgentsConf.Parallel
	}

	var resp *protocol.Response
	var err error
	var totalInputTokens int
	var totalOutputTokens int
	var totalToolCalls int
	var allToolCallDetails []ToolCallDetail
	var budgetExceeded bool

	depth := 0 // Current depth
	effectiveMaxDepth := 3
	if k.Config.SubAgentsConf.MaxDepth > 0 && k.Config.SubAgentsConf.MaxDepth <= 5 {
		effectiveMaxDepth = k.Config.SubAgentsConf.MaxDepth
	}

	if len(req.Tools) == 0 {
		// Single-shot: no tools, no conversation loop
		if streamEnabled {
			if sp, ok := prov.(protocol.StreamProvider); ok {
				stream, streamErr := sp.SendStream(ctx, req)
				if streamErr != nil {
					return nil, nil, 0, 0, 0, false, k.MapProviderError(streamErr)
				}
				var textWriter io.Writer
				if !k.JsonOutput {
					textWriter = k.Stdout
				}
				resp, err = k.DrainEventStream(stream, textWriter)
				if err != nil {
					return nil, nil, 0, 0, 0, false, k.MapProviderError(err)
				}
			} else {
				resp, err = prov.Send(ctx, req)
			}
		} else {
			resp, err = prov.Send(ctx, req)
		}
		if err != nil {
			return nil, nil, 0, 0, 0, false, k.MapProviderError(err)
		}
		totalInputTokens = resp.InputTokens
		totalOutputTokens = resp.OutputTokens
		k.BudgetTracker.Add(resp.InputTokens, resp.OutputTokens)

		if k.BudgetTracker.Exceeded() {
			budgetExceeded = true
			_, _ = fmt.Fprintf(k.Stderr, "budget exceeded: used %d of %d tokens\n", k.BudgetTracker.Used(), k.BudgetTracker.Max())
		}

		if k.Verbose {
			durationMs := time.Since(start).Milliseconds()
			_, _ = fmt.Fprintf(k.Stderr, "Duration: %dms\n", durationMs)
			if k.BudgetTracker.Max() > 0 {
				_, _ = fmt.Fprintf(k.Stderr, "Tokens:   %d input, %d output (cumulative, budget: %d/%d)\n", resp.InputTokens, resp.OutputTokens, k.BudgetTracker.Used(), k.BudgetTracker.Max())
			} else {
				_, _ = fmt.Fprintf(k.Stderr, "Tokens:   %d input, %d output\n", resp.InputTokens, resp.OutputTokens)
			}
			_, _ = fmt.Fprintf(k.Stderr, "Stop:     %s\n", resp.StopReason)
		}
	} else {
		// Conversation loop: handle tool calls
		for turn := 0; turn < maxConversationTurns; turn++ {
			// Check budget before making LLM call
			if k.BudgetTracker.Exceeded() {
				break
			}

			if k.Verbose {
				pendingToolCalls := 0
				for _, m := range req.Messages {
					if m.Role == "tool" {
						pendingToolCalls += len(m.ToolResults)
					}
				}
				_, _ = fmt.Fprintf(k.Stderr, "[turn %d] Sending request (%d messages, %d tool calls pending)\n", turn+1, len(req.Messages), pendingToolCalls)
			}

			if streamEnabled {
				if sp, ok := prov.(protocol.StreamProvider); ok {
					stream, streamErr := sp.SendStream(ctx, req)
					if streamErr != nil {
						return nil, nil, 0, 0, 0, false, k.MapProviderError(streamErr)
					}
					var textWriter io.Writer
					if !k.JsonOutput {
						textWriter = k.Stdout
					}
					resp, err = k.DrainEventStream(stream, textWriter)
					if err != nil {
						return nil, nil, 0, 0, 0, false, k.MapProviderError(err)
					}
				} else {
					resp, err = prov.Send(ctx, req)
				}
			} else {
				resp, err = prov.Send(ctx, req)
			}
			if err != nil {
				return nil, nil, 0, 0, 0, false, k.MapProviderError(err)
			}

			totalInputTokens += resp.InputTokens
			totalOutputTokens += resp.OutputTokens
			k.BudgetTracker.Add(resp.InputTokens, resp.OutputTokens)

			if k.Verbose {
				label := "Received response"
				if streamEnabled {
					if _, ok := prov.(protocol.StreamProvider); ok {
						label = "Stream complete"
					}
				}
				_, _ = fmt.Fprintf(k.Stderr, "[turn %d] %s: %s (%d tool calls)\n", turn+1, label, resp.StopReason, len(resp.ToolCalls))
			}

			// No tool calls: conversation is done
			if len(resp.ToolCalls) == 0 {
				break
			}

			// Stop before executing tools if budget is exceeded
			if k.BudgetTracker.Exceeded() {
				break
			}

			// Append assistant message with tool calls
			assistantMsg := protocol.Message{
				Role:      "assistant",
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			}
			req.Messages = append(req.Messages, assistantMsg)

			// Execute tool calls
			results := k.ExecuteToolCalls(ctx, resp.ToolCalls, registry, mcpRouter, depth, effectiveMaxDepth, parallel)
			totalToolCalls += len(resp.ToolCalls)

			if k.JsonOutput {
				for i, tc := range resp.ToolCalls {
					input := tc.Arguments
					if input == nil {
						input = map[string]string{}
					}

					allToolCallDetails = append(allToolCallDetails, ToolCallDetail{
						Name:       tc.Name,
						Input:      input,
						Output:     k.TruncateOutput(results[i].Result.Content),
						IsError:    results[i].Result.IsError,
						Turn:       turn,
						DurationMs: results[i].Duration.Milliseconds(),
					})
				}
			}

			// Append tool result message
			toolResults := make([]protocol.ToolResult, len(results))
			for i, r := range results {
				toolResults[i] = r.Result
			}
			toolMsg := protocol.Message{
				Role:        "tool",
				ToolResults: toolResults,
			}
			req.Messages = append(req.Messages, toolMsg)
		}

		// Check if we exhausted turns
		if resp != nil && len(resp.ToolCalls) > 0 {
			return nil, nil, 0, 0, 0, false, fmt.Errorf("agent exceeded maximum conversation turns (%d)", maxConversationTurns)
		}

		// Check if budget was exceeded
		if k.BudgetTracker.Exceeded() {
			budgetExceeded = true
			_, _ = fmt.Fprintf(k.Stderr, "budget exceeded: used %d of %d tokens\n", k.BudgetTracker.Used(), k.BudgetTracker.Max())
		}

		if k.Verbose {
			durationMs := time.Since(start).Milliseconds()
			_, _ = fmt.Fprintf(k.Stderr, "Duration: %dms\n", durationMs)
			if k.BudgetTracker.Max() > 0 {
				_, _ = fmt.Fprintf(k.Stderr, "Tokens:   %d input, %d output (cumulative, budget: %d/%d)\n", totalInputTokens, totalOutputTokens, k.BudgetTracker.Used(), k.BudgetTracker.Max())
			} else {
				_, _ = fmt.Fprintf(k.Stderr, "Tokens:   %d input, %d output (cumulative)\n", totalInputTokens, totalOutputTokens)
			}
			_, _ = fmt.Fprintf(k.Stderr, "Stop:     %s\n", resp.StopReason)
		}
	}

	return resp, allToolCallDetails, totalInputTokens, totalOutputTokens, totalToolCalls, budgetExceeded, nil
}

func (k *Kernel) TruncateOutput(s string) string {
	if len(s) <= maxToolOutputBytes {
		return s
	}

	// Backtrack from the byte limit to avoid splitting a multi-byte UTF-8 rune.
	i := maxToolOutputBytes
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}

	return s[:i] + "... (truncated)"
}

func (k *Kernel) ExecuteToolCalls(ctx context.Context, toolCalls []protocol.ToolCall, registry Registry, mcpRouter *mcpclient.Router, depth, maxDepth int, parallel bool) []ToolExecResult {
	results := make([]ToolExecResult, len(toolCalls))

	if len(toolCalls) == 1 || !parallel {
		// Sequential execution (also used for single call)
		for i, tc := range toolCalls {
			start := time.Now()
			var r protocol.ToolResult
			if tc.Name == toolname.CallAgent {
				r = k.ExecuteCallAgent(ctx, tc, depth, maxDepth)
			} else {
				r = k.DispatchToolCall(ctx, tc, registry, mcpRouter)
			}
			results[i] = ToolExecResult{Result: r, Duration: time.Since(start)}
		}
	} else {
		// Parallel execution
		type indexedResult struct {
			index  int
			result ToolExecResult
		}
		ch := make(chan indexedResult, len(toolCalls))
		for i, tc := range toolCalls {
			go func(idx int, call protocol.ToolCall) {
				start := time.Now()
				var res protocol.ToolResult
				if call.Name == toolname.CallAgent {
					res = k.ExecuteCallAgent(ctx, call, depth, maxDepth)
				} else {
					res = k.DispatchToolCall(ctx, call, registry, mcpRouter)
				}
				ch <- indexedResult{index: idx, result: ToolExecResult{Result: res, Duration: time.Since(start)}}
			}(i, tc)
		}
		for range toolCalls {
			ir := <-ch
			results[ir.index] = ir.result
		}
	}

	return results
}

func (k *Kernel) DispatchToolCall(ctx context.Context, tc protocol.ToolCall, registry Registry, mcpRouter *mcpclient.Router) protocol.ToolResult {
	if mcpRouter != nil && mcpRouter.Has(tc.Name) {
		if k.Verbose && k.Stderr != nil {
			if serverName, ok := mcpRouter.ServerName(tc.Name); ok {
				_, _ = fmt.Fprintf(k.Stderr, "[mcp] Routing tool %q to server %q\n", tc.Name, serverName)
			}
		}
		result, err := mcpRouter.Dispatch(ctx, tc)
		if err != nil {
			return protocol.ToolResult{CallID: tc.ID, Content: err.Error(), IsError: true}
		}
		return result
	}

	result, dispatchErr := registry.Dispatch(ctx, tc, ExecContext{
		Workdir:         k.Workdir,
		Stderr:          k.Stderr,
		Verbose:         k.Verbose,
		AllowedHosts:    k.Config.AllowedHosts,
		ArtifactDir:     k.ArtifactDir,
		ArtifactTracker: k.ArtifactTracker,
	})
	if dispatchErr != nil {
		return protocol.ToolResult{CallID: tc.ID, Content: dispatchErr.Error(), IsError: true}
	}
	return result
}

// CallAgentTool returns the call_agent tool definition.
func CallAgentTool(allowedAgents []string) protocol.ToolDefinition {
	agentList := strings.Join(allowedAgents, ", ")
	return protocol.ToolDefinition{
		Name:        toolname.CallAgent,
		Description: "Delegate a task to a sub-agent. The sub-agent runs independently with its own context and returns only its final result. Available agents: " + agentList,
		Parameters: map[string]protocol.ToolParameter{
			"agent": {
				Type:        "string",
				Description: "Name of the sub-agent to invoke (must be one of: " + agentList + ")",
				Required:    true,
			},
			"task": {
				Type:        "string",
				Description: "What you need the sub-agent to do",
				Required:    true,
			},
			"context": {
				Type:        "string",
				Description: "Additional context from your conversation to pass along",
				Required:    false,
			},
		},
	}
}

// ExecuteCallAgent executes a sub-agent delegation.
func (k *Kernel) ExecuteCallAgent(ctx context.Context, call protocol.ToolCall, depth, maxDepth int) protocol.ToolResult {
	agentName := call.Arguments["agent"]
	task := call.Arguments["task"]
	taskContext := call.Arguments["context"]

	if agentName == "" {
		return protocol.ToolResult{CallID: call.ID, Content: `call_agent error: "agent" argument is required`, IsError: true}
	}
	if task == "" {
		return protocol.ToolResult{CallID: call.ID, Content: `call_agent error: "task" argument is required`, IsError: true}
	}

	allowed := false
	for _, a := range k.Config.SubAgents {
		if a == agentName {
			allowed = true
			break
		}
	}
	if !allowed {
		return protocol.ToolResult{CallID: call.ID, Content: fmt.Sprintf("call_agent error: agent %q is not in this agent's sub_agents list", agentName), IsError: true}
	}

	if depth >= maxDepth {
		return protocol.ToolResult{CallID: call.ID, Content: fmt.Sprintf("call_agent error: maximum sub-agent depth (%d) reached", maxDepth), IsError: true}
	}

	if k.Verbose && k.Stderr != nil {
		taskPreview := task
		if len(taskPreview) > 80 {
			taskPreview = taskPreview[:80] + "..."
		}
		_, _ = fmt.Fprintf(k.Stderr, "[sub-agent] Calling %q (depth %d) with task: %s\n", agentName, depth+1, taskPreview)
	}

	start := time.Now()

	searchDirs := agent.BuildSearchDirs(k.AgentsDir, k.Workdir)
	cfg, err := agent.Load(agentName, searchDirs)
	if err != nil {
		return k.errorResult(call.ID, agentName, fmt.Sprintf("failed to load agent %q: %s", agentName, err))
	}

	provName, modelName, err := k.parseModel(cfg.Model)
	if err != nil {
		return k.errorResult(call.ID, agentName, fmt.Sprintf("invalid model for agent %q: %s", agentName, err))
	}

	workdir, err := resolve.Workdir("", cfg.Workdir)
	if err != nil {
		return k.errorResult(call.ID, agentName, fmt.Sprintf("failed to resolve workdir for agent %q: %s", agentName, err))
	}

	files, err := resolve.Files(cfg.Files, workdir)
	if err != nil {
		return k.errorResult(call.ID, agentName, fmt.Sprintf("failed to resolve files for agent %q: %s", agentName, err))
	}

	configDir, err := xdg.GetConfigDir()
	if err != nil {
		return k.errorResult(call.ID, agentName, fmt.Sprintf("failed to get config dir: %s", err))
	}

	skillContent, err := resolve.Skill(cfg.Skill, configDir)
	if err != nil {
		return k.errorResult(call.ID, agentName, fmt.Sprintf("failed to load skill for agent %q: %s", agentName, err))
	}

	systemPrompt := resolve.BuildSystemPrompt(cfg.SystemPrompt, skillContent, files)

	if cfg.Memory.Enabled {
		memPath, memErr := memory.FilePath(agentName, cfg.Memory.Path)
		if memErr == nil {
			entries, _ := memory.LoadEntries(memPath, cfg.Memory.LastN)
			if entries != "" {
				systemPrompt += "\n\n---\n\n## Memory\n\n" + entries
			}
		}
	}

	apiKey := k.GlobalCfg.ResolveAPIKey(provName)
	baseURL := k.GlobalCfg.ResolveBaseURL(provName)

	if provider.Supported(provName) && provName != "ollama" && apiKey == "" {
		envVar := config.APIKeyEnvVar(provName)
		return k.errorResult(call.ID, agentName, fmt.Sprintf("API key for provider %q is not configured (set %s or add to config.toml)", provName, envVar))
	}

	prov, err := provider.New(provName, apiKey, baseURL)
	if err != nil {
		return k.errorResult(call.ID, agentName, fmt.Sprintf("failed to create provider for agent %q: %s", agentName, err))
	}

	var userMessage string
	if strings.TrimSpace(taskContext) != "" {
		userMessage = fmt.Sprintf("Task: %s\n\nContext:\n%s", task, taskContext)
	} else {
		userMessage = fmt.Sprintf("Task: %s", task)
	}

	req := &protocol.Request{
		Model:       modelName,
		System:      systemPrompt,
		Messages:    []protocol.Message{{Role: "user", Content: userMessage}},
		Temperature: cfg.Params.Temperature,
		MaxTokens:   cfg.Params.MaxTokens,
	}

	newDepth := depth + 1
	if len(cfg.SubAgents) > 0 && newDepth < maxDepth {
		req.Tools = []protocol.ToolDefinition{CallAgentTool(cfg.SubAgents)}
	}

	// We'll need a way to create a new registry for the sub-agent.
	// But wait, we can just pass the original registry or create a new one.
	// For simplicity, let's assume we can use the same registry type.
	// However, we need a concrete implementation of Registry to instantiate it.
	// Let's assume there is a NewRegistry implementation provided by the caller.
	// Actually, k.RegisterTools can be used.
}

func (k *Kernel) errorResult(callID, agentName, errMsg string) protocol.ToolResult {
	if k.Verbose && k.Stderr != nil {
		_, _ = fmt.Fprintf(k.Stderr, "[sub-agent] %q failed: %s\n", agentName, errMsg)
	}
	return protocol.ToolResult{
		CallID:  callID,
		Content: fmt.Sprintf("Error: sub-agent %q failed - %s. You may retry or proceed without this result.", agentName, errMsg),
		IsError: true,
	}
}

func (k *Kernel) parseModel(model string) (string, string, error) {
	idx := strings.Index(model, "/")
	if idx < 0 {
		return "", "", fmt.Errorf("invalid model format %q: expected provider/model-name", model)
	}
	return model[:idx], model[idx+1:], nil
}

func (k *Kernel) DrainEventStream(stream protocol.EventStream, w io.Writer) (*protocol.Response, error) {
	defer func() { _ = stream.Close() }()

	var content strings.Builder
	var toolCalls []protocol.ToolCall
	var inputTokens, outputTokens int
	var stopReason string

	type pendingCall struct {
		id   string
		name string
		args strings.Builder
	}
	pending := make(map[string]*pendingCall)

	for {
		ev, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch ev.Type {
		case protocol.StreamEventText:
			content.WriteString(ev.Text)
			if w != nil {
				_, _ = io.WriteString(w, ev.Text)
			}

		case protocol.StreamEventToolStart:
			pc := &pendingCall{id: ev.ToolCallID, name: ev.ToolName}
			if ev.ToolInput != "" {
				pc.args.WriteString(ev.ToolInput)
			}
			pending[ev.ToolCallID] = pc

		case protocol.StreamEventToolDelta:
			if pc, ok := pending[ev.ToolCallID]; ok {
				pc.args.WriteString(ev.ToolInput)
			}

		case protocol.StreamEventToolEnd:
			if pc, ok := pending[ev.ToolCallID]; ok {
				args := make(map[string]string)
				raw := pc.args.String()
				if raw != "" {
					var parsed map[string]interface{}
					if jsonErr := json.Unmarshal([]byte(raw), &parsed); jsonErr == nil {
						for k, v := range parsed {
							args[k] = fmt.Sprintf("%v", v)
						}
					}
				}
				toolCalls = append(toolCalls, protocol.ToolCall{
					ID:        pc.id,
					Name:      pc.name,
					Arguments: args,
				})
				delete(pending, ev.ToolCallID)
			}

		case protocol.StreamEventDone:
			inputTokens = ev.InputTokens
			outputTokens = ev.OutputTokens
			stopReason = ev.StopReason
		}
	}

	return &protocol.Response{
		Content:      content.String(),
		ToolCalls:    toolCalls,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		StopReason:   stopReason,
	}, nil
}

func (k *Kernel) MapProviderError(err error) error {
	var provErr *protocol.ProviderError
	if errors.As(err, &provErr) {
		return provErr
	}
	return err
}
