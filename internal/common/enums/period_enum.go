package enums

type Period string

const (
	Day   Period = "day"  // 内部使用：自定义日期范围的占位维度
	Week  Period = "week"
	Month Period = "month"
	Year  Period = "year"
	All   Period = "all"  // 内部使用：游戏详情页统计
)

var AllPeriodTypes = []struct {
	Value  Period
	TSName string
}{
	{Day, "DAY"},
	{Week, "WEEK"},
	{Month, "MONTH"},
	{Year, "YEAR"},
	{All, "ALL"},
}
