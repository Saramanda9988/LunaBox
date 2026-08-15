package s3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lunabox/internal/utils/proxyutils"
	"lunabox/internal/version"
)

type manualProxyConfig struct {
	url string
}

func (config manualProxyConfig) NetworkProxyConfig() (string, string) {
	return proxyutils.ProxyModeManual, config.url
}

func TestS3ProviderUsesProxyAndApplicationUserAgent(t *testing.T) {
	var userAgent string
	proxyServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Host != "s3-test.invalid" {
			t.Fatalf("expected request through proxy for s3-test.invalid, got %q", request.URL.String())
		}
		userAgent = request.Header.Get("User-Agent")
		writer.Header().Set("Content-Type", "application/xml")
		_, _ = writer.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <MaxKeys>1</MaxKeys>
  <KeyCount>0</KeyCount>
  <IsTruncated>false</IsTruncated>
		</ListBucketResult>`))
	}))
	defer proxyServer.Close()

	provider, err := NewS3Provider(S3Config{
		Endpoint:    "http://s3-test.invalid",
		Region:      "auto",
		Bucket:      "test-bucket",
		AccessKey:   "test-access-key",
		SecretKey:   "test-secret-key",
		ProxyConfig: manualProxyConfig{url: proxyServer.URL},
	})
	if err != nil {
		t.Fatalf("create S3 provider: %v", err)
	}
	if err := provider.TestConnection(context.Background()); err != nil {
		t.Fatalf("test S3 connection: %v", err)
	}

	if !strings.Contains(userAgent, version.UserAgent()) {
		t.Fatalf("expected S3 user agent to contain application identifier %q, got %q", version.UserAgent(), userAgent)
	}
}
