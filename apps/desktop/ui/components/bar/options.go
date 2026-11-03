package bar

import (
	"lunabox/backend/common/enums"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/locales"
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

func literal(s string) func() string { return func() string { return s } }

// SourceOptions 对应 metadataSourceOptions（来源名称同 importFlow.sourceLabel）。
var SourceOptions = []struct {
	Label func() string
	Value enums.SourceType
}{
	{locales.FilterBarAllMetadataSources, ""},
	{locales.FilterBarNoMetadataSource, enums.Local},
	{literal("Bangumi"), enums.Bangumi},
	{literal("VNDB"), enums.VNDB},
	{locales.GameEditSourceYmgal, enums.Ymgal},
	{locales.GameEditSourceDlsite, enums.DLsite},
	{locales.GameEditSourceTouchGal, enums.TouchGal},
	{locales.GameEditSourceHikarinagi, enums.Hikarinagi},
	{locales.GameEditSourceErogameScape, enums.ErogameScape},
	{literal("Steam"), enums.Steam},
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
