package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hdprajwal/codex2key/internal/codex"
)

func TestTokenAuthRequired(t *testing.T) {
	s := New(Config{Token: "secret"})
	s.SetUpstream(&fakeUpstream{models: []ModelInfo{{ID: "m"}}})
	s.SetBorrow(func() (codex.Credential, error) { return codex.Credential{}, nil })

	// Missing token -> 401
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing token: status = %d, want 401", rec.Code)
	}

	// Correct token -> 200
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer secret")
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("good token: status = %d, want 200", rec.Code)
	}

	// /healthz is exempt.
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz should be exempt: status = %d", rec.Code)
	}
}

func TestUnsupportedEndpoint(t *testing.T) {
	s := testServer(&fakeUpstream{})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil))
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", rec.Code)
	}
}
