package env

import (
	"slices"
	"strings"

	"lunabox/desktop/ui/tagname"
)

// SearchTags 对应 useTagGameFilter 的标签建议：库里名称匹配的标签，加上译名匹配、库里也有的标签
// （enable_tag_translation 时），按原名或显示名过滤，去掉 exclude 里的，最多 50 个。只差大小写的名称算同一个标签
// （筛选也不区分大小写）。查询在后台执行，done 在主线程上收到结果（出错时为空）；调用方自己丢弃过期的结果。
func (e *Env) SearchTags(query string, exclude []string, done func([]string)) {
	q := strings.TrimSpace(query)
	if q == "" || e.Backend == nil {
		done(nil)
		return
	}
	mode := tagname.ModeOf(e.Config())
	var translated []string
	if mode != tagname.Original {
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
			containsFold := func(list []string, s string) bool {
				return slices.ContainsFunc(list, func(v string) bool { return strings.EqualFold(v, s) })
			}
			var merged []string
			for _, n := range append(names, existing...) {
				if !containsFold(merged, n) {
					merged = append(merged, n)
				}
			}
			var out []string
			for _, n := range tagname.FilterByDisplay(merged, q, mode) {
				if !containsFold(exclude, n) {
					out = append(out, n)
				}
			}
			done(out[:min(len(out), 50)])
		}
	})
}
