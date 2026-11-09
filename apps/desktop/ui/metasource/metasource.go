// Package metasource 是界面里元数据来源信息的唯一出处：名称、网页链接、图标、设置页说明，
// 对应原版 utils/metadataSources.ts 与 importFlow.ts 的 sourceLabel。有哪些来源、默认顺序由后端
// enums.MetadataSourceTypes 决定；新增来源时在那里加上类型，再在下面的 sources 里补一项（测试会检查两边一致）。
// 各处的来源列表、下拉框、筛选项都从这里取，不要另写 switch 或列表。
package metasource

import (
	"net/url"
	"slices"
	"strings"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/desktop/ui/locales"
)

// Source 是一个元数据来源在界面上需要的信息。
type Source struct {
	// Label 对应 importFlow.ts 的 sourceLabel（也是编辑面板 sourceOptions 的 label）：Bangumi、VNDB、Steam 原版不翻译。
	Label func() string
	// Hint 是元数据设置页（MetadataSettingsPanel）开关下的说明。
	Hint func() string
	// Logo、Compact 对应 METADATA_SOURCE_ICONS、METADATA_SOURCE_COMPACT_ICONS（ui/assets/providers 下的文件名）；
	// Compact 为空时同 Logo。
	Logo, Compact string
	// SquareIcon：紧凑图标是方形图片，按原色 object-cover 显示（编辑面板默认来源的图标），其他是只保留轮廓的 logo。
	SquareIcon bool
	// URL 对应 getMetadataSourceURL：id 已经 encodeURIComponent，erogameScapeBase 是用户设置的 ErogameScape 地址。
	URL func(id, erogameScapeBase string) string
}

func literal(s string) func() string { return func() string { return s } }

func prefix(p string) func(id, _ string) string { return func(id, _ string) string { return p + id } }

var sources = map[enums.SourceType]Source{
	enums.Bangumi: {
		Label: literal("Bangumi"), Hint: locales.SettingsMetadataSourceHintsBangumi,
		Logo: "bangumi-logo.png", Compact: "bangumi-icon.png", SquareIcon: true,
		URL: prefix("https://bgm.tv/subject/"),
	},
	enums.VNDB: {
		Label: literal("VNDB"), Hint: locales.SettingsMetadataSourceHintsVndb,
		Logo: "vndb-logo.svg",
		URL:  prefix("https://vndb.org/"),
	},
	enums.Ymgal: {
		Label: locales.GameEditSourceYmgal, Hint: locales.SettingsMetadataSourceHintsYmgal,
		Logo: "ymgal-logo.png",
		URL:  prefix("https://www.ymgal.games/ga/"),
	},
	enums.DLsite: {
		Label: locales.GameEditSourceDlsite, Hint: locales.SettingsMetadataSourceHintsDlsite,
		Logo: "dlsite-logo.png",
		URL: func(id, _ string) string {
			return "https://www.dlsite.com/maniax/work/=/product_id/" + id + ".html"
		},
	},
	enums.TouchGal: {
		Label: locales.GameEditSourceTouchGal, Hint: locales.SettingsMetadataSourceHintsTouchgal,
		Logo: "touchgal-logo.webp",
		URL:  prefix("https://www.touchgal.ink/"),
	},
	enums.Hikarinagi: {
		Label: locales.GameEditSourceHikarinagi, Hint: locales.SettingsMetadataSourceHintsHikarinagi,
		Logo: "hikarinagi-logo.svg", Compact: "hikarinagi-icon.webp", SquareIcon: true,
		URL: prefix("https://www.hikarinagi.org/galgames/"),
	},
	enums.Kungal: {
		Label: locales.GameEditSourceKungal, Hint: locales.SettingsMetadataSourceHintsKungal,
		Logo: "kungal-logo.webp", SquareIcon: true,
		URL: prefix("https://www.kungal.com/galgame/"),
	},
	enums.ErogameScape: {
		Label: locales.GameEditSourceErogameScape, Hint: locales.SettingsMetadataSourceHintsErogamescape,
		Logo: "erogamescape-logo.png",
		URL: func(id, base string) string {
			base = strings.TrimRight(strings.TrimSpace(base), "/")
			if base == "" {
				base = appconf.DefaultErogameScapeBaseURL
			}
			return base + "/game.php?game=" + id
		},
	},
	enums.Steam: {
		Label: literal("Steam"), Hint: locales.SettingsMetadataSourceHintsSteam,
		Logo: "steam-logo.png",
		URL:  prefix("https://store.steampowered.com/app/"),
	},
}

// All 是 ALL_METADATA_SOURCES：全部元数据来源，按默认顺序。
func All() []enums.SourceType { return slices.Clone(enums.MetadataSourceTypes) }

// Ordered 返回全部来源：先按 first 的顺序，其余按 All 的顺序接在后面。面板有自己的显示顺序时用它，
// 以后新增的来源即使没写进 first 也不会漏掉。
func Ordered(first ...enums.SourceType) []enums.SourceType {
	out := make([]enums.SourceType, 0, len(enums.MetadataSourceTypes))
	for _, s := range first {
		if Known(s) && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	for _, s := range enums.MetadataSourceTypes {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

// Known 判断是不是元数据来源（local 等不是）。
func Known(s enums.SourceType) bool { return slices.Contains(enums.MetadataSourceTypes, s) }

// Get 返回来源的信息；不是元数据来源时 ok 为假。
func Get(s enums.SourceType) (src Source, ok bool) {
	if !Known(s) {
		return Source{}, false
	}
	src, ok = sources[s]
	return src, ok
}

// Label 是来源的显示名称；未知来源原样显示。
func Label(s enums.SourceType) string {
	if src, ok := Get(s); ok {
		return src.Label()
	}
	return string(s)
}

// Hint 是元数据设置页里来源的说明；未知来源为空。
func Hint(s enums.SourceType) string {
	if src, ok := Get(s); ok && src.Hint != nil {
		return src.Hint()
	}
	return ""
}

// LogoFile 对应 getMetadataSourceIcon(source, compact ? "compact" : "logo")；没有图标时为空。
func LogoFile(s enums.SourceType, compact bool) string {
	src, ok := Get(s)
	if !ok {
		return ""
	}
	if compact && src.Compact != "" {
		return src.Compact
	}
	return src.Logo
}

// SquareIcon 判断紧凑图标是不是方形图片（见 Source.SquareIcon）。
func SquareIcon(s enums.SourceType) bool {
	src, ok := Get(s)
	return ok && src.SquareIcon
}

// URL 对应 getMetadataSourceURL：来源网页上这个游戏的地址；来源未知或 ID 为空时为空。
func URL(s enums.SourceType, id, erogameScapeBase string) string {
	id = strings.TrimSpace(id)
	src, ok := Get(s)
	if !ok || id == "" {
		return ""
	}
	id = strings.ReplaceAll(url.QueryEscape(id), "+", "%20") // encodeURIComponent
	return src.URL(id, erogameScapeBase)
}
