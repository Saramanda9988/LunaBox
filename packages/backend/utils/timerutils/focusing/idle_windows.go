//go:build windows

package focusing

import (
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// lastInputInfo 对应 Win32 LASTINPUTINFO。
type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

// xinputState 对应 XINPUT_STATE（含 XINPUT_GAMEPAD）。
type xinputState struct {
	dwPacketNumber uint32
	wButtons       uint16
	bLeftTrigger   uint8
	bRightTrigger  uint8
	sThumbLX       int16
	sThumbLY       int16
	sThumbRX       int16
	sThumbRY       int16
}

const (
	xinputMaxControllers = 4
	// 未连接的槽位查询开销较大，隔一段时间再探测。
	xinputDisconnectedRetry = 5 * time.Second
	// 与 XInput.h 中的默认死区一致，过滤摇杆漂移和扳机噪声。
	xinputLeftThumbDeadzone  = 7849
	xinputRightThumbDeadzone = 8689
	xinputTriggerThreshold   = 30
	errorSuccess             = 0
)

var (
	procGetLastInputInfo = user32.NewProc("GetLastInputInfo")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGetTickCount     = kernel32.NewProc("GetTickCount")

	gamepad gamepadActivity
)

// GetIdleDuration 返回用户最近一次输入距今的时长。
// 键鼠来自 GetLastInputInfo；XInput 手柄不会刷新系统空闲计时，单独轮询补上。
func GetIdleDuration() (time.Duration, bool) {
	info := lastInputInfo{cbSize: uint32(unsafe.Sizeof(lastInputInfo{}))}
	ret, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return 0, false
	}
	tick, _, _ := procGetTickCount.Call()
	// 两者都是 32 位毫秒计数，无符号减法可正确处理约 49.7 天的回绕。
	idle := time.Duration(uint32(tick)-info.dwTime) * time.Millisecond

	if gamepadIdle, ok := gamepad.idleDuration(time.Now()); ok && gamepadIdle < idle {
		idle = gamepadIdle
	}
	return idle, true
}

type gamepadSlot struct {
	connected bool
	nextProbe time.Time
	last      xinputState
}

type gamepadActivity struct {
	once      sync.Once
	proc      *syscall.LazyProc
	mu        sync.Mutex
	slots     [xinputMaxControllers]gamepadSlot
	lastInput time.Time
}

func (g *gamepadActivity) load() *syscall.LazyProc {
	g.once.Do(func() {
		// xinput1_4 随 Windows 8+ 提供；xinput9_1_0 兼容更老的系统。
		for _, name := range []string{"xinput1_4.dll", "xinput9_1_0.dll"} {
			dll := syscall.NewLazyDLL(name)
			if dll.Load() != nil {
				continue
			}
			proc := dll.NewProc("XInputGetState")
			if proc.Find() == nil {
				g.proc = proc
				return
			}
		}
	})
	return g.proc
}

// idleDuration 轮询所有手柄，返回最近一次有效手柄输入距今的时长；从未检测到输入时返回 false。
func (g *gamepadActivity) idleDuration(now time.Time) (time.Duration, bool) {
	proc := g.load()
	if proc == nil {
		return 0, false
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	for i := range g.slots {
		slot := &g.slots[i]
		if !slot.connected && now.Before(slot.nextProbe) {
			continue
		}
		var state xinputState
		ret, _, _ := proc.Call(uintptr(i), uintptr(unsafe.Pointer(&state)))
		if ret != errorSuccess {
			slot.connected = false
			slot.nextProbe = now.Add(xinputDisconnectedRetry)
			continue
		}
		if slot.connected && state.dwPacketNumber != slot.last.dwPacketNumber && isGamepadActive(slot.last, state) {
			g.lastInput = now
		}
		slot.connected = true
		slot.last = state
	}

	if g.lastInput.IsZero() {
		return 0, false
	}
	return now.Sub(g.lastInput), true
}

// isGamepadActive 判断两次状态之间是否有真实操作：按键变化，或扳机/摇杆越过死区。
// 只看数据包序号会被摇杆漂移误判为一直在操作。
func isGamepadActive(prev, cur xinputState) bool {
	if prev.wButtons != cur.wButtons {
		return true
	}
	if cur.bLeftTrigger > xinputTriggerThreshold || cur.bRightTrigger > xinputTriggerThreshold {
		return true
	}
	return outsideDeadzone(cur.sThumbLX, cur.sThumbLY, xinputLeftThumbDeadzone) ||
		outsideDeadzone(cur.sThumbRX, cur.sThumbRY, xinputRightThumbDeadzone)
}

func outsideDeadzone(x, y int16, deadzone int32) bool {
	return abs32(int32(x)) > deadzone || abs32(int32(y)) > deadzone
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}
