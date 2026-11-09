package metasource_test

import (
	"slices"
	"testing"

	"lunabox/backend/common/enums"
	"lunabox/desktop/ui/assets"
	"lunabox/desktop/ui/metasource"
)

// 新增来源时只改 enums.MetadataSourceTypes 会在这里失败，提醒补上界面信息。
func TestEverySourceComplete(t *testing.T) {
	for _, s := range enums.MetadataSourceTypes {
		src, ok := metasource.Get(s)
		if !ok {
			t.Errorf("%s: 没有界面信息，请在 metasource.sources 里补上", s)
			continue
		}
		if src.Label == nil || src.Hint == nil || src.URL == nil || src.Logo == "" {
			t.Errorf("%s: 信息不完整 %+v", s, src)
		}
		for _, compact := range []bool{false, true} {
			if _, err := assets.ProviderFile(metasource.LogoFile(s, compact)); err != nil {
				t.Errorf("%s: 图标文件不存在: %v", s, err)
			}
		}
		if metasource.URL(s, "1", "") == "" {
			t.Errorf("%s: 没有链接", s)
		}
	}
	if metasource.Known(enums.Local) {
		t.Error("local 不是元数据来源")
	}
}

func TestOrdered(t *testing.T) {
	// GameEditPanel.tsx 的 metadataSourceTypes。
	want := []enums.SourceType{
		enums.Bangumi, enums.VNDB, enums.Ymgal, enums.Steam, enums.DLsite, enums.TouchGal,
		enums.Hikarinagi, enums.Kungal, enums.ErogameScape,
	}
	if got := metasource.Ordered(enums.Bangumi, enums.VNDB, enums.Ymgal, enums.Steam); !slices.Equal(got, want) {
		t.Errorf("Ordered = %v, want %v", got, want)
	}
}

func TestURL(t *testing.T) {
	for _, tc := range []struct {
		source enums.SourceType
		id     string
		want   string
	}{
		{enums.Kungal, "42", "https://www.kungal.com/galgame/42"},
		{enums.DLsite, " RJ01 ", "https://www.dlsite.com/maniax/work/=/product_id/RJ01.html"},
		{enums.ErogameScape, "a b", "https://erogamescape.org/~ap2/ero/toukei_kaiseki/game.php?game=a%20b"},
		{enums.Local, "x", ""},
		{enums.VNDB, "  ", ""},
	} {
		if got := metasource.URL(tc.source, tc.id, ""); got != tc.want {
			t.Errorf("URL(%s, %q) = %q, want %q", tc.source, tc.id, got, tc.want)
		}
	}
}
