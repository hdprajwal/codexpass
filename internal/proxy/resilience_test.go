package proxy

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hdprajwal/codexpass/internal/codex"
)

type flakyUpstream struct {
	mu       sync.Mutex
	failures int
	calls    int
}

func (f *flakyUpstream) Complete(context.Context, UpstreamRequest) (UpstreamResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.calls <= f.failures {
		return UpstreamResult{}, errors.New("upstream 503")
	}
	return UpstreamResult{OutputText: "ok"}, nil
}

func (f *flakyUpstream) Stream(_ context.Context, _ UpstreamRequest, onEvent func(StreamEvent) error) error {
	f.mu.Lock()
	f.calls++
	call := f.calls
	f.mu.Unlock()
	if call <= f.failures {
		return errors.New("upstream 503")
	}
	return onEvent(StreamEvent{Kind: "text.delta", TextDelta: "ok"})
}

func (f *flakyUpstream) Models(context.Context) ([]ModelInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.calls <= f.failures {
		return nil, errors.New("upstream 503")
	}
	return []ModelInfo{{ID: "m"}}, nil
}

func TestRetryCompleteSucceedsAfterTransientFailure(t *testing.T) {
	f := &flakyUpstream{failures: 1}
	u := newRetryUpstream(f, RetryConfig{MaxAttempts: 2, BaseDelay: time.Nanosecond})
	res, err := u.Complete(context.Background(), UpstreamRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if res.OutputText != "ok" || f.calls != 2 {
		t.Fatalf("res=%+v calls=%d", res, f.calls)
	}
}

func TestRetryStreamDoesNotRetryAfterEventDelivered(t *testing.T) {
	u := newRetryUpstream(streamAfterEventError{}, RetryConfig{MaxAttempts: 3, BaseDelay: time.Nanosecond})
	var events int
	err := u.Stream(context.Background(), UpstreamRequest{}, func(StreamEvent) error {
		events++
		return nil
	})
	if err == nil || events != 1 {
		t.Fatalf("err=%v events=%d", err, events)
	}
}

type streamAfterEventError struct{}

func (streamAfterEventError) Complete(context.Context, UpstreamRequest) (UpstreamResult, error) {
	return UpstreamResult{}, nil
}
func (streamAfterEventError) Models(context.Context) ([]ModelInfo, error) { return nil, nil }
func (streamAfterEventError) Stream(_ context.Context, _ UpstreamRequest, onEvent func(StreamEvent) error) error {
	_ = onEvent(StreamEvent{Kind: "text.delta", TextDelta: "partial"})
	return errors.New("upstream 503")
}

func TestCachedBorrowerCoalescesConcurrentCalls(t *testing.T) {
	var calls int
	b := newCachedBorrower(func() (codex.Credential, error) {
		calls++
		return codex.Credential{APIKey: "token", Mode: "chatgpt"}, nil
	}, time.Minute)
	b.now = func() time.Time { return time.Unix(100, 0) }

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := b.Borrow(); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}
