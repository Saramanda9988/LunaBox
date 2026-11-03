package locales_test

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/i18n"

	"lunabox/desktop-mygo/tools/i18n/catalogs"
	"lunabox/desktop-mygo/ui/locales"
)

// 原版前端的语言包改了而没有重新生成时失败：两个前端要说同样的话。
func TestCatalogsMatchFrontend(t *testing.T) {
	res, err := catalogs.Convert(catalogs.Options{
		Frontend: filepath.Join("..", "..", "..", "desktop", "frontend", "src", "locales"),
		Extra:    filepath.Join("..", "..", "tools", "i18n", "extra"),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range res.Warnings {
		t.Log(w)
	}
	if diff := res.Diff("."); len(diff) > 0 {
		t.Fatalf("ui/locales 与原版语言包不一致：%v；在 apps/desktop-mygo 下运行 go run ./tools/i18n", diff)
	}
}

func TestMessages(t *testing.T) {
	for _, c := range []struct{ locale, want string }{
		{"", "确定"}, {"zh-CN", "确定"}, {"zh-TW", "確定"}, {"en-US", "Confirm"}, {"ja-JP", "確認"},
	} {
		if got := locales.In(c.locale).CommonConfirm(); got != c.want {
			t.Errorf("%q: CommonConfirm = %q, want %q", c.locale, got, c.want)
		}
	}
	// 插值与按原版 key 查找。
	if got, want := locales.In("en-US").Key("settings.metadata.idPreview.loading"), locales.In("en-US").SettingsMetadataIdPreviewLoadingText(); got != want {
		t.Errorf("Key with rename = %q, want %q", got, want)
	}
	if got := locales.Key("no.such.key"); got != "no.such.key" {
		t.Errorf("missing key = %q", got)
	}
	// 跟随应用的语言。
	i18n.SetLocaleFunc(func() string { return "en-US" })
	defer i18n.SetLocaleFunc(nil)
	if got := locales.CommonCancel(); got != "Cancel" {
		t.Errorf("app locale en-US: CommonCancel = %q", got)
	}
}
