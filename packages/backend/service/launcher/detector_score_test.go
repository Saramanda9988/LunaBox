package launcher

import (
	"errors"
	"lunabox/backend/utils/processutils"
	"math"
	"testing"
	"time"
)

const testDetectionTimeout = 60 * time.Second

type timelineFunc func(at time.Duration) []candidateSample

// runDetection replays a timeline at the detection sample rate and returns the
// first committed candidate with its commit time.
func runDetection(t *testing.T, scorer *processScorer, until time.Duration, timeline timelineFunc) (candidateSample, time.Duration, bool) {
	t.Helper()
	for at := time.Duration(0); at <= until; at += detectionSampleInterval {
		scorer.observe(at, timeline(at))
		if picked, ok := scorer.decide(); ok {
			return picked, at, true
		}
	}
	return candidateSample{}, 0, false
}

func proc(name string, pid uint32) processutils.ProcessInfo {
	return processutils.ProcessInfo{Name: name, PID: pid}
}

// win builds a Windows-style sample where window and foreground are observable.
func win(p processutils.ProcessInfo, links processLinks, window bool, foreground bool) candidateSample {
	return candidateSample{
		Process:         p,
		IsLauncher:      links.IsLauncher,
		SavedName:       links.SavedName,
		Relation:        links.relation(),
		Window:          window,
		WindowKnown:     true,
		Foreground:      foreground,
		ForegroundKnown: true,
	}
}

var (
	launcherInDir   = processLinks{IsLauncher: true, InLaunchDir: true}
	descendantInDir = processLinks{Descendant: true, InLaunchDir: true}
)

func TestEvidenceNormalisesOverObservableChannels(t *testing.T) {
	full := win(proc("game.exe", 1), descendantInDir, true, true)
	if got := full.evidence(); math.Abs(got-1) > 1e-9 {
		t.Fatalf("expected full Windows evidence 1, got %f", got)
	}
	structuralOnly := candidateSample{Process: proc("game", 1), Relation: 0.7}
	if got := structuralOnly.evidence(); math.Abs(got-0.7) > 1e-9 {
		t.Fatalf("expected relation-only evidence to equal the relation, got %f", got)
	}
	foregroundOnly := candidateSample{Process: proc("game", 1), Relation: 1, Foreground: true, ForegroundKnown: true}
	if got := foregroundOnly.evidence(); math.Abs(got-1) > 1e-9 {
		t.Fatalf("expected foreground+relation evidence 1 without window channel, got %f", got)
	}
	background := candidateSample{Process: proc("game", 1), Relation: 1, ForegroundKnown: true}
	if got := background.evidence(); got >= strictCommitThreshold {
		t.Fatalf("expected known background process to stay below the strict threshold, got %f", got)
	}
}

func TestSteamInstallDirIsPrimaryRelation(t *testing.T) {
	if got := (processLinks{InLaunchDir: true, DirIsPrimary: true}).relation(); got != relationDescendantInDir {
		t.Fatalf("expected install dir to be a primary link in Steam mode, got %f", got)
	}
	if got := (processLinks{InLaunchDir: true}).relation(); got != relationLaunchDir {
		t.Fatalf("expected launch dir to be a secondary link in staged mode, got %f", got)
	}
}

func TestScorerCommitsLauncherThatIsTheGameWithinSeconds(t *testing.T) {
	launcher := proc("game.exe", 100)
	scorer := newProcessScorer(testDetectionTimeout)
	picked, at, ok := runDetection(t, scorer, testDetectionTimeout, func(at time.Duration) []candidateSample {
		shown := at >= time.Second
		return []candidateSample{win(launcher, launcherInDir, shown, shown)}
	})
	if !ok || picked.Process.PID != launcher.PID {
		t.Fatalf("expected launcher to be committed, got %+v ok=%v", picked, ok)
	}
	if at < minDetectionObservation || at > 8*time.Second {
		t.Fatalf("expected commit within a few seconds, got %s", at)
	}
}

func TestScorerPrefersRealGameOverSameNameSplash(t *testing.T) {
	splash := proc("game.exe", 100)
	game := proc("game.exe", 200)
	scorer := newProcessScorer(testDetectionTimeout)
	picked, _, ok := runDetection(t, scorer, testDetectionTimeout, func(at time.Duration) []candidateSample {
		if at < 3*time.Second {
			shown := at >= 500*time.Millisecond
			return []candidateSample{win(splash, launcherInDir, shown, shown)}
		}
		return []candidateSample{win(game, descendantInDir, true, true)}
	})
	if !ok || picked.Process.PID != game.PID {
		t.Fatalf("expected re-executed game process to be committed, got %+v ok=%v", picked, ok)
	}
}

func TestScorerWaitsOutQuickLauncherExit(t *testing.T) {
	scorer := newProcessScorer(testDetectionTimeout)
	launcher := win(proc("launcher.exe", 100), processLinks{IsLauncher: true}, true, true)
	scorer.observe(0, []candidateSample{launcher})
	scorer.observe(time.Second, []candidateSample{launcher})
	if _, ok := scorer.decide(); ok {
		t.Fatal("expected no commit before the minimum observation time")
	}
}

func TestScorerDoesNotCommitAmbiguousBackgroundWindows(t *testing.T) {
	scorer := newProcessScorer(testDetectionTimeout)
	_, _, ok := runDetection(t, scorer, testDetectionTimeout, func(time.Duration) []candidateSample {
		return []candidateSample{
			win(proc("a.exe", 200), descendantInDir, true, false),
			win(proc("b.exe", 300), descendantInDir, true, false),
		}
	})
	if ok {
		t.Fatal("expected two equally strong candidates to stay undecided")
	}
	if _, ok := scorer.fallback(); ok {
		t.Fatal("expected tied candidates to have no fallback winner")
	}
}

func TestScorerOnlyCommitsOutOfDirDescendantInRelaxedPhase(t *testing.T) {
	scorer := newProcessScorer(testDetectionTimeout)
	picked, at, ok := runDetection(t, scorer, testDetectionTimeout, func(time.Duration) []candidateSample {
		return []candidateSample{
			win(proc("launcher.exe", 100), processLinks{IsLauncher: true}, false, false),
			win(proc("browser.exe", 200), processLinks{Descendant: true}, true, true),
		}
	})
	if !ok || picked.Process.PID != 200 {
		t.Fatalf("expected out-of-dir descendant to commit eventually, got %+v ok=%v", picked, ok)
	}
	if at < strictCommitPhase {
		t.Fatalf("expected out-of-dir descendant to wait for the relaxed phase, committed at %s", at)
	}
}

func TestScorerCommitsSoleSurvivorWithoutWindowAfterLauncherExit(t *testing.T) {
	scorer := newProcessScorer(testDetectionTimeout)
	picked, at, ok := runDetection(t, scorer, testDetectionTimeout, func(at time.Duration) []candidateSample {
		if at < 2*time.Second {
			return []candidateSample{win(proc("launcher.exe", 100), processLinks{IsLauncher: true}, false, false)}
		}
		return []candidateSample{win(proc("game.exe", 200), descendantInDir, false, false)}
	})
	if !ok || picked.Process.PID != 200 {
		t.Fatalf("expected sole surviving process to commit, got %+v ok=%v", picked, ok)
	}
	if at < 2*time.Second+soleSurvivorAge {
		t.Fatalf("expected sole survivor to be stable for %s first, committed at %s", soleSurvivorAge, at)
	}
}

func TestScorerCommitsSoleSteamProcessWithoutForeground(t *testing.T) {
	// macOS-style channels: no window enumeration, game started in the background.
	steamGame := processLinks{InLaunchDir: true, DirIsPrimary: true}
	scorer := newProcessScorer(testDetectionTimeout)
	picked, at, ok := runDetection(t, scorer, testDetectionTimeout, func(time.Duration) []candidateSample {
		return []candidateSample{{Process: proc("game.exe", 200), Relation: steamGame.relation(), ForegroundKnown: true}}
	})
	if !ok || picked.Process.PID != 200 {
		t.Fatalf("expected sole Steam game process to commit, got %+v ok=%v", picked, ok)
	}
	if at > soleSurvivorAge+time.Second {
		t.Fatalf("expected background Steam game to commit by the sole-survivor rule, got %s", at)
	}
}

func TestScorerScoreIsBoundedAndSampleRateIndependent(t *testing.T) {
	sample := []candidateSample{win(proc("game.exe", 100), processLinks{IsLauncher: true}, true, true)}
	scoreAt := func(step time.Duration) float64 {
		scorer := newProcessScorer(testDetectionTimeout)
		for at := step; at <= 10*time.Minute; at += step {
			scorer.observe(at, sample)
		}
		return scorer.score(100)
	}
	fast, slow := scoreAt(100*time.Millisecond), scoreAt(2*time.Second)
	if fast > 1 || slow > 1 {
		t.Fatalf("expected bounded scores, got fast=%f slow=%f", fast, slow)
	}
	if math.Abs(fast-slow) > 0.01 {
		t.Fatalf("expected sample-rate independent scores, got fast=%f slow=%f", fast, slow)
	}

	early := func(step time.Duration) float64 {
		scorer := newProcessScorer(testDetectionTimeout)
		scorer.observe(0, nil)
		for at := step; at <= 4*time.Second; at += step {
			scorer.observe(at, sample)
		}
		return scorer.score(100)
	}
	if diff := math.Abs(early(250*time.Millisecond) - early(time.Second)); diff > 0.01 {
		t.Fatalf("expected rising score to be sample-rate independent, diff=%f", diff)
	}
}

func TestCommitThresholdDecaysOverAbsoluteRelaxedPhase(t *testing.T) {
	want := (strictCommitThreshold + relaxedCommitThreshold) / 2
	for _, timeout := range []time.Duration{relaxedCommitPhase, 10 * time.Minute} {
		if got := commitThreshold(10*time.Second, timeout); got != strictCommitThreshold {
			t.Fatalf("timeout %s: expected strict threshold early, got %f", timeout, got)
		}
		mid := commitThreshold(strictCommitPhase+(relaxedCommitPhase-strictCommitPhase)/2, timeout)
		if math.Abs(mid-want) > 1e-9 {
			t.Fatalf("timeout %s: expected linear decay midpoint %f, got %f", timeout, want, mid)
		}
		for _, at := range []time.Duration{relaxedCommitPhase, timeout} {
			if got := commitThreshold(at, timeout); got != relaxedCommitThreshold {
				t.Fatalf("timeout %s: expected relaxed threshold at %s, got %f", timeout, at, got)
			}
		}
	}

	// A timeout shorter than the relaxed phase reaches the relaxed threshold at
	// the timeout itself.
	short := 45 * time.Second
	if got := commitThreshold(short, short); got != relaxedCommitThreshold {
		t.Fatalf("expected relaxed threshold at a short timeout, got %f", got)
	}
	if got := commitThreshold(20*time.Second, 5*time.Second); got != strictCommitThreshold {
		t.Fatalf("expected strict threshold when timeout is shorter than the strict phase, got %f", got)
	}
}

// A longer timeout must not delay commits for a process that is already
// present with moderate evidence (window but no foreground). The launcher stays
// open so the sole-survivor rule does not apply.
func TestScorerCommitTimeDoesNotDependOnLongTimeout(t *testing.T) {
	timeline := func(time.Duration) []candidateSample {
		return []candidateSample{
			win(proc("launcher.exe", 100), processLinks{IsLauncher: true}, false, false),
			win(proc("game.exe", 200), descendantInDir, true, false),
		}
	}
	commitAt := func(timeout time.Duration) time.Duration {
		t.Helper()
		picked, at, ok := runDetection(t, newProcessScorer(timeout), timeout, timeline)
		if !ok || picked.Process.PID != 200 {
			t.Fatalf("timeout %s: expected background game to commit, got %+v ok=%v", timeout, picked, ok)
		}
		return at
	}
	short, long := commitAt(testDetectionTimeout), commitAt(10*time.Minute)
	if short != long {
		t.Fatalf("expected the same commit time regardless of timeout, got %s (60s) vs %s (10m)", short, long)
	}
	if long < strictCommitPhase || long > relaxedCommitPhase {
		t.Fatalf("expected moderate evidence to commit during the relaxed phase, got %s", long)
	}
}

func TestChallengerRetargetsFromLauncherThatStaysOpen(t *testing.T) {
	launcher := proc("config.exe", 100)
	game := proc("game.exe", 200)
	scorer := newProcessScorer(testDetectionTimeout)
	committed, commitAt, ok := runDetection(t, scorer, testDetectionTimeout, func(time.Duration) []candidateSample {
		return []candidateSample{win(launcher, launcherInDir, true, true)}
	})
	if !ok || committed.Process.PID != launcher.PID {
		t.Fatalf("expected settings launcher to be committed first, got %+v ok=%v", committed, ok)
	}

	gameStart := 20 * time.Second
	var retargetAt time.Duration
	for at := commitAt + verificationSampleInterval; at <= testDetectionTimeout; at += verificationSampleInterval {
		samples := []candidateSample{win(launcher, launcherInDir, true, at < gameStart)}
		if at >= gameStart {
			samples = append(samples, win(game, descendantInDir, true, true))
		}
		scorer.observe(at, samples)
		if next, ok := scorer.challenger(launcher.PID); ok {
			if next.Process.PID != game.PID {
				t.Fatalf("expected game to take over, got %+v", next)
			}
			retargetAt = at
			break
		}
	}
	if retargetAt == 0 {
		t.Fatal("expected verification to retarget to the game process")
	}
	if retargetAt < gameStart+retargetSustain {
		t.Fatalf("expected challenger to be sustained for %s, retargeted at %s", retargetSustain, retargetAt)
	}
}

func TestChallengerNeverMovesBackToOlderProcess(t *testing.T) {
	settings := proc("settings.exe", 150)
	game := proc("game.exe", 200)
	scorer := newProcessScorer(testDetectionTimeout)
	for at := time.Duration(0); at <= testDetectionTimeout; at += verificationSampleInterval {
		gameFocused := at < 10*time.Second
		samples := []candidateSample{win(settings, descendantInDir, true, !gameFocused)}
		if at >= 2*time.Second {
			samples = append(samples, win(game, descendantInDir, true, gameFocused))
		}
		scorer.observe(at, samples)
		if next, ok := scorer.challenger(game.PID); ok {
			t.Fatalf("expected no retarget away from the game when the user clicks the launcher, got %+v at %s", next, at)
		}
	}
}

func TestChallengerIgnoresExitedTarget(t *testing.T) {
	scorer := newProcessScorer(testDetectionTimeout)
	for at := time.Duration(0); at <= 20*time.Second; at += verificationSampleInterval {
		scorer.observe(at, []candidateSample{win(proc("game.exe", 200), descendantInDir, true, true)})
		if _, ok := scorer.challenger(100); ok {
			t.Fatal("expected successor detection, not verification, to handle an exited target")
		}
	}
}

func TestProcessVerifierOfferReplacesUnconsumedTarget(t *testing.T) {
	verifier := newProcessVerifier(100)
	verifier.offer(proc("a.exe", 200))
	verifier.offer(proc("b.exe", 300))
	if got := verifier.Target(); got != 300 {
		t.Fatalf("expected latest target, got %d", got)
	}
	select {
	case next := <-verifier.Retargets():
		if next.PID != 300 {
			t.Fatalf("expected latest offer, got %+v", next)
		}
	default:
		t.Fatal("expected a pending retarget")
	}

	var nilVerifier *ProcessVerifier
	nilVerifier.SetTarget(1)
	if nilVerifier.Retargets() != nil || nilVerifier.Target() != 0 {
		t.Fatal("expected nil verifier to be inert")
	}
}

type staticSampler struct {
	samples []candidateSample
}

func (s *staticSampler) sample() ([]candidateSample, error) {
	return append([]candidateSample(nil), s.samples...), nil
}

func TestSessionCandidateFilterDropsProcessesStartedBeforeLaunch(t *testing.T) {
	sessionStart := time.Now()
	created := map[uint32]time.Time{
		100: sessionStart.Add(-time.Hour),   // launcher: always kept
		200: sessionStart.Add(-time.Hour),   // stale tool from the game dir
		300: sessionStart.Add(-time.Second), // within tolerance
		400: sessionStart.Add(time.Second),  // started by the launch
		500: sessionStart.Add(-time.Hour),   // saved game name: always kept
	}
	lookups := 0
	inner := &staticSampler{samples: []candidateSample{
		win(proc("game.exe", 100), launcherInDir, true, true),
		win(proc("tool.exe", 200), processLinks{InLaunchDir: true}, true, false),
		win(proc("splash.exe", 300), descendantInDir, true, false),
		win(proc("real.exe", 400), descendantInDir, true, false),
		win(proc("saved.exe", 500), processLinks{SavedName: true}, false, false),
		win(proc("elevated.exe", 600), descendantInDir, true, false), // creation time unreadable
	}}
	filter := newSessionCandidateFilter(inner, StagedProcessDetectionInput{SessionStart: sessionStart}, nil).(*sessionCandidateFilter)
	filter.creationTime = func(pid uint32) (time.Time, error) {
		lookups++
		if at, ok := created[pid]; ok {
			return at, nil
		}
		return time.Time{}, errors.New("access denied")
	}

	for tick := 0; tick < 3; tick++ {
		samples, err := filter.sample()
		if err != nil {
			t.Fatalf("unexpected sample error: %v", err)
		}
		kept := map[uint32]bool{}
		for _, sample := range samples {
			kept[sample.Process.PID] = true
		}
		for _, pid := range []uint32{100, 300, 400, 500, 600} {
			if !kept[pid] {
				t.Fatalf("tick %d: expected PID %d to be kept, got %+v", tick, pid, samples)
			}
		}
		if kept[200] {
			t.Fatalf("tick %d: expected stale PID 200 to be dropped", tick)
		}
	}
	if lookups != 4 {
		t.Fatalf("expected one creation-time lookup per non-exempt process, got %d", lookups)
	}

	// A reused PID with a different name is re-evaluated.
	inner.samples = []candidateSample{win(proc("other.exe", 200), descendantInDir, true, false)}
	created[200] = sessionStart.Add(time.Second)
	samples, _ := filter.sample()
	if len(samples) != 1 || samples[0].Process.Name != "other.exe" {
		t.Fatalf("expected reused PID to be re-evaluated and kept, got %+v", samples)
	}
}

func TestSessionCandidateFilterIsDisabledWithoutSessionStart(t *testing.T) {
	inner := &staticSampler{}
	if got := newSessionCandidateFilter(inner, StagedProcessDetectionInput{}, nil); got != candidateSampler(inner) {
		t.Fatalf("expected filter to be skipped without a session start, got %T", got)
	}
}
