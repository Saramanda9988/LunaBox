//go:build windows

package service

import (
	"errors"
	"os"
	"testing"

	"lunabox/internal/appconf"
	"lunabox/internal/utils/timerutils"
)

func newAudioTestSession() (*StartService, *activePlaySession, timerutils.FocusUpdate) {
	s := NewStartService()
	s.config = &appconf.AppConfig{MuteGameInBackground: true}
	session := &activePlaySession{gameID: "game", sessionID: "session", done: make(chan struct{})}
	s.activeSessions[session.gameID] = session
	update := timerutils.FocusUpdate{
		GameID: session.gameID, SessionID: session.sessionID, ProcessID: uint32(os.Getpid()),
	}
	return s, session, update
}

func TestBackgroundMuteRetainsPartialSuccess(t *testing.T) {
	s, session, update := newAudioTestSession()
	calls := 0
	s.setProcessMuted = func(pid uint32, muted bool) (bool, error) {
		calls++
		if calls == 1 {
			return true, errors.New("second audio session failed")
		}
		return true, nil
	}
	s.handleFocusUpdate(update)
	if !session.audioStateKnown || !session.audioMuted || session.audioPID != update.ProcessID {
		t.Fatal("partial mute success must remain available for restoration")
	}
	s.handleFocusUpdate(update)
	if calls != 2 || session.audioLastError != "" {
		t.Fatal("the same focus state must retry after partial failure")
	}
	update.IsFocused = true
	s.handleFocusUpdate(update)
	if calls != 3 || session.audioMuted {
		t.Fatal("returning to the foreground must restore audio")
	}
}

func TestBackgroundMuteKeepsOldPIDAfterRestoreFailure(t *testing.T) {
	for _, result := range []string{"error", "missing", "partial"} {
		t.Run(result, func(t *testing.T) {
			s, session, update := newAudioTestSession()
			session.audioPID, session.audioMuted, session.audioStateKnown = 42, true, true
			calls := 0
			s.setProcessMuted = func(pid uint32, muted bool) (bool, error) {
				calls++
				if pid != 42 || muted {
					t.Fatalf("old process must be restored before replacing its state: pid=%d muted=%v", pid, muted)
				}
				switch result {
				case "missing":
					return false, nil
				case "partial":
					return true, errors.New("another session could not be restored")
				default:
					return false, errors.New("audio device unavailable")
				}
			}
			s.handleFocusUpdate(update)
			if calls != 1 || session.audioPID != 42 || !session.audioMuted || !session.audioStateKnown {
				t.Fatal("failed restoration must retain the old process state")
			}
		})
	}
}

func TestShutdownPreventsInFlightFocusUpdateFromMutingAgain(t *testing.T) {
	s, session, update := newAudioTestSession()
	session.audioPID, session.audioMuted, session.audioStateKnown = update.ProcessID, true, true
	restoring, proceed := make(chan struct{}), make(chan struct{})
	s.setProcessMuted = func(pid uint32, muted bool) (bool, error) {
		if muted {
			t.Error("focus update muted the game after shutdown cleanup")
			return true, nil
		}
		close(restoring)
		<-proceed
		return true, nil
	}
	finished := make(chan struct{})
	go func() {
		// A tracker or database service is not required for audio cleanup.
		s.CleanupPendingSessions()
		close(finished)
	}()
	<-restoring
	callbackDone := make(chan struct{})
	go func() {
		s.handleFocusUpdate(update)
		close(callbackDone)
	}()
	close(proceed)
	<-finished
	<-callbackDone
	if !session.audioStopped || session.audioStateKnown {
		t.Fatal("shutdown must disable muting and clear restored audio state")
	}
}

func TestFinalAudioCleanupRetriesTransientFailure(t *testing.T) {
	s, session, update := newAudioTestSession()
	session.audioPID, session.audioMuted, session.audioStateKnown = update.ProcessID, true, true
	calls := 0
	s.setProcessMuted = func(pid uint32, muted bool) (bool, error) {
		calls++
		if muted {
			t.Fatal("cleanup requested mute")
		}
		if calls == 1 {
			return false, errors.New("transient audio failure")
		}
		return true, nil
	}
	s.stopSessionAudio(session)
	if calls != 2 || session.audioStateKnown {
		t.Fatal("cleanup should retry transient failure and clear state after success")
	}
}

func TestForegroundPartialRestoreRemainsPending(t *testing.T) {
	s, session, update := newAudioTestSession()
	session.audioPID, session.audioMuted, session.audioStateKnown = update.ProcessID, true, true
	update.IsFocused = true
	s.setProcessMuted = func(uint32, bool) (bool, error) {
		return true, errors.New("one session remains muted")
	}
	s.handleFocusUpdate(update)
	if !session.audioMuted || !session.audioStateKnown {
		t.Fatal("partial restoration must stay pending for the next callback or cleanup")
	}
}

func TestForegroundPartialRestoreFromUnknownStateRemainsRecoverable(t *testing.T) {
	s, session, update := newAudioTestSession()
	update.IsFocused = true
	s.setProcessMuted = func(uint32, bool) (bool, error) {
		return true, errors.New("one session remains muted")
	}
	s.handleFocusUpdate(update)
	if !session.audioMuted || !session.audioStateKnown || session.audioPID != update.ProcessID {
		t.Fatal("partial foreground restoration must remain recoverable")
	}
}

func TestExitedProcessFocusUpdateRestoresAudio(t *testing.T) {
	s, session, update := newAudioTestSession()
	update.ProcessID = ^uint32(0)
	session.audioPID, session.audioMuted, session.audioStateKnown = update.ProcessID, true, true
	calls := 0
	s.setProcessMuted = func(pid uint32, muted bool) (bool, error) {
		calls++
		if pid != update.ProcessID || muted {
			t.Fatalf("expected restoration of exited process, got pid=%d muted=%v", pid, muted)
		}
		return true, nil
	}
	s.handleFocusUpdate(update)
	if calls != 1 || session.audioStateKnown {
		t.Fatal("process exit must restore audio before discarding state")
	}
}
