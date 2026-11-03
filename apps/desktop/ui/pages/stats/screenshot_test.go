package stats

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/models"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/env/envtest"
	"lunabox/desktop-mygo/ui/theme"
)

// TestScreenshots 把整页（月维度、12 个游戏、AI 总结）在深浅两种主题、宽窄两种主内容区下画到
// $DESKTOP_MYGO_PNG/stats-<宽度>-<dark|light>.png，用来对照原版。
func TestScreenshots(t *testing.T) {
	dir := os.Getenv("DESKTOP_MYGO_PNG")
	if dir == "" {
		t.Skip("DESKTOP_MYGO_PNG not set")
	}
	svc := hosttest.NewServices(t)
	svc.Config.TimeZone = ""
	sampleData(t, svc)
	for _, w := range []int{1200, 900} {
		for _, dark := range []bool{true, false} {
			e := &env.Env{Backend: envtest.NewBackend(0), Svc: svc.API(), Router: ui.NewRouter(env.PathStats), Covers: covers.New(),
				Theme: theme.New(dark, 1), MainWidth: float32(w)}
			p := New(e)
			p.Open(time.Now())
			p.changePeriod("month")
			p.aiSummary["month"] = "这个月你在《测试游戏01》上投入了大量时间。\n\n第二段：继续保持！"
			tt := ui.NewTester(p.View, w, 2600)
			tt.SetDark(dark)
			for range 4 {
				tt.Frame()
			}
			mode := "light"
			if dark {
				mode = "dark"
			}
			f, err := os.Create(filepath.Join(dir, fmt.Sprintf("stats-%d-%s.png", w, mode)))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			f.Close()
		}
	}
}

// sampleData 写入 12 个游戏、过去 30 天里分散的游玩记录与几个标签。
func sampleData(t *testing.T, svc *host.Backend) {
	now := time.Now()
	for i := 1; i <= 12; i++ {
		id := fmt.Sprintf("g%d", i)
		hosttest.AddGame(t, svc, models.Game{ID: id, Name: name(i), IsNSFW: i == 2})
		for d := 0; d < 30; d += 1 + i%4 {
			start := time.Date(now.Year(), now.Month(), now.Day(), 8+(i*5+d)%14, 0, 0, 0, time.Local).AddDate(0, 0, -d)
			hosttest.AddSession(t, svc, id, start, (13-i)*600+d*60)
		}
	}
	for i, tag := range []string{"纯爱", "校园", "奇幻", "悬疑", "日常", "科幻"} {
		for g := 1; g <= 6-i; g++ {
			hosttest.AddTag(t, svc, fmt.Sprintf("g%d", g), fmt.Sprintf("t%d-%d", i, g), tag, "user", false)
		}
	}
}
