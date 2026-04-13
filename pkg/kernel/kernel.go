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
	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/tool"
	"github.com/jrswab/axe/internal/wasmloader"
)

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
	HideThinking    bool
}

const maxConversationTurns = 50

// canStream checks if a provider actually supports streaming, not just
// whether it satisfies the StreamProvider interface shape. This prevents
// RetryProvider from falsely triggering SendStream when the wrapped
// provider doesn't support it.
func canStream(prov provider.Provider) (provider.StreamProvider, bool) {
	type streamCapable interface {
		SupportsStream() bool
	}
	if sc, ok := prov.(streamCapable); ok && !sc.SupportsStream() {
		return nil, false
	}
	sp, ok := prov.(provider.StreamProvider)
	return sp, ok
}
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
	Result   provider.ToolResult
	Duration time.Duration
}

// Run executes the agent conversation loop.
func (k *Kernel) Run(ctx context.Context, prov provider.Provider, req *provider.Request, registry *tool.Registry, mcpRouter *mcpclient.Router, streamEnabled bool) (*provider.Response, []ToolCallDetail, int, int, int, bool, error) {
	// Load WASM plugins if a loader and directory are provided
	if k.WasmLoader != nil && k.PluginsDir != "" {
		registry.SetLoader(k.WasmLoader)
		if err := registry.LoadPlugins(ctx, k.PluginsDir); err != nil {
			if k.Verbose {
				_, _ = fmt.Fprintf(k.Stderr, "Warning: failed to load plugins from %s: %v\n", k.PluginsDir, err)
			}
		}
	}

	// Check if the hide_thinking plugin is loaded.
	if registry.Has("hide_thinking") {
		k.HideThinking = true
	}

	start := time.Now()

	// Determine parallel execution setting.
	parallel := true
	if k.Config.SubAgentsConf.Parallel != nil {
		parallel = *k.Config.SubAgentsConf.Parallel
	}

	var resp *provider.Response
	var err error
	var totalInputTokens int
	var totalOutputTokens int
	var totalToolCalls int
	var allToolCallDetails []ToolCallDetail
	var budgetExceeded bool
	var streamedText bool

	depth := 0 // Current depth, could be passed in if we want to support nested kernels
	effectiveMaxDepth := 3
	if k.Config.SubAgentsConf.MaxDepth > 0 && k.Config.SubAgentsConf.MaxDepth <= 5 {
		effectiveMaxDepth = k.Config.SubAgentsConf.MaxDepth
	}

	if len(req.Tools) == 0 {
		// Single-shot: no tools, no conversation loop
		if streamEnabled {
			if sp, ok := canStream(prov); ok {
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
				if !k.JsonOutput {
					streamedText = true
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
		// Strip inline thinking tokens from non-streaming responses.
		if k.HideThinking && resp != nil {
			resp.Content = StripThinkingTokens(resp.Content)
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
		exitedOnBudget := false
		for turn := 0; turn < maxConversationTurns; turn++ {
			// Check budget before making LLM call
			if k.BudgetTracker.Exceeded() {
				exitedOnBudget = true
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
				if sp, ok := canStream(prov); ok {
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
					if !k.JsonOutput && resp.Content != "" {
						streamedText = true
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

			// Strip inline thinking tokens from non-streaming conversation responses.
			if k.HideThinking && resp != nil && !streamEnabled {
				resp.Content = StripThinkingTokens(resp.Content)
			}

			totalInputTokens += resp.InputTokens
			totalOutputTokens += resp.OutputTokens
			k.BudgetTracker.Add(resp.InputTokens, resp.OutputTokens)

			if k.Verbose {
				label := "Received response"
				if streamEnabled {
					if _, ok := canStream(prov); ok {
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
				exitedOnBudget = true
				break
			}

			// Append assistant message with tool calls
			assistantMsg := provider.Message{
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
			toolResults := make([]provider.ToolResult, len(results))
			for i, r := range results {
				toolResults[i] = r.Result
			}
			toolMsg := provider.Message{
				Role:        "tool",
				ToolResults: toolResults,
			}
			req.Messages = append(req.Messages, toolMsg)
		}

		// Determine post-loop outcome
		if exitedOnBudget || k.BudgetTracker.Exceeded() {
			budgetExceeded = true
			_, _ = fmt.Fprintf(k.Stderr, "budget exceeded: used %d of %d tokens\n", k.BudgetTracker.Used(), k.BudgetTracker.Max())
		} else if resp != nil && len(resp.ToolCalls) > 0 {
			return nil, nil, 0, 0, 0, false, fmt.Errorf("agent exceeded maximum conversation turns (%d); reduce prompt complexity or tool recursion", maxConversationTurns)
		}

		if k.Verbose {
			durationMs := time.Since(start).Milliseconds()
			_, _ = fmt.Fprintf(k.Stderr, "Duration: %dms\n", durationMs)
			if k.BudgetTracker.Max() > 0 {
				_, _ = fmt.Fprintf(k.Stderr, "Tokens:   %d input, %d output (cumulative, budget: %d/%d)\n", totalInputTokens, totalOutputTokens, k.BudgetTracker.Used(), k.BudgetTracker.Max())
			} else {
				_, _ = fmt.Fprintf(k.Stderr, "Tokens:   %d input, %d output (cumulative)\n", totalInputTokens, totalOutputTokens)
			}
			if resp != nil {
				_, _ = fmt.Fprintf(k.Stderr, "Stop:     %s\n", resp.StopReason)
			}
		}
	}

	_ = streamedText // Used to decide if we print content later, but it's handled via k.Stdout in DrainEventStream

	return resp, allToolCallDetails, totalInputTokens, totalOutputTokens, totalToolCalls, budgetExceeded, nil
}

// LoadPlugin validates, instantiates and registers a Wasm plugin from bytes.
func (k *Kernel) LoadPlugin(ctx context.Context, registry *tool.Registry, wasmBytes []byte) error {
	if k.WasmLoader == nil {
		return fmt.Errorf("wasm loader not configured")
	}
	if err := k.WasmLoader.ValidateBytes(wasmBytes); err != nil {
		return err
	}
	t, err := k.WasmLoader.InstantiateBytes(ctx, wasmBytes)
	if err != nil {
		return err
	}
	registry.Register(t.Definition().Name, t)
	return nil
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

func (k *Kernel) ExecuteToolCalls(ctx context.Context, toolCalls []provider.ToolCall, registry *tool.Registry, mcpRouter *mcpclient.Router, depth, maxDepth int, parallel bool) []ToolExecResult {
	results := make([]ToolExecResult, len(toolCalls))

	execOpts := tool.ExecuteOptions{
		AllowedAgents:   k.Config.SubAgents,
		ParentModel:     k.Config.Model,
		Depth:           depth,
		MaxDepth:        maxDepth,
		Timeout:         k.Config.SubAgentsConf.Timeout,
		GlobalConfig:    k.GlobalCfg,
		MCPRouter:       mcpRouter,
		Verbose:         k.Verbose,
		Stderr:          k.Stderr,
		BudgetTracker:   k.BudgetTracker,
		AgentsDir:       k.AgentsDir,
		AgentsBase:      k.Workdir,
		AllowedHosts:    k.Config.AllowedHosts,
		ArtifactDir:     k.ArtifactDir,
		ArtifactTracker: k.ArtifactTracker,
	}

	if len(toolCalls) == 1 || !parallel {
		// Sequential execution (also used for single call)
		for i, tc := range toolCalls {
			start := time.Now()
			var r provider.ToolResult
			if mcpRouter != nil && mcpRouter.Has(tc.Name) {
				r = k.DispatchToolCall(ctx, tc, registry, mcpRouter)
			} else if tc.Name == tool.CallAgentToolName {
				r = tool.ExecuteCallAgent(ctx, tc, execOpts)
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
			go func(idx int, call provider.ToolCall) {
				start := time.Now()
				var res provider.ToolResult
				if mcpRouter != nil && mcpRouter.Has(call.Name) {
					res = k.DispatchToolCall(ctx, call, registry, mcpRouter)
				} else if call.Name == tool.CallAgentToolName {
					res = tool.ExecuteCallAgent(ctx, call, execOpts)
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

func (k *Kernel) DispatchToolCall(ctx context.Context, tc provider.ToolCall, registry *tool.Registry, mcpRouter *mcpclient.Router) provider.ToolResult {
	if mcpRouter != nil && mcpRouter.Has(tc.Name) {
		if k.Verbose && k.Stderr != nil {
			if serverName, ok := mcpRouter.ServerName(tc.Name); ok {
				_, _ = fmt.Fprintf(k.Stderr, "[mcp] Routing tool %q to server %q\n", tc.Name, serverName)
			}
		}
		result, err := mcpRouter.Dispatch(ctx, tc)
		if err != nil {
			return provider.ToolResult{CallID: tc.ID, Content: err.Error(), IsError: true}
		}
		return result
	}

	result, dispatchErr := registry.Dispatch(ctx, tc, tool.ExecContext{
		Workdir:         k.Workdir,
		Stderr:          k.Stderr,
		Verbose:         k.Verbose,
		AllowedHosts:    k.Config.AllowedHosts,
		ArtifactDir:     k.ArtifactDir,
		ArtifactTracker: k.ArtifactTracker,
		BudgetTracker:   k.BudgetTracker,
		AgentName:       k.AgentName,
	})
	if dispatchErr != nil {
		return provider.ToolResult{CallID: tc.ID, Content: dispatchErr.Error(), IsError: true}
	}
	return result
}

func (k *Kernel) DrainEventStream(stream provider.EventStream, w io.Writer) (*provider.Response, error) {
	defer func() { _ = stream.Close() }()

	var content strings.Builder
	var toolCalls []provider.ToolCall
	var inputTokens, outputTokens int
	var stopReason string
	var thinkingCloseTag string

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
		case provider.StreamEventText:
			text := ev.Text
			if k.HideThinking {
				text, thinkingCloseTag = filterThinkingChunk(text, thinkingCloseTag)
			}
			if text != "" {
				content.WriteString(text)
				if w != nil {
					_, _ = io.WriteString(w, text)
				}
			}

		case provider.StreamEventToolStart:
			pc := &pendingCall{id: ev.ToolCallID, name: ev.ToolName}
			if ev.ToolInput != "" {
				pc.args.WriteString(ev.ToolInput)
			}
			pending[ev.ToolCallID] = pc

		case provider.StreamEventToolDelta:
			if pc, ok := pending[ev.ToolCallID]; ok {
				pc.args.WriteString(ev.ToolInput)
			}

		case provider.StreamEventToolEnd:
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
				toolCalls = append(toolCalls, provider.ToolCall{
					ID:        pc.id,
					Name:      pc.name,
					Arguments: args,
				})
				delete(pending, ev.ToolCallID)
			}

		case provider.StreamEventDone:
			inputTokens = ev.InputTokens
			outputTokens = ev.OutputTokens
			stopReason = ev.StopReason
		}
	}

	return &provider.Response{
		Content:      content.String(),
		ToolCalls:    toolCalls,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		StopReason:   stopReason,
	}, nil
}

func (k *Kernel) MapProviderError(err error) error {
	var provErr *provider.ProviderError
	if errors.As(err, &provErr) {
		// We can't easily return ExitError from here without importing cmd package
		// (which would be circular). Instead, we return the error and let cmd wrap it.
		// However, the caller might need to know the exit code.
		return provErr
	}
	return err
}
