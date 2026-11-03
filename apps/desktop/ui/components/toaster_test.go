package components

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

func TestSplitToastMessage(t *testing.T) {
	long := strings.Repeat("长", 120)
	for _, tc := range []struct{ msg, title, details string }{
		{"  ", "", ""},
		{"已保存", "已保存", ""},
		{"第一行\n\n 第二行 \n第三行", "第一行", "第二行\n第三行"},
		{"日志导出失败: permission denied", "日志导出失败", "permission denied"},
		{"保存失败：原因很长的错误", "保存失败", "原因很长的错误"},
		{"保存失败：短", "保存失败：短", ""},         // 详情不超过 4 个字符时不拆
		{"失败: 磁盘已满了啊", "失败: 磁盘已满了啊", ""}, // 冒号前不足 4 个字符
		{long, string([]rune(long)[:92]) + "...", long},
	} {
		title, details := splitToastMessage(tc.msg)
		if title != tc.title || details != tc.details {
			t.Errorf("splitToastMessage(%q) = %q, %q; want %q, %q", tc.msg, title, details, tc.title, tc.details)
		}
	}
}

func TestToaster(t *testing.T) {
	th := theme.New(false, 1)
	var add []ui.Toast
	tt := ui.NewTester(func(c *ui.Context) {
		for _, ts := range add {
			c.AddToast(ts)
		}
		add = nil
		Toaster(c, th, 40)
	}, 1000, 700)
	tt.Frame()

	add = []ui.Toast{{ID: "a", Title: "备份失败：磁盘空间不足，无法写入", Type: "error"}}
	tt.Frame()
	tt.Advance(time.Second)
	if !tt.HasText("备份失败") || tt.HasText("磁盘空间不足，无法写入") {
		t.Fatalf("details should start hidden: %q", tt.Texts())
	}
	// 右上角、顶栏下方，宽 380。
	r, ok := tt.Find("备份失败")
	if !ok || r.X < 1000-16-380 || r.Y < 40 {
		t.Fatalf("title at %+v", r)
	}
	if err := tt.Click(locales.CommonToastShowDetails()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("磁盘空间不足，无法写入") || !tt.HasText(locales.CommonToastHideDetails()) {
		t.Fatalf("details not shown: %q", tt.Texts())
	}

	// 两条时展开后新的在上面。
	add = []ui.Toast{{ID: "b", Title: "已保存", Type: "success"}}
	tt.Frame()
	tt.Advance(time.Second)
	newer, _ := tt.Find("已保存")
	tt.Move(newer.X+newer.W/2, newer.Y+newer.H/2)
	tt.Advance(time.Second)
	newer, _ = tt.Find("已保存")
	older, _ := tt.Find("备份失败")
	if newer.Y >= older.Y {
		t.Fatalf("expanded stack: newer %+v, older %+v", newer, older)
	}

	// 关闭按钮关掉一条；没有指针停留时到时间自动关闭。
	if err := tt.Click(locales.CommonToastDismiss()); err != nil {
		t.Fatal(err)
	}
	tt.Move(0, 600)
	tt.Advance(10 * time.Second)
	if tt.HasText("已保存") || tt.HasText("备份失败") {
		t.Fatalf("toasts left: %q", tt.Texts())
	}
}
