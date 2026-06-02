package metadata

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMetadataRateLimiterAppliesUpstreamWindow(t *testing.T) {
	const source MetadataSource = "test"
	start := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	now := start
	waits := []time.Duration{}

	limiter := newMetadataRateLimiter(map[MetadataSource]MetadataRateLimitPolicy{
		source: {
			Source:         source,
			UpstreamLimit:  2,
			UpstreamWindow: 10 * time.Second,
		},
	})
	limiter.now = func() time.Time {
		return now
	}
	limiter.wait = func(ctx context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		now = now.Add(delay)
		return nil
	}

	if err := limiter.Acquire(context.Background(), source); err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	if err := limiter.Acquire(context.Background(), source); err != nil {
		t.Fatalf("second acquire failed: %v", err)
	}
	if len(waits) != 0 {
		t.Fatalf("expected no wait before the window is full, got %v", waits)
	}

	if err := limiter.Acquire(context.Background(), source); err != nil {
		t.Fatalf("third acquire failed: %v", err)
	}
	if len(waits) != 1 || waits[0] != 10*time.Second {
		t.Fatalf("expected one 10s wait after filling the window, got %v", waits)
	}
}

func TestWaitForMetadataRateLimitRetryUsesPolicyRetryDelay(t *testing.T) {
	originalLimiter := sharedMetadataRateLimiter
	defer func() {
		sharedMetadataRateLimiter = originalLimiter
	}()

	limiter := newMetadataRateLimiter(map[MetadataSource]MetadataRateLimitPolicy{
		MetadataSourceVNDB: {
			Source:              MetadataSourceVNDB,
			Interval:            4 * time.Second,
			RateLimitRetryDelay: time.Minute,
		},
	})
	var waited time.Duration
	limiter.wait = func(ctx context.Context, delay time.Duration) error {
		waited = delay
		return nil
	}
	sharedMetadataRateLimiter = limiter

	if err := waitForMetadataRateLimitRetry(context.Background(), MetadataSourceVNDB, ""); err != nil {
		t.Fatalf("retry wait failed: %v", err)
	}
	if waited != time.Minute {
		t.Fatalf("expected VNDB retry delay to use policy retry delay, got %s", waited)
	}
}

func TestVNDBDefaultPolicyIsConservativeForLongBatches(t *testing.T) {
	policy, ok := DefaultMetadataRateLimitPolicies()[MetadataSourceVNDB]
	if !ok {
		t.Fatal("expected VNDB policy")
	}
	if policy.Interval != 4*time.Second {
		t.Fatalf("expected VNDB interval to be 4s, got %s", policy.Interval)
	}
	if policy.UpstreamLimit != 200 || policy.UpstreamWindow != 5*time.Minute {
		t.Fatalf("unexpected VNDB upstream window: limit=%d window=%s", policy.UpstreamLimit, policy.UpstreamWindow)
	}
	if policy.RateLimitRetryDelay != time.Minute {
		t.Fatalf("expected VNDB retry delay to be 1m, got %s", policy.RateLimitRetryDelay)
	}
}

func TestIsRateLimitError(t *testing.T) {
	cases := []error{
		errors.New("vndb metadata request remained rate limited after retry: status 429"),
		errors.New("VNDB API returned status: 429"),
		errors.New("too many requests"),
		errors.New("limit exceeded by upstream"),
		errors.New("request was throttled"),
	}

	for _, err := range cases {
		if !IsRateLimitError(err) {
			t.Fatalf("expected rate limit error for %q", err.Error())
		}
	}

	if IsRateLimitError(errors.New("no results found")) {
		t.Fatal("did not expect no-result errors to be treated as rate limits")
	}
}
