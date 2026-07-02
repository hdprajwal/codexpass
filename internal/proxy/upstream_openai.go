package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hdprajwal/codexpass/internal/codex"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/responses"
	"github.com/openai/openai-go/shared"
)

// openAIUpstream is the real Upstream: it calls the Codex Responses backend
// using a borrowed credential via the openai-go SDK.
type openAIUpstream struct {
	borrow func() (codex.Credential, error)
}

// newOpenAIUpstream builds the production Upstream. borrow supplies a fresh
// (refreshed as needed) credential on every call.
func newOpenAIUpstream(borrow func() (codex.Credential, error)) Upstream {
	return &openAIUpstream{borrow: borrow}
}

// client builds an SDK client from a fresh credential. In chatgpt mode the base
// URL is pointed at the Codex backend and the account id is sent as a header; in
// apikey mode the SDK defaults (api.openai.com) apply.
func (u *openAIUpstream) client() (openai.Client, codex.Credential, error) {
	cred, err := u.borrow()
	if err != nil {
		return openai.Client{}, cred, err
	}
	opts := []option.RequestOption{option.WithAPIKey(cred.APIKey)}
	if base := cred.BaseURL(); base != "" {
		opts = append(opts, option.WithBaseURL(base))
	}
	if cred.AccountID != "" {
		opts = append(opts, option.WithHeader("ChatGPT-Account-ID", cred.AccountID))
	}
	return openai.NewClient(opts...), cred, nil
}

// buildParams maps a normalized request onto SDK Responses params. store is
// always false (the borrowed credential must not persist responses upstream).
// When codexMode is true the sampling/length params (temperature, top_p,
// max_output_tokens) are omitted: the Codex backend rejects them with
// "Unsupported parameter: ...". They are kept for the standard OpenAI API.
func buildParams(req UpstreamRequest, codexMode bool) responses.ResponseNewParams {
	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(req.Model),
		Store: openai.Bool(false),
	}

	if req.Instructions != "" {
		params.Instructions = openai.String(req.Instructions)
	}
	if !codexMode {
		if req.MaxOutputTokens != nil {
			params.MaxOutputTokens = openai.Int(*req.MaxOutputTokens)
		}
		if req.Temperature != nil {
			params.Temperature = openai.Float(*req.Temperature)
		}
		if req.TopP != nil {
			params.TopP = openai.Float(*req.TopP)
		}
	}
	if req.ReasoningEffort != "" {
		params.Reasoning = shared.ReasoningParam{Effort: shared.ReasoningEffort(req.ReasoningEffort)}
	}

	params.Input = responses.ResponseNewParamsInputUnion{OfInputItemList: buildInput(req.Input)}

	if tools := buildTools(req.Tools); len(tools) > 0 {
		params.Tools = tools
	}
	if tc, ok := buildToolChoice(req.ToolChoice); ok {
		params.ToolChoice = tc
	}
	if tf := buildTextFormat(req.TextFormat); tf != nil {
		params.Text = *tf
	}

	return params
}

// buildInput converts normalized input items into SDK input items.
func buildInput(items []InputItem) responses.ResponseInputParam {
	out := make(responses.ResponseInputParam, 0, len(items))
	for _, it := range items {
		switch it.Kind {
		case "function_call":
			out = append(out, responses.ResponseInputItemParamOfFunctionCall(it.Arguments, it.CallID, it.Name))
		case "function_call_output":
			out = append(out, responses.ResponseInputItemParamOfFunctionCallOutput(it.CallID, it.Output))
		default: // "message"
			if it.Role == "assistant" {
				// A replayed assistant turn must not use input_text content parts:
				// the Responses API only allows output_text/refusal there and 400s
				// otherwise. Send it as an easy message with string content, which
				// carries no part type and needs no fabricated item id/status.
				out = append(out, responses.ResponseInputItemParamOfMessage(contentText(it.Content), responses.EasyInputMessageRoleAssistant))
				continue
			}
			out = append(out, responses.ResponseInputItemParamOfInputMessage(buildContent(it.Content), it.Role))
		}
	}
	return out
}

// contentText concatenates the text of a message's text parts, used for
// assistant turns that are replayed as plain-string easy messages.
func contentText(parts []InputContent) string {
	var b strings.Builder
	for _, p := range parts {
		if p.Kind == "input_text" {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

// buildContent converts normalized message content parts into SDK content parts.
func buildContent(parts []InputContent) responses.ResponseInputMessageContentListParam {
	out := make(responses.ResponseInputMessageContentListParam, 0, len(parts))
	for _, p := range parts {
		if p.Kind == "input_image" {
			detail := p.Detail
			if detail == "" {
				detail = "auto"
			}
			img := responses.ResponseInputImageParam{Detail: responses.ResponseInputImageDetail(detail)}
			if p.ImageURL != "" {
				img.ImageURL = openai.String(p.ImageURL)
			}
			out = append(out, responses.ResponseInputContentUnionParam{OfInputImage: &img})
			continue
		}
		out = append(out, responses.ResponseInputContentParamOfInputText(p.Text))
	}
	return out
}

// buildTools converts normalized function tools into SDK tools.
func buildTools(tools []FunctionTool) []responses.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}
	out := make([]responses.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		ft := responses.FunctionToolParam{
			Name:       t.Name,
			Parameters: schemaMap(t.Parameters),
			Strict:     openai.Bool(false),
		}
		if t.Description != "" {
			ft.Description = openai.String(t.Description)
		}
		out = append(out, responses.ToolUnionParam{OfFunction: &ft})
	}
	return out
}

// buildToolChoice maps the normalized tool-choice string to the SDK union. The
// second return is false when no choice was specified (leave the field unset).
func buildToolChoice(choice string) (responses.ResponseNewParamsToolChoiceUnion, bool) {
	switch choice {
	case "":
		return responses.ResponseNewParamsToolChoiceUnion{}, false
	case "auto", "none", "required":
		return responses.ResponseNewParamsToolChoiceUnion{
			OfToolChoiceMode: param.NewOpt(responses.ToolChoiceOptions(choice)),
		}, true
	default: // a specific function name
		return responses.ResponseNewParamsToolChoiceUnion{
			OfFunctionTool: &responses.ToolChoiceFunctionParam{Name: choice},
		}, true
	}
}

// buildTextFormat maps a normalized structured-output format to the SDK text
// config. Returns nil when no format was requested.
func buildTextFormat(tf *TextFormat) *responses.ResponseTextConfigParam {
	if tf == nil {
		return nil
	}
	switch tf.Kind {
	case "json_object":
		return &responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigUnionParam{
				OfJSONObject: &shared.ResponseFormatJSONObjectParam{},
			},
		}
	case "json_schema":
		cfg := responses.ResponseFormatTextConfigParamOfJSONSchema(tf.Name, schemaMap(tf.Schema))
		return &responses.ResponseTextConfigParam{Format: cfg}
	default:
		return nil
	}
}

// schemaMap decodes a raw JSON schema into the map form the SDK expects. An
// empty or invalid schema falls back to a permissive empty object.
func schemaMap(raw json.RawMessage) map[string]any {
	if len(raw) > 0 {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err == nil && m != nil {
			return m
		}
	}
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func (u *openAIUpstream) Complete(ctx context.Context, req UpstreamRequest) (UpstreamResult, error) {
	// The Codex backend rejects non-streaming requests ("Stream must be set to
	// true") and delivers output only through streaming events (its completed
	// event carries an empty output array), so a non-streaming Complete drives the
	// streaming API and accumulates the events into a single result.
	var res UpstreamResult
	var text strings.Builder
	err := u.Stream(ctx, req, func(ev StreamEvent) error {
		switch ev.Kind {
		case "text.delta":
			text.WriteString(ev.TextDelta)
		case "tool_call":
			if ev.ToolCall != nil {
				res.ToolCalls = append(res.ToolCalls, *ev.ToolCall)
			}
		case "completed":
			if ev.Usage != nil {
				res.Usage = *ev.Usage
			}
		}
		return nil
	})
	if err != nil {
		return UpstreamResult{}, err
	}
	res.OutputText = text.String()
	return res, nil
}

func (u *openAIUpstream) Stream(ctx context.Context, req UpstreamRequest, onEvent func(StreamEvent) error) error {
	client, cred, err := u.client()
	if err != nil {
		return err
	}
	// chatgpt mode targets the Codex backend (BaseURL set); apikey mode uses the
	// standard OpenAI API, which accepts the sampling/length params.
	stream := client.Responses.NewStreaming(ctx, buildParams(req, cred.BaseURL() != ""))
	for stream.Next() {
		se, ok := normalizeEvent(stream.Current())
		if !ok {
			continue
		}
		if err := onEvent(se); err != nil {
			_ = stream.Close()
			return err
		}
	}
	return stream.Err()
}

func (u *openAIUpstream) Models(ctx context.Context) ([]ModelInfo, error) {
	cred, err := u.borrow()
	if err != nil {
		return nil, err
	}
	return FetchModels(ctx, cred)
}

// FetchModels returns the models visible to one borrowed credential.
func FetchModels(ctx context.Context, cred codex.Credential) ([]ModelInfo, error) {
	if base := cred.BaseURL(); base != "" {
		return codexModels(ctx, base, cred)
	}
	return openAIModels(ctx, cred)
}

// codexModels queries the Codex-specific /models endpoint (not exposed by the
// SDK) and keeps only the models usable through the API.
func codexModels(ctx context.Context, base string, cred codex.Credential) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models?client_version=1.0.0", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cred.APIKey)
	if cred.AccountID != "" {
		req.Header.Set("ChatGPT-Account-ID", cred.AccountID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("codex models request failed: %s: %s", resp.Status, body)
	}
	var payload struct {
		Models []struct {
			Slug           string `json:"slug"`
			SupportedInAPI bool   `json:"supported_in_api"`
			Visibility     string `json:"visibility"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	var out []ModelInfo
	for _, m := range payload.Models {
		if m.SupportedInAPI && m.Visibility == "list" {
			out = append(out, ModelInfo{ID: m.Slug})
		}
	}
	return out, nil
}

// openAIModels queries the standard api.openai.com /v1/models endpoint (apikey
// mode).
func openAIModels(ctx context.Context, cred codex.Credential) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.openai.com/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cred.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("openai models request failed: %s: %s", resp.Status, body)
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	var out []ModelInfo
	for _, m := range payload.Data {
		out = append(out, ModelInfo{ID: m.ID})
	}
	return out, nil
}

// normalizeEvent maps one SDK streaming event to a normalized StreamEvent. Only
// text deltas, completed function-call items, and the terminal completed event
// are surfaced; ok is false for every other event type (the caller skips it).
func normalizeEvent(ev responses.ResponseStreamEventUnion) (StreamEvent, bool) {
	switch ev.Type {
	case "response.output_text.delta":
		return StreamEvent{Kind: "text.delta", TextDelta: ev.Delta.OfString}, true
	case "response.output_item.done":
		if ev.Item.Type == "function_call" {
			return StreamEvent{
				Kind: "tool_call",
				ToolCall: &ToolCall{
					ID:        ev.Item.CallID,
					Name:      ev.Item.Name,
					Arguments: ev.Item.Arguments,
				},
			}, true
		}
		return StreamEvent{}, false
	case "response.completed":
		return StreamEvent{
			Kind: "completed",
			Usage: &Usage{
				InputTokens:  ev.Response.Usage.InputTokens,
				OutputTokens: ev.Response.Usage.OutputTokens,
			},
		}, true
	default:
		return StreamEvent{}, false
	}
}
