// Package tagname 对应原版 frontend/src/utils/tagTranslation.ts：VNDB 标签按界面语言显示译名，
// 以及用译名搜索标签。译名数据是原版的 utils/vndbTagTranslations.<语言>.json（MIT，来源见文件里的
// _attribution），由 tools/i18n 原样复制到 data/，TestDataMatchesFrontend 检查两边一致。
//
// 英语界面不翻译；日语用 ja-JP，繁体中文（zh-TW、zh-HK、zh-MO、zh-Hant）用 zh-TW，其余中文用 zh-CN。
package tagname

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/egoist/mygo/i18n"

	"lunabox/backend/appconf"
)

//go:embed data/*.json
var data embed.FS

// Languages 是有译名数据的语言，data/<语言>.json。
var Languages = []string{"ja-JP", "zh-CN", "zh-TW"}

// table 是一种语言的译名：names 是原名到译名；translated 是规范化的译名到原名，按译名在文件里第一次出现的顺序
// （原版 normalizedTranslatedIndexesByLanguage 是 Map，按插入顺序遍历）。
type table struct {
	names      map[string]string
	translated []string
	raw        map[string][]string
}

var (
	mu     sync.Mutex
	tables = map[string]*table{}
)

// language 对应 getCurrentTagTranslationLanguage。locale 为空（应用没有运行，如测试）时同源语言 zh-CN。
func language(locale string) string {
	switch {
	case locale == "":
		return "zh-CN"
	case strings.HasPrefix(locale, "en"):
		return ""
	case strings.HasPrefix(locale, "ja"):
		return "ja-JP"
	case locale == "zh-TW" || locale == "zh-HK" || locale == "zh-MO" || strings.HasPrefix(strings.ToLower(locale), "zh-hant"):
		return "zh-TW"
	case strings.HasPrefix(locale, "zh"):
		return "zh-CN"
	}
	return ""
}

// current 返回界面语言的译名，没有时为 nil。数据在第一次用到时解析。
func current() *table {
	lang := language(i18n.Locale())
	if lang == "" {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	if tb, ok := tables[lang]; ok {
		return tb
	}
	tb, err := load(lang)
	if err != nil {
		panic(err) // 数据随程序嵌入，解析失败是构建问题
	}
	tables[lang] = tb
	return tb
}

// load 按文件里的顺序读取译名，跳过 _ 开头的键与不是字符串的值（createTranslationMap）。
func load(lang string) (*table, error) {
	b, err := data.ReadFile("data/" + lang + ".json")
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, fmt.Errorf("tagname: data/%s.json 不是对象", lang) // i18n:keep：给开发者看的错误
	}
	tb := &table{names: map[string]string{}, raw: map[string][]string{}}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := tok.(string)
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		var name string
		if strings.HasPrefix(key, "_") || json.Unmarshal(value, &name) != nil {
			continue
		}
		tb.names[key] = name
		norm := normalize(name)
		if norm == "" || norm == normalize(key) {
			continue
		}
		if _, ok := tb.raw[norm]; !ok {
			tb.translated = append(tb.translated, norm)
		}
		tb.raw[norm] = append(tb.raw[norm], key)
	}
	return tb, nil
}

// Enabled 是配置的 enable_tag_translation（config?.enable_tag_translation ?? true）。
func Enabled(cfg *appconf.AppConfig) bool { return cfg == nil || cfg.EnableTagTranslation }

// normalize 对应 normalizeTagSearchText。
func normalize(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// Display 对应 getTagDisplayName：enabled（配置的 enable_tag_translation）时返回界面语言的译名，没有译名时原样返回。
func Display(name string, enabled bool) string {
	if !enabled {
		return name
	}
	if tb := current(); tb != nil {
		if v, ok := tb.names[name]; ok {
			return v
		}
	}
	return name
}

// FindRaw 对应 findRawTagNamesByTranslatedQuery：译名包含 query 的标签原名（去重，按译名在文件里的顺序）。
func FindRaw(query string) []string {
	q := normalize(query)
	tb := current()
	if q == "" || tb == nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, tr := range tb.translated {
		if !strings.Contains(tr, q) {
			continue
		}
		for _, raw := range tb.raw[tr] {
			if !seen[raw] {
				seen[raw] = true
				out = append(out, raw)
			}
		}
	}
	return out
}

// FilterByDisplay 对应 filterTagNamesByDisplayQuery：原名或显示名包含 query 的标签。
func FilterByDisplay(names []string, query string, enabled bool) []string {
	q := normalize(query)
	if q == "" {
		return names
	}
	var out []string
	for _, n := range names {
		if strings.Contains(normalize(n), q) || strings.Contains(normalize(Display(n, enabled)), q) {
			out = append(out, n)
		}
	}
	return out
}
