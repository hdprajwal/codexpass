package proxy

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/hdprajwal/codexpass/internal/codex"
)

type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
}

type retryUpstream struct {
	next Upstream
	cfg  RetryConfig
}

func newRetryUpstream(next Upstream, cfg RetryConfig) Upstream {
	if cfg.MaxAttempts <= 1 {
		return next
	}
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = 25 * time.Millisecond
	}
	return &retryUpstream{next: next, cfg: cfg}
}

func (r *retryUpstream) Complete(ctx context.Context, req UpstreamRequest) (UpstreamResult, error) {
	var last error
	for attempt := 1; attempt <= r.cfg.MaxAttempts; attempt++ {
		res, err := r.next.Complete(ctx, req)
		if err == nil || !retryable(err) || attempt == r.cfg.MaxAttempts {
			return res, err
		}
		last = err
		if err := sleepBackoff(ctx, r.cfg.BaseDelay, attempt); err != nil {
			return UpstreamResult{}, err
		}
	}
	return UpstreamResult{}, last
}

func (r *retryUpstream) Stream(ctx context.Context, req UpstreamRequest, onEvent func(StreamEvent) error) error {
	var last error
	for attempt := 1; attempt <= r.cfg.MaxAttempts; attempt++ {
		delivered := false
		err := r.next.Stream(ctx, req, func(e StreamEvent) error {
			delivered = true
			return onEvent(e)
		})
		if err == nil || delivered || !retryable(err) || attempt == r.cfg.MaxAttempts {
			return err
		}
		last = err
		if err := sleepBackoff(ctx, r.cfg.BaseDelay, attempt); err != nil {
			return err
		}
	}
	return last
}

func (r *retryUpstream) Models(ctx context.Context) ([]ModelInfo, error) {
	var last error
	for attempt := 1; attempt <= r.cfg.MaxAttempts; attempt++ {
		models, err := r.next.Models(ctx)
		if err == nil || !retryable(err) || attempt == r.cfg.MaxAttempts {
			return models, err
		}
		last = err
		if err := sleepBackoff(ctx, r.cfg.BaseDelay, attempt); err != nil {
			return nil, err
		}
	}
	return nil, last
}

func retryable(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "429") ||
		strings.Contains(msg, "500") ||
		strings.Contains(msg, "502") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "504")
}

func sleepBackoff(ctx context.Context, base time.Duration, attempt int) error {
	d := base * time.Duration(1<<(attempt-1))
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

type cachedBorrower struct {
	fn  func() (codex.Credential, error)
	ttl time.Duration
	now func() time.Time

	mu      sync.Mutex
	cred    codex.Credential
	expires time.Time
}

func newCachedBorrower(fn func() (codex.Credential, error), ttl time.Duration) *cachedBorrower {
	return &cachedBorrower{fn: fn, ttl: ttl, now: time.Now}
}

func (b *cachedBorrower) Borrow() (codex.Credential, error) {
	if b.ttl <= 0 {
		return b.fn()
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	if b.cred.APIKey != "" && now.Before(b.expires) {
		return b.cred, nil
	}
	cred, err := b.fn()
	if err != nil {
		return codex.Credential{}, err
	}
	b.cred = cred
	b.expires = now.Add(b.ttl)
	return cred, nil
}
