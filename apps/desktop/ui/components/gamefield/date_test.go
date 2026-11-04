package gamefield

import (
	"testing"
	"time"
)

func TestDateHelpers(t *testing.T) {
	if FormatDateInputValue("2023-02-29") != "" || FormatDateInputValue(" 2024-02-29 ") != "2024-02-29" {
		t.Error("FormatDateInputValue")
	}
	rows := calendarRows(time.Date(2024, 5, 1, 0, 0, 0, 0, time.Local))
	if got := rows[0].dates[0].Format("2006-01-02"); got != "2024-04-28" {
		t.Errorf("first calendar day = %s", got)
	}
}
