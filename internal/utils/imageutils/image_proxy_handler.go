package imageutils

import (
	"fmt"
	"io"
	"lunabox/internal/utils/downloadutils"
	"lunabox/internal/utils/proxyutils"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const remoteImageProxyMaxBytes int64 = 30 * 1024 * 1024

type RemoteImageProxyHandler struct {
	proxyConfig proxyutils.ProxyConfigProvider
}

func NewRemoteImageProxyHandler(proxyConfig proxyutils.ProxyConfigProvider) *RemoteImageProxyHandler {
	return &RemoteImageProxyHandler{
		proxyConfig: proxyConfig,
	}
}

func (h *RemoteImageProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	imageURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if imageURL == "" {
		http.Error(w, "image url is required", http.StatusBadRequest)
		return
	}
	if err := downloadutils.ValidateDownloadURL(imageURL); err != nil {
		http.Error(w, "invalid image url", http.StatusBadRequest)
		return
	}

	client, _, err := downloadutils.NewSecureHTTPClientFromConfig(30*time.Second, h.proxyConfig)
	if err != nil {
		http.Error(w, "failed to create image proxy client", http.StatusInternalServerError)
		return
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return http.ErrUseLastResponse
		}
		if req == nil || req.URL == nil {
			return fmt.Errorf("redirect url is required")
		}
		return downloadutils.ValidateDownloadURL(req.URL.String())
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, imageURL, nil)
	if err != nil {
		http.Error(w, "failed to build image request", http.StatusBadRequest)
		return
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("User-Agent", "LunaBox")
	if rangeHeader := strings.TrimSpace(r.Header.Get("Range")); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	resp, err := client.Do(req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		http.Error(w, "failed to fetch image", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		http.Error(w, fmt.Sprintf("image request failed: status %d", resp.StatusCode), http.StatusBadGateway)
		return
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	if contentType != "" && !strings.HasPrefix(contentType, "image/") {
		http.Error(w, "response is not an image", http.StatusBadGateway)
		return
	}
	if resp.ContentLength > remoteImageProxyMaxBytes {
		http.Error(w, "image is too large", http.StatusRequestEntityTooLarge)
		return
	}

	copyHeader(w.Header(), resp.Header, "Content-Type")
	copyHeader(w.Header(), resp.Header, "Content-Length")
	copyHeader(w.Header(), resp.Header, "Accept-Ranges")
	copyHeader(w.Header(), resp.Header, "Content-Range")
	copyHeader(w.Header(), resp.Header, "ETag")
	copyHeader(w.Header(), resp.Header, "Last-Modified")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if resp.ContentLength < 0 {
		w.Header().Del("Content-Length")
	}

	w.WriteHeader(resp.StatusCode)
	if r.Method == http.MethodHead {
		return
	}

	limitedBody := &io.LimitedReader{
		R: resp.Body,
		N: remoteImageProxyMaxBytes + 1,
	}
	_, _ = io.Copy(w, limitedBody)
}

func copyHeader(target http.Header, source http.Header, name string) {
	value := strings.TrimSpace(source.Get(name))
	if value == "" {
		return
	}
	if name == "Content-Length" {
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return
		}
	}
	target.Set(name, value)
}
