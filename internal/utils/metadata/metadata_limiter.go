package metadata

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MetadataSource string

const (
	MetadataSourceBangumi       MetadataSource = "bangumi"
	MetadataSourceVNDB          MetadataSource = "vndb"
	MetadataSourceYMGal         MetadataSource = "ymgal"
	MetadataSourceSteam         MetadataSource = "steam"
	MetadataSourceDLsite        MetadataSource = "dlsite"
	MetadataSourceErogameScape  MetadataSource = "erogamescape"
	metadataMaxRateLimitRetries                = 1
)

type MetadataRateLimitPolicy struct {
	Source              MetadataSource
	Interval            time.Duration
	UpstreamLimit       int
	UpstreamWindow      time.Duration
	RateLimitRetryDelay time.Duration
}

func DefaultMetadataRateLimitPolicies() map[MetadataSource]MetadataRateLimitPolicy {
	policies := map[MetadataSource]MetadataRateLimitPolicy{
		MetadataSourceBangumi: {
			Source:         MetadataSourceBangumi,
			Interval:       time.Second,
			UpstreamLimit:  3000,
			UpstreamWindow: 10 * time.Minute,
		},
		MetadataSourceVNDB: {
			Source:              MetadataSourceVNDB,
			Interval:            4 * time.Second,
			UpstreamLimit:       200,
			UpstreamWindow:      5 * time.Minute,
			RateLimitRetryDelay: time.Minute,
		},
		MetadataSourceYMGal: {
			Source:   MetadataSourceYMGal,
			Interval: 2 * time.Second,
		},
		MetadataSourceSteam: {
			Source:   MetadataSourceSteam,
			Interval: time.Second,
		},
		MetadataSourceDLsite: {
			Source:   MetadataSourceDLsite,
			Interval: 2 * time.Second,
		},
		MetadataSourceErogameScape: {
			Source:   MetadataSourceErogameScape,
			Interval: 2 * time.Second,
		},
	}

	return policies
}

type metadataRateLimiter struct {
	mu       sync.Mutex
	policies map[MetadataSource]MetadataRateLimitPolicy
	sources  map[MetadataSource]*metadataSourceLimiter
	now      func() time.Time
	wait     func(context.Context, time.Duration) error
}

type metadataSourceLimiter struct {
	mu            sync.Mutex
	nextAllowedAt time.Time
	requestTimes  []time.Time
}

var sharedMetadataRateLimiter = newMetadataRateLimiter(DefaultMetadataRateLimitPolicies())

func newMetadataRateLimiter(policies map[MetadataSource]MetadataRateLimitPolicy) *metadataRateLimiter {
	copied := make(map[MetadataSource]MetadataRateLimitPolicy, len(policies))
	for source, policy := range policies {
		copied[source] = policy
	}

	return &metadataRateLimiter{
		policies: copied,
		sources:  make(map[MetadataSource]*metadataSourceLimiter, len(copied)),
		now:      time.Now,
		wait:     contextAwareSleep,
	}
}

func (l *metadataRateLimiter) Acquire(ctx context.Context, source MetadataSource) error {
	if l == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	policy, sourceLimiter, ok := l.sourceState(source)
	if !ok || !policy.hasLimit() {
		return nil
	}

	sourceLimiter.mu.Lock()
	defer sourceLimiter.mu.Unlock()

	for {
		now := l.now()
		waitFor := sourceLimiter.nextWaitDuration(policy, now)
		if waitFor <= 0 {
			sourceLimiter.reserve(policy, now)
			return nil
		}
		if err := l.wait(ctx, waitFor); err != nil {
			return err
		}
	}
}

func (p MetadataRateLimitPolicy) hasLimit() bool {
	return p.Interval > 0 || (p.UpstreamLimit > 0 && p.UpstreamWindow > 0)
}

func (l *metadataRateLimiter) Policy(source MetadataSource) (MetadataRateLimitPolicy, bool) {
	if l == nil {
		return MetadataRateLimitPolicy{}, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	policy, ok := l.policies[source]
	return policy, ok
}

func (l *metadataRateLimiter) sourceState(source MetadataSource) (MetadataRateLimitPolicy, *metadataSourceLimiter, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	policy, ok := l.policies[source]
	if !ok {
		return MetadataRateLimitPolicy{}, nil, false
	}
	sourceLimiter := l.sources[source]
	if sourceLimiter == nil {
		sourceLimiter = &metadataSourceLimiter{}
		l.sources[source] = sourceLimiter
	}
	return policy, sourceLimiter, true
}

func (l *metadataSourceLimiter) nextWaitDuration(policy MetadataRateLimitPolicy, now time.Time) time.Duration {
	var waitFor time.Duration
	if policy.Interval > 0 {
		waitFor = maxMetadataWait(waitFor, l.nextAllowedAt.Sub(now))
	}
	if policy.UpstreamLimit > 0 && policy.UpstreamWindow > 0 {
		l.pruneRequestTimes(now.Add(-policy.UpstreamWindow))
		if len(l.requestTimes) >= policy.UpstreamLimit {
			waitFor = maxMetadataWait(waitFor, l.requestTimes[0].Add(policy.UpstreamWindow).Sub(now))
		}
	}
	return waitFor
}

func (l *metadataSourceLimiter) reserve(policy MetadataRateLimitPolicy, now time.Time) {
	if policy.Interval > 0 {
		l.nextAllowedAt = now.Add(policy.Interval)
	}
	if policy.UpstreamLimit > 0 && policy.UpstreamWindow > 0 {
		l.requestTimes = append(l.requestTimes, now)
	}
}

func (l *metadataSourceLimiter) pruneRequestTimes(cutoff time.Time) {
	keepFrom := 0
	for keepFrom < len(l.requestTimes) && !l.requestTimes[keepFrom].After(cutoff) {
		keepFrom++
	}
	if keepFrom == 0 {
		return
	}
	copy(l.requestTimes, l.requestTimes[keepFrom:])
	l.requestTimes = l.requestTimes[:len(l.requestTimes)-keepFrom]
}

func maxMetadataWait(current time.Duration, candidate time.Duration) time.Duration {
	if candidate > current {
		return candidate
	}
	return current
}

func contextAwareSleep(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func doLimitedMetadataRequest(client *http.Client, req *http.Request, source MetadataSource) (*http.Response, error) {
	if client == nil {
		return nil, errors.New("metadata HTTP client is nil")
	}
	if req == nil {
		return nil, errors.New("metadata HTTP request is nil")
	}

	currentReq := req
	for attempt := 0; attempt <= metadataMaxRateLimitRetries; attempt++ {
		if err := sharedMetadataRateLimiter.Acquire(currentReq.Context(), source); err != nil {
			return nil, fmt.Errorf("%s metadata rate limit wait failed: %w", source, err)
		}

		resp, err := client.Do(currentReq)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		closeResponseBody(resp.Body)
		if attempt >= metadataMaxRateLimitRetries {
			return nil, fmt.Errorf("%s metadata request remained rate limited after retry: status %d, body: %s", source, resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}

		if err := waitForMetadataRateLimitRetry(currentReq.Context(), source, resp.Header.Get("Retry-After")); err != nil {
			return nil, err
		}
		retryReq, err := cloneMetadataRequest(currentReq)
		if err != nil {
			return nil, err
		}
		currentReq = retryReq
	}

	return nil, fmt.Errorf("%s metadata request remained rate limited after retry", source)
}

func doLimitedMetadataRequestBody(client *http.Client, req *http.Request, source MetadataSource) (int, http.Header, []byte, error) {
	resp, err := doLimitedMetadataRequest(client, req, source)
	if err != nil {
		return 0, nil, nil, err
	}
	defer closeResponseBody(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, resp.Header, nil, err
	}
	return resp.StatusCode, resp.Header, bodyBytes, nil
}

func waitForMetadataRateLimitRetry(ctx context.Context, source MetadataSource, retryAfter string) error {
	delay := parseRetryAfter(retryAfter)
	if delay <= 0 {
		if policy, ok := sharedMetadataRateLimiter.Policy(source); ok {
			delay = policy.RateLimitRetryDelay
			if delay <= 0 {
				delay = policy.Interval
			}
		}
	}
	if delay <= 0 {
		return nil
	}
	if err := sharedMetadataRateLimiter.wait(ctx, delay); err != nil {
		return fmt.Errorf("%s metadata retry wait failed: %w", source, err)
	}
	return nil
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds <= 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}

	retryAt, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	delay := time.Until(retryAt)
	if delay <= 0 {
		return 0
	}
	return delay
}

func cloneMetadataRequest(req *http.Request) (*http.Request, error) {
	if req == nil {
		return nil, errors.New("metadata HTTP request is nil")
	}

	cloned := req.Clone(req.Context())
	if req.Body == nil || req.Body == http.NoBody {
		cloned.Body = nil
		return cloned, nil
	}
	if req.GetBody == nil {
		return nil, errors.New("metadata request body cannot be replayed")
	}

	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	cloned.Body = body
	return cloned, nil
}

func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "rate limit") ||
		strings.Contains(message, "rate limited") ||
		strings.Contains(message, "too many requests") ||
		strings.Contains(message, "status 429") ||
		strings.Contains(message, "status: 429") ||
		strings.Contains(message, "http 429") ||
		strings.Contains(message, "limit exceeded") ||
		strings.Contains(message, "throttle")
}
