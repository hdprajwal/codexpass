package proxy

import "time"

// streamState converts normalized upstream events into chat.completion.chunk
// values. Function calls are buffered and emitted whole when complete.
type streamState struct {
	id           string
	model        string
	created      int64
	includeUsage bool
	roleSent     bool
	toolIndex    int
	usage        *Usage
}

func newStreamState(model string, includeUsage bool) *streamState {
	return &streamState{
		id:           "chatcmpl-" + randID(),
		model:        model,
		created:      time.Now().Unix(),
		includeUsage: includeUsage,
	}
}

// base returns a chunk envelope with one choice.
func (s *streamState) base(delta Delta, finish *string) ChatCompletionChunk {
	return ChatCompletionChunk{
		ID:      s.id,
		Object:  "chat.completion.chunk",
		Created: s.created,
		Model:   s.model,
		Choices: []ChunkChoice{{Index: 0, Delta: delta, FinishReason: finish}},
	}
}

// onEvent maps one upstream event to zero or more chunks.
func (s *streamState) onEvent(e StreamEvent) ([]ChatCompletionChunk, error) {
	var out []ChatCompletionChunk
	switch e.Kind {
	case "text.delta":
		if !s.roleSent {
			out = append(out, s.base(Delta{Role: "assistant"}, nil))
			s.roleSent = true
		}
		out = append(out, s.base(Delta{Content: e.TextDelta}, nil))
	case "tool_call":
		if !s.roleSent {
			out = append(out, s.base(Delta{Role: "assistant"}, nil))
			s.roleSent = true
		}
		if e.ToolCall != nil {
			idx := s.toolIndex
			s.toolIndex++
			out = append(out, s.base(Delta{ToolCalls: []OutToolCall{{
				Index:    &idx,
				ID:       e.ToolCall.ID,
				Type:     "function",
				Function: OutFunc{Name: e.ToolCall.Name, Arguments: e.ToolCall.Arguments},
			}}}, nil))
		}
	case "completed":
		s.usage = e.Usage
	}
	return out, nil
}

// finalChunks emits the terminating finish_reason chunk (and usage if asked).
func (s *streamState) finalChunks() []ChatCompletionChunk {
	finish := "stop"
	if s.toolIndex > 0 {
		finish = "tool_calls"
	}
	final := s.base(Delta{}, &finish)
	if s.includeUsage && s.usage != nil {
		final.Usage = &OutUsage{
			PromptTokens:     s.usage.InputTokens,
			CompletionTokens: s.usage.OutputTokens,
			TotalTokens:      s.usage.InputTokens + s.usage.OutputTokens,
		}
	}
	return []ChatCompletionChunk{final}
}
