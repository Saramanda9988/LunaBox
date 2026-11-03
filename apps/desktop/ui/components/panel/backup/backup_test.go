package backup

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/theme"
)

type harness struct {
	svc   *host.Backend
	env   *env.Env
	game  models.Game
	state State
	tt    *ui.Tester
}

// newHarness 在真实服务（临时 DuckDB 与数据目录）上打开面板，外面套一层详情页的 p-8 与背景。
func newHarness(t *testing.T, g models.Game) *harness {
	t.Helper()
	h := &harness{svc: hosttest.NewServices(t), game: g}
	hosttest.AddGame(t, h.svc, g)
	h.env = &env.Env{Svc: h.svc.API(), Covers: covers.New(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: 1024}
	h.tt = ui.NewTester(h.view, 1024, 900)
	h.tt.SetScale(1.5)
	h.tt.SetDark(true)
	h.frames(3)
	return h
}

func (h *harness) view(c *ui.Context) {
	t := h.env.Theme
	ui.Column(c).Fill().Padding(32).Background(t.AppBg).Children(func() {
		View(c, t, &h.state, panel.Game{Env: h.env, Game: &h.game, OnChange: func(models.Game) {}, OnSaved: func() {}, OnDeleted: func() {}})
	})
}

func (h *harness) frames(n int) {
	for range n {
		h.tt.Frame()
	}
}

func (h *harness) click(t *testing.T, s string) {
	t.Helper()
	if err := h.tt.Click(s); err != nil {
		t.Fatalf("%v; texts %q", err, h.tt.Texts())
	}
	h.frames(2)
}

func (h *harness) expect(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !h.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, h.tt.Texts())
		}
	}
}

// saveDir 返回一个存档目录，含 save.dat 与 system.dat。两个文件：RestoreBackup 把只含一个文件的压缩包
// 当作单文件存档，会恢复到目录路径本身。
func saveDir(t *testing.T, content string) (dir, file string) {
	dir = t.TempDir()
	file = filepath.Join(dir, "save.dat")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "system.dat"), []byte("sys"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, file
}

func TestNoSavePath(t *testing.T) {
	h := newHarness(t, models.Game{ID: "backup-nopath", Name: "无存档"})
	h.expect(t, "存档备份", "请先在编辑页面设置存档路径（文件或文件夹）", "打开备份文件夹", "立即备份",
		"本地备份", "暂无本地备份记录", "云备份未启用", "前往设置页面配置云备份，将存档同步到云端")
	if h.tt.HasText("自动备份已启用") {
		t.Error("auto backup badge shown while disabled")
	}
}

func TestCreateRestoreDelete(t *testing.T) {
	dir, file := saveDir(t, "v1")
	h := newHarness(t, models.Game{ID: "backup-crud", Name: "备份测试", SavePath: dir})
	h.expect(t, "存档路径: "+dir, "暂无本地备份记录")

	h.click(t, "立即备份")
	backups, err := h.svc.Backup.GetGameBackups("backup-crud")
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups = %v, %v", backups, err)
	}
	h.expect(t, "备份创建成功", "大小: "+FormatFileSize(backups[0].Size))
	if h.state.backingUp || h.state.loadingLocal {
		t.Errorf("still busy: %+v", h.state)
	}

	// 恢复：改动存档后从备份恢复。
	if err := os.WriteFile(file, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.click(t, "恢复存档")
	h.expect(t, "恢复存档", "的备份吗？当前存档将被覆盖。", "取消", "确定")
	h.click(t, "确定")
	if b, _ := os.ReadFile(file); string(b) != "v1" {
		t.Errorf("restored save = %q", b)
	}
	h.expect(t, "存档已恢复")

	// 删除：取消不删，确定才删。
	h.click(t, "删除备份")
	h.expect(t, "确定要删除此本地备份吗？此操作无法撤销。")
	h.click(t, "取消")
	if h.state.confirm.open {
		t.Fatal("confirm still open after cancel")
	}
	if b, _ := h.svc.Backup.GetGameBackups("backup-crud"); len(b) != 1 {
		t.Fatalf("cancel deleted the backup: %v", b)
	}
	h.click(t, "删除备份")
	h.click(t, "确定")
	if b, _ := h.svc.Backup.GetGameBackups("backup-crud"); len(b) != 0 {
		t.Fatalf("backup not deleted: %v", b)
	}
	h.expect(t, "备份已删除", "暂无本地备份记录")
}

func TestAutoBackupBadge(t *testing.T) {
	svc := hosttest.NewServices(t)
	old := svc.Config.AutoBackupGameSave
	svc.Config.AutoBackupGameSave = true
	t.Cleanup(func() { svc.Config.AutoBackupGameSave = old })
	h := &harness{svc: svc, game: models.Game{ID: "backup-badge", Name: "徽标"}}
	hosttest.AddGame(t, svc, h.game)
	h.env = &env.Env{Svc: svc.API(), Covers: covers.New(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: 1024}
	h.tt = ui.NewTester(h.view, 1024, 900)
	h.frames(2)
	h.expect(t, "自动备份已启用")
}

// 换游戏时页面把 State 清零：面板重新读取，旧游戏的结果不会留下。
func TestReloadsOnGameChange(t *testing.T) {
	dir, _ := saveDir(t, "x")
	h := newHarness(t, models.Game{ID: "backup-a", Name: "A", SavePath: dir})
	h.click(t, "立即备份")
	h.expect(t, "大小: ")
	hosttest.AddGame(t, h.svc, models.Game{ID: "backup-b", Name: "B"})
	h.game = models.Game{ID: "backup-b", Name: "B"}
	h.state = State{}
	h.frames(2)
	h.expect(t, "暂无本地备份记录")
}

func TestFormatFileSize(t *testing.T) {
	for _, tc := range []struct {
		in   int64
		want string
	}{
		{0, "0 B"}, {-5, "0 B"}, {512, "512.0 B"}, {1024, "1.0 KB"}, {1280, "1.3 KB"}, {1536, "1.5 KB"},
		{5 * 1024 * 1024, "5.0 MB"}, {3 << 30, "3.0 GB"},
	} {
		if got := FormatFileSize(tc.in); got != tc.want {
			t.Errorf("FormatFileSize(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// DESKTOP_MYGO_PNG=<目录> 时把备份面板（本地列表、云端列表、确认弹窗）存成图片，用于目测。
func TestScreenshotDump(t *testing.T) {
	out := os.Getenv("DESKTOP_MYGO_PNG")
	if out == "" {
		t.Skip("DESKTOP_MYGO_PNG not set")
	}
	save := func(h *harness, name string) {
		f, err := os.Create(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, h.tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	dir, _ := saveDir(t, "save")
	h := newHarness(t, models.Game{ID: "backup-shot", Name: "截图", SavePath: dir})
	h.click(t, "立即备份")
	h.frames(2)
	save(h, "backup-local.png")

	// 云端列表：直接放入状态（测试环境没有配置云备份，也不访问网络）。
	h.state.cloudStatus = &vo.CloudBackupStatus{Enabled: true, Configured: true}
	h.state.cloudBackups = []vo.CloudBackupItem{
		{Key: "k1", Name: "2026-10-01T12-00-00.zip", CreatedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.Local)},
		{Key: "k2", CreatedAt: time.Date(2026, 9, 30, 8, 30, 0, 0, time.Local)},
	}
	h.frames(2)
	save(h, "backup-cloud.png")

	h.state.cloudStatus = nil
	h.click(t, "删除备份")
	h.frames(2)
	save(h, "backup-confirm.png")
}
