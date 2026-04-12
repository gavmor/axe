package plugintest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/eliben/watgo"
)

func compileWAT(t *testing.T, wat string) []byte {
	t.Helper()
	wasm, err := watgo.CompileWATToWASM([]byte(wat))
	if err != nil {
		t.Fatalf("failed to compile WAT: %v\nWAT source:\n%s", err, wat)
	}
	return wasm
}

func TestHarness_Metadata(t *testing.T) {
	metadata := `{\"Name\":\"harness_test\",\"Description\":\"a test tool\"}`
	wat := `(module
		(memory (export "memory") 1)
		(data (i32.const 0) "` + metadata + `")
		(func (export "_initialize"))
		(func (export "allocate") (param i32) (result i32) (i32.const 1024))
		(func (export "Metadata") (result i64)
			(i64.or
				(i64.shl (i64.const 0) (i64.const 32))
				(i64.const ` + fmt.Sprintf("%d", len(`{"Name":"harness_test","Description":"a test tool"}`)) + `))
		)
		(func (export "Execute") (param i32 i32) (result i64) (i64.const 0))
	)`
	wasmBytes := compileWAT(t, wat)

	h := NewHarness()
	defer h.Close()

	if err := h.Load(wasmBytes); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	def, err := h.CallMetadata()
	if err != nil {
		t.Fatalf("CallMetadata failed: %v", err)
	}

	if def.Name != "harness_test" {
		t.Errorf("expected name 'harness_test', got %q", def.Name)
	}
	if def.Description != "a test tool" {
		t.Errorf("expected description 'a test tool', got %q", def.Description)
	}
}

func TestHarness_Execute(t *testing.T) {
	metadata := `{\"Name\":\"exec_test\"}`
	metadataJSON := `{"Name":"exec_test"}`
	resWAT := `{\"CallID\":\"1\",\"Content\":\"ok\",\"IsError\":false}`
	resJSON := `{"CallID":"1","Content":"ok","IsError":false}`
	wat := `(module
		(memory (export "memory") 1)
		(data (i32.const 0) "` + metadata + `")
		(data (i32.const 512) "` + resWAT + `")
		(func (export "_initialize"))
		(func (export "allocate") (param i32) (result i32) (i32.const 1024))
		(func (export "Metadata") (result i64)
			(i64.const ` + fmt.Sprintf("%d", len(metadataJSON)) + `)
		)
		(func (export "Execute") (param i32 i32) (result i64)
			(i64.or
				(i64.shl (i64.const 512) (i64.const 32))
				(i64.const ` + fmt.Sprintf("%d", len(resJSON)) + `))
		)
	)`
	wasmBytes := compileWAT(t, wat)

	h := NewHarness()
	defer h.Close()

	if err := h.Load(wasmBytes); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	result, err := h.CallExecuteRaw([]byte(`{"ID":"1","Name":"exec_test"}`))
	if err != nil {
		t.Fatalf("CallExecuteRaw failed: %v", err)
	}

	if result.IsError {
		t.Errorf("expected no error, got: %s", result.Content)
	}
	if result.Content != "ok" {
		t.Errorf("expected content 'ok', got %q", result.Content)
	}
}

func TestHarness_ArtifactTracking(t *testing.T) {
	metadata := `{\"Name\":\"artifact_test\"}`
	metadataJSON := `{"Name":"artifact_test"}`
	resWAT := `{\"CallID\":\"1\",\"Content\":\"done\"}`
	resJSON := `{"CallID":"1","Content":"done"}`
	artifactPath := "test-file.txt"
	wat := `(module
		(import "axe_kernel" "track_artifact" (func $track (param i32 i32 i64)))
		(memory (export "memory") 1)
		(data (i32.const 0) "` + metadata + `")
		(data (i32.const 100) "` + artifactPath + `")
		(data (i32.const 512) "` + resWAT + `")
		(func (export "_initialize"))
		(func (export "allocate") (param i32) (result i32) (i32.const 1024))
		(func (export "Metadata") (result i64)
			(i64.const ` + fmt.Sprintf("%d", len(metadataJSON)) + `)
		)
		(func (export "Execute") (param i32 i32) (result i64)
			(call $track (i32.const 100) (i32.const ` + fmt.Sprintf("%d", len(artifactPath)) + `) (i64.const 4096))
			(i64.or
				(i64.shl (i64.const 512) (i64.const 32))
				(i64.const ` + fmt.Sprintf("%d", len(resJSON)) + `))
		)
	)`
	wasmBytes := compileWAT(t, wat)

	h := NewHarness().WithMockArtifactTracker()
	defer h.Close()

	if err := h.Load(wasmBytes); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	_, err := h.CallExecuteRaw([]byte(`{"ID":"1"}`))
	if err != nil {
		t.Fatalf("CallExecuteRaw failed: %v", err)
	}

	artifacts := h.TrackedArtifacts()
	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(artifacts))
	}
	if artifacts[0].Path != "test-file.txt" {
		t.Errorf("expected path 'test-file.txt', got %q", artifacts[0].Path)
	}
	if artifacts[0].Size != 4096 {
		t.Errorf("expected size 4096, got %d", artifacts[0].Size)
	}
}

func TestHarness_BudgetMock(t *testing.T) {
	metadata := `{\"Name\":\"budget_test\"}`
	metadataJSON := `{"Name":"budget_test"}`
	resWAT := `{\"CallID\":\"1\",\"Content\":\"budget_ok\"}`
	resJSON := `{"CallID":"1","Content":"budget_ok"}`
	wat := `(module
		(import "axe_kernel" "get_budget_used" (func $used (result i64)))
		(import "axe_kernel" "get_budget_max" (func $max (result i64)))
		(memory (export "memory") 1)
		(data (i32.const 0) "` + metadata + `")
		(data (i32.const 512) "` + resWAT + `")
		(func (export "_initialize"))
		(func (export "allocate") (param i32) (result i32) (i32.const 1024))
		(func (export "Metadata") (result i64)
			(i64.const ` + fmt.Sprintf("%d", len(metadataJSON)) + `)
		)
		(func (export "Execute") (param i32 i32) (result i64)
			;; Call both budget functions to verify they don't trap
			(drop (call $used))
			(drop (call $max))
			(i64.or
				(i64.shl (i64.const 512) (i64.const 32))
				(i64.const ` + fmt.Sprintf("%d", len(resJSON)) + `))
		)
	)`
	wasmBytes := compileWAT(t, wat)

	h := NewHarness().WithBudget(100, 1000)
	defer h.Close()

	if err := h.Load(wasmBytes); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	result, err := h.CallExecuteRaw([]byte(`{"ID":"1"}`))
	if err != nil {
		t.Fatalf("CallExecuteRaw failed: %v", err)
	}
	if result.Content != "budget_ok" {
		t.Errorf("expected 'budget_ok', got %q", result.Content)
	}
}

func TestHarness_LoadFailure(t *testing.T) {
	h := NewHarness()
	defer h.Close()

	err := h.Load([]byte("not valid wasm"))
	if err == nil {
		t.Fatal("expected error loading invalid wasm, got nil")
	}
	if !strings.Contains(err.Error(), "failed to compile plugin") {
		t.Errorf("expected compile error, got: %v", err)
	}
}
