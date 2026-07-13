package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hdprajwal/codexpass/internal/codex"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestResponsesProxyDefaultsStoreAndResolvesAlias(t *testing.T) {
	old := passthroughHTTPClient
	defer func() { passthroughHTTPClient = old }()
	var gotBody map[string]any
	var gotAuth, gotAccount string
	var gotBeta []string
	passthroughHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotAuth = r.Header.Get("Authorization")
		gotAccount = r.Header.Get("ChatGPT-Account-ID")
		gotBeta = r.Header.Values("OpenAI-Beta")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"resp_1"}`)),
		}, nil
	})}

	s := New(Config{ModelAliases: map[string]string{"gpt-local": "gpt-5.4"}})
	s.SetBorrow(func() (codex.Credential, error) {
		return codex.Credential{APIKey: "access-token", AccountID: "acct", Mode: "chatgpt"}, nil
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-local","input":"hi"}`))
	req.Header.Set("Authorization", "Bearer client-token-that-must-not-be-forwarded")
	req.Header.Set("ChatGPT-Account-ID", "client-account-that-must-not-be-forwarded")
	req.Header.Add("OpenAI-Beta", "responses_multi_agent=v1")
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if gotAuth != "Bearer access-token" || gotAccount != "acct" {
		t.Fatalf("headers auth=%q account=%q", gotAuth, gotAccount)
	}
	if len(gotBeta) != 1 || gotBeta[0] != "responses_multi_agent=v1" {
		t.Fatalf("OpenAI-Beta headers = %q", gotBeta)
	}
	if gotBody["model"] != "gpt-5.4" || gotBody["store"] != false {
		t.Fatalf("body = %+v", gotBody)
	}
}

func TestNormalizeResponsesBodyPreservesUnknownFieldsLosslessly(t *testing.T) {
	const largeInteger = "1234567890123456789012345678901234567890"
	input := `{"model":"gpt-local","store":true,"future":{"large":` + largeInteger + `,"nested":[true,null,{"mode":"new"}]}}`

	body, requested, resolved, err := normalizeResponsesBody(
		strings.NewReader(input),
		NewModelRegistry(map[string]string{"gpt-local": "gpt-5.6-sol"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if requested != "gpt-local" || resolved != "gpt-5.6-sol" {
		t.Fatalf("models requested=%q resolved=%q", requested, resolved)
	}
	if !strings.Contains(string(body), `"large":`+largeInteger) {
		t.Fatalf("large integer changed: %s", body)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if string(payload["model"]) != `"gpt-5.6-sol"` {
		t.Fatalf("model = %s", payload["model"])
	}
	if string(payload["store"]) != "true" {
		t.Fatalf("explicit store changed: %s", payload["store"])
	}
	wantFuture := `{"large":` + largeInteger + `,"nested":[true,null,{"mode":"new"}]}`
	if string(payload["future"]) != wantFuture {
		t.Fatalf("unknown field changed:\n got: %s\nwant: %s", payload["future"], wantFuture)
	}
}

func TestResponsesProxyRejectsMalformedJSON(t *testing.T) {
	s := New(Config{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{`))
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}
