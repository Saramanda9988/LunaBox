package metadata

import (
	"os"
	"testing"
)

func TestNormalizeErogameScapeID(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{name: "numeric", raw: "12345", want: "12345", ok: true},
		{name: "trim leading zeros", raw: "0012345", want: "12345", ok: true},
		{name: "relative url", raw: "game.php?game=67890#ad", want: "67890", ok: true},
		{name: "absolute url", raw: "https://erogamescape.org/~ap2/ero/toukei_kaiseki/game.php?game=24680", want: "24680", ok: true},
		{name: "invalid text", raw: "game-12345", ok: false},
		{name: "zero", raw: "0", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NormalizeErogameScapeID(tt.raw)
			if ok != tt.ok {
				t.Fatalf("expected ok=%v, got %v", tt.ok, ok)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestErogameScapeNormalizeRatingAndDate(t *testing.T) {
	if got := parseErogameScapeRating("中央値 82"); got != 8.2 {
		t.Fatalf("expected rating 8.2, got %v", got)
	}
	if got := parseErogameScapeRating("7.5"); got != 7.5 {
		t.Fatalf("expected rating 7.5, got %v", got)
	}
	if got := normalizeErogameScapeDate("2024年5月24日"); got != "2024-05-24" {
		t.Fatalf("expected date 2024-05-24, got %q", got)
	}
}

func TestParseErogameScapeSearchDocument(t *testing.T) {
	doc := mustGoqueryDocument(t, `
<table id="result">
  <tr><th>ゲーム名</th><th>ブランド名</th><th>発売日</th></tr>
  <tr>
    <td><a href="game.php?game=12345#ad">Sample Game</a><span> 初回版</span></td>
    <td><a>Brand</a></td>
    <td>2024-05-24</td>
  </tr>
</table>`)

	items := parseErogameScapeSearchDocument(doc)
	if len(items) != 1 {
		t.Fatalf("expected one search result, got %#v", items)
	}
	if items[0].ID != "12345" || items[0].Name != "Sample Game 初回版" {
		t.Fatalf("unexpected item: %#v", items[0])
	}
}

func TestParseErogameScapeMetadataDocument(t *testing.T) {
	doc := mustGoqueryDocument(t, `
<html>
  <body>
    <div id="soft-title"><span>[18禁]</span><span class="bold">Sample EGS Game</span></div>
    <table>
      <tr id="brand"><td>Brand Name</td></tr>
      <tr id="sellday"><td>2024年5月24日</td></tr>
      <tr id="median"><td>82</td></tr>
      <tr id="erogame"><td>18禁 抜きゲー 和姦もの</td></tr>
    </table>
    <div id="main_image"><img src="/images/sample.jpg"></div>
    <table id="att_pov_table">
      <tr><th>ジャンル</th><td><a>ADV</a></td></tr>
      <tr><th>タグ</th><td><a>純愛</a><a>ADV</a></td></tr>
    </table>
  </body>
</html>`)

	result, err := parseErogameScapeMetadataDocument(doc, "12345")
	if err != nil {
		t.Fatalf("parseErogameScapeMetadataDocument failed: %v", err)
	}

	if result.Game.Name != "Sample EGS Game" {
		t.Fatalf("unexpected name: %q", result.Game.Name)
	}
	if result.Game.Company != "Brand Name" {
		t.Fatalf("unexpected company: %q", result.Game.Company)
	}
	if result.Game.ReleaseDate != "2024-05-24" {
		t.Fatalf("unexpected release date: %q", result.Game.ReleaseDate)
	}
	if result.Game.Rating != 8.2 {
		t.Fatalf("unexpected rating: %v", result.Game.Rating)
	}
	if result.Game.CoverURL != erogamescapeBaseURL+"/images/sample.jpg" {
		t.Fatalf("unexpected cover URL: %q", result.Game.CoverURL)
	}
	if len(result.Tags) != 5 {
		t.Fatalf("expected 5 tags after de-duplication, got %#v", result.Tags)
	}
}

func TestKokoErogameScapeMirrorLive(t *testing.T) {
	if os.Getenv("RUN_KOKO_EROGAMESCAPE_TEST") != "1" {
		t.Skip("set RUN_KOKO_EROGAMESCAPE_TEST=1 to test the koko.kyara.top live mirror")
	}

	query := os.Getenv("KOKO_EROGAMESCAPE_QUERY")
	if query == "" {
		query = "サクラノ詩"
	}

	getter := newErogameScapeInfoGetterWithBaseURL("https://koko.kyara.top")
	result, err := getter.FetchMetadataByName(query, "")
	if err != nil {
		t.Fatalf("FetchMetadataByName(%q) via koko mirror failed: %v", query, err)
	}
	if result.Game.Name == "" {
		t.Fatal("expected non-empty game name")
	}
	if result.Game.SourceID == "" {
		t.Fatal("expected non-empty source id")
	}
	if result.Game.SourceType != "erogamescape" {
		t.Fatalf("expected source type erogamescape, got %q", result.Game.SourceType)
	}

	t.Logf(
		"koko mirror scraped source_id=%s name=%q brand=%q release_date=%q rating=%v tags=%d cover=%q",
		result.Game.SourceID,
		result.Game.Name,
		result.Game.Company,
		result.Game.ReleaseDate,
		result.Game.Rating,
		len(result.Tags),
		result.Game.CoverURL,
	)
}
