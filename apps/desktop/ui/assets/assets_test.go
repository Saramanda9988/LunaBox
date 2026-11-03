package assets

import "testing"

// 所有来源图标都能解码（SVG 能解析、webp/png 能解码）。
func TestProviders(t *testing.T) {
	for source := range providerFiles {
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
