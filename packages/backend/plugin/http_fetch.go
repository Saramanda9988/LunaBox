package plugin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// pluginFetchHeaderTimeout 是全局 fetch 等待响应头的上限。
const pluginFetchHeaderTimeout = 30 * time.Second

// newPluginFetchClient 供插件全局 fetch 使用：不设 Client.Timeout（它把读取响应体也算在内，
// 会把 LLM 等长时间的流式响应截断），只限制等待响应头的时间；读响应体期间的中止靠 AbortSignal 与插件作用域。
func (m *Manager) newPluginFetchClient() *http.Client {
	client := m.newPluginHTTPClient(0)
	client.Transport = &headerTimeoutTransport{base: client.Transport, timeout: pluginFetchHeaderTimeout}
	return client
}

type headerTimeoutTransport struct {
	base    http.RoundTripper
	timeout time.Duration
}

func (t *headerTimeoutTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancelCause(req.Context())
	timer := time.AfterFunc(t.timeout, func() {
		cancel(fmt.Errorf("等待响应头超时（%s）", t.timeout))
	})
	resp, err := t.base.RoundTrip(req.WithContext(ctx))
	timer.Stop()
	if err != nil {
		if cause := context.Cause(ctx); cause != nil && ctx.Err() != nil && req.Context().Err() == nil {
			err = cause
		}
		cancel(nil)
		return nil, err
	}
	resp.Body = &cancelOnCloseBody{ReadCloser: resp.Body, cancel: func() { cancel(nil) }}
	return resp, nil
}

// cancelOnCloseBody 在响应体关闭时释放请求 context。
type cancelOnCloseBody struct {
	io.ReadCloser
	cancel func()
}

func (b *cancelOnCloseBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}
