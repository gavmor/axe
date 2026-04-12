package plugintest

import (
	"fmt"
	"strings"
	"testing"
)

func TestValidateABI_ValidPlugin(t *testing.T) {
	metadata := `{\"Name\":\"valid_plugin\"}`
	metadataJSON := `{"Name":"valid_plugin"}`
	resWAT := `{\"CallID\":\"1\",\"Content\":\"ok\"}`
	resJSON := `{"CallID":"1","Content":"ok"}`
	wat := `(module
		(import "axe_kernel" "track_artifact" (func $track (param i32 i32 i64)))
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
			(i64.or
				(i64.shl (i64.const 512) (i64.const 32))
				(i64.const ` + fmt.Sprintf("%d", len(resJSON)) + `))
		)
	)`
	wasmBytes := compileWAT(t, wat)

	report := ValidateABI(wasmBytes)
	if !report.Valid() {
		t.Fatalf("expected valid, got errors: %v", report.Error())
	}
	if len(report.Warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", report.Warnings)
	}
}

func TestValidateABI_MissingExport(t *testing.T) {
	// Missing Metadata export
	wat := `(module
		(memory (export "memory") 1)
		(func (export "_initialize"))
		(func (export "allocate") (param i32) (result i32) (i32.const 1024))
		(func (export "Execute") (param i32 i32) (result i64) (i64.const 0))
	)`
	wasmBytes := compileWAT(t, wat)

	report := ValidateABI(wasmBytes)
	if report.Valid() {
		t.Fatal("expected errors for missing Metadata export")
	}

	found := false
	for _, e := range report.Errors {
		if e.Export == "Metadata" && strings.Contains(e.Detail, "missing required export") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about missing Metadata, got: %v", report.Errors)
	}
}

func TestValidateABI_WrongSignature_Execute(t *testing.T) {
	// Execute takes (i32) instead of (i32, i32)
	wat := `(module
		(memory (export "memory") 1)
		(func (export "_initialize"))
		(func (export "allocate") (param i32) (result i32) (i32.const 1024))
		(func (export "Metadata") (result i64) (i64.const 0))
		(func (export "Execute") (param i32) (result i64) (i64.const 0))
	)`
	wasmBytes := compileWAT(t, wat)

	report := ValidateABI(wasmBytes)
	if report.Valid() {
		t.Fatal("expected errors for wrong Execute signature")
	}

	found := false
	for _, e := range report.Errors {
		if e.Export == "Execute" && strings.Contains(e.Detail, "parameter mismatch") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected parameter mismatch for Execute, got: %v", report.Errors)
	}
}

func TestValidateABI_WrongSignature_Metadata(t *testing.T) {
	// Metadata returns i32 instead of i64
	wat := `(module
		(memory (export "memory") 1)
		(func (export "_initialize"))
		(func (export "allocate") (param i32) (result i32) (i32.const 1024))
		(func (export "Metadata") (result i32) (i32.const 0))
		(func (export "Execute") (param i32 i32) (result i64) (i64.const 0))
	)`
	wasmBytes := compileWAT(t, wat)

	report := ValidateABI(wasmBytes)
	if report.Valid() {
		t.Fatal("expected errors for wrong Metadata signature")
	}

	found := false
	for _, e := range report.Errors {
		if e.Export == "Metadata" && strings.Contains(e.Detail, "result mismatch") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected result mismatch for Metadata, got: %v", report.Errors)
	}
}

func TestValidateABI_WrongImportSignature(t *testing.T) {
	// track_artifact with wrong params: (i32, i32) instead of (i32, i32, i64)
	wat := `(module
		(import "axe_kernel" "track_artifact" (func $track (param i32 i32)))
		(memory (export "memory") 1)
		(func (export "_initialize"))
		(func (export "allocate") (param i32) (result i32) (i32.const 1024))
		(func (export "Metadata") (result i64) (i64.const 0))
		(func (export "Execute") (param i32 i32) (result i64) (i64.const 0))
	)`
	wasmBytes := compileWAT(t, wat)

	report := ValidateABI(wasmBytes)
	if report.Valid() {
		t.Fatal("expected errors for wrong track_artifact signature")
	}

	found := false
	for _, e := range report.Errors {
		if e.Export == "import:track_artifact" && strings.Contains(e.Detail, "signature mismatch") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected signature mismatch for track_artifact import, got: %v", report.Errors)
	}
}

func TestValidateABI_UnknownImport(t *testing.T) {
	// Unknown axe_kernel function
	wat := `(module
		(import "axe_kernel" "unknown_func" (func $unk))
		(memory (export "memory") 1)
		(func (export "_initialize"))
		(func (export "allocate") (param i32) (result i32) (i32.const 1024))
		(func (export "Metadata") (result i64) (i64.const 0))
		(func (export "Execute") (param i32 i32) (result i64) (i64.const 0))
	)`
	wasmBytes := compileWAT(t, wat)

	report := ValidateABI(wasmBytes)
	// Should be valid (unknown imports are warnings, not errors)
	if !report.Valid() {
		t.Fatalf("expected valid (unknown imports are warnings), got errors: %v", report.Error())
	}

	found := false
	for _, w := range report.Warnings {
		if strings.Contains(w, "unknown_func") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about unknown_func, got warnings: %v", report.Warnings)
	}
}

func TestValidateABI_InvalidWasm(t *testing.T) {
	report := ValidateABI([]byte("not valid wasm"))
	if report.Valid() {
		t.Fatal("expected errors for invalid wasm bytes")
	}

	found := false
	for _, e := range report.Errors {
		if e.Export == "(module)" && strings.Contains(e.Detail, "failed to decode") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected decode error, got: %v", report.Errors)
	}
}
