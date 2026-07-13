package proxy

import (
	"encoding/json"
	"strings"
	"testing"
)

// A replayed assistant turn must serialize as an easy message with plain-string
// content. The Codex Responses backend rejects input_text parts inside an
// assistant message ("Supported values are: 'output_text' and 'refusal'"), so
// the assistant turn must carry no content-part type at all. User turns keep
// their input_text parts.
func TestBuildInputAssistantUsesStringContent(t *testing.T) {
	req, err := toUpstream(ChatRequest{
		Model: "gpt-5.4",
		Messages: []ChatMessage{
			{Role: "user", Content: []byte(`"Hello"`)},
			{Role: "assistant", Content: []byte(`"Hi there"`)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	b, err := json.Marshal(buildParams(req, true).Input)
	if err != nil {
		t.Fatal(err)
	}

	var items []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(b, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("input items = %d, want 2\n%s", len(items), b)
	}

	// User turn keeps a structured content list with input_text parts.
	if items[0].Role != "user" {
		t.Fatalf("item[0] role = %q, want user", items[0].Role)
	}
	var userStr string
	if json.Unmarshal(items[0].Content, &userStr) == nil {
		t.Errorf("user content should be a parts list, got string %q", userStr)
	}

	// Assistant turn is an easy message: content is a plain JSON string, so it
	// carries no input_text part to be rejected.
	if items[1].Role != "assistant" {
		t.Fatalf("item[1] role = %q, want assistant", items[1].Role)
	}
	var asstStr string
	if err := json.Unmarshal(items[1].Content, &asstStr); err != nil {
		t.Fatalf("assistant content must be a plain string, got: %s", items[1].Content)
	}
	if asstStr != "Hi there" {
		t.Errorf("assistant content = %q, want %q", asstStr, "Hi there")
	}
}

// GPT-5.6 adds values that are newer than the constants shipped by the current
// SDK. The SDK fields are string-backed, so buildParams must preserve those
// values instead of validating or narrowing them locally.
func TestBuildParamsPreservesGPT56ChatFields(t *testing.T) {
	chatReq, err := DecodeChatRequest(strings.NewReader(`{
		"model":"gpt-5.6-sol",
		"reasoning_effort":"max",
		"safety_identifier":"sha256:user-123",
		"messages":[{"role":"user","content":[
			{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA","detail":"original"}}
		]}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	req, err := toUpstream(chatReq)
	if err != nil {
		t.Fatal(err)
	}

	b, err := json.Marshal(buildParams(req, true))
	if err != nil {
		t.Fatal(err)
	}

	var body struct {
		Reasoning struct {
			Effort string `json:"effort"`
		} `json:"reasoning"`
		SafetyIdentifier string `json:"safety_identifier"`
		Input            []struct {
			Content []struct {
				Type   string `json:"type"`
				Detail string `json:"detail"`
			} `json:"content"`
		} `json:"input"`
	}
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	if body.Reasoning.Effort != "max" {
		t.Errorf("reasoning effort = %q, want max\n%s", body.Reasoning.Effort, b)
	}
	if body.SafetyIdentifier != "sha256:user-123" {
		t.Errorf("safety identifier = %q\n%s", body.SafetyIdentifier, b)
	}
	if len(body.Input) != 1 || len(body.Input[0].Content) != 1 {
		t.Fatalf("unexpected input shape: %s", b)
	}
	image := body.Input[0].Content[0]
	if image.Type != "input_image" || image.Detail != "original" {
		t.Errorf("image content = %+v, want input_image with original detail\n%s", image, b)
	}
}
