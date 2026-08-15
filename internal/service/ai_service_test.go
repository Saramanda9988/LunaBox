package service

import (
	"strings"
	"testing"
	"time"
)

func TestBuildContextPromptFormatsShortDurationsWithoutRoundingToZeroHours(t *testing.T) {
	data := &AIStatsData{
		DateRange:         "2025-07-26 至 2026-07-25",
		TotalPlayCount:    3,
		TotalPlayDuration: 9382,
		TopGames: []GamePlayInfo{
			{Name: "夏日口袋", Duration: 9360},
		},
		RecentSessions: []SessionInfo{
			{
				GameName: "美好的每一天",
				StartTime: time.Date(
					2026, time.July, 25, 1, 2, 3, 0, time.Local,
				),
				Duration: 125,
				Hour:     1,
			},
			{
				GameName: "女装山脉",
				StartTime: time.Date(
					2026, time.July, 24, 23, 2, 3, 0, time.Local,
				),
				Duration: 0,
				Hour:     23,
			},
		},
	}

	prompt := (&AiService{}).buildContextPrompt(data)

	for _, expected := range []string{
		"合计 2小时36分钟",
		"《夏日口袋》 — 2小时36分钟",
		"《美好的每一天》 — 2分钟",
		"《女装山脉》 — 0秒",
	} {
		if !strings.Contains(prompt, expected) {
			t.Errorf("prompt does not contain %q:\n%s", expected, prompt)
		}
	}
	if strings.Contains(prompt, "0.0 小时") {
		t.Fatalf("prompt still contains a duration rounded to zero hours:\n%s", prompt)
	}
}

func TestBuildTaskPromptUsesYearPeriodName(t *testing.T) {
	prompt := (&AiService{}).buildTaskPrompt(&AIStatsData{Dimension: "year"})

	if !strings.Contains(prompt, "最近1年") {
		t.Fatalf("year prompt has an unexpected period name:\n%s", prompt)
	}
}
