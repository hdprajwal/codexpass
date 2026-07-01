package proxy

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// toChatCompletion builds a chat/completions response from a normalized result.
func toChatCompletion(model string, res UpstreamResult) ChatCompletion {
	msg := OutMessage{Role: "assistant"}
	content := res.OutputText
	msg.Content = &content
	finish := "stop"
	for _, tc := range res.ToolCalls {
		msg.ToolCalls = append(msg.ToolCalls, OutToolCall{
			ID:       tc.ID,
			Type:     "function",
			Function: OutFunc{Name: tc.Name, Arguments: tc.Arguments},
		})
	}
	if len(res.ToolCalls) > 0 {
		finish = "tool_calls"
	}
	return ChatCompletion{
		ID:      "chatcmpl-" + randID(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []Choice{{Index: 0, Message: msg, FinishReason: finish}},
		Usage: &OutUsage{
			PromptTokens:     res.Usage.InputTokens,
			CompletionTokens: res.Usage.OutputTokens,
			TotalTokens:      res.Usage.InputTokens + res.Usage.OutputTokens,
		},
	}
}

// randID returns a short random hex id for response ids.
func randID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
