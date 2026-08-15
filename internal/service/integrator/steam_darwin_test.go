//go:build darwin

package integrator

import (
	"lunabox/internal/models"
	"testing"
)

func TestNativeSteamLaunchIDAcceptsOnlyNativeAppID(t *testing.T) {
	tests := []struct {
		name string
		game models.Game
		want string
		ok   bool
	}{
		{name: "native", game: models.Game{SteamLaunchKind: "native", SteamLaunchID: "123456"}, want: "123456", ok: true},
		{name: "shortcut", game: models.Game{SteamLaunchKind: "shortcut", SteamLaunchID: "123456"}},
		{name: "empty kind", game: models.Game{SteamLaunchID: "123456"}},
		{name: "invalid app id", game: models.Game{SteamLaunchKind: "native", SteamLaunchID: "not-an-id"}},
		{name: "zero app id", game: models.Game{SteamLaunchKind: "native", SteamLaunchID: "0"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := nativeSteamLaunchID(test.game)
			if ok != test.ok || got != test.want {
				t.Fatalf("nativeSteamLaunchID(%+v) = %q, %v; want %q, %v", test.game, got, ok, test.want, test.ok)
			}
		})
	}
}
