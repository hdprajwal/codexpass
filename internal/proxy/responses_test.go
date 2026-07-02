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
	passthroughHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotAuth = r.Header.Get("Authorization")
		gotAccount = r.Header.Get("ChatGPT-Account-ID")
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
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if gotAuth != "Bearer access-token" || gotAccount != "acct" {
		t.Fatalf("headers auth=%q account=%q", gotAuth, gotAccount)
	}
	if gotBody["model"] != "gpt-5.4" || gotBody["store"] != false {
		t.Fatalf("body = %+v", gotBody)
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
