package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func postChat(s *Server, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestChatNonStreamingText(t *testing.T) {
	s := testServer(&fakeUpstream{result: UpstreamResult{
		OutputText: "Hello there",
		Usage:      Usage{InputTokens: 3, OutputTokens: 2},
	}})
	rec := postChat(s, `{"model":"gpt-5.4","messages":[{"role":"user","content":"hi"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var got ChatCompletion
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Object != "chat.completion" || got.Model != "gpt-5.4" {
		t.Errorf("envelope wrong: %+v", got)
	}
	if len(got.Choices) != 1 || got.Choices[0].Message.Content == nil || *got.Choices[0].Message.Content != "Hello there" {
		t.Errorf("content wrong: %s", rec.Body.String())
	}
	if got.Choices[0].FinishReason != "stop" {
		t.Errorf("finish_reason = %q", got.Choices[0].FinishReason)
	}
	if got.Usage == nil || got.Usage.TotalTokens != 5 {
		t.Errorf("usage wrong: %+v", got.Usage)
	}
}

func TestChatStreamingText(t *testing.T) {
	s := testServer(&fakeUpstream{events: []StreamEvent{
		{Kind: "text.delta", TextDelta: "Hi"},
		{Kind: "completed", Usage: &Usage{InputTokens: 1, OutputTokens: 1}},
	}})
	rec := postChat(s, `{"model":"gpt-5.4","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	body := rec.Body.String()
	if !strings.Contains(body, `"chat.completion.chunk"`) {
		t.Errorf("no chunk object: %s", body)
	}
	if !strings.Contains(body, `"content":"Hi"`) {
		t.Errorf("no content delta: %s", body)
	}
	if !strings.Contains(body, "data: [DONE]") {
		t.Errorf("no DONE sentinel: %s", body)
	}
}

func TestChatNonStreamToolCalls(t *testing.T) {
	s := testServer(&fakeUpstream{result: UpstreamResult{
		ToolCalls: []ToolCall{{ID: "call_1", Name: "get_weather", Arguments: `{"city":"NYC"}`}},
	}})
	rec := postChat(s, `{"model":"gpt-5.4","messages":[{"role":"user","content":"weather?"}]}`)
	var got ChatCompletion
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("finish = %q", got.Choices[0].FinishReason)
	}
	if len(got.Choices[0].Message.ToolCalls) != 1 || got.Choices[0].Message.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("tool_calls wrong: %s", rec.Body.String())
	}
}

func TestChatStreamToolCalls(t *testing.T) {
	s := testServer(&fakeUpstream{events: []StreamEvent{
		{Kind: "tool_call", ToolCall: &ToolCall{ID: "call_1", Name: "get_weather", Arguments: `{"city":"NYC"}`}},
		{Kind: "completed"},
	}})
	rec := postChat(s, `{"model":"gpt-5.4","stream":true,"messages":[{"role":"user","content":"weather?"}]}`)
	body := rec.Body.String()
	if !strings.Contains(body, `"get_weather"`) || !strings.Contains(body, `"tool_calls"`) {
		t.Errorf("no tool_call chunk / finish: %s", body)
	}
}
