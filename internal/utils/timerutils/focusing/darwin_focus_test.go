//go:build darwin

package focusing

import "testing"

func TestSameBundlePathNormalizesCaseAndTrailingSlash(t *testing.T) {
	if !sameBundlePath("/Applications/Game.app/", "/applications/game.app") {
		t.Fatalf("expected bundle paths to match after normalization")
	}
}

func TestSameBundlePathRejectsEmptyPath(t *testing.T) {
	if sameBundlePath("", "/Applications/Game.app") {
		t.Fatalf("expected empty bundle path not to match")
	}
}
