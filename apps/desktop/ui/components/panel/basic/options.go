package basic

import (
	"math"
	"strconv"

	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/env"
)

// appZoomOptions 对应 appZoomOptions：标签是 Math.round(value * 100)%，值是 String(value)。
func appZoomOptions() []better.SelectOption {
	out := make([]better.SelectOption, len(env.ZoomLevels))
	for i, v := range env.ZoomLevels {
		out[i] = better.SelectOption{Label: strconv.Itoa(int(math.Round(v*100))) + "%", Value: zoomValue(v)}
	}
	return out
}

// zoomValue 是 JavaScript 的 String(number)：1 → "1"，1.25 → "1.25"。
func zoomValue(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// languageOptions 对应 languageOptions：每种语言用它自己的名字。
func languageOptions() []better.SelectOption {
	return []better.SelectOption{
		{Value: "zh-CN", Label: "简体中文"}, // i18n:keep：语言名称用该语言自己书写
		{Value: "zh-TW", Label: "繁體中文"}, // i18n:keep：语言名称用该语言自己书写
		{Value: "en-US", Label: "English"},
		{Value: "ja-JP", Label: "日本語"}, // i18n:keep：语言名称用该语言自己书写
	}
}

// commonTimezones 对应 BasicSettingsPanel 的 COMMON_TIMEZONES（原版的标签是写死的英文）。
var commonTimezones = []better.SelectOption{
	{Value: "Asia/Shanghai", Label: "China Standard Time (UTC+8)"},    // i18n:keep：原版写死的英文时区名
	{Value: "Asia/Tokyo", Label: "Japan Standard Time (UTC+9)"},       // i18n:keep：原版写死的英文时区名
	{Value: "Asia/Seoul", Label: "Korea Standard Time (UTC+9)"},       // i18n:keep：原版写死的英文时区名
	{Value: "Asia/Hong_Kong", Label: "Hong Kong Time (UTC+8)"},        // i18n:keep：原版写死的英文时区名
	{Value: "Asia/Taipei", Label: "Taipei Time (UTC+8)"},              // i18n:keep：原版写死的英文时区名
	{Value: "Asia/Singapore", Label: "Singapore Time (UTC+8)"},        // i18n:keep：原版写死的英文时区名
	{Value: "Asia/Bangkok", Label: "Bangkok Time (UTC+7)"},            // i18n:keep：原版写死的英文时区名
	{Value: "Asia/Dubai", Label: "Dubai Time (UTC+4)"},                // i18n:keep：原版写死的英文时区名
	{Value: "Europe/London", Label: "London Time (UTC+0)"},            // i18n:keep：原版写死的英文时区名
	{Value: "Europe/Paris", Label: "Paris Time (UTC+1)"},              // i18n:keep：原版写死的英文时区名
	{Value: "Europe/Berlin", Label: "Berlin Time (UTC+1)"},            // i18n:keep：原版写死的英文时区名
	{Value: "Europe/Moscow", Label: "Moscow Time (UTC+3)"},            // i18n:keep：原版写死的英文时区名
	{Value: "America/New_York", Label: "New York Time (UTC-5)"},       // i18n:keep：原版写死的英文时区名
	{Value: "America/Chicago", Label: "Chicago Time (UTC-6)"},         // i18n:keep：原版写死的英文时区名
	{Value: "America/Denver", Label: "Denver Time (UTC-7)"},           // i18n:keep：原版写死的英文时区名
	{Value: "America/Los_Angeles", Label: "Los Angeles Time (UTC-8)"}, // i18n:keep：原版写死的英文时区名
	{Value: "America/Sao_Paulo", Label: "São Paulo Time (UTC-3)"},     // i18n:keep：原版写死的英文时区名
	{Value: "Australia/Sydney", Label: "Sydney Time (UTC+10)"},        // i18n:keep：原版写死的英文时区名
	{Value: "Pacific/Auckland", Label: "Auckland Time (UTC+12)"},      // i18n:keep：原版写死的英文时区名
	{Value: "UTC", Label: "Coordinated Universal Time (UTC)"},         // i18n:keep：原版写死的英文时区名
}
