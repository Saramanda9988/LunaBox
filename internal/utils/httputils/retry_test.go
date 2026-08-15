package httputils

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	httpDate := now.Add(45 * time.Second).Format(http.TimeFormat)

	tests := []struct {
		name   string
		value  string
		want   time.Duration
		wantOK bool
	}{
		{name: "seconds", value: "12", want: 12 * time.Second, wantOK: true},
		{name: "HTTP date", value: httpDate, want: 45 * time.Second, wantOK: true},
		{name: "past HTTP date", value: now.Add(-time.Minute).Format(http.TimeFormat), want: 0, wantOK: true},
		{name: "empty", value: "", want: 0, wantOK: false},
		{name: "negative seconds", value: "-1", want: 0, wantOK: false},
		{name: "invalid", value: "later", want: 0, wantOK: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ParseRetryAfter(test.value, now)
			if ok != test.wantOK || got != test.want {
				t.Fatalf("ParseRetryAfter(%q) = (%s, %v), want (%s, %v)", test.value, got, ok, test.want, test.wantOK)
			}
		})
	}
}

func TestDoWithRetrySucceedsAndClosesRetryResponse(t *testing.T) {
	var attempts atomic.Int32
	firstBody := &trackingReadCloser{Reader: strings.NewReader("rate limited")}
	client := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		attempt := attempts.Add(1)
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if string(bodyBytes) != `{"id":1}` {
			t.Fatalf("unexpected request body on attempt %d: %q", attempt, bodyBytes)
		}
		if attempt == 1 {
			return response(req, http.StatusTooManyRequests, firstBody), nil
		}
		return response(req, http.StatusOK, io.NopCloser(strings.NewReader("ok"))), nil
	})}
	req, err := http.NewRequest(http.MethodPost, "https://example.com", bytes.NewBufferString(`{"id":1}`))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	resp, err := DoWithRetry(context.Background(), client, req, RetryPolicy{
		MaxRetries:    2,
		FallbackDelay: time.Second,
		Wait: func(context.Context, time.Duration) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("DoWithRetry failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts.Load())
	}
	if !firstBody.closed.Load() {
		t.Fatal("retry response body was not closed")
	}
}

func TestDoWithRetryReturnsLastResponseAtRetryLimit(t *testing.T) {
	var attempts atomic.Int32
	client := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		attempts.Add(1)
		return response(req, http.StatusTooManyRequests, io.NopCloser(strings.NewReader("rate limited"))), nil
	})}
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	resp, err := DoWithRetry(context.Background(), client, req, RetryPolicy{
		MaxRetries: 2,
		Wait: func(context.Context, time.Duration) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("DoWithRetry failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	if attempts.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestDoWithRetryRetriesTransientServerStatus(t *testing.T) {
	var attempts atomic.Int32
	client := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if attempts.Add(1) == 1 {
			return response(req, http.StatusServiceUnavailable, io.NopCloser(strings.NewReader("unavailable"))), nil
		}
		return response(req, http.StatusOK, io.NopCloser(strings.NewReader("ok"))), nil
	})}
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	resp, err := DoWithRetry(context.Background(), client, req, RetryPolicy{
		MaxRetries: 1,
		Wait:       func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatalf("DoWithRetry failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || attempts.Load() != 2 {
		t.Fatalf("status = %d, attempts = %d; want status 200 after 2 attempts", resp.StatusCode, attempts.Load())
	}
}

func TestDoWithRetryRetriesEOFAndReplaysBody(t *testing.T) {
	var attempts atomic.Int32
	client := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		attempt := attempts.Add(1)
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if string(body) != "cover" {
			t.Fatalf("request body on attempt %d = %q, want cover", attempt, body)
		}
		if attempt == 1 {
			return nil, io.EOF
		}
		return response(req, http.StatusOK, io.NopCloser(strings.NewReader("ok"))), nil
	})}
	req, err := http.NewRequest(http.MethodPut, "https://example.com", strings.NewReader("cover"))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	resp, err := DoWithRetry(context.Background(), client, req, RetryPolicy{
		MaxRetries: 1,
		Wait:       func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatalf("DoWithRetry failed: %v", err)
	}
	defer resp.Body.Close()
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
}

func TestIsRetryableHTTPStatus(t *testing.T) {
	for _, status := range []int{408, 425, 429, 500, 502, 503, 504} {
		if !IsRetryableHTTPStatus(status) {
			t.Errorf("IsRetryableHTTPStatus(%d) = false, want true", status)
		}
	}
	for _, status := range []int{400, 401, 403, 404, 409} {
		if IsRetryableHTTPStatus(status) {
			t.Errorf("IsRetryableHTTPStatus(%d) = true, want false", status)
		}
	}
}

func TestIsRetryableTransportError(t *testing.T) {
	for _, err := range []error{io.EOF, io.ErrUnexpectedEOF, syscall.ECONNRESET, syscall.EPIPE} {
		if !IsRetryableTransportError(err) {
			t.Errorf("IsRetryableTransportError(%v) = false, want true", err)
		}
	}
	if IsRetryableTransportError(context.Canceled) {
		t.Error("IsRetryableTransportError(context.Canceled) = true, want false")
	}
}

func TestDoWithRetryRejectsUnsafeRequestReplay(t *testing.T) {
	firstBody := &trackingReadCloser{Reader: strings.NewReader("rate limited")}
	client := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return response(req, http.StatusTooManyRequests, firstBody), nil
	})}
	req, err := http.NewRequest(http.MethodPost, "https://example.com", io.NopCloser(strings.NewReader("body")))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	_, err = DoWithRetry(context.Background(), client, req, RetryPolicy{MaxRetries: 1})
	if err == nil || !strings.Contains(err.Error(), "cannot be replayed") {
		t.Fatalf("expected request replay error, got %v", err)
	}
	if !firstBody.closed.Load() {
		t.Fatal("retry response body was not closed after replay failure")
	}
}

func TestDoWithRetryCancelsDuringWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		cancel()
		resp := response(req, http.StatusTooManyRequests, io.NopCloser(strings.NewReader("rate limited")))
		resp.Header.Set("Retry-After", "60")
		return resp, nil
	})}
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	_, err = DoWithRetry(ctx, client, req, RetryPolicy{MaxRetries: 1})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestWaitForRetryObservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := WaitForRetry(ctx, time.Minute); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type trackingReadCloser struct {
	io.Reader
	closed atomic.Bool
}

func (c *trackingReadCloser) Close() error {
	c.closed.Store(true)
	return nil
}

func response(req *http.Request, status int, body io.ReadCloser) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       body,
		Request:    req,
	}
}
