package tool

import (
	"fmt"

	"github.com/jrswab/axe/internal/provider"
)

// toolVerboseLog emits a single log line to ec.Stderr when ec.Verbose is true.
// Format: [tool] <toolName>: <summary> (success|error)
func toolVerboseLog(ec ExecContext, toolName string, result provider.ToolResult, summary string) {
	if !ec.Verbose || ec.Stderr == nil {
		return
	}

	status := "success"
	if result.IsError {
		status = "error"
	}

	_, _ = fmt.Fprintf(ec.Stderr, "[tool] %s: %s (%s)\n", toolName, summary, status)
}
