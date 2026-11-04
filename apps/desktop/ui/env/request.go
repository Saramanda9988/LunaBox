package env

import "context"

// Request 是一个可以取消的后台请求（搜索、授权等耗时的网络请求），零值可用。方法都在主线程上调用：
// Start 返回交给服务的 ctx，结果回到主线程后用 Finish 判断它是否还有效；Cancel 取消进行中的请求。
type Request struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// Start 取消上一个还没完成的请求，开始一个新的，返回它的 ctx。
func (r *Request) Start() context.Context {
	r.Cancel()
	r.ctx, r.cancel = context.WithCancel(context.Background())
	return r.ctx
}

// Cancel 取消进行中的请求，返回是否有请求被取消。
func (r *Request) Cancel() bool {
	if r.cancel == nil {
		return false
	}
	r.cancel()
	r.ctx, r.cancel = nil, nil
	return true
}

// Finish 在 ctx 的请求返回后调用：它仍是当前的请求时结束它并返回 true；已被取消或被新的请求取代时返回 false，
// 结果应当丢弃。
func (r *Request) Finish(ctx context.Context) bool {
	if ctx == nil || ctx != r.ctx || ctx.Err() != nil {
		return false
	}
	r.cancel()
	r.ctx, r.cancel = nil, nil
	return true
}

// Active 报告是否有进行中的请求。
func (r *Request) Active() bool { return r.cancel != nil }
