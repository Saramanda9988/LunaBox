package importpath

import "testing"

func TestCleanSearchName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"[110624][Navel] 俺たちに翼はない R -Limited Edition- (nrg)", "俺たちに翼はない R"},
		{"[110624][Navel]俺たちに翼はない_R", "俺たちに翼はない R"},
		{"【汉化】[ゆずソフト] 千恋＊万花 ch dl file", "千恋＊万花"},
		{"220930 Summer Pockets REFLECTION BLUE Ver.1.02", "Summer Pockets REFLECTION BLUE"},
		{"2022-09-30 Summer Pockets ver 1.02 chs", "Summer Pockets"},
		{"RJ01234567 某同人ゲーム 体験版", "某同人ゲーム"},
		{"WHITE ALBUM2 初回限定版（汉化硬盘版）", "WHITE ALBUM2"},
		{"ef - a fairy tale of the two.", "ef - a fairy tale of the two"},
		{"Steins;Gate 0", "Steins;Gate 0"},
		{"Fate／stay night [Réalta Nua] {DL版}", "Fate／stay night"},
		{"【俺たちに翼はない】汉化版", "俺たちに翼はない"},
		{"[Navel]", "Navel"},
		{"Game", "Game"},
		{"  ", ""},
	}
	for _, c := range cases {
		if got := CleanSearchName(c.in); got != c.want {
			t.Errorf("CleanSearchName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
