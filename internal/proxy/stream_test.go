package proxy

import (
	"strings"
	"testing"
)

func TestStreamStateTextThenComplete(t *testing.T) {
	st := newStreamState("gpt-5.4", true)
	var chunks []ChatCompletionChunk

	c, _ := st.onEvent(StreamEvent{Kind: "text.delta", TextDelta: "Hel"})
	chunks = append(chunks, c...)
	c, _ = st.onEvent(StreamEvent{Kind: "text.delta", TextDelta: "lo"})
	chunks = append(chunks, c...)
	c, _ = st.onEvent(StreamEvent{Kind: "completed", Usage: &Usage{InputTokens: 1, OutputTokens: 2}})
	chunks = append(chunks, c...)
	chunks = append(chunks, st.finalChunks()...)

	if chunks[0].Choices[0].Delta.Role != "assistant" {
		t.Errorf("first chunk role = %q", chunks[0].Choices[0].Delta.Role)
	}
	var text strings.Builder
	var sawFinish bool
	var usage *OutUsage
	for _, ch := range chunks {
		text.WriteString(ch.Choices[0].Delta.Content)
		if ch.Choices[0].FinishReason != nil {
			sawFinish = true
			if *ch.Choices[0].FinishReason != "stop" {
				t.Errorf("finish = %q", *ch.Choices[0].FinishReason)
			}
		}
		if ch.Usage != nil {
			usage = ch.Usage
		}
	}
	if text.String() != "Hello" {
		t.Errorf("text = %q", text.String())
	}
	if !sawFinish {
		t.Error("no finish_reason chunk")
	}
	if usage == nil || usage.TotalTokens != 3 {
		t.Errorf("usage = %+v", usage)
	}
}
