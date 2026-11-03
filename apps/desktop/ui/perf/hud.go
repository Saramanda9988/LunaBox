package perf

import (
	"fmt"
	"math"
	"os"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// HUD 是左下角的性能面板（F2 切换），extra 是调用方补充的行（数据量、封面等）。
// 它每秒随内存采样重绘一次，测空闲 CPU 时请先关掉。
func HUD(c *ui.Context, p *Perf, b *Bench, extra []string) {
	now := time.Now()
	lines := []string{
		fmt.Sprintf("fps %d   view build %.2f ms", p.FPS(now), float64(p.lastBuild.Microseconds())/1000),
	}
	if mem := p.mem.Load(); mem != nil {
		lines = append(lines,
			fmt.Sprintf("working set %.1f MB   private %.1f MB", mb(mem.workingSet), mb(mem.private)),
			fmt.Sprintf("go heap live %.1f MB   objects %.1f MB", mb(mem.heapLive), mb(mem.heapAlloc)))
	}
	lines = append(lines, extra...)
	lines = append(lines, fmt.Sprintf("first frame %d ms", p.firstFrameMs.Load()))
	switch {
	case b.phase != benchIdle:
		lines = append(lines, "bench running...")
	case b.report != "":
		lines = append(lines, "last bench: see bench-result.txt")
	}
	lines = append(lines, "F2 hide HUD   F5 scroll bench")

	ui.Overlay(c, func() {
		ui.Column(c).Absolute().Left(12).Bottom(12).Padding(8, 10).Radius(8).Gap(2).
			Background(ui.RGBA(0, 0, 0, 0.72)).PassThrough().Children(func() {
			for _, l := range lines {
				ui.Text(c, l).FontSize(11).Font("Consolas").TextColor(ui.Hex("#ffffff")).NoWrap()
			}
		})
	})
}

type benchPhase int

const (
	benchIdle benchPhase = iota
	benchWarmup
	benchDown
	benchUp
)

// benchSpeed 是自动滚动速度（DIP/秒），约等于快速拖动滚动条。
const benchSpeed = 2400

// Bench 是自动滚动压测：预热 1.5s 等数据与封面加载，然后滚到底再滚回顶部，报告写入 bench-result.txt。
type Bench struct {
	Exit bool // 结束后退出程序

	perf     *Perf
	phase    benchPhase
	since    time.Time
	last     time.Time
	scrolled float32
	before   *memSample
	report   string
}

// NewBench 返回使用 p 记录帧的压测。
func NewBench(p *Perf, exit bool) *Bench { return &Bench{perf: p, Exit: exit} }

// Arm 开始一次压测。
func (b *Bench) Arm() {
	b.phase = benchWarmup
	b.since = time.Now()
	b.report = ""
}

// Target 是被压测的滚动区域。
type Target struct {
	Scroll *ui.ScrollState
	Ready  bool // 数据已加载
	Items  int
}

// Step 在每次 view 时推进压测。
func (b *Bench) Step(c *ui.Context, target Target) {
	if b.phase == benchIdle || target.Scroll == nil {
		return
	}
	st := target.Scroll
	now := time.Now()
	c.AnimationFrame()
	if b.phase == benchWarmup {
		if now.Sub(b.since) < 1500*time.Millisecond || !target.Ready {
			return
		}
		st.Y = 0
		b.before = readMem()
		b.phase, b.since, b.last, b.scrolled = benchDown, now, now, 0
		b.perf.startRecording()
		return
	}

	dt := float32(now.Sub(b.last).Seconds())
	b.last = now
	delta := benchSpeed * dt
	b.scrolled += delta
	switch b.phase {
	case benchDown:
		st.Y += delta
		if st.MaxY > 0 && st.Y >= st.MaxY {
			st.Y = st.MaxY
			b.phase = benchUp
		}
	case benchUp:
		st.Y = float32(math.Max(0, float64(st.Y-delta)))
		if st.Y == 0 {
			b.finish(target.Items, now)
		}
	}
	if b.phase == benchDown && st.MaxY == 0 && now.Sub(b.since) > 2*time.Second {
		b.finish(target.Items, now)
	}
}

func (b *Bench) finish(items int, now time.Time) {
	frames := b.perf.stopRecording()
	b.report = b.perf.benchReport(frames, items, b.scrolled, now.Sub(b.since), b.before, readMem())
	b.phase = benchIdle
	fmt.Print(b.report)
	if f, err := os.OpenFile("bench-result.txt", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		fmt.Fprintf(f, "%s\n", b.report)
		f.Close()
	}
	if b.Exit {
		mygo.App.Quit()
	}
}
