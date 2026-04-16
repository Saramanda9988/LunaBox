package enums

type Period string

const (
	Day   Period = "day"
	Week  Period = "week"
	Month Period = "month"
	All   Period = "all"
)

var AllPeriodTypes = []struct {
	Value  Period
	TSName string
}{
	{Day, "DAY"},
	{Week, "WEEK"},
	{Month, "MONTH"},
	{All, "ALL"},
}
