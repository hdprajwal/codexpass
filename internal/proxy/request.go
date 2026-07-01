package proxy

import (
	"encoding/json"
	"io"
)

// ChatRequest is the subset of the OpenAI chat/completions request we read.
type ChatRequest struct {
	Model               string          `json:"model"`
	Messages            []ChatMessage   `json:"messages"`
	Stream              bool            `json:"stream"`
	StreamOptions       *StreamOptions  `json:"stream_options"`
	Temperature         *float64        `json:"temperature"`
	TopP                *float64        `json:"top_p"`
	MaxTokens           *int64          `json:"max_tokens"`
	MaxCompletionTokens *int64          `json:"max_completion_tokens"`
	Tools               []ChatTool      `json:"tools"`
	ToolChoice          json.RawMessage `json:"tool_choice"`
	ResponseFormat      *ResponseFormat `json:"response_format"`
	ReasoningEffort     string          `json:"reasoning_effort"`
}

// ChatMessage is one chat message. Content is a string or an array of parts.
type ChatMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	Name       string          `json:"name"`
	ToolCalls  []ChatToolCall  `json:"tool_calls"`
	ToolCallID string          `json:"tool_call_id"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type ChatTool struct {
	Type     string       `json:"type"`
	Function ChatFunction `json:"function"`
}

type ChatFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ChatToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type ResponseFormat struct {
	Type       string          `json:"type"` // text | json_object | json_schema
	JSONSchema *JSONSchemaSpec `json:"json_schema"`
}

type JSONSchemaSpec struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
}

// ContentPart is one element of an array-form message content.
type ContentPart struct {
	Type     string `json:"type"` // text | image_url
	Text     string `json:"text"`
	ImageURL *struct {
		URL    string `json:"url"`
		Detail string `json:"detail"`
	} `json:"image_url"`
}

// DecodeChatRequest reads and validates a chat/completions request body.
func DecodeChatRequest(r io.Reader) (ChatRequest, error) {
	var req ChatRequest
	dec := json.NewDecoder(r)
	if err := dec.Decode(&req); err != nil {
		return ChatRequest{}, err
	}
	return req, nil
}

// textContent returns the plain-text content of a message, treating a JSON
// string as-is and concatenating the text parts of an array. ok is false when
// content is absent.
func (m ChatMessage) textContent() (string, bool) {
	if len(m.Content) == 0 {
		return "", false
	}
	var s string
	if err := json.Unmarshal(m.Content, &s); err == nil {
		return s, true
	}
	var parts []ContentPart
	if err := json.Unmarshal(m.Content, &parts); err == nil {
		var b string
		for _, p := range parts {
			if p.Type == "text" {
				b += p.Text
			}
		}
		return b, true
	}
	return "", false
}

// contentParts returns the array-form parts of a message, or nil if the content
// is a plain string / absent.
func (m ChatMessage) contentParts() []ContentPart {
	if len(m.Content) == 0 {
		return nil
	}
	var parts []ContentPart
	if err := json.Unmarshal(m.Content, &parts); err == nil {
		return parts
	}
	return nil
}
