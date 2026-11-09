package bar

import (
	"lunabox/backend/common/enums"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/metasource"
)

// StatusOptions 对应 consts/options.ts 的 statusOptions。
var StatusOptions = []struct {
	Label func() string
	Value enums.GameStatus
}{
	{locales.CommonAllStatus, ""},
	{locales.CommonNotStarted, enums.StatusNotStarted},
	{locales.CommonWantToPlay, enums.StatusWantToPlay},
	{locales.CommonPlaying, enums.StatusPlaying},
	{locales.CommonCompleted, enums.StatusCompleted},
	{locales.CommonOnHold, enums.StatusOnHold},
	{locales.CommonDropped, enums.StatusDropped},
}

// StatusLabel 是状态在 statusOptions 里的名称。
func StatusLabel(s enums.GameStatus) string {
	for _, o := range StatusOptions {
		if o.Value == s {
			return o.Label()
		}
	}
	return string(s)
}

// SourceOption 是来源筛选的一项。
type SourceOption struct {
	Label func() string
	Value enums.SourceType
}

// SourceOptions 对应 metadataSourceOptions：全部、无元数据来源，再按 ALL_METADATA_SOURCES 列出来源。
var SourceOptions = sourceOptions()

func sourceOptions() []SourceOption {
	opts := []SourceOption{
		{locales.FilterBarAllMetadataSources, ""},
		{locales.FilterBarNoMetadataSource, enums.Local},
	}
	for _, s := range metasource.All() {
		opts = append(opts, SourceOption{func() string { return metasource.Label(s) }, s})
	}
	return opts
}

// GameSortOptions 对应 consts/options.ts 的 sortOptions。文字按当前语言取，每帧重建。
func GameSortOptions() []better.SelectOption {
	return []better.SelectOption{
		{Label: locales.CommonName(), Value: string(enums.GameListSortByName)},
		{Label: locales.CommonCompany(), Value: string(enums.GameListSortByCompany)},
		{Label: locales.CommonLastPlayedAt(), Value: string(enums.GameListSortByLastPlayedAt)},
		{Label: locales.CommonCreatedAt(), Value: string(enums.GameListSortByCreatedAt)},
		{Label: locales.CommonRating(), Value: string(enums.GameListSortByRating)},
		{Label: locales.CommonReleaseDate(), Value: string(enums.GameListSortByReleaseDate)},
	}
}
