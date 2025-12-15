package enums

type Period string

const (
	Year  Period = "year"
	Month Period = "month"
	Week  Period = "week"
)

var AllPeriodTypes = []struct {
	Value  Period
	TSName string
}{
	{Year, "YEAR"},
	{Month, "MONTH"},
	{Week, "WEEK"},
}
