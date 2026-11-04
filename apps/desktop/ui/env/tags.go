package env

import (
	"slices"
	"strings"

	"lunabox/desktop/ui/tagname"
)

// SearchTags 对应 useTagGameFilter 的标签建议：库里名称匹配的标签，加上译名匹配、库里也有的标签
// （enable_tag_translation 时），按原名或显示名过滤，去掉 exclude 里的，最多 50 个。查询在后台执行，
// done 在主线程上收到结果（出错时为空）；调用方自己丢弃过期的结果。
func (e *Env) SearchTags(query string, exclude []string, done func([]string)) {
	q := strings.TrimSpace(query)
	if q == "" || e.Backend == nil {
		done(nil)
		return
	}
	translate := tagname.Enabled(e.Config())
	var translated []string
	if translate {
		translated = tagname.FindRaw(q) // 按界面语言，在主线程取
	}
	e.Async(func() func() {
		names, err := e.Backend.SearchTagsInLibrary(q)
		var existing []string
		if err == nil && len(translated) > 0 {
			existing, err = e.Backend.FilterExistingTagNames(translated)
		}
		return func() {
			if err != nil {
				done(nil)
				return
			}
			var merged []string
			for _, n := range append(names, existing...) {
				if !slices.Contains(merged, n) {
					merged = append(merged, n)
				}
			}
			var out []string
			for _, n := range tagname.FilterByDisplay(merged, q, translate) {
				if !slices.Contains(exclude, n) {
					out = append(out, n)
				}
			}
			done(out[:min(len(out), 50)])
		}
	})
}
