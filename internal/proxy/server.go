package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/hdprajwal/codexpass/internal/codex"
)

// Config holds proxy server configuration.
type Config struct {
	Host          string // default 127.0.0.1
	Port          int    // default 8080
	Token         string // optional client bearer secret; empty = no client auth
	Verbose       bool
	LogFormat     string
	Metrics       bool
	StatsPath     string
	ModelAliases  map[string]string
	ModelCacheTTL time.Duration
	Clients       []ClientPolicy
}

// Server is the OpenAI-compatible proxy.
type Server struct {
	cfg      Config
	borrow   func() (codex.Credential, error)
	upstream Upstream
	models   ModelRegistry
	policy   *policyEngine
	stats    *StatsRecorder
}

// New builds a Server, applying defaults.
func New(cfg Config) *Server {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	return &Server{cfg: cfg, borrow: codex.Borrow, models: NewModelRegistry(cfg.ModelAliases), policy: newPolicyEngine(cfg), stats: NewStatsRecorder()}
}

// SetUpstream overrides the upstream backend (tests).
func (s *Server) SetUpstream(u Upstream) { s.upstream = newCachedUpstream(u, s.cfg.ModelCacheTTL) }

// SetBorrow overrides the credential source (tests).
func (s *Server) SetBorrow(fn func() (codex.Credential, error)) { s.borrow = fn }

// Handler returns the HTTP routes, wrapped with optional client auth.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	if s.cfg.Metrics {
		mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; version=0.0.4")
			fmt.Fprint(w, s.stats.Metrics())
		})
	}
	mux.HandleFunc("GET /v1/models", s.handleModels)
	mux.HandleFunc("POST /v1/chat/completions", s.handleChat)
	mux.HandleFunc("POST /v1/responses", s.handleResponsesPassthrough)
	for _, p := range []string{"POST /v1/embeddings", "POST /v1/images/generations", "POST /v1/audio/speech", "POST /v1/audio/transcriptions"} {
		mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusNotImplemented, "not_implemented", "the Codex backend does not serve this endpoint")
		})
	}
	return s.auth(s.observe(mux))
}

// auth enforces the optional client token (except /healthz) and does verbose
// metadata logging.
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Verbose {
			fmt.Fprintf(os.Stderr, "%s %s\n", r.Method, r.URL.Path)
		}
		if r.URL.Path != "/healthz" {
			client, status, msg := s.policy.authorize(r)
			if status != http.StatusOK {
				typ := "authentication_error"
				if status == http.StatusForbidden {
					typ = "permission_error"
				}
				if status == http.StatusTooManyRequests {
					typ = "rate_limit_error"
				}
				if status == http.StatusRequestEntityTooLarge {
					typ = "request_too_large"
				}
				writeError(w, status, typ, msg)
				return
			}
			r = r.WithContext(withClient(r.Context(), client))
		}
		next.ServeHTTP(w, r)
	})
}

// ListenAndServe runs until ctx is cancelled, then shuts down gracefully.
func (s *Server) ListenAndServe(ctx context.Context) error {
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	if s.cfg.Host != "127.0.0.1" && s.cfg.Host != "localhost" && s.cfg.Host != "::1" {
		fmt.Fprintf(os.Stderr, "warning: binding non-loopback host %q exposes your Codex credential\n", s.cfg.Host)
	}
	if s.upstream == nil {
		s.upstream = newCachedUpstream(newOpenAIUpstream(s.borrow), s.cfg.ModelCacheTTL)
	}
	srv := &http.Server{Addr: addr, Handler: s.Handler()}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()
	fmt.Fprintf(os.Stderr, "codexpass serve listening on http://%s\n", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
