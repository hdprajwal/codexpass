package proxy

import (
	"encoding/json"
	"net/http"
)

// writeJSON writes v as a JSON response with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes an OpenAI-shaped error body.
func writeError(w http.ResponseWriter, status int, typ, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{"message": msg, "type": typ},
	})
}

// ChatCompletion is a non-streaming chat/completions response.
type ChatCompletion struct {
	ID      string    `json:"id"`
	Object  string    `json:"object"`
	Created int64     `json:"created"`
	Model   string    `json:"model"`
	Choices []Choice  `json:"choices"`
	Usage   *OutUsage `json:"usage,omitempty"`
}

type Choice struct {
	Index        int        `json:"index"`
	Message      OutMessage `json:"message"`
	FinishReason string     `json:"finish_reason"`
}

type OutMessage struct {
	Role      string        `json:"role"`
	Content   *string       `json:"content"`
	ToolCalls []OutToolCall `json:"tool_calls,omitempty"`
}

type OutToolCall struct {
	Index    *int    `json:"index,omitempty"`
	ID       string  `json:"id,omitempty"`
	Type     string  `json:"type,omitempty"`
	Function OutFunc `json:"function"`
}

type OutFunc struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments"`
}

type OutUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

// ChatCompletionChunk is one streamed chat/completions chunk.
type ChatCompletionChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
	Usage   *OutUsage     `json:"usage,omitempty"`
}

type ChunkChoice struct {
	Index        int     `json:"index"`
	Delta        Delta   `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

type Delta struct {
	Role      string        `json:"role,omitempty"`
	Content   string        `json:"content,omitempty"`
	ToolCalls []OutToolCall `json:"tool_calls,omitempty"`
}
