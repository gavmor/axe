package plugintest

import (
	"fmt"
	"testing"

	"github.com/eliben/watgo"
	"github.com/jrswab/axe/pkg/protocol"
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
	metadataJSON := `{"Name":"harness_test","Description":"a test tool"}`
	metadataWAT := `{\"Name\":\"harness_test\",\"Description\":\"a test tool\"}`
	wat := `(module
		(memory (export "memory") 1)
		(data (i32.const 0) "` + metadataWAT + `")
		(func (export "_initialize"))
		(func (export "allocate") (param i32) (result i32) (i32.const 1024))
		(func (export "Metadata") (result i64)
			(i64.or
				(i64.shl (i64.const 0) (i64.const 32))
				(i64.const ` + fmt.Sprintf("%d", len(metadataJSON)) + `))
		)
		(func (export "Execute") (param i32 i32) (result i64) (i64.const 0))
	)`

	wasm := compileWAT(t, wat)
	h := NewHarness(t)
	defer h.Close()

	if err := h.Load(wasm); err != nil {
		t.Fatal(err)
	}

	def, err := h.CallMetadata()
	if err != nil {
		t.Fatal(err)
	}

	if def.Name != "harness_test" {
		t.Errorf("expected name 'harness_test', got %q", def.Name)
	}
}

func TestHarness_Execute(t *testing.T) {
	metadataWAT := `{\"Name\":\"exec_test\"}`
	metadataJSON := `{"Name":"exec_test"}`
	resWAT := `{\"CallID\":\"1\",\"Content\":\"ok\",\"IsError\":false}`
	resJSON := `{"CallID":"1","Content":"ok","IsError":false}`
	wat := `(module
		(memory (export "memory") 1)
		(data (i32.const 0) "` + metadataWAT + `")
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

	wasm := compileWAT(t, wat)
	h := NewHarness(t)
	defer h.Close()

	h.Load(wasm)
	res, err := h.CallExecute(protocol.ToolCall{ID: "1"})
	if err != nil {
		t.Fatal(err)
	}

	if res.Content != "ok" {
		t.Errorf("expected content 'ok', got %q", res.Content)
	}
}

func TestHarness_ArtifactTracking(t *testing.T) {
	metadataWAT := `{\"Name\":\"artifact_test\"}`
	metadataJSON := `{"Name":"artifact_test"}`
	resWAT := `{\"CallID\":\"1\",\"Content\":\"done\"}`
	resJSON := `{"CallID":"1","Content":"done"}`
	artifactPath := "test-file.txt"
	wat := `(module
		(import "axe_kernel" "track_artifact" (func $track (param i32 i32 i64)))
		(memory (export "memory") 1)
		(data (i32.const 0) "` + metadataWAT + `")
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

	wasm := compileWAT(t, wat)
	h := NewHarness(t).WithMockArtifactTracker()
	defer h.Close()

	h.Load(wasm)
	h.CallExecute(protocol.ToolCall{})

	artifacts := h.TrackedArtifacts()
	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(artifacts))
	}
	if artifacts[0].Path != artifactPath {
		t.Errorf("expected path %q, got %q", artifactPath, artifacts[0].Path)
	}
}

func TestHarness_BudgetMock(t *testing.T) {
	metadataWAT := `{\"Name\":\"budget_test\"}`
	metadataJSON := `{"Name":"budget_test"}`
	resWAT := `{\"CallID\":\"1\",\"Content\":\"budget_ok\"}`
	resJSON := `{"CallID":"1","Content":"budget_ok"}`
	wat := `(module
		(import "axe_kernel" "get_budget_used" (func $used (result i64)))
		(import "axe_kernel" "get_budget_max" (func $max (result i64)))
		(memory (export "memory") 1)
		(data (i32.const 0) "` + metadataWAT + `")
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

	wasm := compileWAT(t, wat)
	h := NewHarness(t).WithBudget(100, 1000)
	defer h.Close()

	h.Load(wasm)
	res, _ := h.CallExecute(protocol.ToolCall{})
	if res.Content != "budget_ok" {
		t.Errorf("expected content 'budget_ok', got %q", res.Content)
	}
}
