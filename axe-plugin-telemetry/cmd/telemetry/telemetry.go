//go:build wasip1

package main

import (
	"encoding/json"
	"fmt"
	"unsafe"

	"github.com/jrswab/axe/pkg/protocol"
)

func main() {} // Required but unused in Reactor mode

//go:wasmexport Metadata
func Metadata() uint64 {
	def := protocol.ToolDefinition{
		Name:        "telemetry",
		Description: "Records agent performance and token usage events.",
	}
	b, _ := json.Marshal(def)
	return packPtrLen(uint32(uintptr(unsafe.Pointer(&b[0]))), uint32(len(b)))
}

//go:wasmexport Execute
func Execute(ptr uint32, length uint32) uint64 {
	// 1. Read input from host memory
	payload := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), length)

	var call protocol.ToolCall
	if err := json.Unmarshal(payload, &call); err != nil {
		res, _ := json.Marshal(protocol.ToolResult{Content: fmt.Sprintf("unmarshal call error: %v", err), IsError: true})
		return packPtrLen(uint32(uintptr(unsafe.Pointer(&res[0]))), uint32(len(res)))
	}

	// 2. Delegate to internal logic
	var event protocol.Event
	eventJSON := call.Arguments["event_json"]
	if err := json.Unmarshal([]byte(eventJSON), &event); err != nil {
		res, _ := json.Marshal(protocol.ToolResult{Content: fmt.Sprintf("unmarshal event error: %v", err), IsError: true})
		return packPtrLen(uint32(uintptr(unsafe.Pointer(&res[0]))), uint32(len(res)))
	}

	// Simple mock telemetry logging
	fmt.Printf("[plugin-telemetry] Event received: %s\n", event.Topic)

	if event.Topic == "core.response_received" {
		trackArtifactHelper("/tmp/telemetry.json", 100)
	}

	// 3. Return result as fat pointer
	res, _ := json.Marshal(protocol.ToolResult{Content: "Telemetry recorded"})
	return packPtrLen(uint32(uintptr(unsafe.Pointer(&res[0]))), uint32(len(res)))
}

//go:wasmimport axe_kernel track_artifact
func trackArtifact(ptr uint32, len uint32, size int64)

func trackArtifactHelper(path string, size int64) {
	// In a real WASM build, this calls the host. 
	// In a standard GOARCH build (like during 'go test' if not careful), this fails.
	// But our TestPlugin_ArtifactTracking builds with GOOS=wasip1 so it should work.
	// The issue might be that the package itself is being checked by the host 'go test' before the sub-build.
	trackArtifact(
		uint32(uintptr(unsafe.Pointer(&[]byte(path)[0]))), uint32(len(path)),
		size,
	)
}

//go:wasmexport allocate
func allocate(size uint32) uint32 {
	buf := make([]byte, size)
	return uint32(uintptr(unsafe.Pointer(&buf[0])))
}

func packPtrLen(ptr, length uint32) uint64 {
	return uint64(ptr)<<32 | uint64(length)
}
