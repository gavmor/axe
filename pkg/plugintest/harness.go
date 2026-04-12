// Package plugintest provides a test harness and ABI validator for Axe
// WebAssembly plugins.
//
// Plugin authors import this package in their test files to load compiled
// .wasm modules, call exported functions (Metadata, Execute), and verify
// results and side effects (artifact tracking, budget queries) without
// running the full Axe kernel.
//
// Example usage:
//
//	h := plugintest.NewHarness().WithMockArtifactTracker()
//	defer h.Close()
//	if err := h.Load(wasmBytes); err != nil { t.Fatal(err) }
//	def, err := h.CallMetadata()
//	// ... assert def.Name, etc.
package plugintest

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/jrswab/axe/pkg/protocol"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/api"
)

// TrackedArtifact records one call to track_artifact by the plugin.
type TrackedArtifact struct {
	Path string
	Size int64
}

// Harness provides a self-contained wazero runtime with mock host
// functions for testing compiled Axe plugins outside the full kernel.
type Harness struct {
	mu      sync.Mutex
	runtime wazero.Runtime
	module  api.Module

	// Mock state
	artifacts  []TrackedArtifact
	budgetUsed uint64
	budgetMax  uint64
}

// NewHarness creates a Harness with default settings.
// Call Load to compile and instantiate a plugin, then Close when done.
func NewHarness() *Harness {
	return &Harness{}
}

// WithMockArtifactTracker enables artifact tracking. Calls to
// track_artifact inside the plugin will be recorded and retrievable
// via TrackedArtifacts.
func (h *Harness) WithMockArtifactTracker() *Harness {
	return h
}

// WithBudget sets the values returned by get_budget_used and get_budget_max
// host functions.
func (h *Harness) WithBudget(used, max uint64) *Harness {
	h.budgetUsed = used
	h.budgetMax = max
	return h
}

// Load compiles and instantiates the given .wasm bytes, registering mock
// host functions in the axe_kernel module.
func (h *Harness) Load(wasmBytes []byte) error {
	ctx := context.Background()
	h.runtime = wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig())
        wasi_snapshot_preview1.MustInstantiate(ctx, h.runtime)

	builder := h.runtime.NewHostModuleBuilder("axe_kernel")

	// track_artifact: (i32, i32, i64) -> ()
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
			ptr := uint32(stack[0])
			size := uint32(stack[1])
			artifactSize := int64(stack[2])
			pathBytes, ok := m.Memory().Read(ptr, size)
			if !ok {
				return
			}
			h.mu.Lock()
			h.artifacts = append(h.artifacts, TrackedArtifact{
				Path: string(pathBytes),
				Size: artifactSize,
			})
			h.mu.Unlock()
		}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI64}, []api.ValueType{}).
		Export("track_artifact")

	// get_budget_used: () -> i64
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
			stack[0] = h.budgetUsed
		}), []api.ValueType{}, []api.ValueType{api.ValueTypeI64}).
		Export("get_budget_used")

	// get_budget_max: () -> i64
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
			stack[0] = h.budgetMax
		}), []api.ValueType{}, []api.ValueType{api.ValueTypeI64}).
		Export("get_budget_max")

	if _, err := builder.Instantiate(ctx); err != nil {
		return fmt.Errorf("failed to instantiate axe_kernel mock: %w", err)
	}

	compiled, err := h.runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return fmt.Errorf("failed to compile plugin: %w", err)
	}

	config := wazero.NewModuleConfig().
		WithName("").
		WithStartFunctions("_initialize")

	mod, err := h.runtime.InstantiateModule(ctx, compiled, config)
	if err != nil {
		return fmt.Errorf("failed to instantiate plugin: %w", err)
	}
	h.module = mod

	return nil
}

// CallMetadata invokes the Metadata export and returns the parsed ToolDefinition.
func (h *Harness) CallMetadata() (protocol.ToolDefinition, error) {
	ctx := context.Background()
	fn := h.module.ExportedFunction("Metadata")
	if fn == nil {
		return protocol.ToolDefinition{}, fmt.Errorf("Metadata not exported")
	}

	results, err := fn.Call(ctx)
	if err != nil {
		return protocol.ToolDefinition{}, fmt.Errorf("Metadata call failed: %w", err)
	}
	if len(results) == 0 {
		return protocol.ToolDefinition{}, fmt.Errorf("Metadata returned no results")
	}

	ptrLen := results[0]
	ptr := uint32(ptrLen >> 32)
	size := uint32(ptrLen)

	if ptr == 0 && size == 0 {
		return protocol.ToolDefinition{}, fmt.Errorf("Metadata returned null pointer")
	}

	data, ok := h.module.Memory().Read(ptr, size)
	if !ok {
		return protocol.ToolDefinition{}, fmt.Errorf("failed to read memory at %d+%d", ptr, size)
	}

	var def protocol.ToolDefinition
	if err := json.Unmarshal(data, &def); err != nil {
		return protocol.ToolDefinition{}, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	return def, nil
}

// CallExecute marshals the ToolCall to JSON, passes it via the allocate-write-call
// protocol, and returns the parsed ToolResult.
func (h *Harness) CallExecute(call protocol.ToolCall) (protocol.ToolResult, error) {
	payload, err := json.Marshal(call)
	if err != nil {
		return protocol.ToolResult{}, fmt.Errorf("failed to marshal ToolCall: %w", err)
	}
	return h.CallExecuteRaw(payload)
}

// CallExecuteRaw passes raw bytes to the Execute function, bypassing JSON
// marshaling. Useful for testing plugin error handling of malformed input.
func (h *Harness) CallExecuteRaw(payload []byte) (protocol.ToolResult, error) {
	ctx := context.Background()

	allocFn := h.module.ExportedFunction("allocate")
	if allocFn == nil {
		return protocol.ToolResult{}, fmt.Errorf("allocate not exported")
	}
	allocRes, err := allocFn.Call(ctx, uint64(len(payload)))
	if err != nil {
		return protocol.ToolResult{}, fmt.Errorf("allocate call failed: %w", err)
	}
	if len(allocRes) == 0 {
		return protocol.ToolResult{}, fmt.Errorf("allocate returned no results")
	}
	ptr := uint32(allocRes[0])

	if !h.module.Memory().Write(ptr, payload) {
		return protocol.ToolResult{}, fmt.Errorf("memory write failed at %d", ptr)
	}

	fn := h.module.ExportedFunction("Execute")
	if fn == nil {
		return protocol.ToolResult{}, fmt.Errorf("Execute not exported")
	}
	results, err := fn.Call(ctx, uint64(ptr), uint64(len(payload)))
	if err != nil {
		return protocol.ToolResult{}, fmt.Errorf("Execute call failed: %w", err)
	}
	if len(results) == 0 {
		return protocol.ToolResult{}, fmt.Errorf("Execute returned no results")
	}

	ptrLen := results[0]
	resPtr := uint32(ptrLen >> 32)
	resSize := uint32(ptrLen)

	if resPtr == 0 && resSize == 0 {
		return protocol.ToolResult{}, fmt.Errorf("Execute returned null pointer")
	}

	resData, ok := h.module.Memory().Read(resPtr, resSize)
	if !ok {
		return protocol.ToolResult{}, fmt.Errorf("failed to read result from memory")
	}

	var result protocol.ToolResult
	if err := json.Unmarshal(resData, &result); err != nil {
		return protocol.ToolResult{}, fmt.Errorf("failed to unmarshal result: %w", err)
	}
	return result, nil
}

// TrackedArtifacts returns all artifacts recorded by the mock track_artifact.
func (h *Harness) TrackedArtifacts() []TrackedArtifact {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]TrackedArtifact, len(h.artifacts))
	copy(out, h.artifacts)
	return out
}

// Close releases all wazero resources.
func (h *Harness) Close() error {
	if h.runtime != nil {
		return h.runtime.Close(context.Background())
	}
	return nil
}
