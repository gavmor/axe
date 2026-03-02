package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/toolname"
)

// writeFileEntry returns the ToolEntry for the write_file tool.
func writeFileEntry() ToolEntry {
	return ToolEntry{
		Definition: writeFileDefinition,
		Execute:    writeFileExecute,
	}
}

func writeFileDefinition() provider.Tool {
	return provider.Tool{
		Name:        toolname.WriteFile,
		Description: "Create or overwrite a file relative to the working directory. Creates parent directories as needed. Overwrites the file if it already exists.",
		Parameters: map[string]provider.ToolParameter{
			"path": {
				Type:        "string",
				Description: "Relative path to the file to write.",
				Required:    true,
			},
			"content": {
				Type:        "string",
				Description: "The content to write to the file.",
				Required:    false,
			},
		},
	}
}

func writeFileExecute(ctx context.Context, call provider.ToolCall, ec ExecContext) (result provider.ToolResult) {
	path := call.Arguments["path"]

	defer func() {
		var summary string
		if result.IsError {
			summary = fmt.Sprintf("path %q: %s", path, result.Content)
		} else {
			summary = fmt.Sprintf("path %q (%d bytes)", path, len(call.Arguments["content"]))
		}
		toolVerboseLog(ec, toolname.WriteFile, result, summary)
	}()

	// Empty path check.
	if path == "" {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: "path is required",
			IsError: true,
		}
	}

	// Absolute path check.
	if filepath.IsAbs(path) {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: "absolute paths are not allowed",
			IsError: true,
		}
	}

	// Traversal fast-path check.
	cleanWorkdir := filepath.Clean(ec.Workdir)
	resolved := filepath.Clean(filepath.Join(cleanWorkdir, path))

	if !isWithinDir(resolved, cleanWorkdir) {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: "path escapes workdir",
			IsError: true,
		}
	}

	// Create parent directories.
	parent := filepath.Dir(resolved)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: err.Error(),
			IsError: true,
		}
	}

	// Symlink escape check on parent directory.
	evalParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: err.Error(),
			IsError: true,
		}
	}

	evalWorkdir, err := filepath.EvalSymlinks(cleanWorkdir)
	if err != nil {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: err.Error(),
			IsError: true,
		}
	}

	if !isWithinDir(evalParent, evalWorkdir) {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: "path escapes workdir",
			IsError: true,
		}
	}

	// Extract content (missing or empty key writes 0-byte file).
	content := call.Arguments["content"]

	// Write file.
	data := []byte(content)
	if err := os.WriteFile(resolved, data, 0o644); err != nil {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: err.Error(),
			IsError: true,
		}
	}

	return provider.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("wrote %d bytes to %s", len(data), path),
		IsError: false,
	}
}
