package tool

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/jrswab/axe/internal/toolname"
	"github.com/jrswab/axe/internal/wasmloader"
	"github.com/jrswab/axe/pkg/kernel"
	"github.com/jrswab/axe/pkg/protocol"
)

// builtinToolAdapter wraps functional tools to satisfy the protocol.Tool interface.
type builtinToolAdapter struct {
	definition func() protocol.ToolDefinition
	execute    func(ctx context.Context, call protocol.ToolCall, ec kernel.ExecContext) protocol.ToolResult
	ec         kernel.ExecContext
}

func (a *builtinToolAdapter) Definition() protocol.ToolDefinition {
	if a.definition == nil {
		return protocol.ToolDefinition{}
	}
	return a.definition()
}

func (a *builtinToolAdapter) Execute(ctx context.Context, call protocol.ToolCall) protocol.ToolResult {
	if a.execute == nil {
		return protocol.ToolResult{CallID: call.ID, Content: "tool has no executor", IsError: true}
	}
	return a.execute(ctx, call, a.ec)
}

// Registry maps tool names to their implementations.
type Registry struct {
	mu      sync.RWMutex
	tools   map[string]protocol.Tool
	loader  *wasmloader.Loader
}

// NewRegistry returns a new Registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]protocol.Tool),
	}
}

// SetLoader sets the Wasm loader for the registry.
func (r *Registry) SetLoader(l *wasmloader.Loader) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loader = l
}

// Register adds a tool implementation to the registry.
func (r *Registry) Register(name string, t protocol.Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[name] = t
}

// Has returns true if a tool exists.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.tools[name]
	return ok
}

// Resolve returns tool definitions for the given names.
func (r *Registry) Resolve(names []string) ([]protocol.ToolDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]protocol.ToolDefinition, 0, len(names))
	for _, name := range names {
		t, ok := r.tools[name]
		if !ok {
			return nil, fmt.Errorf("unknown tool %q", name)
		}
		def := t.Definition()
		if def.Name == "" {
			return nil, fmt.Errorf("tool %q has no definition", name)
		}
		defs = append(defs, def)
	}
	return defs, nil
}

// Dispatch executes the named tool.
func (r *Registry) Dispatch(ctx context.Context, call protocol.ToolCall, ec kernel.ExecContext) (protocol.ToolResult, error) {
	r.mu.RLock()
	t, ok := r.tools[call.Name]
	r.mu.RUnlock()

	if !ok {
		return protocol.ToolResult{}, fmt.Errorf("unknown tool %q", call.Name)
	}

	// Check for nil executor if it's a built-in adapter
	if adapter, ok := t.(*builtinToolAdapter); ok {
		if adapter.execute == nil {
			return protocol.ToolResult{}, fmt.Errorf("tool %q has no executor", call.Name)
		}
		adapter.ec = ec
	}

	// For Wasm tools, we inject the kernel state into the context.
	if r.loader != nil {
		ctx = wasmloader.WithKernelState(ctx, wasmloader.KernelState{
			ArtifactTracker: ec.ArtifactTracker,
			AgentName:       "axe", // Should be passed in properly
		})
	}

	return t.Execute(ctx, call), nil
}

// LoadPlugins scans a directory for .wasm files and registers them as tools.
func (r *Registry) LoadPlugins(ctx context.Context, dir string) error {
	if r.loader == nil {
		return fmt.Errorf("wasm loader not configured")
	}

	pattern := filepath.Join(dir, "*.wasm")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, path := range matches {
		if err := r.loader.Validate(path); err != nil {
			continue // Skip invalid plugins
		}

		t, err := r.loader.Instantiate(ctx, path)
		if err != nil {
			return fmt.Errorf("failed to instantiate plugin %s: %w", path, err)
		}

		r.Register(t.Definition().Name, t)
	}

	return nil
}

// RegisterAll registers all built-in tool entries.
func RegisterAll(r kernel.Registry) {
	reg, ok := r.(*Registry)
	if !ok {
		return
	}
	reg.RegisterBuiltin(toolname.ListDirectory, listDirectoryEntry())
	reg.RegisterBuiltin(toolname.ReadFile, readFileEntry())
	reg.RegisterBuiltin(toolname.WriteFile, writeFileEntry())
	reg.RegisterBuiltin(toolname.EditFile, editFileEntry())
	reg.RegisterBuiltin(toolname.RunCommand, runCommandEntry())
	reg.RegisterBuiltin(toolname.URLFetch, urlFetchEntry())
	reg.RegisterBuiltin(toolname.WebSearch, webSearchEntry())
}

// RegisterBuiltin registers a built-in tool entry (legacy compatibility).
func (r *Registry) RegisterBuiltin(name string, entry kernel.ToolEntry) {
	r.Register(name, &builtinToolAdapter{
		definition: entry.Definition,
		execute:    entry.Execute,
	})
}

// Re-export kernel types for compatibility where needed.
type (
	ExecContext = kernel.ExecContext
	ToolEntry   = kernel.ToolEntry
)
