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

func TestModelRegistryExposesAndResolvesGPT56Alias(t *testing.T) {
	reg := NewModelRegistry(nil)
	if got := reg.Resolve("gpt-5.6"); got != "gpt-5.6-sol" {
		t.Fatalf("alias resolved before catalog discovery = %q", got)
	}

	got := reg.Expose([]ModelInfo{
		{ID: "gpt-5.6-sol"},
		{ID: "gpt-5.6-terra"},
		{ID: "gpt-5.6-luna"},
	})
	want := []string{"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.6"}
	if len(got) != len(want) {
		t.Fatalf("models = %+v", got)
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("models[%d].ID = %q, want %q", i, got[i].ID, id)
		}
	}
	if got := reg.Resolve("gpt-5.6"); got != "gpt-5.6-sol" {
		t.Fatalf("resolved alias = %q", got)
	}
}

func TestModelRegistryDoesNotSynthesizeGPT56WithoutTarget(t *testing.T) {
	reg := NewModelRegistry(nil)
	got := reg.Expose([]ModelInfo{{ID: "gpt-5.5"}})
	if len(got) != 1 || got[0].ID != "gpt-5.5" {
		t.Fatalf("models = %+v", got)
	}
	if resolved := reg.Resolve("gpt-5.6"); resolved != "gpt-5.6" {
		t.Fatalf("alias without catalog target resolved to %q", resolved)
	}
}

func TestModelRegistryPreservesRealGPT56Model(t *testing.T) {
	reg := NewModelRegistry(nil)
	got := reg.Expose([]ModelInfo{
		{ID: "gpt-5.6"},
		{ID: "gpt-5.6-sol"},
		{ID: "gpt-5.6-terra"},
		{ID: "gpt-5.6-luna"},
	})
	if len(got) != 4 {
		t.Fatalf("models = %+v", got)
	}
	if resolved := reg.Resolve("gpt-5.6"); resolved != "gpt-5.6" {
		t.Fatalf("real upstream model resolved to %q", resolved)
	}
}

func TestModelRegistryCatalogRefreshRemovesGPT56Alias(t *testing.T) {
	reg := NewModelRegistry(nil)
	reg.Expose([]ModelInfo{{ID: "gpt-5.6-sol"}})
	if got := reg.Resolve("gpt-5.6"); got != "gpt-5.6-sol" {
		t.Fatalf("resolved alias = %q", got)
	}

	reg.Expose([]ModelInfo{{ID: "gpt-5.6"}, {ID: "gpt-5.6-sol"}})
	if got := reg.Resolve("gpt-5.6"); got != "gpt-5.6" {
		t.Fatalf("resolved real model after refresh = %q", got)
	}

	reg.Expose([]ModelInfo{{ID: "gpt-5.6-sol"}})
	if got := reg.Resolve("gpt-5.6"); got != "gpt-5.6-sol" {
		t.Fatalf("resolved reintroduced alias = %q", got)
	}

	reg.Expose([]ModelInfo{{ID: "gpt-5.5"}})
	if got := reg.Resolve("gpt-5.6"); got != "gpt-5.6" {
		t.Fatalf("resolved alias after target removal = %q", got)
	}
}

func TestModelRegistryUserAliasOverridesBuiltInGPT56Alias(t *testing.T) {
	reg := NewModelRegistry(map[string]string{"gpt-5.6": "gpt-5.6-terra"})
	got := reg.Expose([]ModelInfo{
		{ID: "gpt-5.6-sol"},
		{ID: "gpt-5.6-terra"},
		{ID: "gpt-5.6-luna"},
	})
	if len(got) != 4 || got[3].ID != "gpt-5.6" {
		t.Fatalf("models = %+v", got)
	}
	if resolved := reg.Resolve("gpt-5.6"); resolved != "gpt-5.6-terra" {
		t.Fatalf("configured alias resolved to %q", resolved)
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

func TestModelsListExposesGPT56FamilyAndOfficialAlias(t *testing.T) {
	s := New(Config{})
	s.SetUpstream(&fakeUpstream{models: []ModelInfo{
		{ID: "gpt-5.6-sol"},
		{ID: "gpt-5.6-terra"},
		{ID: "gpt-5.6-luna"},
	}})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	for _, id := range []string{"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.6"} {
		if !strings.Contains(rec.Body.String(), `"id":"`+id+`"`) {
			t.Fatalf("body does not contain %q: %s", id, rec.Body.String())
		}
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

func TestChatGPT56AliasResolvesAfterModelDiscovery(t *testing.T) {
	u := &fakeUpstream{
		models: []ModelInfo{
			{ID: "gpt-5.6-sol"},
			{ID: "gpt-5.6-terra"},
			{ID: "gpt-5.6-luna"},
		},
		result: UpstreamResult{OutputText: "ok"},
	}
	s := New(Config{})
	s.SetUpstream(u)

	modelsRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(modelsRec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if modelsRec.Code != http.StatusOK {
		t.Fatalf("models status = %d body=%s", modelsRec.Code, modelsRec.Body.String())
	}

	chatRec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
	  "model":"gpt-5.6",
	  "messages":[{"role":"user","content":"hi"}]
	}`))
	s.Handler().ServeHTTP(chatRec, req)
	if chatRec.Code != http.StatusOK {
		t.Fatalf("chat status = %d body=%s", chatRec.Code, chatRec.Body.String())
	}
	if u.gotReq.Model != "gpt-5.6-sol" {
		t.Fatalf("upstream model = %q", u.gotReq.Model)
	}
	if !strings.Contains(chatRec.Body.String(), `"model":"gpt-5.6"`) {
		t.Fatalf("response should echo requested model: %s", chatRec.Body.String())
	}
}

func TestChatGPT56AliasResolvesBeforeModelDiscovery(t *testing.T) {
	u := &fakeUpstream{result: UpstreamResult{OutputText: "ok"}}
	s := New(Config{})
	s.SetUpstream(u)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
	  "model":"gpt-5.6",
	  "messages":[{"role":"user","content":"hi"}]
	}`))
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if u.gotReq.Model != "gpt-5.6-sol" {
		t.Fatalf("upstream model = %q", u.gotReq.Model)
	}
	if !strings.Contains(rec.Body.String(), `"model":"gpt-5.6"`) {
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
