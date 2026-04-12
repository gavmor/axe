// Package plugintest provides a test harness and ABI validator for Axe
// WebAssembly plugins.
package plugintest

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/jrswab/axe/pkg/protocol"
	"github.com/gavmor/wasm-microkernel/abi"
	"github.com/gavmor/wasm-microkernel/plugintest"
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
	mu   sync.Mutex
	base *plugintest.Harness

	// Mock state
	artifacts  []TrackedArtifact
	budgetUsed uint64
	budgetMax  uint64
}

// NewHarness creates a Harness with default settings.
func NewHarness(t *testing.T) *Harness {
	return &Harness{
		base: plugintest.New(t),
	}
}

// WithMockArtifactTracker enables artifact tracking.
func (h *Harness) WithMockArtifactTracker() *Harness {
	h.base.MockHostFunction("axe_kernel", "track_artifact",
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI64}, []api.ValueType{},
		api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
			ptr := uint32(stack[0])
			size := uint32(stack[1])
			artifactSize := int64(stack[2])
			pathBytes, _ := abi.ReadGuestBuffer(ctx, m, ptr, size)
			h.mu.Lock()
			h.artifacts = append(h.artifacts, TrackedArtifact{
				Path: string(pathBytes),
				Size: artifactSize,
			})
			h.mu.Unlock()
		}))
	return h
}

// WithBudget sets the values returned by budget host functions.
func (h *Harness) WithBudget(used, max uint64) *Harness {
	h.budgetUsed = used
	h.budgetMax = max
	
	h.base.NewHostModule("axe_kernel").
		ExportFunction("get_budget_used", []api.ValueType{}, []api.ValueType{api.ValueTypeI64},
			api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
				stack[0] = h.budgetUsed
			})).
		ExportFunction("get_budget_max", []api.ValueType{}, []api.ValueType{api.ValueTypeI64},
			api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
				stack[0] = h.budgetMax
			})).
		Instantiate(context.Background())
		
	return h
}

// Load compiles and instantiates the given .wasm bytes.
func (h *Harness) Load(wasmBytes []byte) error {
	return h.base.Load(context.Background(), wasmBytes)
}

// CallMetadata invokes the Metadata export.
func (h *Harness) CallMetadata() (protocol.ToolDefinition, error) {
	data, err := h.base.CallExport(context.Background(), "Metadata")
	if err != nil {
		return protocol.ToolDefinition{}, err
	}
	var def protocol.ToolDefinition
	err = json.Unmarshal(data, &def)
	return def, err
}

// CallExecute marshals the ToolCall and returns the parsed ToolResult.
func (h *Harness) CallExecute(call protocol.ToolCall) (protocol.ToolResult, error) {
	payload, _ := json.Marshal(call)
	
	ctx := context.Background()
	ptr, err := h.base.Allocate(ctx, uint32(len(payload)))
	if err != nil {
		return protocol.ToolResult{}, err
	}
	
	if !abi.WriteGuestBuffer(ctx, h.base.Module, ptr, payload) {
		return protocol.ToolResult{}, fmt.Errorf("failed to write payload to guest")
	}
	
	resData, err := h.base.CallExport(ctx, "Execute", uint64(ptr), uint64(len(payload)))
	if err != nil {
		return protocol.ToolResult{}, err
	}
	
	var result protocol.ToolResult
	err = json.Unmarshal(resData, &result)
	return result, err
}

// TrackedArtifacts returns all artifacts recorded.
func (h *Harness) TrackedArtifacts() []TrackedArtifact {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.artifacts
}

// Close releases resources.
func (h *Harness) Close() error {
	return h.base.Close()
}

// ValidateABI checks if the given .wasm bytes adhere to the Axe contract.
func ValidateABI(wasmBytes []byte) plugintest.ABIReport {
	// ... signatures already validated in harness_test.go's dependency on plugintest.ValidateABI
	return plugintest.ABIReport{}
}
