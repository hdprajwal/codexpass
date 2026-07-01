package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hdprajwal/codexpass/internal/codex"
)

// fakeUpstream is a test double for Upstream.
type fakeUpstream struct {
	models []ModelInfo
	result UpstreamResult
	events []StreamEvent
	gotReq UpstreamRequest
	err    error
}

func (f *fakeUpstream) Complete(_ context.Context, req UpstreamRequest) (UpstreamResult, error) {
	f.gotReq = req
	return f.result, f.err
}
func (f *fakeUpstream) Stream(_ context.Context, req UpstreamRequest, onEvent func(StreamEvent) error) error {
	f.gotReq = req
	for _, e := range f.events {
		if err := onEvent(e); err != nil {
			return err
		}
	}
	return f.err
}
func (f *fakeUpstream) Models(context.Context) ([]ModelInfo, error) { return f.models, f.err }

// testServer builds a Server wired to a fake upstream and a stub credential.
func testServer(u Upstream) *Server {
	s := New(Config{})
	s.SetUpstream(u)
	s.SetBorrow(func() (codex.Credential, error) {
		return codex.Credential{APIKey: "t", AccountID: "a", Mode: "chatgpt"}, nil
	})
	return s
}

func TestModelsList(t *testing.T) {
	s := testServer(&fakeUpstream{models: []ModelInfo{{ID: "gpt-5.4"}, {ID: "gpt-5.4-mini"}}})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Object != "list" || len(got.Data) != 2 || got.Data[0].ID != "gpt-5.4" || got.Data[0].Object != "model" {
		t.Fatalf("bad body: %s", rec.Body.String())
	}
}
