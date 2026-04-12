package plugintest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/eliben/watgo"
	"github.com/eliben/watgo/wasmir"
	"github.com/jrswab/axe/internal/artifact"
	"github.com/jrswab/axe/internal/wasmloader"
	"github.com/jrswab/axe/pkg/protocol"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// ABIReport contains the results of an ABI validation check.
type ABIReport struct {
	MissingExports []string
	Errors         []error
}

func (r *ABIReport) Valid() bool {
	return len(r.MissingExports) == 0 && len(r.Errors) == 0
}

func (r *ABIReport) Error() string {
	return fmt.Sprintf("Missing exports: %v, Errors: %v", r.MissingExports, r.Errors)
}

// ValidateABI checks if a Wasm binary meets the Axe microkernel requirements.
func ValidateABI(wasmBytes []byte) *ABIReport {
	report := &ABIReport{}
	module, err := watgo.DecodeWASM(wasmBytes)
	if err != nil {
		report.Errors = append(report.Errors, err)
		return report
	}

	required := []string{"_initialize", "allocate", "Metadata", "Execute"}
	exports := make(map[string]bool)
	for _, exp := range module.Exports {
		if exp.Kind == wasmir.ExternalKindFunction {
			exports[exp.Name] = true
		}
	}

	for _, req := range required {
		if !exports[req] {
			report.MissingExports = append(report.MissingExports, req)
		}
	}

	return report
}

// Harness provides a controlled environment for testing Wasm plugins.
type Harness struct {
	ctx             context.Context
	runtime         wazero.Runtime
	mod             api.Module
	artifactTracker *artifact.Tracker
}

// NewHarness creates a new test harness.
func NewHarness() *Harness {
	return &Harness{
		ctx: context.Background(),
	}
}

// WithMockArtifactTracker initializes a mock artifact tracker in the harness.
func (h *Harness) WithMockArtifactTracker() *Harness {
	h.artifactTracker = artifact.NewTracker()
	return h
}

// TrackedArtifacts returns all artifacts recorded by the mock tracker.
func (h *Harness) TrackedArtifacts() []artifact.Entry {
	if h.artifactTracker == nil {
		return nil
	}
	return h.artifactTracker.Entries()
}

// Load compiles and instantiates the Wasm binary.
func (h *Harness) Load(wasmBytes []byte) error {
	if h.runtime == nil {
		h.runtime = wazero.NewRuntime(h.ctx)
		wasi_snapshot_preview1.MustInstantiate(h.ctx, h.runtime)

		// Instantiate host module with our tracker
		state := wasmloader.KernelState{
			ArtifactTracker: h.artifactTracker,
			AgentName:       "test-agent",
		}
		h.ctx = wasmloader.WithKernelState(h.ctx, state)
		_ = wasmloader.InstantiateHostModule(h.ctx, h.runtime)
	}

	compiled, err := h.runtime.CompileModule(h.ctx, wasmBytes)
	if err != nil {
		return err
	}

	config := wazero.NewModuleConfig().
		WithName("").
		WithStartFunctions("_initialize").
		WithStdout(os.Stdout).
		WithStderr(os.Stderr)

	mod, err := h.runtime.InstantiateModule(h.ctx, compiled, config)
	if err != nil {
		return err
	}
	h.mod = mod
	return nil
}

// CallMetadata retrieves the plugin's metadata.
func (h *Harness) CallMetadata() (protocol.ToolDefinition, error) {
	if h.mod == nil {
		return protocol.ToolDefinition{}, fmt.Errorf("module not loaded")
	}
	fn := h.mod.ExportedFunction("Metadata")
	results, err := fn.Call(h.ctx)
	if err != nil {
		return protocol.ToolDefinition{}, err
	}

	ptrLen := results[0]
	ptr := uint32(ptrLen >> 32)
	size := uint32(ptrLen)

	data, ok := h.mod.Memory().Read(ptr, size)
	if !ok {
		return protocol.ToolDefinition{}, fmt.Errorf("failed to read memory")
	}

	var def protocol.ToolDefinition
	err = json.Unmarshal(data, &def)
	return def, err
}

// CallExecute invokes the plugin logic with a ToolCall.
func (h *Harness) CallExecute(call protocol.ToolCall) (protocol.ToolResult, error) {
	if h.mod == nil {
		return protocol.ToolResult{}, fmt.Errorf("module not loaded")
	}
	payload, _ := json.Marshal(call)

	// Allocate memory in guest
	allocFn := h.mod.ExportedFunction("allocate")
	res, err := allocFn.Call(h.ctx, uint64(len(payload)))
	if err != nil {
		return protocol.ToolResult{}, fmt.Errorf("allocate failed: %w", err)
	}
	ptr := uint32(res[0])

	// Write payload
	if !h.mod.Memory().Write(ptr, payload) {
		return protocol.ToolResult{}, fmt.Errorf("failed to write payload to memory")
	}

	// Execute
	execFn := h.mod.ExportedFunction("Execute")
	results, err := execFn.Call(h.ctx, uint64(ptr), uint64(len(payload)))
	if err != nil {
		return protocol.ToolResult{}, err
	}

	// Read result
	ptrLen := results[0]
	resPtr := uint32(ptrLen >> 32)
	resSize := uint32(ptrLen)

	resData, ok := h.mod.Memory().Read(resPtr, resSize)
	if !ok {
		return protocol.ToolResult{}, fmt.Errorf("failed to read result from memory")
	}
	var result protocol.ToolResult
	err = json.Unmarshal(resData, &result)
	return result, err
}

func (h *Harness) Close() error {
	return h.runtime.Close(h.ctx)
}
