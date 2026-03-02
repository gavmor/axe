package tool

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/toolname"
)

// editFileEntry returns the ToolEntry for the edit_file tool.
func editFileEntry() ToolEntry {
	return ToolEntry{
		Definition: editFileDefinition,
		Execute:    editFileExecute,
	}
}

func editFileDefinition() provider.Tool {
	return provider.Tool{
		Name:        toolname.EditFile,
		Description: "Find and replace exact text in an existing file relative to the working directory. Supports single replacement (default) and replace-all mode.",
		Parameters: map[string]provider.ToolParameter{
			"path": {
				Type:        "string",
				Description: "Relative path to the file to edit.",
				Required:    true,
			},
			"old_string": {
				Type:        "string",
				Description: "The exact text to find in the file.",
				Required:    true,
			},
			"new_string": {
				Type:        "string",
				Description: "The text to replace old_string with.",
				Required:    true,
			},
			"replace_all": {
				Type:        "string",
				Description: "When set to \"true\", all occurrences are replaced. Defaults to false (single replacement).",
				Required:    false,
			},
		},
	}
}

func editFileExecute(ctx context.Context, call provider.ToolCall, ec ExecContext) (result provider.ToolResult) {
	path := call.Arguments["path"]
	replacedCount := 0

	defer func() {
		var summary string
		if result.IsError {
			summary = fmt.Sprintf("path %q: %s", path, result.Content)
		} else {
			summary = fmt.Sprintf("path %q (%d occurrence(s))", path, replacedCount)
		}
		toolVerboseLog(ec, toolname.EditFile, result, summary)
	}()

	// Path validation (handles empty, absolute, traversal, symlink escape, nonexistent).
	resolved, err := validatePath(ec.Workdir, path)
	if err != nil {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: err.Error(),
			IsError: true,
		}
	}

	// Directory rejection.
	info, err := os.Stat(resolved)
	if err != nil {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: err.Error(),
			IsError: true,
		}
	}
	if info.IsDir() {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: "path is a directory, not a file",
			IsError: true,
		}
	}

	// Extract old_string — empty is rejected.
	oldString := call.Arguments["old_string"]
	if oldString == "" {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: "old_string is required",
			IsError: true,
		}
	}

	// Extract new_string — empty is valid (deletion).
	newString := call.Arguments["new_string"]

	// Read file contents.
	data, err := os.ReadFile(resolved)
	if err != nil {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: err.Error(),
			IsError: true,
		}
	}
	content := string(data)

	// Parse replace_all.
	replaceAll := false
	if v := call.Arguments["replace_all"]; v != "" {
		replaceAll, err = strconv.ParseBool(v)
		if err != nil {
			return provider.ToolResult{
				CallID:  call.ID,
				Content: fmt.Sprintf("invalid replace_all value %q: %s", v, err.Error()),
				IsError: true,
			}
		}
	}

	// Count occurrences.
	count := strings.Count(content, oldString)

	if count == 0 {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: "old_string not found in file",
			IsError: true,
		}
	}

	if count > 1 && !replaceAll {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("old_string found %d times; set replace_all to true or provide a more unique string", count),
			IsError: true,
		}
	}

	// Perform replacement.
	var newContent string
	var replaced int
	if replaceAll {
		newContent = strings.ReplaceAll(content, oldString, newString)
		replaced = count
	} else {
		newContent = strings.Replace(content, oldString, newString, 1)
		replaced = 1
	}

	// Write file back.
	if err := os.WriteFile(resolved, []byte(newContent), 0o644); err != nil {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: err.Error(),
			IsError: true,
		}
	}

	replacedCount = replaced

	return provider.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("replaced %d occurrence(s) in %s", replaced, path),
		IsError: false,
	}
}
