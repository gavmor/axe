package protocol

import (
	"context"
	"fmt"
)

// ErrorCategory classifies provider errors for exit code mapping.
type ErrorCategory string

const (
	ErrCategoryAuth       ErrorCategory = "auth"
	ErrCategoryRateLimit  ErrorCategory = "rate_limit"
	ErrCategoryTimeout    ErrorCategory = "timeout"
	ErrCategoryOverloaded ErrorCategory = "overloaded"
	ErrCategoryBadRequest ErrorCategory = "bad_request"
	ErrCategoryServer     ErrorCategory = "server"
)

// ToolParameter describes a single parameter of a tool definition.
type ToolParameter struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// ToolDefinition represents a tool definition sent to the LLM.
type ToolDefinition struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Parameters  map[string]ToolParameter `json:"parameters"`
}

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments"`
}

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	CallID  string `json:"call_id"`
	Content string `json:"content"`
	IsError bool   `json:"is_error"`
}

// Tool interface defines an executable tool the agent can use.
type Tool interface {
	Definition() ToolDefinition
	Execute(ctx context.Context, call ToolCall) ToolResult
}

// Message represents a single message in the conversation.
type Message struct {
	Role        string       `json:"role"`
	Content     string       `json:"content"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	ToolResults []ToolResult `json:"tool_results,omitempty"`
}

// Request represents an LLM completion request.
type Request struct {
	Model       string                 `json:"model"`
	System      string                 `json:"system"`
	Messages    []Message              `json:"messages"`
	Temperature float64                `json:"temperature"`
	MaxTokens   int                    `json:"max_tokens"`
	Tools       []ToolDefinition       `json:"tools,omitempty"`
	Extensions  map[string]interface{} `json:"extensions,omitempty"`
}

// Response represents an LLM completion response.
type Response struct {
	Content      string                 `json:"content"`
	Model        string                 `json:"model"`
	InputTokens  int                    `json:"input_tokens"`
	OutputTokens int                    `json:"output_tokens"`
	StopReason   string                 `json:"stop_reason"`
	ToolCalls    []ToolCall             `json:"tool_calls,omitempty"`
	Extensions   map[string]interface{} `json:"extensions,omitempty"`
}

// Provider defines the interface for LLM providers.
type Provider interface {
	Send(ctx context.Context, req *Request) (*Response, error)
	// SupportsExtension allows the host or plugins to query capability support.
	SupportsExtension(key string, value interface{}) bool
}

// --- Event Bus Protocol ---

type Event struct {
	Topic    string                 `json:"topic"`
	Payload  map[string]interface{} `json:"payload"`
	Metadata map[string]string      `json:"metadata"`
}

const (
	TopicResponseReceived = "core.response_received"
	TopicToolExecuted     = "core.tool_executed"
	TopicErrorOccurred    = "core.error"
)

// --- Provider Streaming ---

const (
	StreamEventText      = "text"
	StreamEventToolStart = "tool_start"
	StreamEventToolDelta = "tool_delta"
	StreamEventToolEnd   = "tool_end"
	StreamEventDone      = "done"
)

type StreamEvent struct {
	Type         string `json:"type"`
	Text         string `json:"text,omitempty"`
	ToolCallID   string `json:"tool_call_id,omitempty"`
	ToolName     string `json:"tool_name,omitempty"`
	ToolInput    string `json:"tool_input,omitempty"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
}

type EventStream interface {
	Next() (StreamEvent, error)
	Close() error
}

type StreamProvider interface {
	Provider
	SendStream(ctx context.Context, req *Request) (EventStream, error)
}

// ProviderError wraps provider-specific errors.
type ProviderError struct {
	Category ErrorCategory
	Status   int
	Message  string
	Err      error
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s: %s", e.Category, e.Message)
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}
