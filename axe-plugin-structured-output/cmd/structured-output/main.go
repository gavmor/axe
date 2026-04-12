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
		Name:        "structured_output_validator",
		Description: "Validates if a provider supports the requested structured output format.",
		Parameters: map[string]protocol.ToolParameter{
			"provider": {Type: "string", Description: "The provider name (e.g. anthropic, openai)", Required: true},
			"format":   {Type: "string", Description: "The requested format (json or schema)", Required: true},
		},
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
		return errorResult(call.ID, fmt.Sprintf("failed to unmarshal call: %v", err))
	}

	provider := call.Arguments["provider"]
	_ = call.Arguments["format"] // Format could be used for more specific validation logic

	// 2. Delegate to internal logic (simplified here)
	supported := false
	switch provider {
	case "anthropic", "openai", "ollama":
		supported = true
	case "bedrock":
		// Bedrock currently only supports specific models or might not support JSON mode easily
		supported = false
	case "google", "gemini":
		supported = true
	}

	if !supported {
		return errorResult(call.ID, fmt.Sprintf("provider %q does not support structured output", provider))
	}

	// 3. Return result as fat pointer
	res, _ := json.Marshal(protocol.ToolResult{
		CallID:  call.ID,
		Content: "valid",
	})
	return packPtrLen(uint32(uintptr(unsafe.Pointer(&res[0]))), uint32(len(res)))
}

func errorResult(callID, msg string) uint64 {
	res, _ := json.Marshal(protocol.ToolResult{
		CallID:  callID,
		Content: msg,
		IsError: true,
	})
	return packPtrLen(uint32(uintptr(unsafe.Pointer(&res[0]))), uint32(len(res)))
}

//go:wasmexport allocate
func allocate(size uint32) uint32 {
	buf := make([]byte, size)
	return uint32(uintptr(unsafe.Pointer(&buf[0])))
}

func packPtrLen(ptr, length uint32) uint64 {
	return uint64(ptr)<<32 | uint64(length)
}
