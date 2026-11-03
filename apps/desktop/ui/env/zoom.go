package env

import "math"

// ZoomLevels 对应 consts/options.ts 的 APP_ZOOM_LEVELS：界面缩放（window_zoom_factor）的各档。
var ZoomLevels = []float64{0.8, 0.9, 1, 1.1, 1.25, 1.5}

// NormalizeZoom 对应 normalizeAppZoomFactor：没有设置（0）或无效时是 1，其余取最近的一档。
func NormalizeZoom(v float64) float64 {
	if math.IsNaN(v) || v <= 0 {
		return 1
	}
	nearest := ZoomLevels[0]
	for _, z := range ZoomLevels {
		if math.Abs(v-z) < math.Abs(v-nearest) {
			nearest = z
		}
	}
	return nearest
}

// NextZoom 对应 getNextAppZoomFactor：从 cur 所在的一档往 dir（1 放大、-1 缩小）走一档，到头时不变。
func NextZoom(cur float64, dir int) float64 {
	cur = NormalizeZoom(cur)
	i := 0
	for j, z := range ZoomLevels {
		if z == cur {
			i = j
		}
	}
	return ZoomLevels[min(len(ZoomLevels)-1, max(0, i+dir))]
}
