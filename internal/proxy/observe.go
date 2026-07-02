package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

type StatsRecorder struct {
	mu           sync.Mutex
	requests     map[string]int64
	statuses     map[int]int64
	inputTokens  int64
	outputTokens int64
}

func NewStatsRecorder() *StatsRecorder {
	return &StatsRecorder{requests: map[string]int64{}, statuses: map[int]int64{}}
}

func (s *StatsRecorder) ObserveRequest(endpoint string, status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests[endpoint]++
	s.statuses[status]++
}

func (s *StatsRecorder) ObserveUsage(u Usage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inputTokens += u.InputTokens
	s.outputTokens += u.OutputTokens
}

func (s *StatsRecorder) Metrics() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out string
	for endpoint, count := range s.requests {
		out += fmt.Sprintf("codexpass_requests_total{endpoint=%q} %d\n", endpoint, count)
	}
	for status, count := range s.statuses {
		out += fmt.Sprintf("codexpass_responses_total{status=%q} %d\n", fmt.Sprint(status), count)
	}
	out += fmt.Sprintf("codexpass_input_tokens_total %d\n", s.inputTokens)
	out += fmt.Sprintf("codexpass_output_tokens_total %d\n", s.outputTokens)
	return out
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

type requestLog struct {
	RequestID string `json:"request_id"`
	Client    string `json:"client,omitempty"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Endpoint  string `json:"endpoint"`
	Status    int    `json:"status"`
	LatencyMS int64  `json:"latency_ms"`
}

type usageLog struct {
	Time         time.Time `json:"time"`
	Client       string    `json:"client,omitempty"`
	Model        string    `json:"model"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
}

func (s *Server) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		if r.Header.Get("X-Request-ID") == "" {
			r.Header.Set("X-Request-ID", "req-"+randID())
		}
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		status := sw.status
		if status == 0 {
			status = http.StatusOK
		}
		endpoint := endpointName(r.URL.Path)
		if s.stats != nil {
			s.stats.ObserveRequest(endpoint, status)
		}
		if s.cfg.Verbose || s.cfg.LogFormat != "" {
			s.writeRequestLog(r, endpoint, status, time.Since(start))
		}
	})
}

func (s *Server) writeRequestLog(r *http.Request, endpoint string, status int, dur time.Duration) {
	client := ""
	if c, ok := clientFromContext(r.Context()); ok {
		client = c.Name
	}
	entry := requestLog{
		RequestID: r.Header.Get("X-Request-ID"),
		Client:    client,
		Method:    r.Method,
		Path:      r.URL.Path,
		Endpoint:  endpoint,
		Status:    status,
		LatencyMS: dur.Milliseconds(),
	}
	if s.cfg.LogFormat == "json" {
		_ = json.NewEncoder(os.Stderr).Encode(entry)
		return
	}
	fmt.Fprintf(os.Stderr, "%s %s status=%d endpoint=%s client=%s latency_ms=%d\n",
		entry.Method, entry.Path, entry.Status, entry.Endpoint, entry.Client, entry.LatencyMS)
}

func (s *Server) recordUsage(r *http.Request, model string, usage Usage) {
	if s.stats != nil {
		s.stats.ObserveUsage(usage)
	}
	if s.cfg.StatsPath == "" {
		return
	}
	client := ""
	if c, ok := clientFromContext(r.Context()); ok {
		client = c.Name
	}
	f, err := os.OpenFile(s.cfg.StatsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_ = json.NewEncoder(f).Encode(usageLog{
		Time:         time.Now().UTC(),
		Client:       client,
		Model:        model,
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
	})
}
