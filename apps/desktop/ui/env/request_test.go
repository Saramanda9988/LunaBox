package env

import "testing"

func TestRequest(t *testing.T) {
	var r Request
	if r.Active() || r.Cancel() || r.Finish(nil) {
		t.Fatal("zero Request should be idle")
	}

	first := r.Start()
	if !r.Active() {
		t.Fatal("Start should make the request active")
	}
	second := r.Start() // 新的请求取消上一个
	if first.Err() == nil {
		t.Fatal("Start should cancel the previous request")
	}
	if r.Finish(first) {
		t.Fatal("a superseded request should not finish")
	}
	if !r.Finish(second) || r.Active() {
		t.Fatal("the current request should finish")
	}
	if second.Err() == nil {
		t.Fatal("Finish should release the request's context")
	}

	third := r.Start()
	if !r.Cancel() || third.Err() == nil || r.Active() {
		t.Fatal("Cancel should cancel the active request")
	}
	if r.Finish(third) {
		t.Fatal("a cancelled request should not finish")
	}
}
