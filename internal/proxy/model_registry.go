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
}

func NewModelRegistry(aliases map[string]string) ModelRegistry {
	cp := map[string]string{}
	for k, v := range aliases {
		if k != "" && v != "" {
			cp[k] = v
		}
	}
	return ModelRegistry{aliases: cp}
}

func (r ModelRegistry) Resolve(model string) string {
	if v, ok := r.aliases[model]; ok {
		return v
	}
	return model
}

func (r ModelRegistry) Expose(models []ModelInfo) []ModelInfo {
	if len(r.aliases) == 0 {
		return models
	}
	seen := map[string]bool{}
	targets := map[string]bool{}
	out := make([]ModelInfo, 0, len(models)+len(r.aliases))
	for _, m := range models {
		if seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		targets[m.ID] = true
		out = append(out, m)
	}
	var aliases []string
	for alias, target := range r.aliases {
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
