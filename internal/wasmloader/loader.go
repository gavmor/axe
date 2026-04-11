package wasmloader

import (
	"context"
	"fmt"
	"os"

	"github.com/eliben/watgo"
	"github.com/eliben/watgo/wasmir"
	"github.com/tetratelabs/wazero"
	"github.com/jrswab/axe/pkg/protocol"
)

// Loader handles the validation and execution of Wasm-based tools.
type Loader struct {
	runtime wazero.Runtime
	cache   wazero.CompilationCache
}

// New returns a new Loader with an initialized wazero runtime and cache.
func New(ctx context.Context) (*Loader, error) {
	cache := wazero.NewCompilationCache()
	runtime := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCompilationCache(cache))

	if err := InstantiateHostModule(ctx, runtime); err != nil {
		runtime.Close(ctx)
		return nil, err
	}

	return &Loader{
		runtime: runtime,
		cache:   cache,
	}, nil
}

// Validate checks if a Wasm binary satisfies the Axe microkernel contract.
func (l *Loader) Validate(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read wasm file: %w", err)
	}
	return l.ValidateBytes(data)
}

// ValidateBytes checks if Wasm binary bytes satisfy the Axe microkernel contract.
func (l *Loader) ValidateBytes(data []byte) error {
	module, err := watgo.DecodeWASM(data)
	if err != nil {
		return fmt.Errorf("failed to decode wasm binary: %w", err)
	}

	// Required exports for our microkernel contract
	required := []string{"_initialize", "Execute", "Metadata"}
	exports := make(map[string]bool)
	for _, exp := range module.Exports {
		if exp.Kind == wasmir.ExternalKindFunction {
			exports[exp.Name] = true
		}
	}

	for _, req := range required {
		if !exports[req] {
			return fmt.Errorf("missing required export: %s", req)
		}
	}

	return nil
}

// Instantiate creates a new Tool from a Wasm binary using instance pooling.
func (l *Loader) Instantiate(ctx context.Context, path string) (protocol.Tool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read wasm file: %w", err)
	}
	return l.InstantiateBytes(ctx, data)
}

// InstantiateBytes creates a new Tool from Wasm binary bytes.
func (l *Loader) InstantiateBytes(ctx context.Context, data []byte) (protocol.Tool, error) {
	compiled, err := l.runtime.CompileModule(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("failed to compile wasm module: %w", err)
	}

	return NewWasmTool(l.runtime, compiled), nil
}

// Runtime returns the wazero runtime used by the loader.
func (l *Loader) Runtime() wazero.Runtime {
	return l.runtime
}

// Close cleans up the loader's runtime and cache.
func (l *Loader) Close(ctx context.Context) error {
	return l.runtime.Close(ctx)
}
