// Package perf 收集帧耗时、内存与启动时间，提供性能 HUD 与自动滚动压测。
package perf

import (
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"runtime/metrics"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// frameSample 是 MYGO_FRAME_STATS 打出的一帧耗时（毫秒）。
type frameSample struct {
	at                                   time.Time
	total, build, layout, paint, present float64
}

type memSample struct {
	workingSet, private uint64 // 进程级，字节
	// heapLive 是上一次 GC 后仍然存活的 Go 堆（/gc/heap/live:bytes）；heapAlloc 是现在堆上的对象，
	// 含还没回收的垃圾（MemStats.HeapAlloc）。GOGC=100 时 heapAlloc 在 heapLive 与它的两倍之间来回。
	heapLive, heapAlloc uint64
}

// Perf 收集帧耗时、内存和启动时间。
type Perf struct {
	statsOn bool

	mu        sync.Mutex
	recording bool
	frames    []frameSample
	recent    []time.Time // 最近 1s 内的帧，用于 HUD 的 FPS

	mem atomic.Pointer[memSample]

	processStart time.Time
	firstView    time.Time
	firstFrame   time.Time
	firstFrameMs atomic.Int64

	// 主线程字段
	viewTimes []time.Duration
	lastBuild time.Duration
}

var frameLine = regexp.MustCompile(`mygo: frame \d+ took ([\d.]+) ms: build ([\d.]+)[^,]*, layout ([\d.]+), paint ([\d.]+), present ([\d.]+)`)

// New 开始截获 MyGo 的帧日志（log 输出），应在创建窗口前调用。
func New() *Perf {
	p := &Perf{processStart: processStartTime()}
	p.statsOn = os.Getenv("MYGO_FRAME_STATS") != ""
	log.SetOutput(&frameLogWriter{p: p, next: os.Stderr, passthrough: os.Getenv("DESKTOP_MYGO_FRAME_LOG") != ""})
	return p
}

// frameLogWriter 截获 mygo 的帧日志；MYGO_FRAME_STATS=all 时每帧一行，其余日志原样输出。
type frameLogWriter struct {
	p           *Perf
	next        io.Writer
	passthrough bool // DESKTOP_MYGO_FRAME_LOG=1 时帧日志也照常输出
}

func (w *frameLogWriter) Write(b []byte) (int, error) {
	m := frameLine.FindSubmatch(b)
	if m == nil {
		return w.next.Write(b)
	}
	s := frameSample{at: time.Now()}
	s.total, _ = strconv.ParseFloat(string(m[1]), 64)
	s.build, _ = strconv.ParseFloat(string(m[2]), 64)
	s.layout, _ = strconv.ParseFloat(string(m[3]), 64)
	s.paint, _ = strconv.ParseFloat(string(m[4]), 64)
	s.present, _ = strconv.ParseFloat(string(m[5]), 64)
	w.p.frame(s)
	if w.passthrough {
		return w.next.Write(b)
	}
	return len(b), nil
}

func (p *Perf) frame(s frameSample) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.firstFrame.IsZero() {
		p.firstFrame = s.at
		p.firstFrameMs.Store(s.at.Sub(p.processStart).Milliseconds())
	}
	if p.recording {
		p.frames = append(p.frames, s)
	}
	p.recent = append(p.recent, s.at)
}

// ViewCalled 在每次 view 结束时调用（主线程），未开启 MYGO_FRAME_STATS 时用它估算帧。
func (p *Perf) ViewCalled(now time.Time, build time.Duration) {
	p.lastBuild = build
	if p.firstView.IsZero() {
		p.firstView = now
		if !p.statsOn {
			p.firstFrameMs.Store(now.Sub(p.processStart).Milliseconds())
		}
	}
	if p.statsOn {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.recording {
		p.frames = append(p.frames, frameSample{at: now, build: float64(build.Microseconds()) / 1000})
	}
	p.recent = append(p.recent, now)
}

// FPS 返回最近一秒的帧数。
func (p *Perf) FPS(now time.Time) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	cut := now.Add(-time.Second)
	i := 0
	for i < len(p.recent) && p.recent[i].Before(cut) {
		i++
	}
	p.recent = p.recent[i:]
	return len(p.recent)
}

func (p *Perf) startRecording() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.frames = p.frames[:0]
	p.recording = true
}

func (p *Perf) stopRecording() []frameSample {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.recording = false
	return slices.Clone(p.frames)
}

// SampleMemory 在 HUD 可见时每秒采样一次内存并触发重绘；HUD 隐藏时什么都不做，保证空闲 0 CPU。
func (p *Perf) SampleMemory(visible func() bool, redraw func()) {
	for {
		if visible() {
			p.mem.Store(readMem())
			redraw()
		}
		time.Sleep(time.Second)
	}
}

// memMetrics 用 runtime/metrics 读取，不像 ReadMemStats 那样暂停整个程序。
var memMetrics = []metrics.Sample{{Name: "/gc/heap/live:bytes"}, {Name: "/memory/classes/heap/objects:bytes"}}

func readMem() *memSample {
	ws, priv := processMemory()
	s := slices.Clone(memMetrics)
	metrics.Read(s)
	return &memSample{workingSet: ws, private: priv, heapLive: s[0].Value.Uint64(), heapAlloc: s[1].Value.Uint64()}
}

func mb(b uint64) float64 { return float64(b) / (1 << 20) }

// benchReport 汇总一次自动滚动压测。
func (p *Perf) benchReport(frames []frameSample, items int, scrolled float32, d time.Duration, before, after *memSample) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "== desktop-mygo scroll bench (%s) ==\n", time.Now().Format(time.DateTime))
	fmt.Fprintf(&sb, "items=%d  scrolled=%.0f DIP  duration=%s  frames=%d  avg fps=%.1f\n",
		items, scrolled, d.Round(time.Millisecond), len(frames), float64(len(frames))/d.Seconds())

	intervals := make([]float64, 0, len(frames))
	for i := 1; i < len(frames); i++ {
		intervals = append(intervals, float64(frames[i].at.Sub(frames[i-1].at).Microseconds())/1000)
	}
	fmt.Fprintf(&sb, "frame interval ms: %s\n", pctl(intervals))
	slow := 0
	for _, v := range intervals {
		if v > 25 {
			slow++
		}
	}
	if len(intervals) > 0 {
		fmt.Fprintf(&sb, "intervals > 25ms (dropped at 60Hz): %d (%.1f%%)\n", slow, 100*float64(slow)/float64(len(intervals)))
	}
	if p.statsOn {
		col := func(f func(frameSample) float64) []float64 {
			out := make([]float64, len(frames))
			for i, s := range frames {
				out[i] = f(s)
			}
			return out
		}
		fmt.Fprintf(&sb, "cpu frame ms:      %s\n", pctl(col(func(s frameSample) float64 { return s.total })))
		fmt.Fprintf(&sb, "  build ms:        %s\n", pctl(col(func(s frameSample) float64 { return s.build })))
		fmt.Fprintf(&sb, "  layout ms:       %s\n", pctl(col(func(s frameSample) float64 { return s.layout })))
		fmt.Fprintf(&sb, "  paint ms:        %s\n", pctl(col(func(s frameSample) float64 { return s.paint })))
		fmt.Fprintf(&sb, "  present ms:      %s\n", pctl(col(func(s frameSample) float64 { return s.present })))
	} else {
		builds := make([]float64, len(frames))
		for i, s := range frames {
			builds[i] = s.build
		}
		fmt.Fprintf(&sb, "view build ms:     %s\n", pctl(builds))
		sb.WriteString("(set MYGO_FRAME_STATS=all for layout/paint/present breakdown)\n")
	}
	if before != nil && after != nil {
		fmt.Fprintf(&sb, "memory before: working set %.1f MB, private %.1f MB, go heap live %.1f MB (objects %.1f MB)\n", mb(before.workingSet), mb(before.private), mb(before.heapLive), mb(before.heapAlloc))
		fmt.Fprintf(&sb, "memory after:  working set %.1f MB, private %.1f MB, go heap live %.1f MB (objects %.1f MB)\n", mb(after.workingSet), mb(after.private), mb(after.heapLive), mb(after.heapAlloc))
	}
	fmt.Fprintf(&sb, "startup: first frame %d ms after process start\n", p.firstFrameMs.Load())
	return sb.String()
}

func pctl(v []float64) string {
	if len(v) == 0 {
		return "n/a"
	}
	s := slices.Clone(v)
	slices.Sort(s)
	at := func(q float64) float64 { return s[min(len(s)-1, int(q*float64(len(s))))] }
	sum := 0.0
	for _, x := range s {
		sum += x
	}
	return fmt.Sprintf("avg %.2f  p50 %.2f  p95 %.2f  p99 %.2f  max %.2f", sum/float64(len(s)), at(0.5), at(0.95), at(0.99), s[len(s)-1])
}
