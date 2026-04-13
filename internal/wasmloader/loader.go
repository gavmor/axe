package wasmloader

import (
	"context"
	"fmt"
	"os"

	"github.com/gavmor/wasm-microkernel/abi"
	"github.com/tetratelabs/wazero"
	"github.com/jrswab/axe/pkg/protocol"
)

// Loader handles the validation and execution of Wasm-based tools.
type Loader struct {
	runtime     wazero.Runtime
	cache       wazero.CompilationCache
	hostModules map[string]map[string]bool // module name → allowed functions (nil means all)
}

// New returns a new Loader with an initialized wazero runtime and cache.
func New(ctx context.Context) (*Loader, error) {
	cache := wazero.NewCompilationCache()
	runtime := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCompilationCache(cache))

	if err := InstantiateHostModule(ctx, runtime); err != nil {
		_ = runtime.Close(ctx)
		_ = cache.Close(ctx)
		return nil, err
	}

	// Copy HostCapabilities so AllowHostModule doesn't mutate the package-level map.
	hostModules := make(map[string]map[string]bool, len(HostCapabilities))
	for mod, funcs := range HostCapabilities {
		hostModules[mod] = funcs
	}

	return &Loader{
		runtime:     runtime,
		cache:       cache,
		hostModules: hostModules,
	}, nil
}

// AllowHostModule registers an additional host module name as valid for import validation.
// A nil function set means all functions in the module are allowed.
func (l *Loader) AllowHostModule(name string) {
	l.hostModules[name] = nil
}

// Validate checks if a Wasm binary satisfies the Axe microkernel contract.
func (l *Loader) Validate(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read wasm file %q: %w", path, err)
	}
	return l.ValidateBytes(data)
}

// ValidateBytes checks if Wasm binary bytes satisfy the Axe microkernel contract.
func (l *Loader) ValidateBytes(data []byte) error {
	required := []string{"_initialize", "Execute", "Metadata"}
	return abi.ValidateABI(data, required, l.hostModules)
}

// Instantiate creates a new Tool from a Wasm binary using instance pooling.
func (l *Loader) Instantiate(ctx context.Context, path string) (protocol.Tool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read wasm file %q: %w", path, err)
	}
	return l.InstantiateBytes(ctx, data)
}

// InstantiateBytes creates a new Tool from Wasm binary bytes.
func (l *Loader) InstantiateBytes(ctx context.Context, data []byte) (protocol.Tool, error) {
	compiled, err := l.runtime.CompileModule(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("failed to compile wasm module: %w", err)
	}

	return NewWasmTool(l.runtime, compiled)
}

// Runtime returns the wazero runtime used by the loader.
func (l *Loader) Runtime() wazero.Runtime {
	return l.runtime
}

// Close cleans up the loader's runtime and cache.
func (l *Loader) Close(ctx context.Context) error {
	if err := l.runtime.Close(ctx); err != nil {
		_ = l.cache.Close(ctx)
		return err
	}
	return l.cache.Close(ctx)
}
