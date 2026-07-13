package proxy

import (
	"context"
	"sort"
	"sync"
	"time"
)

// ModelRegistry resolves local aliases and exposes aliases in /v1/models.
type ModelRegistry struct {
	aliases map[string]string
	state   *modelRegistryState
}

type modelRegistryState struct {
	mu         sync.RWMutex
	observed   bool
	discovered map[string]string
}

var builtInModelAliases = map[string]string{
	"gpt-5.6": "gpt-5.6-sol",
}

func NewModelRegistry(aliases map[string]string) ModelRegistry {
	cp := map[string]string{}
	for k, v := range aliases {
		if k != "" && v != "" {
			cp[k] = v
		}
	}
	return ModelRegistry{
		aliases: cp,
		state:   &modelRegistryState{discovered: map[string]string{}},
	}
}

func (r ModelRegistry) Resolve(model string) string {
	// Explicit configuration always takes precedence over aliases inferred from
	// a discovered upstream catalog.
	if v, ok := r.aliases[model]; ok {
		return v
	}
	if r.state != nil {
		r.state.mu.RLock()
		resolved, ok := r.state.discovered[model]
		observed := r.state.observed
		r.state.mu.RUnlock()
		if ok {
			return resolved
		}
		// Before the upstream catalog has been observed, provisionally resolve
		// official aliases so direct requests work without a preceding
		// /v1/models call. Once observed, the catalog is authoritative.
		if !observed {
			if resolved, ok := builtInModelAliases[model]; ok {
				return resolved
			}
		}
	}
	return model
}

func (r ModelRegistry) Expose(models []ModelInfo) []ModelInfo {
	seen := map[string]bool{}
	targets := map[string]bool{}
	out := make([]ModelInfo, 0, len(models)+len(r.aliases)+len(builtInModelAliases))
	for _, m := range models {
		if seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		targets[m.ID] = true
		out = append(out, m)
	}

	// Built-in aliases are catalog-dependent: synthesize one only when its
	// target exists and the upstream does not already provide the alias as a
	// real model. Remember the result so subsequent requests resolve exactly
	// the aliases advertised by /v1/models.
	discovered := make(map[string]string)
	for alias, target := range builtInModelAliases {
		if _, configured := r.aliases[alias]; configured {
			continue
		}
		if targets[target] && !targets[alias] {
			discovered[alias] = target
		}
	}
	if r.state != nil {
		r.state.mu.Lock()
		r.state.observed = true
		r.state.discovered = discovered
		r.state.mu.Unlock()
	}

	effective := make(map[string]string, len(discovered)+len(r.aliases))
	for alias, target := range discovered {
		effective[alias] = target
	}
	for alias, target := range r.aliases {
		effective[alias] = target
	}
	var aliases []string
	for alias, target := range effective {
		if targets[target] && !seen[alias] {
			aliases = append(aliases, alias)
		}
	}
	sort.Strings(aliases)
	for _, alias := range aliases {
		out = append(out, ModelInfo{ID: alias})
	}
	return out
}

// ValidateAliases reports aliases that collide with real models or point to
// unknown targets.
func (r ModelRegistry) ValidateAliases(models []ModelInfo) []string {
	real := map[string]bool{}
	for _, m := range models {
		real[m.ID] = true
	}
	var problems []string
	for alias, target := range r.aliases {
		if real[alias] {
			problems = append(problems, "model alias "+alias+" collides with a real model")
		}
		if !real[target] {
			problems = append(problems, "model alias "+alias+" points to unknown model "+target)
		}
	}
	sort.Strings(problems)
	return problems
}

type cachedUpstream struct {
	next Upstream
	ttl  time.Duration
	now  func() time.Time

	mu      sync.Mutex
	models  []ModelInfo
	expires time.Time
}

func newCachedUpstream(next Upstream, ttl time.Duration) Upstream {
	if ttl <= 0 {
		return next
	}
	return &cachedUpstream{next: next, ttl: ttl, now: time.Now}
}

func (c *cachedUpstream) Complete(ctx context.Context, req UpstreamRequest) (UpstreamResult, error) {
	return c.next.Complete(ctx, req)
}

func (c *cachedUpstream) Stream(ctx context.Context, req UpstreamRequest, onEvent func(StreamEvent) error) error {
	return c.next.Stream(ctx, req, onEvent)
}

func (c *cachedUpstream) Models(ctx context.Context) ([]ModelInfo, error) {
	now := c.now()
	c.mu.Lock()
	if len(c.models) > 0 && now.Before(c.expires) {
		out := append([]ModelInfo(nil), c.models...)
		c.mu.Unlock()
		return out, nil
	}
	c.mu.Unlock()

	models, err := c.next.Models(ctx)
	if err != nil {
		c.mu.Lock()
		if len(c.models) > 0 {
			out := append([]ModelInfo(nil), c.models...)
			c.mu.Unlock()
			return out, nil
		}
		c.mu.Unlock()
		return nil, err
	}

	c.mu.Lock()
	c.models = append([]ModelInfo(nil), models...)
	c.expires = now.Add(c.ttl)
	out := append([]ModelInfo(nil), c.models...)
	c.mu.Unlock()
	return out, nil
}
