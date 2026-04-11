package kernel_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/eliben/watgo"
	"github.com/jrswab/axe/internal/agent"
	"github.com/jrswab/axe/internal/artifact"
	"github.com/jrswab/axe/internal/tool"
	"github.com/jrswab/axe/internal/wasmloader"
	"github.com/jrswab/axe/pkg/kernel"
	"github.com/jrswab/axe/pkg/protocol"
	"github.com/tetratelabs/wazero/api"
)

// compileWAT is a helper to avoid checking in binary .wasm files.
func compileWAT(t *testing.T, wat string) []byte {
	t.Helper()
	wasm, err := watgo.CompileWATToWASM([]byte(wat))
	if err != nil {
		t.Fatalf("failed to compile WAT: %v\nWAT source:\n%s", err, wat)
	}
	return wasm
}

func TestPluginRejection_InvalidContract(t *testing.T) {
	ctx := context.Background()
	loader, err := wasmloader.New(ctx)
	if err != nil {
		t.Fatalf("failed to create wasm loader: %v", err)
	}
	defer loader.Close(ctx)

	k := &kernel.Kernel{
		WasmLoader: loader,
		Config:     &agent.AgentConfig{},
	}
	reg := tool.NewRegistry()
	reg.SetLoader(loader)

	// A plugin missing the Metadata export
	badPluginWAT := `(module
		(func (export "_initialize"))
		(func (export "Execute") (param i32 i32) (result i64) (i64.const 0))
	)`

	wasmBytes := compileWAT(t, badPluginWAT)
	err = k.LoadPlugin(ctx, reg, wasmBytes)

	if err == nil {
		t.Fatal("expected error for plugin missing 'Metadata' export, got nil")
	}
	if !strings.Contains(err.Error(), "missing required export: Metadata") {
		t.Errorf("expected error to mention 'Metadata', got: %v", err)
	}
}

func TestPluginLifecycle_ReactorInit(t *testing.T) {
	ctx := context.Background()
	loader, err := wasmloader.New(ctx)
	if err != nil {
		t.Fatalf("failed to create wasm loader: %v", err)
	}
	defer loader.Close(ctx)

	k := &kernel.Kernel{
		WasmLoader: loader,
		Config:     &agent.AgentConfig{},
	}
	reg := tool.NewRegistry()
	reg.SetLoader(loader)

	metadata := `{\"Name\":\"test_tool\"}`
	size := 20
	wat := `(module
		(memory (export "memory") 1)
		(data (i32.const 0) "` + metadata + `")
		(func (export "_initialize"))
		(func (export "allocate") (param i32) (result i32) (i32.const 1024))
		(func (export "Metadata") (result i64)
			(i64.const ` + fmt.Sprintf("%d", size) + `)
		)
		(func (export "Execute") (param i32 i32) (result i64) (i64.const 0))
	)`

	wasmBytes := compileWAT(t, wat)
	err = k.LoadPlugin(ctx, reg, wasmBytes)

	if err != nil {
		t.Fatalf("failed to load valid plugin: %v", err)
	}

	if !reg.Has("test_tool") {
		t.Fatal("registry does not have 'test_tool' after loading plugin")
	}
}

func TestRegistry_FactoryDispensation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		
		var instantiations atomic.Int32
		blockCh := make(chan struct{})
		
		loader, err := wasmloader.New(ctx)
		if err != nil {
			t.Fatalf("failed to create wasm loader: %v", err)
		}
		defer loader.Close(ctx)

		_, err = loader.Runtime().NewHostModuleBuilder("test_env").
			NewFunctionBuilder().
			WithGoFunction(api.GoFunc(func(ctx context.Context, stack []uint64) {
				instantiations.Add(1)
			}), []api.ValueType{}, []api.ValueType{}).
			Export("record_init").
			NewFunctionBuilder().
			WithGoFunction(api.GoFunc(func(ctx context.Context, stack []uint64) {
				<-blockCh
			}), []api.ValueType{}, []api.ValueType{}).
			Export("block").
			Instantiate(ctx)
		if err != nil {
			t.Fatalf("failed to instantiate test_env host module: %v", err)
		}

		k := &kernel.Kernel{
			WasmLoader: loader,
			Config:     &agent.AgentConfig{},
		}
		reg := tool.NewRegistry()
	reg.SetLoader(loader)

		metadata := `{\"Name\":\"concurrent_tool\"}`
		resJSON := `{\"CallID\":\"1\",\"Content\":\"ok\"}`
		
		wat := `(module
			(import "test_env" "record_init" (func $record_init))
			(import "test_env" "block" (func $block))
			(memory (export "memory") 1)
			(data (i32.const 0) "` + metadata + `")
			(data (i32.const 512) "` + resJSON + `")
			(func (export "_initialize") (call $record_init))
			(func (export "allocate") (param i32) (result i32) (i32.const 1024))
			(func (export "Metadata") (result i64)
				(i64.const ` + fmt.Sprintf("%d", len(`{"Name":"concurrent_tool"}`)) + `)
			)
			(func (export "Execute") (param i32 i32) (result i64)
				(call $block)
				(i64.const 0x00000200000000` + fmt.Sprintf("%02X", len(`{"CallID":"1","Content":"ok"}`)) + `)
			)
		)`

		wasmBytes := compileWAT(t, wat)
		err = k.LoadPlugin(ctx, reg, wasmBytes)
		if err != nil {
			t.Fatalf("failed to load plugin: %v", err)
		}

		if instantiations.Load() != 1 {
			t.Errorf("expected 1 instantiation after load, got %d", instantiations.Load())
		}

		var wg sync.WaitGroup
		wg.Add(2)
		
		go func() {
			defer wg.Done()
			k.DispatchToolCall(ctx, protocol.ToolCall{ID: "1", Name: "concurrent_tool"}, reg, nil)
		}()
		
		go func() {
			defer wg.Done()
			k.DispatchToolCall(ctx, protocol.ToolCall{ID: "2", Name: "concurrent_tool"}, reg, nil)
		}()

		synctest.Wait()
		
		if instantiations.Load() < 2 {
			t.Errorf("expected at least 2 instantiations for concurrent calls, got %d", instantiations.Load())
		}
		
		close(blockCh)
		wg.Wait()
	})
}

func TestHostFunction_CapabilityInjection(t *testing.T) {
	ctx := context.Background()
	loader, err := wasmloader.New(ctx)
	if err != nil {
		t.Fatalf("failed to create wasm loader: %v", err)
	}
	defer loader.Close(ctx)

	tracker := artifact.NewTracker()
	k := &kernel.Kernel{
		WasmLoader:      loader,
		ArtifactTracker: tracker,
		AgentName:       "test-agent",
		Config:          &agent.AgentConfig{},
	}
	reg := tool.NewRegistry()
	reg.SetLoader(loader)

	metadata := `{\"Name\":\"artifact_tool\"}`
	resJSON := `{\"CallID\":\"1\",\"Content\":\"ok\"}`
	
	wat := `(module
		(import "axe_kernel" "track_artifact" (func $track (param i32 i32 i64)))
		(memory (export "memory") 1)
		(data (i32.const 0) "` + metadata + `")
		(data (i32.const 100) "test-artifact")
		(data (i32.const 512) "` + resJSON + `")
		(func (export "_initialize"))
		(func (export "allocate") (param i32) (result i32) (i32.const 1024))
		(func (export "Metadata") (result i64)
			(i64.const ` + fmt.Sprintf("%d", len(`{"Name":"artifact_tool"}`)) + `)
		)
		(func (export "Execute") (param i32 i32) (result i64)
			(call $track (i32.const 100) (i32.const 13) (i64.const 1234))
			(i64.const 0x00000200000000` + fmt.Sprintf("%02X", len(`{"CallID":"1","Content":"ok"}`)) + `)
		)
	)`

	wasmBytes := compileWAT(t, wat)
	err = k.LoadPlugin(ctx, reg, wasmBytes)
	if err != nil {
		t.Fatalf("failed to load plugin: %v", err)
	}

	if !reg.Has("artifact_tool") {
		t.Fatal("registry does not have 'artifact_tool'")
	}

	res := k.DispatchToolCall(ctx, protocol.ToolCall{ID: "1", Name: "artifact_tool"}, reg, nil)
	if res.IsError {
		t.Fatalf("tool execution failed: %s", res.Content)
	}

	entries := tracker.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 artifact recorded, got %d", len(entries))
	}
	if entries[0].Path != "test-artifact" {
		t.Errorf("expected path 'test-artifact', got %q", entries[0].Path)
	}
}
