package kernel

import "testing"

func TestStripThinkingTokens(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no thinking tokens",
			input: `{"title":"Hello"}`,
			want:  `{"title":"Hello"}`,
		},
		{
			name:  "qwen think tags",
			input: `<think>Let me reason about this...</think>{"title":"Hello"}`,
			want:  `{"title":"Hello"}`,
		},
		{
			name:  "gemma channel tags",
			input: `<|channel>thought I should analyze this carefully<channel|>{"title":"Hello"}`,
			want:  `{"title":"Hello"}`,
		},
		{
			name:  "multiline thinking",
			input: "<think>\nStep 1: analyze\nStep 2: plan\n</think>\nThe answer is 42",
			want:  "The answer is 42",
		},
		{
			name:  "empty after strip",
			input: "<think>only thinking here</think>",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripThinkingTokens(tt.input)
			if got != tt.want {
				t.Errorf("StripThinkingTokens() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterThinkingChunk(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		closeTag     string
		wantText     string
		wantCloseTag string
	}{
		{
			name:         "normal text",
			text:         "hello world",
			closeTag:     "",
			wantText:     "hello world",
			wantCloseTag: "",
		},
		{
			name:         "complete think block in one chunk",
			text:         "<think>reasoning</think>answer",
			closeTag:     "",
			wantText:     "answer",
			wantCloseTag: "",
		},
		{
			name:         "think opens, no close yet",
			text:         "prefix<think>start of reasoning",
			closeTag:     "",
			wantText:     "prefix",
			wantCloseTag: "</think>",
		},
		{
			name:         "inside thinking, no close yet",
			text:         "more reasoning text",
			closeTag:     "</think>",
			wantText:     "",
			wantCloseTag: "</think>",
		},
		{
			name:         "close tag arrives",
			text:         "end of reasoning</think>actual output",
			closeTag:     "</think>",
			wantText:     "actual output",
			wantCloseTag: "",
		},
		{
			name:         "gemma open",
			text:         "<|channel>thought analyzing the problem",
			closeTag:     "",
			wantText:     "",
			wantCloseTag: "<channel|>",
		},
		{
			name:         "gemma close",
			text:         "done thinking<channel|>The result is:",
			closeTag:     "<channel|>",
			wantText:     "The result is:",
			wantCloseTag: "",
		},
		{
			name:         "complete gemma block",
			text:         "<|channel>thought something<channel|>output",
			closeTag:     "",
			wantText:     "output",
			wantCloseTag: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotText, gotClose := filterThinkingChunk(tt.text, tt.closeTag)
			if gotText != tt.wantText {
				t.Errorf("text = %q, want %q", gotText, tt.wantText)
			}
			if gotClose != tt.wantCloseTag {
				t.Errorf("closeTag = %q, want %q", gotClose, tt.wantCloseTag)
			}
		})
	}
}
