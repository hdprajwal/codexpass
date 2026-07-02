package proxy

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ClientPolicy struct {
	Name               string
	Token              string
	AllowedEndpoints   []string
	AllowedModels      []string
	MaxBodyBytes       int64
	RateLimitPerMinute int
	AllowFallback      bool
	Disabled           bool
}

type clientContextKey struct{}

func clientFromContext(ctx context.Context) (ClientPolicy, bool) {
	c, ok := ctx.Value(clientContextKey{}).(ClientPolicy)
	return c, ok
}

func withClient(ctx context.Context, c ClientPolicy) context.Context {
	return context.WithValue(ctx, clientContextKey{}, c)
}

type policyEngine struct {
	clients []ClientPolicy
	limits  map[string]*rateWindow
	mu      sync.Mutex
	now     func() time.Time
}

type rateWindow struct {
	start time.Time
	count int
}

func newPolicyEngine(cfg Config) *policyEngine {
	var clients []ClientPolicy
	if cfg.Token != "" {
		clients = append(clients, ClientPolicy{Name: "default", Token: cfg.Token})
	}
	clients = append(clients, cfg.Clients...)
	if len(clients) == 0 {
		return nil
	}
	return &policyEngine{clients: clients, limits: map[string]*rateWindow{}, now: time.Now}
}

func (p *policyEngine) authorize(r *http.Request) (ClientPolicy, int, string) {
	if p == nil {
		return ClientPolicy{}, http.StatusOK, ""
	}
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ClientPolicy{}, http.StatusUnauthorized, "missing or invalid API key"
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	for _, c := range p.clients {
		if c.Disabled {
			continue
		}
		if c.Token == token {
			if !endpointAllowed(c, endpointName(r.URL.Path)) {
				return c, http.StatusForbidden, "client is not allowed to use this endpoint"
			}
			if c.MaxBodyBytes > 0 && r.ContentLength > c.MaxBodyBytes {
				return c, http.StatusRequestEntityTooLarge, "request body is too large"
			}
			if !p.take(c) {
				return c, http.StatusTooManyRequests, "rate limit exceeded"
			}
			return c, http.StatusOK, ""
		}
	}
	return ClientPolicy{}, http.StatusUnauthorized, "missing or invalid API key"
}

func (p *policyEngine) take(c ClientPolicy) bool {
	if c.RateLimitPerMinute <= 0 {
		return true
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	w := p.limits[c.Name]
	if w == nil || now.Sub(w.start) >= time.Minute {
		p.limits[c.Name] = &rateWindow{start: now, count: 1}
		return true
	}
	if w.count >= c.RateLimitPerMinute {
		return false
	}
	w.count++
	return true
}

func endpointName(path string) string {
	switch {
	case path == "/v1/models":
		return "models"
	case path == "/v1/chat/completions":
		return "chat.completions"
	case path == "/v1/responses":
		return "responses"
	case strings.HasPrefix(path, "/v1/embeddings"):
		return "embeddings"
	case strings.HasPrefix(path, "/v1/images"):
		return "images"
	case strings.HasPrefix(path, "/v1/audio"):
		return "audio"
	default:
		return path
	}
}

func endpointAllowed(c ClientPolicy, endpoint string) bool {
	if len(c.AllowedEndpoints) == 0 {
		return true
	}
	for _, allowed := range c.AllowedEndpoints {
		if allowed == endpoint {
			return true
		}
	}
	return false
}

func modelAllowed(c ClientPolicy, requested, resolved string) bool {
	if len(c.AllowedModels) == 0 {
		return true
	}
	for _, allowed := range c.AllowedModels {
		if allowed == requested || allowed == resolved {
			return true
		}
	}
	return false
}
