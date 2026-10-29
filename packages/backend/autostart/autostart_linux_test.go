//go:build linux

package autostart

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecPathRoundTrip(t *testing.T) {
	for _, exe := range []string{
		"/usr/bin/lunabox",
		"/home/u/My Games/LunaBox/LunaBox",
		`/opt/a"b$c\d` + "`e",
		"/opt/100%free/lunabox",
	} {
		if got := execPathOf(desktopEntry(exe)); got != exe {
			t.Errorf("execPathOf(desktopEntry(%q)) = %q", exe, got)
		}
	}
}

func TestQuoteExec(t *testing.T) {
	cases := map[string]string{
		"/usr/bin/lunabox":   "/usr/bin/lunabox",
		"/a b/c":             `"/a b/c"`,
		`/a"b`:               `"/a\"b"`,
		"/100%f/lunabox":     "/100%%f/lunabox",
		"/x y/100%u/lunabox": `"/x y/100%%u/lunabox"`,
	}
	for in, want := range cases {
		if got := quoteExec(in); got != want {
			t.Errorf("quoteExec(%q) = %q, want %q", in, got, want)
		}
	}
}

// 与 Wails 写出的 Exec（引号内单层转义）互认。
func TestExecPathOfWailsEntry(t *testing.T) {
	entry := "[Desktop Entry]\nType=Application\nName=LunaBox\nExec=\"/home/u/My Games/LunaBox\" --autostart\n"
	if got := execPathOf(entry); got != "/home/u/My Games/LunaBox" {
		t.Errorf("execPathOf = %q", got)
	}
}

// 关闭时 lunabox.desktop 指向别处（程序挪过位置）也要删掉。
func TestSetDisableRemovesMovedEntry(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	dir := filepath.Join(cfg, "autostart")
	path := filepath.Join(dir, autostartFileName)

	if err := Set(true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := os.WriteFile(path, []byte(desktopEntry("/old/place/LunaBox")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Set(false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("disable left %s behind (err=%v)", autostartFileName, err)
	}
}

func TestRemoveOwn(t *testing.T) {
	dir := t.TempDir()
	exe := "/opt/lunabox/LunaBox"
	write := func(name, exe string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(desktopEntry(exe)), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	keep := write(autostartFileName, exe)
	stale := write("lunabox-old.desktop", exe)
	other := write("other.desktop", "/usr/bin/other")

	if err := removeOwn(dir, exe, keep); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]bool{keep: true, stale: false, other: true} {
		if _, err := os.Stat(path); (err == nil) != want {
			t.Errorf("%s exists = %v, want %v", filepath.Base(path), err == nil, want)
		}
	}
}
