package provider

import (
	"github.com/jrswab/axe/pkg/protocol"
)

// Re-export types from pkg/protocol to maintain compatibility during migration.
type (
	ErrorCategory    = protocol.ErrorCategory
	ToolParameter    = protocol.ToolParameter
	Tool             = protocol.ToolDefinition
	ToolCall         = protocol.ToolCall
	ToolResult       = protocol.ToolResult
	Message          = protocol.Message
	FormatType       = protocol.FormatType
	ResponseFormat   = protocol.ResponseFormat
	Request          = protocol.Request
	Response         = protocol.Response
	Provider         = protocol.Provider
	ProviderError    = protocol.ProviderError
	ToolCallResult   = protocol.ToolResult // Alias for migration if needed
	StreamEvent      = protocol.StreamEvent
	EventStream      = protocol.EventStream
	StreamProvider   = protocol.StreamProvider
)

const (
	ErrCategoryAuth       = protocol.ErrCategoryAuth
	ErrCategoryRateLimit  = protocol.ErrCategoryRateLimit
	ErrCategoryTimeout    = protocol.ErrCategoryTimeout
	ErrCategoryOverloaded = protocol.ErrCategoryOverloaded
	ErrCategoryBadRequest = protocol.ErrCategoryBadRequest
	ErrCategoryServer     = protocol.ErrCategoryServer

	FormatNone   = protocol.FormatNone
	FormatJSON   = protocol.FormatJSON
	FormatSchema = protocol.FormatSchema

	StreamEventText      = protocol.StreamEventText
	StreamEventToolStart = protocol.StreamEventToolStart
	StreamEventToolDelta = protocol.StreamEventToolDelta
	StreamEventToolEnd   = protocol.StreamEventToolEnd
	StreamEventDone      = protocol.StreamEventDone
)
