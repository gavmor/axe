package protocol

import (
	"context"
	"fmt"

	"github.com/gavmor/axe-protocol"
)

// Re-export types from the domain protocol for host-side convenience.
type ToolDefinition = protocol.ToolDefinition
type ToolCall = protocol.ToolCall
type ToolResult = protocol.ToolResult
type EventDTO = protocol.EventDTO
type ToolParameter = protocol.ToolParameter

// ErrorCategory classifies provider errors for exit code mapping.
type ErrorCategory string

const (
	ErrCategoryAuth       ErrorCategory = "auth"
	ErrCategoryRateLimit  ErrorCategory = "rate_limit"
	ErrCategoryTimeout    ErrorCategory = "timeout"
	ErrCategoryOverloaded ErrorCategory = "overloaded"
	ErrCategoryBadRequest ErrorCategory = "bad_request"
	ErrCategoryServer     ErrorCategory = "server"
	ErrCategoryInput      ErrorCategory = "input"
	ErrCategoryNetwork    ErrorCategory = "network"
	ErrCategoryUnknown    ErrorCategory = "unknown"
)

// Tool interface defines an executable tool the agent can use.
type Tool interface {
	Definition() ToolDefinition
	Execute(ctx context.Context, call ToolCall) ToolResult
}

// Message represents a single message in the conversation.
type Message struct {
	Role        string       `json:"role"`
	Content     string       `json:"content"`
	ToolCalls   []ToolCall   // Tool calls in an assistant message
	ToolResults []ToolResult // Tool results in a tool-result message
}

// Request represents an LLM completion request.
type Request struct {
	Model       string
	System      string
	Messages    []Message
	Temperature float64
	MaxTokens   int
	Tools       []ToolDefinition
	Think       *bool
}

// Response represents an LLM completion response.
type Response struct {
	Content      string
	Model        string
	InputTokens  int
	OutputTokens int
	StopReason   string
	ToolCalls    []ToolCall
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

func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s: %s", e.Category, e.Message)
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}
