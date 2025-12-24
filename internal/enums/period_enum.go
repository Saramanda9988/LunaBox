package enums

type Period string

const (
	Day   Period = "day"
	Week  Period = "week"
	Month Period = "month"
)

var AllPeriodTypes = []struct {
	Value  Period
	TSName string
}{
	{Day, "DAY"},
	{Week, "WEEK"},
	{Month, "MONTH"},
}
