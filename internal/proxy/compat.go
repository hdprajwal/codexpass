package proxy

import (
	"encoding/json"
	"fmt"
)

// ValidateChatRequest checks the request shape we promise to support before it
// reaches the translator. Unknown JSON fields are accepted by DecodeChatRequest;
// this function only rejects fields that would produce ambiguous behavior.
func ValidateChatRequest(req ChatRequest) error {
	if req.Model == "" {
		return fmt.Errorf("model is required")
	}
	if len(req.Messages) == 0 {
		return fmt.Errorf("messages is required and must contain at least one message")
	}
	for i, m := range req.Messages {
		switch m.Role {
		case "system", "developer", "user", "assistant", "tool":
		default:
			return fmt.Errorf("messages[%d].role %q is not supported", i, m.Role)
		}
		if m.Role == "tool" && m.ToolCallID == "" {
			return fmt.Errorf("messages[%d].tool_call_id is required for tool messages", i)
		}
	}
	for i, t := range req.Tools {
		if t.Type != "function" {
			return fmt.Errorf("tools[%d].type %q is not supported; only function tools are supported", i, t.Type)
		}
		if t.Function.Name == "" {
			return fmt.Errorf("tools[%d].function.name is required", i)
		}
	}
	if err := validateToolChoice(req.ToolChoice); err != nil {
		return err
	}
	return nil
}

func validateToolChoice(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch s {
		case "auto", "none", "required":
			return nil
		default:
			return fmt.Errorf("tool_choice %q is not supported", s)
		}
	}
	var obj struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return fmt.Errorf("tool_choice must be a string or function choice object")
	}
	if obj.Type != "function" {
		return fmt.Errorf("tool_choice.type %q is not supported", obj.Type)
	}
	if obj.Function.Name == "" {
		return fmt.Errorf("tool_choice.function.name is required")
	}
	return nil
}
