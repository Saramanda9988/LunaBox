package theme

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var procSystemParametersInfo = windows.NewLazySystemDLL("user32.dll").NewProc("SystemParametersInfoW")

// nonClientMetrics 对应 NONCLIENTMETRICSW（不含 iPaddedBorderWidth 之后的扩展无影响）。
type nonClientMetrics struct {
	cbSize           uint32
	iBorderWidth     int32
	iScrollWidth     int32
	iScrollHeight    int32
	iCaptionWidth    int32
	iCaptionHeight   int32
	lfCaptionFont    logFont
	iSmCaptionWidth  int32
	iSmCaptionHeight int32
	lfSmCaptionFont  logFont
	iMenuWidth       int32
	iMenuHeight      int32
	lfMenuFont       logFont
	lfStatusFont     logFont
	lfMessageFont    logFont
	iPaddedBorder    int32
}

type logFont struct {
	height, width, escapement, orientation, weight int32
	italic, underline, strikeOut, charSet          byte
	outPrecision, clipPrecision, quality, pitch    byte
	faceName                                       [32]uint16
}

// systemUIFont 返回 Chromium 用作 CSS system-ui 的字体：系统消息字体（中文 Windows 上是 Microsoft YaHei UI）。
// 原版字体栈的 ui-sans-serif 在 Chromium 中不被支持，所以实际生效的就是它。
func systemUIFont() string {
	var m nonClientMetrics
	m.cbSize = uint32(unsafe.Sizeof(m))
	const spiGetNonClientMetrics = 0x0029
	r, _, _ := procSystemParametersInfo.Call(spiGetNonClientMetrics, uintptr(m.cbSize), uintptr(unsafe.Pointer(&m)), 0)
	if r == 0 {
		return ""
	}
	return windows.UTF16ToString(m.lfMessageFont.faceName[:])
}
