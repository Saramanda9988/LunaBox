package components

import (
	"fmt"
	"math"
)

// FormatFileSize 对应 utils/size.ts 的 formatFileSize：二进制单位，保留一位小数（0、负数与非有限值是 0 B）。
func FormatFileSize(bytes float64) string {
	if math.IsNaN(bytes) || math.IsInf(bytes, 0) || bytes <= 0 {
		return "0 B"
	}
	sizes := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	i := min(len(sizes)-1, max(0, int(math.Floor(math.Log(bytes)/math.Log(1024)))))
	v := bytes / math.Pow(1024, float64(i))
	// toFixed(1) 在恰好一半时进位（1280 B 是 1.3 KB），Go 的 %.1f 取偶数，所以先自己舍入。
	return fmt.Sprintf("%.1f %s", math.Floor(v*10+0.5)/10, sizes[i])
}
