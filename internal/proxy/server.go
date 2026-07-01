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

	"github.com/hdprajwal/codex2key/internal/codex"
)

// Config holds proxy server configuration.
type Config struct {
	Host    string // default 127.0.0.1
	Port    int    // default 8080
	Token   string // optional client bearer secret; empty = no client auth
	Verbose bool
}

// Server is the OpenAI-compatible proxy.
type Server struct {
	cfg      Config
	borrow   func() (codex.Credential, error)
	upstream Upstream
}

// New builds a Server, applying defaults.
func New(cfg Config) *Server {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	return &Server{cfg: cfg, borrow: codex.Borrow}
}

// SetUpstream overrides the upstream backend (tests).
func (s *Server) SetUpstream(u Upstream) { s.upstream = u }

// SetBorrow overrides the credential source (tests).
func (s *Server) SetBorrow(fn func() (codex.Credential, error)) { s.borrow = fn }

// Handler returns the HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("GET /v1/models", s.handleModels)
	mux.HandleFunc("POST /v1/chat/completions", s.handleChat)
	return mux
}

// ListenAndServe runs until ctx is cancelled, then shuts down gracefully.
func (s *Server) ListenAndServe(ctx context.Context) error {
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	if s.cfg.Host != "127.0.0.1" && s.cfg.Host != "localhost" && s.cfg.Host != "::1" {
		fmt.Fprintf(os.Stderr, "warning: binding non-loopback host %q exposes your Codex credential\n", s.cfg.Host)
	}
	srv := &http.Server{Addr: addr, Handler: s.Handler()}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()
	fmt.Fprintf(os.Stderr, "codex2key serve listening on http://%s\n", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
