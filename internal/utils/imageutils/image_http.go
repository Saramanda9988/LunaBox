package imageutils

import (
	"context"
	"net/http"
	"time"

	"lunabox/internal/utils/httputils"
	"lunabox/internal/utils/proxyutils"
	"lunabox/internal/version"
	"resty.dev/v3"
)

const (
	imageDownloadMaxRetries    = 5
	imageDownloadFallbackDelay = 2 * time.Second
	imageDownloadMaxRetryDelay = 30 * time.Second
)

func setImageRequestHeaders(req *http.Request) {
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("User-Agent", version.UserAgent())
}

func newImageRestyClient(timeout time.Duration, mode string, manualURL string) (*resty.Client, error) {
	client, _, err := httputils.NewRestyClient(httputils.ClientOptions{
		Timeout:   timeout,
		ProxyMode: mode,
		ProxyURL:  manualURL,
	})
	return client, err
}

func newImageRestyClientFromConfig(timeout time.Duration, config proxyutils.ProxyConfigProvider) (*resty.Client, error) {
	client, _, err := httputils.NewRestyClient(httputils.ClientOptions{
		Timeout:     timeout,
		ProxyConfig: config,
	})
	return client, err
}

func newSystemImageRestyClient(timeout time.Duration) (*resty.Client, error) {
	client, _, err := httputils.NewRestyClient(httputils.ClientOptions{Timeout: timeout})
	return client, err
}

func newImageRestyClientWithHTTPClient(client *http.Client) (*resty.Client, error) {
	return httputils.NewRestyClientWithHTTPClient(client, "")
}

func newImageRequest(ctx context.Context, client *resty.Client) *resty.Request {
	if ctx == nil {
		ctx = context.Background()
	}
	return client.R().
		SetContext(ctx).
		SetHeader("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8").
		SetHeader("User-Agent", version.UserAgent()).
		SetRetryCount(imageDownloadMaxRetries).
		SetRetryWaitTime(imageDownloadFallbackDelay).
		SetRetryMaxWaitTime(imageDownloadMaxRetryDelay).
		AddRetryConditions(
			resty.RetryConditionStatusTooManyRequests,
			resty.RetryConditionStatus5XX,
		)
}
