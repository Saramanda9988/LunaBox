package service

import "testing"

func TestStartupServiceKeepsLatestFailureAndEmitsIt(t *testing.T) {
	startup := NewStartupService()
	want := StartupFailure{Message: "database unavailable"}

	var eventName string
	var eventFailure StartupFailure
	startup.SetEventEmitter(func(name string, values ...interface{}) {
		eventName = name
		if len(values) == 1 {
			eventFailure, _ = values[0].(StartupFailure)
		}
	})
	startup.ReportFailure(want.Message)

	if got := startup.GetFailure(); got != want {
		t.Fatalf("unexpected startup failure: got %#v, want %#v", got, want)
	}
	if eventName != startupFailedEvent {
		t.Fatalf("unexpected event name: %q", eventName)
	}
	if eventFailure != want {
		t.Fatalf("unexpected event payload: got %#v, want %#v", eventFailure, want)
	}
}
