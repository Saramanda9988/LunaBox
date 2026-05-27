package metadata

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestNormalizeDLsiteID(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{name: "uppercase RJ", raw: "RJ123456", want: "RJ123456", ok: true},
		{name: "lowercase RE", raw: "re654321", want: "RE654321", ok: true},
		{name: "url VJ", raw: "https://www.dlsite.com/pro/work/=/product_id/VJ001234.html", want: "VJ001234", ok: true},
		{name: "invalid prefix", raw: "AB123456", ok: false},
		{name: "too short", raw: "RJ123", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NormalizeDLsiteID(tt.raw)
			if ok != tt.ok {
				t.Fatalf("expected ok=%v, got %v", tt.ok, ok)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestNormalizeDLsiteDate(t *testing.T) {
	tests := map[string]string{
		"2024年5月24日": "2024-05-24",
		"2024/05/24": "2024-05-24",
		"2024-5-4":   "2024-05-04",
		"":           "",
	}

	for raw, want := range tests {
		if got := normalizeDLsiteDate(raw); got != want {
			t.Fatalf("normalizeDLsiteDate(%q) expected %q, got %q", raw, want, got)
		}
	}
}

func TestParseDLsiteMetadataDocument(t *testing.T) {
	doc := mustGoqueryDocument(t, `
<html>
  <body>
    <h1 id="work_name"> Sample DLsite Game </h1>
    <span class="maker_name"><a> Circle Name </a></span>
    <div itemprop="description"> Line 1
      <br> Line 2
    </div>
    <table>
      <tr><th>販売日</th><td>2024年5月24日</td></tr>
      <tr><th>作品形式</th><td><a>ADV</a><a>音声あり</a></td></tr>
    </table>
    <div class="main_genre"><a>萌え</a><a>ADV</a></div>
    <img data-src="//img.dlsite.jp/modpub/images2/work/doujin/RJ123000/RJ123456_img_main.jpg">
  </body>
</html>`)

	result, err := parseDLsiteMetadataDocument(doc, "RJ123456")
	if err != nil {
		t.Fatalf("parseDLsiteMetadataDocument failed: %v", err)
	}

	if result.Game.Name != "Sample DLsite Game" {
		t.Fatalf("unexpected name: %q", result.Game.Name)
	}
	if result.Game.Company != "Circle Name" {
		t.Fatalf("unexpected company: %q", result.Game.Company)
	}
	if result.Game.ReleaseDate != "2024-05-24" {
		t.Fatalf("unexpected release date: %q", result.Game.ReleaseDate)
	}
	if result.Game.CoverURL != "https://img.dlsite.jp/modpub/images2/work/doujin/RJ123000/RJ123456_img_main.jpg" {
		t.Fatalf("unexpected cover URL: %q", result.Game.CoverURL)
	}
	if len(result.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %#v", result.Tags)
	}
}

func mustGoqueryDocument(t *testing.T, html string) *goquery.Document {
	t.Helper()

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}
	return doc
}
