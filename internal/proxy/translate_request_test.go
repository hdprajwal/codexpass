package proxy

import "testing"

func TestToUpstreamTextAndSystem(t *testing.T) {
	temp := 0.5
	req := ChatRequest{
		Model:       "gpt-5.4",
		Temperature: &temp,
		Messages: []ChatMessage{
			{Role: "system", Content: []byte(`"Be brief."`)},
			{Role: "user", Content: []byte(`"Hello"`)},
			{Role: "assistant", Content: []byte(`"Hi"`)},
		},
	}
	up, err := toUpstream(req)
	if err != nil {
		t.Fatal(err)
	}
	if up.Model != "gpt-5.4" {
		t.Errorf("model = %q", up.Model)
	}
	if up.Instructions != "Be brief." {
		t.Errorf("instructions = %q", up.Instructions)
	}
	if up.Temperature == nil || *up.Temperature != 0.5 {
		t.Errorf("temperature not carried")
	}
	if len(up.Input) != 2 {
		t.Fatalf("input len = %d, want 2 (user, assistant)", len(up.Input))
	}
	if up.Input[0].Kind != "message" || up.Input[0].Role != "user" {
		t.Errorf("input[0] = %+v", up.Input[0])
	}
	if len(up.Input[0].Content) != 1 || up.Input[0].Content[0].Text != "Hello" {
		t.Errorf("user content = %+v", up.Input[0].Content)
	}
}

func TestToUpstreamMaxTokens(t *testing.T) {
	mt := int64(128)
	up, _ := toUpstream(ChatRequest{Model: "m", MaxTokens: &mt})
	if up.MaxOutputTokens == nil || *up.MaxOutputTokens != 128 {
		t.Fatalf("max_output_tokens not mapped")
	}
}
