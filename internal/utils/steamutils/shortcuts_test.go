package steamutils

import (
	"reflect"
	"testing"
)

func TestSteamShortcutsVDFRoundTrip(t *testing.T) {
	original := []binaryVDFEntry{
		binaryVDFObjectEntry("shortcuts", []binaryVDFEntry{
			binaryVDFObjectEntry("0", []binaryVDFEntry{
				binaryVDFIntEntry("appid", 0x81234567),
				binaryVDFStringEntry("AppName", "Existing Game"),
				binaryVDFStringEntry("Exe", `"C:\Games\Existing\game.exe"`),
				binaryVDFObjectEntry("tags", []binaryVDFEntry{
					binaryVDFStringEntry("0", "Favorite"),
				}),
			}),
		}),
	}

	encoded, err := encodeBinaryVDF(original)
	if err != nil {
		t.Fatalf("encode binary VDF: %v", err)
	}
	decoded, err := parseBinaryVDF(encoded)
	if err != nil {
		t.Fatalf("parse binary VDF: %v", err)
	}
	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("round trip changed entries:\nwant: %#v\ngot:  %#v", original, decoded)
	}
}

func TestAppendSteamShortcutProducesLaunchableIdentity(t *testing.T) {
	const executable = `C:\Games\Sample\sample.exe`
	entries, appID, err := appendSteamShortcut(nil, "Sample Game", executable, "LANG=zh_CN.UTF-8 %command%")
	if err != nil {
		t.Fatalf("append Steam shortcut: %v", err)
	}

	encoded, err := encodeBinaryVDF(entries)
	if err != nil {
		t.Fatalf("encode Steam shortcuts: %v", err)
	}
	decoded, err := parseBinaryVDF(encoded)
	if err != nil {
		t.Fatalf("parse Steam shortcuts: %v", err)
	}

	launchID := steamShortcutLongID(appID)
	foundAppID, found := findSteamShortcut(decoded, executable, launchID)
	if !found {
		t.Fatal("new shortcut was not found by executable and launch ID")
	}
	if foundAppID != appID {
		t.Fatalf("shortcut app ID changed: want %d, got %d", appID, foundAppID)
	}
	if parsedAppID, ok := steamShortcutAppIDFromLongID(launchID); !ok || parsedAppID != appID {
		t.Fatalf("invalid long launch ID %q", launchID)
	}
}

func TestSetSteamShortcutLaunchOptions(t *testing.T) {
	const executable = `C:\Games\Sample\sample.exe`
	file := NewShortcutFile()
	appID, err := file.Add("Sample Game", executable, "")
	if err != nil {
		t.Fatalf("add shortcut: %v", err)
	}

	updatedAppID, updated := file.SetLaunchOptions(executable, steamShortcutLongID(appID), "LANG=zh_CN.UTF-8 %command%")
	if !updated || updatedAppID != appID {
		t.Fatalf("expected launch options update for app %d, got %d (%v)", appID, updatedAppID, updated)
	}

	encoded, err := file.MarshalBinary()
	if err != nil {
		t.Fatalf("encode shortcuts: %v", err)
	}
	decoded, err := parseBinaryVDF(encoded)
	if err != nil {
		t.Fatalf("parse shortcuts: %v", err)
	}
	_, shortcut, found := findSteamShortcutEntry(decoded, executable, steamShortcutLongID(appID))
	if !found || shortcut == nil {
		t.Fatal("updated shortcut was not found")
	}
	launchOptions := binaryVDFEntryByKey(shortcut.Children, "LaunchOptions")
	if launchOptions == nil || launchOptions.String != "LANG=zh_CN.UTF-8 %command%" {
		t.Fatalf("unexpected LaunchOptions entry: %#v", launchOptions)
	}
}

func TestParseBinaryVDFRejectsTruncatedFile(t *testing.T) {
	_, err := parseBinaryVDF([]byte{binaryVDFObject, 'x', 0, binaryVDFString, 'y', 0})
	if err == nil {
		t.Fatal("expected a truncated binary VDF error")
	}
}

func TestFindSteamShortcutValidatesStoredLaunchIDAgainstExecutable(t *testing.T) {
	entries := []binaryVDFEntry{
		binaryVDFObjectEntry("shortcuts", []binaryVDFEntry{
			binaryVDFObjectEntry("0", []binaryVDFEntry{
				binaryVDFIntEntry("appid", 0x81234567),
				binaryVDFStringEntry("Exe", `"C:\Games\Existing\game.exe" --launcher`),
			}),
		}),
	}
	launchID := steamShortcutLongID(0x81234567)

	if _, found := findSteamShortcut(entries, `C:\Games\Existing\game.exe`, launchID); !found {
		t.Fatal("quoted executable with launch arguments was not recognized")
	}
	if _, found := findSteamShortcut(entries, `C:\Games\Other\game.exe`, launchID); found {
		t.Fatal("stored launch ID matched after the executable changed")
	}
}
