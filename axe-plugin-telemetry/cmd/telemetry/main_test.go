package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jrswab/axe/pkg/plugintest"
	"github.com/jrswab/axe/pkg/protocol"
)

// buildPlugin is a helper to compile our code to WASM before each test run
func buildPlugin(t *testing.T) string {
	t.Helper()
	wasmPath := filepath.Join(t.TempDir(), "plugin.wasm")
	
	// Ensure we are in the directory containing the plugin source
	cmd := exec.Command("go", "build", "-buildmode=c-shared", "-o", wasmPath, ".")
	cmd.Dir = "../../cmd/telemetry"
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

	if def.Name != "telemetry" {
		t.Errorf("expected name 'telemetry', got %q", def.Name)
	}
}

func TestPlugin_Execute_Success(t *testing.T) {
	wasmPath := buildPlugin(t)
	wasmBytes, _ := os.ReadFile(wasmPath)

	h := plugintest.NewHarness()
	defer h.Close()
	h.Load(wasmBytes)

	call := protocol.ToolCall{
		Name: "telemetry",
		Arguments: map[string]string{
			"event_json": func() string {
				// Simplified for test
				return "{}"
			}(),
		},
	}

	result, err := h.CallExecute(call)
	if err != nil {
		t.Fatal(err)
	}

	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}
	
	if result.Content != "Telemetry recorded" {
		t.Errorf("expected 'Telemetry recorded', got %q", result.Content)
	}
}

func TestPlugin_ArtifactTracking(t *testing.T) {
	wasmPath := buildPlugin(t)
	wasmBytes, _ := os.ReadFile(wasmPath)

	// Enable the mock artifact tracker in the harness
	h := plugintest.NewHarness().WithMockArtifactTracker()
	defer h.Close()
	h.Load(wasmBytes)

	call := protocol.ToolCall{
		Name: "telemetry",
		Arguments: map[string]string{
			"event_json": `{"topic":"core.response_received"}`,
		},
	}

	_, err := h.CallExecute(call)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the plugin called the host function 'track_artifact'
	artifacts := h.TrackedArtifacts()
	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact to be tracked, got %d", len(artifacts))
	}
	if artifacts[0].Path != "/tmp/telemetry.json" {
		t.Errorf("expected path '/tmp/telemetry.json', got %q", artifacts[0].Path)
	}
}
