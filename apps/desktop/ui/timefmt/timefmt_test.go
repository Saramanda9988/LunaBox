package timefmt

import (
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"
)

func TestFormats(t *testing.T) {
	at := time.Date(2026, 9, 11, 13, 5, 9, 0, time.UTC)
	for _, c := range []struct{ locale, duration, chart, date, dateTime string }{
		{"", "1小时5分钟", "1时 5分", "2026/09/11", "2026/09/11 13:05:09"},
		{"en-US", "1H5M", "1h 5m", "09/11/2026", "09/11/2026, 13:05:09"},
		{"ja-JP", "", "", "2026/09/11", "2026/09/11 13:05:09"},
	} {
		i18n.SetLocaleFunc(func() string { return c.locale })
		if c.duration != "" {
			if got := Duration(3900); got != c.duration {
				t.Errorf("%q Duration = %q, want %q", c.locale, got, c.duration)
			}
			if got := DurationChart(3900); got != c.chart {
				t.Errorf("%q DurationChart = %q, want %q", c.locale, got, c.chart)
			}
		}
		if got := Date(at, time.UTC); got != c.date {
			t.Errorf("%q Date = %q, want %q", c.locale, got, c.date)
		}
		if got := DateTime(at, time.UTC); got != c.dateTime {
			t.Errorf("%q DateTime = %q, want %q", c.locale, got, c.dateTime)
		}
	}
	i18n.SetLocaleFunc(nil)
}
