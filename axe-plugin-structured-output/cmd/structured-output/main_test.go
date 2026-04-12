package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jrswab/axe/pkg/plugintest"
	"github.com/jrswab/axe/pkg/protocol"
)

// buildPlugin compiles the code to WASM
func buildPlugin(t *testing.T) string {
	t.Helper()
	wasmPath := filepath.Join(t.TempDir(), "plugin.wasm")
	
	cmd := exec.Command("go", "build", "-buildmode=c-shared", "-o", wasmPath, ".")
	cmd.Dir = "./"
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to compile plugin: %v\nOutput: %s", err, string(out))
	}
	return wasmPath
}

func TestPlugin_ABIConformance(t *testing.T) {
	wasmPath := buildPlugin(t)
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		t.Fatal(err)
	}

	report := plugintest.ValidateABI(wasmBytes)
	if !report.Valid() {
		t.Fatalf("ABI Validation failed:\n%s", report.Error())
	}
}

func TestPlugin_Metadata(t *testing.T) {
	wasmPath := buildPlugin(t)
	wasmBytes, _ := os.ReadFile(wasmPath)

	h := plugintest.NewHarness()
	defer h.Close()
	
	if err := h.Load(wasmBytes); err != nil {
		t.Fatal(err)
	}

	def, err := h.CallMetadata()
	if err != nil {
		t.Fatal(err)
	}

	if def.Name != "structured_output_validator" {
		t.Errorf("expected name 'structured_output_validator', got %q", def.Name)
	}
}

func TestPlugin_Execute_Success(t *testing.T) {
	wasmPath := buildPlugin(t)
	wasmBytes, _ := os.ReadFile(wasmPath)

	h := plugintest.NewHarness()
	defer h.Close()
	h.Load(wasmBytes)

	call := protocol.ToolCall{
		Name: "structured_output_validator",
		Arguments: map[string]string{
			"provider": "anthropic",
			"format":   "json",
		},
	}

	result, err := h.CallExecute(call)
	if err != nil {
		t.Fatal(err)
	}

	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}
	
	if result.Content != "valid" {
		t.Errorf("expected 'valid', got %q", result.Content)
	}
}

func TestPlugin_Execute_BedrockFailure(t *testing.T) {
	wasmPath := buildPlugin(t)
	wasmBytes, _ := os.ReadFile(wasmPath)

	h := plugintest.NewHarness()
	defer h.Close()
	h.Load(wasmBytes)

	call := protocol.ToolCall{
		Name: "structured_output_validator",
		Arguments: map[string]string{
			"provider": "bedrock",
			"format":   "json",
		},
	}

	result, err := h.CallExecute(call)
	if err != nil {
		t.Fatal(err)
	}

	if !result.IsError {
		t.Fatal("expected error for bedrock")
	}
	
	if result.Content != `provider "bedrock" does not support structured output` {
		t.Errorf("unexpected error content: %q", result.Content)
	}
}
