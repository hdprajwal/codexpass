package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientPolicyEndpointDenied(t *testing.T) {
	s := New(Config{Clients: []ClientPolicy{{
		Name:             "models-only",
		Token:            "secret",
		AllowedEndpoints: []string{"models"},
	}}})
	s.SetUpstream(&fakeUpstream{result: UpstreamResult{OutputText: "ok"}})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer secret")
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestClientPolicyModelDenied(t *testing.T) {
	s := New(Config{Clients: []ClientPolicy{{
		Name:          "limited",
		Token:         "secret",
		AllowedModels: []string{"allowed"},
	}}})
	s.SetUpstream(&fakeUpstream{result: UpstreamResult{OutputText: "ok"}})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"blocked","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer secret")
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestClientPolicyBodyLimit(t *testing.T) {
	s := New(Config{Clients: []ClientPolicy{{
		Name:         "tiny",
		Token:        "secret",
		MaxBodyBytes: 2,
	}}})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"m"}`))
	req.Header.Set("Authorization", "Bearer secret")
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestClientPolicyRateLimit(t *testing.T) {
	s := New(Config{Clients: []ClientPolicy{{
		Name:               "slow",
		Token:              "secret",
		AllowedEndpoints:   []string{"models"},
		RateLimitPerMinute: 1,
	}}})
	s.SetUpstream(&fakeUpstream{models: []ModelInfo{{ID: "m"}}})

	for i, want := range []int{http.StatusOK, http.StatusTooManyRequests} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		req.Header.Set("Authorization", "Bearer secret")
		s.Handler().ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("request %d status = %d, want %d body=%s", i, rec.Code, want, rec.Body.String())
		}
	}
}
