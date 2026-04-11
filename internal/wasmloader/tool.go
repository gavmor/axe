package wasmloader

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/jrswab/axe/pkg/protocol"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// WasmTool wraps a wazero module instance and implements the protocol.Tool interface using instance pooling.
type WasmTool struct {
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
	pool     sync.Pool
	def      protocol.ToolDefinition
}

// NewWasmTool creates a new Tool implementation from a compiled wazero module.
func NewWasmTool(r wazero.Runtime, compiled wazero.CompiledModule) *WasmTool {
	t := &WasmTool{
		runtime:  r,
		compiled: compiled,
	}

	t.pool.New = func() interface{} {
		// Use anonymous name to avoid collisions in the runtime registry
		config := wazero.NewModuleConfig().
			WithName("").
			WithStartFunctions("_initialize")

		// Use background context for initialization as the pool New doesn't have a request context
		mod, err := r.InstantiateModule(context.Background(), compiled, config)
		if err != nil {
			return nil
		}
		return mod
	}

	// Pre-load metadata using a temporary instance from the pool
	modObj := t.pool.Get()
	if modObj != nil {
		mod := modObj.(api.Module)
		def, err := t.fetchMetadata(context.Background(), mod)
		if err == nil {
			t.def = def
		}
		t.pool.Put(mod)
	}

	return t
}

// Definition returns the tool's metadata.
func (w *WasmTool) Definition() protocol.ToolDefinition {
	return w.def
}

// fetchMetadata retrieves tool metadata from a specific Wasm module instance.
func (w *WasmTool) fetchMetadata(ctx context.Context, mod api.Module) (protocol.ToolDefinition, error) {
	fn := mod.ExportedFunction("Metadata")
	if fn == nil {
		return protocol.ToolDefinition{}, fmt.Errorf("metadata function not exported")
	}

	results, err := fn.Call(ctx)
	if err != nil {
		return protocol.ToolDefinition{}, fmt.Errorf("failed to call Metadata: %w", err)
	}

	if len(results) == 0 {
		return protocol.ToolDefinition{}, fmt.Errorf("metadata function returned no results")
	}

	// Assuming the result is a pointer and length in linear memory (ABI convention)
	ptrLen := results[0]
	ptr := uint32(ptrLen >> 32)
	size := uint32(ptrLen)

	data, ok := mod.Memory().Read(ptr, size)
	if !ok {
		return protocol.ToolDefinition{}, fmt.Errorf("failed to read linear memory")
	}

	var def protocol.ToolDefinition
	if err := json.Unmarshal(data, &def); err != nil {
		return protocol.ToolDefinition{}, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return def, nil
}

// Execute triggers the tool's core logic within a pooled Wasm sandbox.
func (w *WasmTool) Execute(ctx context.Context, call protocol.ToolCall) protocol.ToolResult {
	modObj := w.pool.Get()
	if modObj == nil {
		return protocol.ToolResult{CallID: call.ID, Content: "failed to acquire wasm instance from pool", IsError: true}
	}
	mod := modObj.(api.Module)
	defer w.pool.Put(mod)

	fn := mod.ExportedFunction("Execute")
	if fn == nil {
		return protocol.ToolResult{CallID: call.ID, Content: "execute function not exported", IsError: true}
	}

	// Marshal call to JSON to pass to Wasm linear memory
	payload, err := json.Marshal(call)
	if err != nil {
		return protocol.ToolResult{CallID: call.ID, Content: fmt.Sprintf("failed to marshal tool call: %v", err), IsError: true}
	}

	// Allocation
	allocFn := mod.ExportedFunction("allocate")
	if allocFn == nil {
		return protocol.ToolResult{CallID: call.ID, Content: "guest must export 'allocate' function", IsError: true}
	}

	allocResults, err := allocFn.Call(ctx, uint64(len(payload)))
	if err != nil {
		return protocol.ToolResult{CallID: call.ID, Content: fmt.Sprintf("failed to allocate memory in guest: %v", err), IsError: true}
	}
	if len(allocResults) == 0 {
		return protocol.ToolResult{CallID: call.ID, Content: "allocate function returned no results", IsError: true}
	}
	ptr := uint32(allocResults[0])

	if !mod.Memory().Write(ptr, payload) {
		return protocol.ToolResult{CallID: call.ID, Content: "failed to write to linear memory", IsError: true}
	}

	results, err := fn.Call(ctx, uint64(ptr), uint64(len(payload)))
	if err != nil {
		return protocol.ToolResult{CallID: call.ID, Content: fmt.Sprintf("failed to call Execute: %v", err), IsError: true}
	}

	if len(results) == 0 {
		return protocol.ToolResult{CallID: call.ID, Content: "execute function returned no results", IsError: true}
	}

	// Read result from linear memory
	ptrLen := results[0]
	resPtr := uint32(ptrLen >> 32)
	resSize := uint32(ptrLen)

	resData, ok := mod.Memory().Read(resPtr, resSize)
	if !ok {
		return protocol.ToolResult{CallID: call.ID, Content: "failed to read result from linear memory", IsError: true}
	}

	var result protocol.ToolResult
	if err := json.Unmarshal(resData, &result); err != nil {
		return protocol.ToolResult{CallID: call.ID, Content: fmt.Sprintf("failed to unmarshal result: %v", err), IsError: true}
	}

	return result
}
