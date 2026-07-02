package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsRecordsChatUsage(t *testing.T) {
	s := New(Config{Metrics: true})
	s.SetUpstream(&fakeUpstream{result: UpstreamResult{OutputText: "ok", Usage: Usage{InputTokens: 3, OutputTokens: 4}}})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"secret prompt"}]}`))
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()
	if !strings.Contains(body, "codexpass_input_tokens_total 3") || !strings.Contains(body, "codexpass_output_tokens_total 4") {
		t.Fatalf("metrics = %s", body)
	}
	if strings.Contains(body, "secret prompt") {
		t.Fatalf("metrics leaked prompt: %s", body)
	}
}
