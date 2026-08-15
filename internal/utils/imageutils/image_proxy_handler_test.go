package imageutils

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lunabox/internal/utils/proxyutils"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestRemoteImageProxyReturnsNotModifiedForConditionalRequest(t *testing.T) {
	var upstreamRequest *http.Request
	handler := &RemoteImageProxyHandler{
		clientFactory: func(time.Duration, proxyutils.ProxyConfigProvider) (*http.Client, string, error) {
			return &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					upstreamRequest = req.Clone(req.Context())
					headers := make(http.Header)
					headers.Set("ETag", `"cover-v2"`)
					headers.Set("Last-Modified", "Sun, 10 Aug 2025 08:00:00 GMT")
					return &http.Response{
						StatusCode: http.StatusNotModified,
						Header:     headers,
						Body:       io.NopCloser(strings.NewReader("")),
						Request:    req,
					}, nil
				}),
			}, "test", nil
		},
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/proxy/image?url=https%3A%2F%2Fimages.example.com%2Fcover.webp",
		nil,
	)
	request.Header.Set("If-None-Match", `"cover-v2"`)
	request.Header.Set("If-Modified-Since", "Sun, 10 Aug 2025 08:00:00 GMT")
	request.Header.Set("Range", "bytes=0-1023")
	request.Header.Set("If-Range", `"cover-v2"`)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotModified)
	}
	if upstreamRequest == nil {
		t.Fatal("upstream request was not made")
	}
	if got := upstreamRequest.Header.Get("If-None-Match"); got != `"cover-v2"` {
		t.Fatalf("If-None-Match = %q", got)
	}
	if got := upstreamRequest.Header.Get("If-Modified-Since"); got != "Sun, 10 Aug 2025 08:00:00 GMT" {
		t.Fatalf("If-Modified-Since = %q", got)
	}
	if got := upstreamRequest.Header.Get("Range"); got != "bytes=0-1023" {
		t.Fatalf("Range = %q", got)
	}
	if got := upstreamRequest.Header.Get("If-Range"); got != `"cover-v2"` {
		t.Fatalf("If-Range = %q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != remoteImageProxyCacheControl {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := recorder.Header().Get("ETag"); got != `"cover-v2"` {
		t.Fatalf("ETag = %q", got)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("response body length = %d, want 0", recorder.Body.Len())
	}
}

func TestRemoteImageProxyCachesSuccessfulResponseForOneYear(t *testing.T) {
	handler := &RemoteImageProxyHandler{
		clientFactory: func(time.Duration, proxyutils.ProxyConfigProvider) (*http.Client, string, error) {
			return &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					headers := make(http.Header)
					headers.Set("Content-Type", "image/webp")
					headers.Set("Content-Length", "10")
					headers.Set("ETag", `"cover-v2"`)
					return &http.Response{
						StatusCode:    http.StatusOK,
						ContentLength: int64(len("image-data")),
						Header:        headers,
						Body:          io.NopCloser(strings.NewReader("image-data")),
						Request:       req,
					}, nil
				}),
			}, "test", nil
		},
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/proxy/image?url=https%3A%2F%2Fimages.example.com%2Fcover.webp",
		nil,
	)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Cache-Control"); got != remoteImageProxyCacheControl {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := recorder.Body.String(); got != "image-data" {
		t.Fatalf("response body = %q", got)
	}
}
