package proxy

import (
	"encoding/json"
	"strings"
)

// toUpstream translates a chat/completions request into a normalized Responses
// request. System/developer messages become instructions; the rest become input.
func toUpstream(req ChatRequest) (UpstreamRequest, error) {
	up := UpstreamRequest{
		Model:           req.Model,
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		ReasoningEffort: req.ReasoningEffort,
		Stream:          req.Stream,
	}
	if req.MaxCompletionTokens != nil {
		up.MaxOutputTokens = req.MaxCompletionTokens
	} else if req.MaxTokens != nil {
		up.MaxOutputTokens = req.MaxTokens
	}
	if req.StreamOptions != nil {
		up.IncludeUsage = req.StreamOptions.IncludeUsage
	}

	var instructions []string
	for _, m := range req.Messages {
		switch m.Role {
		case "system", "developer":
			if t, ok := m.textContent(); ok {
				instructions = append(instructions, t)
			}
		case "user":
			up.Input = append(up.Input, userInputItem(m))
		case "assistant":
			up.Input = append(up.Input, assistantInputItems(m)...)
		case "tool":
			up.Input = append(up.Input, InputItem{
				Kind:   "function_call_output",
				CallID: m.ToolCallID,
				Output: toolOutputString(m),
			})
		}
	}
	up.Instructions = strings.Join(instructions, "\n\n")

	for _, t := range req.Tools {
		if t.Type != "function" {
			continue
		}
		up.Tools = append(up.Tools, FunctionTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
		})
	}
	up.ToolChoice = parseToolChoice(req.ToolChoice)

	return up, nil
}

// parseToolChoice normalizes tool_choice: a bare string ("auto"/"none"/
// "required") stays as-is; an object {"type":"function","function":{"name":..}}
// yields the function name.
func parseToolChoice(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj struct {
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj.Function.Name
	}
	return ""
}

// userInputItem builds a user message input item (text only for now; images
// added in the vision task).
func userInputItem(m ChatMessage) InputItem {
	text, _ := m.textContent()
	return InputItem{
		Kind:    "message",
		Role:    "user",
		Content: []InputContent{{Kind: "input_text", Text: text}},
	}
}

// assistantInputItems builds assistant text and any prior tool calls.
func assistantInputItems(m ChatMessage) []InputItem {
	var items []InputItem
	if t, ok := m.textContent(); ok && t != "" {
		items = append(items, InputItem{
			Kind:    "message",
			Role:    "assistant",
			Content: []InputContent{{Kind: "input_text", Text: t}},
		})
	}
	for _, tc := range m.ToolCalls {
		items = append(items, InputItem{
			Kind:      "function_call",
			CallID:    tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}
	return items
}

// toolOutputString returns a tool result's content as a plain string.
func toolOutputString(m ChatMessage) string {
	t, _ := m.textContent()
	return t
}
