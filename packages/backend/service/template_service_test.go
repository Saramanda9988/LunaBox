package service

import (
	"strings"
	"testing"

	"lunabox/backend/common/vo"
)

func TestRenderTemplateKeepsCoverDataURL(t *testing.T) {
	cover := "data:image/png;base64,iVBORw0KGgo="
	data := vo.StatsExportData{Leaderboard: []vo.StatsGameItem{
		{Rank: 1, GameName: "g1", CoverBase64: cover},
		{Rank: 2, GameName: "g2", CoverBase64: "javascript:alert(1)", CoverURL: "/local/covers/g2.jpg"},
	}}
	for _, id := range []string{"editorial_report", "retro_futurism_report", "scrapbook_report"} {
		resp, err := NewTemplateService().RenderTemplate(vo.RenderTemplateRequest{TemplateID: id, Data: data})
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if !strings.Contains(resp.HTML, `src="`+cover+`"`) {
			t.Errorf("%s: cover data URL not rendered", id)
		}
		if strings.Contains(resp.HTML, "ZgotmplZ") || strings.Contains(resp.HTML, "javascript:") {
			t.Errorf("%s: unsafe or filtered cover URL in output", id)
		}
		if !strings.Contains(resp.HTML, `src="/local/covers/g2.jpg"`) {
			t.Errorf("%s: untrusted cover did not fall back to CoverURL", id)
		}
	}
}

func TestPrepareExportDataKeepsTopTenLeaderboard(t *testing.T) {
	var stats vo.PeriodStats
	for i := range 15 {
		stats.PlayTimeLeaderboard = append(stats.PlayTimeLeaderboard, vo.GamePlayStats{GameID: string(rune('a' + i)), TotalDuration: 100 - i})
	}
	data, err := NewTemplateService().PrepareExportData(stats, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Leaderboard) != posterLeaderboardSize {
		t.Fatalf("poster leaderboard has %d games, want %d", len(data.Leaderboard), posterLeaderboardSize)
	}
	if last := data.Leaderboard[len(data.Leaderboard)-1]; last.Rank != posterLeaderboardSize || last.GameID != "j" {
		t.Fatalf("last poster game = rank %d %q, want rank %d %q", last.Rank, last.GameID, posterLeaderboardSize, "j")
	}
}
