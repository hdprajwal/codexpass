package proxy

import (
	"strings"
	"testing"
)

func TestValidateChatRequestRequiresModelAndMessages(t *testing.T) {
	if err := ValidateChatRequest(ChatRequest{Messages: []ChatMessage{{Role: "user", Content: []byte(`"hi"`)}}}); err == nil || !strings.Contains(err.Error(), "model") {
		t.Fatalf("model error = %v", err)
	}
	if err := ValidateChatRequest(ChatRequest{Model: "gpt-5.4"}); err == nil || !strings.Contains(err.Error(), "messages") {
		t.Fatalf("messages error = %v", err)
	}
}

func TestValidateChatRequestRejectsUnsupportedRole(t *testing.T) {
	err := ValidateChatRequest(ChatRequest{Model: "gpt-5.4", Messages: []ChatMessage{{Role: "banana", Content: []byte(`"hi"`)}}})
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateChatRequestToolValidation(t *testing.T) {
	req := ChatRequest{
		Model:    "gpt-5.4",
		Messages: []ChatMessage{{Role: "user", Content: []byte(`"hi"`)}},
		Tools:    []ChatTool{{Type: "retrieval"}},
	}
	if err := ValidateChatRequest(req); err == nil || !strings.Contains(err.Error(), "only function tools") {
		t.Fatalf("err = %v", err)
	}

	req.Tools = []ChatTool{{Type: "function"}}
	if err := ValidateChatRequest(req); err == nil || !strings.Contains(err.Error(), "function.name") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateChatRequestToolChoice(t *testing.T) {
	req := ChatRequest{Model: "gpt-5.4", Messages: []ChatMessage{{Role: "user", Content: []byte(`"hi"`)}}}
	req.ToolChoice = []byte(`{"type":"function","function":{"name":"lookup"}}`)
	if err := ValidateChatRequest(req); err != nil {
		t.Fatalf("valid function choice: %v", err)
	}
	req.ToolChoice = []byte(`"sometimes"`)
	if err := ValidateChatRequest(req); err == nil || !strings.Contains(err.Error(), "tool_choice") {
		t.Fatalf("err = %v", err)
	}
}
