package timerutils

import (
	"context"
	"lunabox/backend/utils/processutils"
	"testing"
	"time"
)

func TestFocusUpdateIncludesCurrentProcess(t *testing.T) {
	tracker := NewActiveTimeTracker(context.Background(), nil)
	var received FocusUpdate
	tracker.SetFocusUpdateHandler(func(update FocusUpdate) {
		received = update
	})
	session := &TrackingSession{
		SessionID: "session-1",
		GameID:    "game-1",
		ProcessID: 42,
	}

	tracker.emitFocusUpdate(session, true)

	if received.SessionID != session.SessionID || received.GameID != session.GameID {
		t.Fatalf("unexpected focus update identity: %#v", received)
	}
	if received.ProcessID != session.ProcessID || !received.IsFocused {
		t.Fatalf("unexpected focus update state: %#v", received)
	}
}

func TestActiveTrackFocusByBundlePath(t *testing.T) {
	restore := stubFocusFunctions(t)
	defer restore()
	isBundlePathFocused = func(bundlePath string) bool {
		return bundlePath == "/Applications/Game.app"
	}

	tracker := NewActiveTimeTracker(context.Background(), nil)
	session := &TrackingSession{
		ProcessID: 99,
		ActiveTrack: ActiveTrack{
			Kind:       ActiveTrackBundlePath,
			BundlePath: "/Applications/Game.app",
		},
	}

	if !tracker.isSessionFocused(session) {
		t.Fatalf("expected bundle path session to be focused")
	}
}

func TestActiveTrackFocusByProcessTreeRoot(t *testing.T) {
	restore := stubFocusFunctions(t)
	defer restore()
	getForegroundProcessID = func() (uint32, bool) {
		return 100, true
	}

	tracker := NewActiveTimeTracker(context.Background(), nil)
	session := &TrackingSession{
		ProcessID: 100,
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackProcessTree,
		},
	}

	if !tracker.isSessionFocused(session) {
		t.Fatal("expected process tree root to be focused")
	}
}

func TestActiveTrackFocusByProcessTreeDescendant(t *testing.T) {
	restore := stubFocusFunctions(t)
	defer restore()
	getForegroundProcessID = func() (uint32, bool) {
		return 200, true
	}
	isProcessDescendant = func(rootPID uint32, candidatePID uint32) bool {
		if rootPID != 100 || candidatePID != 200 {
			t.Fatalf("unexpected descendant lookup: root=%d candidate=%d", rootPID, candidatePID)
		}
		return true
	}

	tracker := NewActiveTimeTracker(context.Background(), nil)
	session := &TrackingSession{
		ProcessID: 100,
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackProcessTree,
		},
	}

	if !tracker.isSessionFocused(session) {
		t.Fatal("expected process tree descendant to be focused")
	}
}

func TestActiveTrackRejectsUnrelatedProcess(t *testing.T) {
	restore := stubFocusFunctions(t)
	defer restore()
	getForegroundProcessID = func() (uint32, bool) {
		return 300, true
	}
	isProcessDescendant = func(rootPID uint32, candidatePID uint32) bool {
		return false
	}

	tracker := NewActiveTimeTracker(context.Background(), nil)
	session := &TrackingSession{
		ProcessID: 100,
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackProcessTree,
		},
	}

	if tracker.isSessionFocused(session) {
		t.Fatal("expected unrelated process not to be focused")
	}
}

func TestActiveTrackFocusByWineRootDescendant(t *testing.T) {
	restore := stubFocusFunctions(t)
	defer restore()
	getForegroundProcessID = func() (uint32, bool) {
		return 200, true
	}
	isProcessDescendant = func(rootPID uint32, candidatePID uint32) bool {
		if rootPID != 100 || candidatePID != 200 {
			t.Fatalf("unexpected descendant lookup: root=%d candidate=%d", rootPID, candidatePID)
		}
		return true
	}

	tracker := NewActiveTimeTracker(context.Background(), nil)
	session := &TrackingSession{
		ProcessID: 100,
		ActiveTrack: ActiveTrack{
			Kind:    ActiveTrackWineRootPID,
			RootPID: 100,
		},
	}

	if !tracker.isSessionFocused(session) {
		t.Fatalf("expected wine descendant session to be focused")
	}
}

func TestActiveTrackFocusByWineRootAliveWhenForegroundUnavailable(t *testing.T) {
	restore := stubFocusFunctions(t)
	defer restore()
	isProcessPresent = func(pid uint32) bool {
		return pid == 100
	}

	tracker := NewActiveTimeTracker(context.Background(), nil)
	session := &TrackingSession{
		ProcessID: 100,
		ActiveTrack: ActiveTrack{
			Kind:    ActiveTrackWineRootPID,
			RootPID: 100,
		},
	}

	if !tracker.isSessionFocused(session) {
		t.Fatalf("expected wine root session to be active when foreground pid is unavailable")
	}
}

func TestFocusUpdateUsesLastFocusedWineDescendant(t *testing.T) {
	restore := stubFocusFunctions(t)
	defer restore()
	foregroundPID := uint32(200)
	getForegroundProcessID = func() (uint32, bool) {
		return foregroundPID, true
	}
	isProcessDescendant = func(rootPID uint32, candidatePID uint32) bool {
		return candidatePID == 200
	}

	tracker := NewActiveTimeTracker(context.Background(), nil)
	session := &TrackingSession{
		SessionID: "session-1",
		GameID:    "game-1",
		ProcessID: 100,
		ActiveTrack: ActiveTrack{
			Kind:    ActiveTrackWineRootPID,
			RootPID: 100,
		},
	}

	if !tracker.isSessionFocused(session) {
		t.Fatal("expected wine descendant session to be focused")
	}
	foregroundPID = 300
	if tracker.isSessionFocused(session) {
		t.Fatal("expected wine session to lose focus")
	}

	var received FocusUpdate
	tracker.SetFocusUpdateHandler(func(update FocusUpdate) {
		received = update
	})
	tracker.emitFocusUpdate(session, false)
	if received.ProcessID != 200 || received.IsFocused {
		t.Fatalf("unexpected focus update after Wine game lost focus: %#v", received)
	}
}

func TestActiveTrackFocusByDetachedWineTarget(t *testing.T) {
	restore := stubFocusFunctions(t)
	defer restore()
	getForegroundProcessID = func() (uint32, bool) {
		return 200, true
	}
	getDescendantProcesses = func(parentPID uint32) ([]processutils.ProcessInfo, error) {
		return nil, nil
	}
	getProcessCommandInfo = func(pid uint32) (processutils.ProcessCommandInfo, error) {
		if pid != 200 {
			t.Fatalf("expected foreground pid 200, got %d", pid)
		}
		return processutils.ProcessCommandInfo{
			Arguments:   []string{"wine64-preloader", `Z:\Users\u\games\Game.exe`},
			Environment: map[string]string{"CX_BOTTLE": "test1"},
		}, nil
	}

	tracker := NewActiveTimeTracker(context.Background(), nil)
	session := &TrackingSession{
		ProcessID: 100,
		ActiveTrack: ActiveTrack{
			Kind:           ActiveTrackWineRootPID,
			RootPID:        100,
			ExecutablePath: "/Users/u/games/Game.exe",
			Bottle:         "test1",
		},
	}

	if !tracker.isSessionFocused(session) {
		t.Fatal("expected detached Wine target session to be focused")
	}
}

func TestActiveTrackRejectsDetachedWineTargetFromDifferentBottle(t *testing.T) {
	restore := stubFocusFunctions(t)
	defer restore()
	getForegroundProcessID = func() (uint32, bool) {
		return 200, true
	}
	getProcessCommandInfo = func(pid uint32) (processutils.ProcessCommandInfo, error) {
		return processutils.ProcessCommandInfo{
			Arguments:   []string{`Z:\Users\u\games\Game.exe`},
			Environment: map[string]string{"CX_BOTTLE": "other"},
		}, nil
	}

	tracker := NewActiveTimeTracker(context.Background(), nil)
	session := &TrackingSession{
		ProcessID: 100,
		ActiveTrack: ActiveTrack{
			Kind:           ActiveTrackWineRootPID,
			RootPID:        100,
			ExecutablePath: "/Users/u/games/Game.exe",
			Bottle:         "test1",
		},
	}

	if tracker.isSessionFocused(session) {
		t.Fatal("expected Wine target from a different bottle not to be focused")
	}
}

func TestActiveTrackRejectsDetachedWineTargetFromDifferentPrefix(t *testing.T) {
	restore := stubFocusFunctions(t)
	defer restore()
	getForegroundProcessID = func() (uint32, bool) {
		return 200, true
	}
	getProcessCommandInfo = func(pid uint32) (processutils.ProcessCommandInfo, error) {
		return processutils.ProcessCommandInfo{
			Arguments:   []string{`Z:\Users\u\games\Game.exe`},
			Environment: map[string]string{"WINEPREFIX": "/home/u/.wine-other"},
		}, nil
	}

	tracker := NewActiveTimeTracker(context.Background(), nil)
	session := &TrackingSession{
		ProcessID: 100,
		ActiveTrack: ActiveTrack{
			Kind:           ActiveTrackWineRootPID,
			RootPID:        100,
			ExecutablePath: "/home/u/games/Game.exe",
			WinePrefix:     "/home/u/.wine-game",
		},
	}

	if tracker.isSessionFocused(session) {
		t.Fatal("expected Wine target from a different prefix not to be focused")
	}
}

func TestActiveTrackFocusByLauncherPID(t *testing.T) {
	restore := stubFocusFunctions(t)
	defer restore()
	getForegroundProcessID = func() (uint32, bool) {
		return 300, true
	}

	tracker := NewActiveTimeTracker(context.Background(), nil)
	session := &TrackingSession{
		ProcessID: 300,
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackLauncherPID,
		},
	}

	if !tracker.isSessionFocused(session) {
		t.Fatalf("expected launcher pid session to be focused")
	}
}

func stubFocusFunctions(t *testing.T) func() {
	t.Helper()
	origBundleFocused := isBundlePathFocused
	origPID := getForegroundProcessID
	origIsDescendant := isProcessDescendant
	origDescendants := getDescendantProcesses
	origPresent := isProcessPresent
	origProcessCommandInfo := getProcessCommandInfo
	origFocused := isProcessFocused
	origIdle := getIdleDuration

	isBundlePathFocused = func(bundlePath string) bool { return false }
	getForegroundProcessID = func() (uint32, bool) { return 0, false }
	isProcessDescendant = func(rootPID uint32, candidatePID uint32) bool { return false }
	getDescendantProcesses = func(parentPID uint32) ([]processutils.ProcessInfo, error) { return nil, nil }
	isProcessPresent = func(processID uint32) bool { return false }
	getProcessCommandInfo = func(pid uint32) (processutils.ProcessCommandInfo, error) {
		return processutils.ProcessCommandInfo{}, nil
	}
	isProcessFocused = func(processID uint32) bool { return false }
	getIdleDuration = func() (time.Duration, bool) { return 0, false }

	return func() {
		isBundlePathFocused = origBundleFocused
		getForegroundProcessID = origPID
		isProcessDescendant = origIsDescendant
		getDescendantProcesses = origDescendants
		isProcessPresent = origPresent
		getProcessCommandInfo = origProcessCommandInfo
		isProcessFocused = origFocused
		getIdleDuration = origIdle
	}
}

func TestShouldCountTickPausesAfterIdleThreshold(t *testing.T) {
	restore := stubFocusFunctions(t)
	defer restore()

	idle := time.Duration(0)
	getIdleDuration = func() (time.Duration, bool) { return idle, true }

	tracker := NewActiveTimeTracker(context.Background(), nil)
	tracker.SetIdlePauseEnabled(func() bool { return true })
	session := &TrackingSession{GameID: "game-1"}

	if !tracker.shouldCountTick(session, true) {
		t.Fatal("expected recent input to count")
	}
	idle = IdlePauseThreshold - time.Second
	if !tracker.shouldCountTick(session, true) {
		t.Fatal("expected idle below threshold to count")
	}
	idle = IdlePauseThreshold
	if tracker.shouldCountTick(session, true) || !session.idlePaused {
		t.Fatal("expected idle at threshold to pause")
	}
	idle = 0
	if !tracker.shouldCountTick(session, true) || session.idlePaused {
		t.Fatal("expected new input to resume")
	}
}

func TestShouldCountTickIgnoresIdleWhenDisabledOrUnsupported(t *testing.T) {
	restore := stubFocusFunctions(t)
	defer restore()

	tracker := NewActiveTimeTracker(context.Background(), nil)
	session := &TrackingSession{GameID: "game-1"}

	getIdleDuration = func() (time.Duration, bool) { return time.Hour, true }
	if !tracker.shouldCountTick(session, true) {
		t.Fatal("expected idle to be ignored without a setting")
	}
	tracker.SetIdlePauseEnabled(func() bool { return false })
	if !tracker.shouldCountTick(session, true) {
		t.Fatal("expected idle to be ignored when disabled")
	}

	tracker.SetIdlePauseEnabled(func() bool { return true })
	getIdleDuration = func() (time.Duration, bool) { return 0, false }
	if !tracker.shouldCountTick(session, true) {
		t.Fatal("expected unsupported idle detection to keep counting")
	}
	if tracker.shouldCountTick(session, false) {
		t.Fatal("expected unfocused tick not to count")
	}
}
