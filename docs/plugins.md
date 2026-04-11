# Axe Plugin Development Guide

Welcome to the Axe plugin ecosystem! Axe utilizes a WebAssembly (Wasm) microkernel architecture to safely and dynamically load third-party extensions (like telemetry, logging, and custom tools) at runtime. 

Because Axe executes plugins within a strictly isolated Wasm sandbox, your code cannot crash the host process or access unauthorized system resources. This guide will walk you through building your first Axe-compatible plugin using Go.

## Prerequisites
* **Go 1.24 or higher:** (Go 1.26 is highly recommended, as its runtime manages Wasm heap memory in much smaller increments, significantly reducing the memory footprint for lightweight plugins).
* Access to the Axe shared protocol package (`pkg/protocol`).

## Architecture: The WASI Reactor Model
Axe plugins are not standard executables. They are compiled as **WASI Reactors**.

Unlike a standard Wasm "command" module that runs its `main()` function and immediately terminates, a reactor instance remains continuously alive in memory after its initial setup. This allows the Axe kernel to invoke your plugin's exported functions repeatedly without paying the overhead of re-initialization.

## Step 1: Implement the Contract
First, import the Axe protocol package. Your plugin must expose specific lifecycle functions that the Axe kernel expects: `Metadata`, `Execute`, and `allocate`.

To make a Go function available to the Axe Wasm host, you must annotate it with the `//go:wasmexport` compiler directive.

```go
package main

import (
	"encoding/json"
	"unsafe"
	
	"github.com/jrswab/axe/pkg/protocol"
)

// The main function is required by the compiler but will not be automatically 
// invoked in reactor mode.
func main() {}

//go:wasmexport Metadata
func Metadata() uint64 {
    def := protocol.ToolDefinition{
        Name: "my_custom_tool",
        Description: "A custom tool running in Wasm",
        Parameters: map[string]protocol.ToolParameter{
            "input": {Type: "string", Description: "The input string", Required: true},
        },
    }
    bytes, _ := json.Marshal(def)
    return packPtrLen(uint32(uintptr(unsafe.Pointer(&bytes[0]))), uint32(len(bytes)))
}

//go:wasmexport Execute
func Execute(payloadPtr uint32, payloadLen uint32) uint64 {
    // Read the payload passed by the host from linear memory
    // ... Process the tool call ...
    
    result := protocol.ToolResult{
        Content: "Success from Wasm!",
        IsError: false,
    }
    bytes, _ := json.Marshal(result)
    return packPtrLen(uint32(uintptr(unsafe.Pointer(&bytes[0]))), uint32(len(bytes)))
}

// Utility to pack pointer and length into a single uint64
func packPtrLen(ptr uint32, len uint32) uint64 {
    return uint64(ptr)<<32 | uint64(len)
}

//go:wasmexport allocate
func allocate(size uint32) uint32 {
    buf := make([]byte, size)
    return uint32(uintptr(unsafe.Pointer(&buf[0])))
}
```

## Step 2: Using Host Functions (Capability Injection)
Because your plugin runs in a sandbox, it cannot directly write to the host's filesystem or network. Instead, Axe injects "Host Functions" into your Wasm environment via the `axe_kernel` module.

You can consume these host capabilities using the `//go:wasmimport` directive. 

**Important Memory Note:** WebAssembly utilizes a 32-bit address space, meaning any memory address is represented by a 32-bit integer. You cannot pass complex Go structs or nested pointers directly across the ABI boundary. Instead, you must pass data as a "fat pointer"—a combination of a 32-bit memory address and a 32-bit length.

```go
// Import host functions from the Axe kernel
//go:wasmimport axe_kernel track_artifact
func track_artifact_host(ptr uint32, size uint32, artifactSize int64)

//go:wasmimport axe_kernel get_budget_used
func get_budget_used_host() uint64

// A helper to easily track artifacts back to the host
func TrackArtifact(path string, size int64) {
	ptr := unsafe.Pointer(unsafe.StringData(path))
	track_artifact_host(uint32(uintptr(ptr)), uint32(len(path)), size)
}
```

## Step 3: Compiling Your Plugin
To compile your plugin as a WASI Reactor, you must build it with specific OS/Architecture flags and use the `c-shared` build mode.

Run the following command in your terminal:

```bash
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o my_plugin.wasm main.go
```

By utilizing `-buildmode=c-shared`, the Go linker skips generating a standard `_start` function and instead generates a special `_initialize` function. The Axe kernel loader will automatically invoke `_initialize` to set up your Go runtime and package states before calling any of your `//go:wasmexport` functions.

## Step 4: Installation
Drop your compiled `my_plugin.wasm` file into a directory and point Axe to it using the `--plugins-dir` flag:

```bash
axe run my-agent --plugins-dir ./my-plugins/
```

The Axe kernel will automatically scan the directory, use `watgo` to validate your ABI exports, and hot-load the plugin into the execution pool.
