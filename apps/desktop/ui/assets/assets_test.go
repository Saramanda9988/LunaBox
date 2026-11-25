package assets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
)

// 所有来源图标都能解码（SVG 能解析、webp/png 能解码）。
func TestProviders(t *testing.T) {
	for _, s := range enums.MetadataSourceTypes {
		source := string(s)
		for _, compact := range []bool{false, true} {
			if Provider(source, compact) == nil {
				t.Errorf("Provider(%q, %v) = nil", source, compact)
			}
		}
	}
	if Provider("local", false) != nil {
		t.Error("local should have no icon")
	}
}

// 主题插件替换的品牌图片：SVG 按自身尺寸解析，缺失或损坏的文件返回 nil。
func TestLocalImage(t *testing.T) {
	dir := t.TempDir()
	svg := filepath.Join(dir, "mark.svg")
	if err := os.WriteFile(svg, []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 80 20"><rect width="80" height="20" fill="#39C5BB"/></svg>`), 0o644); err != nil {
		t.Fatal(err)
	}
	img := LocalImage(svg, 520)
	if img == nil {
		t.Fatal("LocalImage(svg) = nil")
	}
	if w, h := ui.ImageSize(img); w != 80 || h != 20 {
		t.Fatalf("svg size = %vx%v, want 80x20", w, h)
	}
	if LocalImage(filepath.Join(dir, "missing.png"), 96) != nil {
		t.Fatal("missing file should return nil")
	}
	bad := filepath.Join(dir, "bad.png")
	if err := os.WriteFile(bad, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if LocalImage(bad, 96) != nil {
		t.Fatal("undecodable file should return nil")
	}
}
