package metadata

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

// TestWithContextCancelsInFlightRequest 确认 WithContext 的 ctx 取消后，还在等待响应的请求立刻返回 ctx 的错误。
func TestWithContextCancelsInFlightRequest(t *testing.T) {
	started := make(chan struct{}, 4)
	client := &http.Client{Transport: metadataRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		started <- struct{}{}
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	getter := NewDLsiteInfoGetter(WithHTTPClient(client), WithContext(ctx))

	done := make(chan error, 1)
	go func() {
		_, err := getter.FetchMetadata("RJ01000000", "")
		done <- err
	}()

	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("FetchMetadata error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("FetchMetadata did not return after the context was cancelled")
	}
}
