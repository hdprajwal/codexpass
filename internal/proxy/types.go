package proxy

import (
	"context"
	"encoding/json"
)

// UpstreamRequest is a normalized Responses request (subset we support).
type UpstreamRequest struct {
	Model           string
	Instructions    string
	Input           []InputItem
	Temperature     *float64
	TopP            *float64
	MaxOutputTokens *int64
	ReasoningEffort string // "", low, medium, high, xhigh
	Tools           []FunctionTool
	ToolChoice      string // "", "auto", "none", "required", or a function name
	TextFormat      *TextFormat
	Stream          bool
	IncludeUsage    bool
}

// InputItem is one Responses input entry: a message, a function call, or a
// function-call output. Kind selects which fields are meaningful.
type InputItem struct {
	Kind      string         // "message" | "function_call" | "function_call_output"
	Role      string         // message: user|assistant|system
	Content   []InputContent // message
	CallID    string         // function_call, function_call_output
	Name      string         // function_call
	Arguments string         // function_call (raw JSON string)
	Output    string         // function_call_output
}

// InputContent is one content part of a message.
type InputContent struct {
	Kind     string // "input_text" | "input_image"
	Text     string // input_text
	ImageURL string // input_image (may be a data: URL)
	Detail   string // input_image (low|high|auto)
}

// FunctionTool is a normalized function tool definition.
type FunctionTool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

// TextFormat is a normalized structured-output format.
type TextFormat struct {
	Kind   string // "json_object" | "json_schema"
	Name   string // json_schema
	Schema json.RawMessage
}

// UpstreamResult is a normalized non-streaming Responses result.
type UpstreamResult struct {
	OutputText string
	ToolCalls  []ToolCall
	Usage      Usage
}

// ToolCall is a normalized function call the model made.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// Usage is normalized token usage.
type Usage struct {
	InputTokens  int64
	OutputTokens int64
}

// StreamEvent is a normalized upstream streaming event.
type StreamEvent struct {
	Kind      string // "text.delta" | "tool_call" | "completed"
	TextDelta string // text.delta
	ToolCall  *ToolCall
	Usage     *Usage // completed
}

// ModelInfo is one available model.
type ModelInfo struct {
	ID string
}

// Upstream is the Codex Responses backend. The only real implementation wraps
// openai-go (added later); tests inject fakes.
type Upstream interface {
	Complete(ctx context.Context, req UpstreamRequest) (UpstreamResult, error)
	Stream(ctx context.Context, req UpstreamRequest, onEvent func(StreamEvent) error) error
	Models(ctx context.Context) ([]ModelInfo, error)
}
