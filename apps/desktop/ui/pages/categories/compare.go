package categories

import (
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// localeCompare 对应 String.prototype.localeCompare：中文按拼音排序。多音字的读音可能与 Chromium 自带的 ICU 不同
// （如“似”），这类名称的先后会和原版不一样。
func localeCompare(locale, a, b string) int {
	return collate.New(language.Make(locale)).CompareString(a, b)
}
