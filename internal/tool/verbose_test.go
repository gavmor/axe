package tool

import (
	"bytes"
	"testing"

	"github.com/jrswab/axe/internal/provider"
)

func TestToolVerboseLog(t *testing.T) {
	tests := []struct {
		name     string
		ec       ExecContext
		toolName string
		result   provider.ToolResult
		summary  string
		want     string
	}{
		{
			name:     "success format",
			ec:       ExecContext{Verbose: true, Stderr: &bytes.Buffer{}},
			toolName: "read_file",
			result:   provider.ToolResult{IsError: false},
			summary:  `path "hello.txt" (10 lines)`,
			want:     "[tool] read_file: path \"hello.txt\" (10 lines) (success)\n",
		},
		{
			name:     "error format",
			ec:       ExecContext{Verbose: true, Stderr: &bytes.Buffer{}},
			toolName: "write_file",
			result:   provider.ToolResult{IsError: true},
			summary:  `path "out.txt": permission denied`,
			want:     "[tool] write_file: path \"out.txt\": permission denied (error)\n",
		},
		{
			name:     "nil stderr does not panic",
			ec:       ExecContext{Verbose: true, Stderr: nil},
			toolName: "read_file",
			result:   provider.ToolResult{IsError: false},
			summary:  "test",
			want:     "",
		},
		{
			name:     "verbose false emits nothing",
			ec:       ExecContext{Verbose: false, Stderr: &bytes.Buffer{}},
			toolName: "read_file",
			result:   provider.ToolResult{IsError: false},
			summary:  "test",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolVerboseLog(tt.ec, tt.toolName, tt.result, tt.summary)

			if tt.ec.Stderr == nil {
				return // nothing to check
			}

			buf, ok := tt.ec.Stderr.(*bytes.Buffer)
			if !ok {
				t.Fatal("stderr is not a *bytes.Buffer")
			}

			got := buf.String()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
