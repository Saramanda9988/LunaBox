package bar

import (
	"slices"
	"testing"

	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/env/envtest"
)

// 标签建议同 useTagGameFilter：库里名称匹配的标签，加上译名匹配、库里也有的标签，按原名或显示名过滤，去掉已选的。
func TestSearchTagsByTranslation(t *testing.T) {
	b := envtest.NewBackend(0)
	b.Tags = []string{"ADV", "AVG", "PC", "冒险岛", "Romance"}
	e := &env.Env{Backend: b}
	search := func(f *Filters, q string) []string {
		f.TagInput = q
		f.searchTags(e, q)
		return f.TagSuggestions
	}

	f := &Filters{}
	// “冒险游戏”是 AVG 与 ADV 的译名（按译名在数据里的顺序），“冒险岛”按原名匹配，排在前面
	if got, want := search(f, "冒险"), []string{"冒险岛", "AVG", "ADV"}; !slices.Equal(got, want) {
		t.Errorf("suggestions = %q, want %q", got, want)
	}
	f.Tags = []string{"AVG"}
	if got, want := search(f, "冒险"), []string{"冒险岛", "ADV"}; !slices.Equal(got, want) {
		t.Errorf("with AVG selected = %q, want %q", got, want)
	}
	if got, want := search(&Filters{}, "pc"), []string{"PC"}; !slices.Equal(got, want) {
		t.Errorf("raw match = %q, want %q", got, want)
	}

	b.Cfg.EnableTagTranslation = false
	if got, want := search(&Filters{}, "冒险"), []string{"冒险岛"}; !slices.Equal(got, want) {
		t.Errorf("translation off = %q, want %q", got, want)
	}
}
