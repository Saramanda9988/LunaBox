package httputils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const retryResponseDrainLimit = 32 * 1024

// RetryPolicy configures retries after responses with retryable status codes.
type RetryPolicy struct {
	MaxRetries      int
	FallbackDelay   time.Duration
	MaxDelay        time.Duration
	RetryableStatus func(int) bool
	RetryableError  func(error) bool
	BeforeAttempt   func(context.Context, int) error
	RetryDelay      func(*http.Response, int) time.Duration
	Wait            func(context.Context, time.Duration) error
}

// DoWithRetry performs an HTTP request and retries matching responses.
// The caller owns the body of the returned response.
func DoWithRetry(
	ctx context.Context,
	client *http.Client,
	req *http.Request,
	policy RetryPolicy,
) (*http.Response, error) {
	if client == nil {
		return nil, errors.New("HTTP client is nil")
	}
	if req == nil {
		return nil, errors.New("HTTP request is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	maxRetries := policy.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	retryableStatus := policy.RetryableStatus
	if retryableStatus == nil {
		retryableStatus = IsRetryableHTTPStatus
	}
	retryableError := policy.RetryableError
	if retryableError == nil {
		retryableError = IsRetryableTransportError
	}
	wait := policy.Wait
	if wait == nil {
		wait = WaitForRetry
	}

	currentReq := req.Clone(ctx)
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if policy.BeforeAttempt != nil {
			if err := policy.BeforeAttempt(ctx, attempt); err != nil {
				closeRequestBody(currentReq)
				return nil, fmt.Errorf("prepare HTTP attempt %d: %w", attempt+1, err)
			}
		}

		resp, err := client.Do(currentReq)
		if err != nil {
			if ctx.Err() != nil || !retryableError(err) || attempt >= maxRetries {
				return nil, err
			}

			retryReq, cloneErr := cloneRequestForRetry(req, ctx)
			if cloneErr != nil {
				closeRetryResponse(resp)
				return nil, cloneErr
			}
			closeRetryResponse(resp)
			delay := retryDelay(policy, resp, attempt)
			if waitErr := wait(ctx, delay); waitErr != nil {
				closeRequestBody(retryReq)
				return nil, fmt.Errorf("wait before HTTP retry %d: %w", attempt+1, waitErr)
			}
			currentReq = retryReq
			continue
		}
		if !retryableStatus(resp.StatusCode) || attempt >= maxRetries {
			return resp, nil
		}

		retryReq, err := cloneRequestForRetry(req, ctx)
		if err != nil {
			closeRetryResponse(resp)
			return nil, err
		}
		delay := retryDelay(policy, resp, attempt)
		closeRetryResponse(resp)
		if err := wait(ctx, delay); err != nil {
			closeRequestBody(retryReq)
			return nil, fmt.Errorf("wait before HTTP retry %d: %w", attempt+1, err)
		}
		currentReq = retryReq
	}

	return nil, errors.New("HTTP retry loop ended unexpectedly")
}

// IsRetryableHTTPStatus reports whether a response commonly represents a
// temporary server, gateway, timeout, or rate-limit condition.
func IsRetryableHTTPStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusRequestTimeout,
		http.StatusTooEarly,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// IsRetryableTransportError reports whether an HTTP transport failure is
// likely temporary. Context cancellation is handled separately by
// DoWithRetry and never retried after the caller's context has ended.
func IsRetryableTransportError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) {
		return true
	}
	if errors.Is(err, syscall.ECONNABORTED) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary())
}

// ParseRetryAfter parses a Retry-After value containing seconds or an HTTP date.
func ParseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}

	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds < 0 || seconds > math.MaxInt64/int64(time.Second) {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}

	retryAt, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	delay := retryAt.Sub(now)
	if delay < 0 {
		delay = 0
	}
	return delay, true
}

// WaitForRetry waits for the delay or returns when the context is canceled.
func WaitForRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
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

func retryDelay(policy RetryPolicy, resp *http.Response, retryIndex int) time.Duration {
	if policy.RetryDelay != nil && resp != nil {
		delay := policy.RetryDelay(resp, retryIndex)
		if delay < 0 {
			return 0
		}
		return delay
	}

	if resp != nil {
		if delay, ok := ParseRetryAfter(resp.Header.Get("Retry-After"), time.Now()); ok {
			return capRetryDelay(delay, policy.MaxDelay)
		}
	}

	delay := exponentialRetryDelay(policy.FallbackDelay, retryIndex)
	return capRetryDelay(delay, policy.MaxDelay)
}

func exponentialRetryDelay(base time.Duration, retryIndex int) time.Duration {
	if base <= 0 {
		return 0
	}
	delay := base
	for index := 0; index < retryIndex; index++ {
		if delay > time.Duration(math.MaxInt64)/2 {
			return time.Duration(math.MaxInt64)
		}
		delay *= 2
	}
	return delay
}

func capRetryDelay(delay time.Duration, maximum time.Duration) time.Duration {
	if delay < 0 {
		return 0
	}
	if maximum > 0 && delay > maximum {
		return maximum
	}
	return delay
}

func cloneRequestForRetry(req *http.Request, ctx context.Context) (*http.Request, error) {
	cloned := req.Clone(ctx)
	if req.Body == nil || req.Body == http.NoBody {
		cloned.Body = nil
		return cloned, nil
	}
	if req.GetBody == nil {
		return nil, errors.New("HTTP request body cannot be replayed")
	}

	body, err := req.GetBody()
	if err != nil {
		return nil, fmt.Errorf("recreate HTTP request body: %w", err)
	}
	cloned.Body = body
	return cloned, nil
}

func closeRetryResponse(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, retryResponseDrainLimit))
	_ = resp.Body.Close()
}

func closeRequestBody(req *http.Request) {
	if req == nil || req.Body == nil || req.Body == http.NoBody {
		return
	}
	_ = req.Body.Close()
}
