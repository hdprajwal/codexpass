package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestModelRegistryResolveAndExpose(t *testing.T) {
	reg := NewModelRegistry(map[string]string{"gpt-local": "gpt-5.4", "missing": "nope"})
	if got := reg.Resolve("gpt-local"); got != "gpt-5.4" {
		t.Fatalf("resolved = %q", got)
	}
	got := reg.Expose([]ModelInfo{{ID: "gpt-5.4"}})
	if len(got) != 2 || got[0].ID != "gpt-5.4" || got[1].ID != "gpt-local" {
		t.Fatalf("models = %+v", got)
	}
	problems := reg.ValidateAliases([]ModelInfo{{ID: "gpt-local"}})
	if len(problems) < 2 {
		t.Fatalf("problems = %+v", problems)
	}
}

func TestModelsListExposesAliases(t *testing.T) {
	s := New(Config{ModelAliases: map[string]string{"gpt-local": "gpt-5.4"}})
	s.SetUpstream(&fakeUpstream{models: []ModelInfo{{ID: "gpt-5.4"}}})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"id":"gpt-local"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestChatAliasResolvesUpstreamModel(t *testing.T) {
	u := &fakeUpstream{result: UpstreamResult{OutputText: "ok"}}
	s := New(Config{ModelAliases: map[string]string{"gpt-local": "gpt-5.4"}})
	s.SetUpstream(u)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
	  "model":"gpt-local",
	  "messages":[{"role":"user","content":"hi"}]
	}`))
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if u.gotReq.Model != "gpt-5.4" {
		t.Fatalf("upstream model = %q", u.gotReq.Model)
	}
	if !strings.Contains(rec.Body.String(), `"model":"gpt-local"`) {
		t.Fatalf("response should echo requested model: %s", rec.Body.String())
	}
}

func TestModelCacheUsesCachedModelsAndStaleOnError(t *testing.T) {
	u := &fakeUpstream{models: []ModelInfo{{ID: "gpt-5.4"}}}
	c := newCachedUpstream(u, time.Minute).(*cachedUpstream)
	now := time.Unix(100, 0)
	c.now = func() time.Time { return now }

	if _, err := c.Models(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Models(context.Background()); err != nil {
		t.Fatal(err)
	}
	if u.modelCalls != 1 {
		t.Fatalf("model calls = %d, want 1", u.modelCalls)
	}
	now = now.Add(2 * time.Minute)
	u.err = errors.New("network down")
	models, err := c.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "gpt-5.4" {
		t.Fatalf("models = %+v", models)
	}
}
