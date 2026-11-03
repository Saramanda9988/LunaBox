package tagname

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/i18n"
)

func TestDataMatchesFrontend(t *testing.T) {
	for _, lang := range Languages {
		want, err := os.ReadFile(filepath.Join("..", "..", "..", "desktop", "frontend", "src", "utils", "vndbTagTranslations."+lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		have, err := data.ReadFile("data/" + lang + ".json")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(want, have) {
			t.Errorf("data/%s.json 与原版不一致；在 apps/desktop-mygo 下运行 go run ./tools/i18n", lang)
		}
	}
}

func TestLanguage(t *testing.T) {
	for locale, want := range map[string]string{
		"": "zh-CN", "zh-CN": "zh-CN", "zh": "zh-CN", "zh-TW": "zh-TW", "zh-HK": "zh-TW", "zh-Hant-TW": "zh-TW",
		"ja-JP": "ja-JP", "en-US": "", "en-XA": "", "fr-FR": "",
	} {
		if got := language(locale); got != want {
			t.Errorf("language(%q) = %q, want %q", locale, got, want)
		}
	}
}

// withLocale 在 locale 下运行 fn（测试里应用没有运行，i18n.Locale() 默认为空，同 zh-CN）。
func withLocale(t *testing.T, locale string, fn func()) {
	t.Helper()
	i18n.SetLocaleFunc(func() string { return locale })
	defer i18n.SetLocaleFunc(nil)
	fn()
}

func TestDisplay(t *testing.T) {
	cases := []struct{ locale, name, want string }{
		{"zh-CN", "ADV", "冒险游戏（ADV）"},
		{"zh-CN", "Romance", "恋爱"},
		{"zh-CN", "汉化", "汉化"}, // 没有译名
		{"zh-TW", "Romance", "戀愛"},
		{"ja-JP", "Romance", "ロマンス"},
		{"en-US", "Romance", "Romance"},
	}
	for _, c := range cases {
		withLocale(t, c.locale, func() {
			if got := Display(c.name, true); got != c.want {
				t.Errorf("%s: Display(%q) = %q, want %q", c.locale, c.name, got, c.want)
			}
		})
	}
	if got := Display("ADV", false); got != "ADV" {
		t.Errorf("disabled: %q", got)
	}
	if got := Display("_attribution", true); got != "_attribution" {
		t.Errorf("_attribution: %q", got)
	}
}

// 期望值由原版 tagTranslation.ts 的算法在同一份数据上算出。
func TestFindRaw(t *testing.T) {
	withLocale(t, "zh-CN", func() {
		if got, want := FindRaw(" 冒险游戏 "), []string{"AVG", "Point-and-Click Adventure Game", "ADV"}; !slices.Equal(got, want) {
			t.Errorf("FindRaw(冒险游戏) = %q, want %q", got, want)
		}
		if got, want := FindRaw("纯爱"), []string{"Pure Love Story"}; !slices.Equal(got, want) {
			t.Errorf("FindRaw(纯爱) = %q, want %q", got, want)
		}
		if got := FindRaw("  "); got != nil {
			t.Errorf("empty query: %q", got)
		}
	})
	withLocale(t, "ja-JP", func() {
		got := FindRaw("ロマンス")
		if want := []string{"Romance", "Boy x Boy Romance", "Girl x Girl Romance", "Incest Romance"}; len(got) < 4 || !slices.Equal(got[:4], want) {
			t.Errorf("FindRaw(ロマンス) = %q…", got[:min(4, len(got))])
		}
	})
	withLocale(t, "en-US", func() {
		if got := FindRaw("romance"); got != nil {
			t.Errorf("en-US: %q", got)
		}
	})
}

func TestFilterByDisplay(t *testing.T) {
	withLocale(t, "zh-CN", func() {
		names := []string{"ADV", "PC", "Romance", "冒险"}
		if got, want := FilterByDisplay(names, "冒险", true), []string{"ADV", "冒险"}; !slices.Equal(got, want) {
			t.Errorf("translated: %q, want %q", got, want)
		}
		if got, want := FilterByDisplay(names, "冒险", false), []string{"冒险"}; !slices.Equal(got, want) {
			t.Errorf("disabled: %q, want %q", got, want)
		}
		if got, want := FilterByDisplay(names, "pc", true), []string{"PC"}; !slices.Equal(got, want) {
			t.Errorf("raw: %q, want %q", got, want)
		}
		if got := FilterByDisplay(names, "", true); !slices.Equal(got, names) {
			t.Errorf("empty: %q", got)
		}
	})
}
