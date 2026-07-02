package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFallbackDisabledReturnsNotImplemented(t *testing.T) {
	s := New(Config{})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(`{}`)))
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFallbackRoutesUnsupportedEndpoint(t *testing.T) {
	old := fallbackHTTPClient
	defer func() { fallbackHTTPClient = old }()
	var gotPath, gotAuth string
	fallbackHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
		}, nil
	})}

	s := New(Config{Fallback: FallbackConfig{Enabled: true, BaseURL: "https://fallback.example/v1", APIKey: "sk-fallback"}})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(`{}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if gotPath != "/v1/embeddings" || gotAuth != "Bearer sk-fallback" {
		t.Fatalf("path=%q auth=%q", gotPath, gotAuth)
	}
}

func TestFallbackDeniedByClientPolicy(t *testing.T) {
	s := New(Config{
		Clients:  []ClientPolicy{{Name: "no-fallback", Token: "secret", AllowFallback: false}},
		Fallback: FallbackConfig{Enabled: true, BaseURL: "https://fallback.example/v1", APIKey: "sk-fallback"},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer secret")
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}
