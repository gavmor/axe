package protocol

import (
	"context"
	"fmt"
)

// ErrorCategory classifies provider errors for exit code mapping.
type ErrorCategory string

const (
	// ErrCategoryAuth indicates authentication failure (missing/invalid API key).
	ErrCategoryAuth ErrorCategory = "auth"
	// ErrCategoryRateLimit indicates the provider rate limited the request.
	ErrCategoryRateLimit ErrorCategory = "rate_limit"
	// ErrCategoryTimeout indicates the request timed out.
	ErrCategoryTimeout ErrorCategory = "timeout"
	// ErrCategoryOverloaded indicates the provider is overloaded (529).
	ErrCategoryOverloaded ErrorCategory = "overloaded"
	// ErrCategoryBadRequest indicates a malformed request (invalid model, etc.).
	ErrCategoryBadRequest ErrorCategory = "bad_request"
	// ErrCategoryServer indicates a provider server error (5xx).
	ErrCategoryServer ErrorCategory = "server"
)

// ToolParameter describes a single parameter of a tool definition.
type ToolParameter struct {
	Type        string
	Description string
	Required    bool
}

// ToolDefinition represents a tool definition sent to the LLM.
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]ToolParameter
}

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	ID        string
	Name      string
	Arguments map[string]string
}

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	CallID  string
	Content string
	IsError bool
}

// EventDTO is the envelope sent to event-processing plugins.
type EventDTO struct {
	Type      string            `json:"type"`
	ID        string            `json:"id,omitempty"`
	Name      string            `json:"name,omitempty"`
	Arguments map[string]string `json:"arguments,omitempty"`
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
	ToolCalls   []ToolCall   // Tool calls in an assistant message (non-nil when LLM called tools)
	ToolResults []ToolResult // Tool results in a tool-result message (non-nil when role is "tool")
}

// Request represents an LLM completion request.
type Request struct {
	Model       string
	System      string
	Messages    []Message
	Temperature float64
	MaxTokens   int
	Tools       []ToolDefinition // Tool definitions to send to the LLM. If nil or empty, no tools are sent.
}

// Response represents an LLM completion response.
type Response struct {
	Content      string
	Model        string
	InputTokens  int
	OutputTokens int
	StopReason   string
	ToolCalls    []ToolCall // Tool calls requested by the LLM. Empty if no tools called.
}

// Provider defines the interface for LLM providers.
type Provider interface {
	Send(ctx context.Context, req *Request) (*Response, error)
}

const (
	StreamEventText      = "text"
	StreamEventToolStart = "tool_start"
	StreamEventToolDelta = "tool_delta"
	StreamEventToolEnd   = "tool_end"
	StreamEventDone      = "done"
)

type StreamEvent struct {
	Type         string
	Text         string
	ToolCallID   string
	ToolName     string
	ToolInput    string
	InputTokens  int
	OutputTokens int
	StopReason   string
}

type EventStream interface {
	Next() (StreamEvent, error)
	Close() error
}

type StreamProvider interface {
	Provider
	SendStream(ctx context.Context, req *Request) (EventStream, error)
}

// ProviderError wraps provider-specific errors with categorization.
type ProviderError struct {
	Category ErrorCategory
	Status   int
	Message  string
	Err      error
}

// Error returns a formatted error message: "<category>: <message>".
func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s: %s", e.Category, e.Message)
}

// Unwrap returns the wrapped error, supporting errors.Is and errors.As.
func (e *ProviderError) Unwrap() error {
	return e.Err
}
