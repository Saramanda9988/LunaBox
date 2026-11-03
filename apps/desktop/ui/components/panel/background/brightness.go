package background

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/gen2brain/avif"
	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// fast-average-color 在 speed 模式下把图片缩到长边 100px 再取色（prepareSizeAndPosition 的 MAX_SIZE、MIN_SIZE）。
const (
	facMaxSize = 100
	facMinSize = 10
)

// localFile 把服务返回的 /local/... 地址（原版由 Wails 的资源服务映射到数据目录）换成数据目录里的文件路径。
func localFile(dataDir, src string) string {
	if rel, ok := strings.CutPrefix(src, "/local/"); ok {
		return filepath.Join(dataDir, filepath.FromSlash(rel))
	}
	return src
}

// DetectImageBrightness 对应 utils/detectImageBrightness.ts 的 detectImageBrightness：读取图片，用 fast-average-color
// 的默认算法取平均色，按 0.299R + 0.587G + 0.114B 算亮度，大于 0.5 为亮色背景。会阻塞，在后台 goroutine 里调用。
func DetectImageBrightness(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return false, fmt.Errorf("decode %s: %w", path, err)
	}
	return IsLightImage(img), nil
}

// IsLightImage 是 detectImageBrightness 读到图片之后的部分。
func IsLightImage(img image.Image) bool {
	c := averageColor(img)
	luminance := (0.299*c[0] + 0.587*c[1] + 0.114*c[2]) / 255
	return luminance > 0.5
}

// averageColor 对应 FastAverageColor.getColor 的默认选项（algorithm "sqrt"、mode "speed"、step 1）：先像 canvas 的
// drawImage 那样缩小，再对未预乘的 RGBA 求按透明度加权的平方平均，返回四舍五入后的 [r, g, b, a]。
func averageColor(img image.Image) [4]float64 {
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= 0 || sh <= 0 {
		return [4]float64{}
	}
	dw, dh := sw, sh
	if sw > sh {
		dw = facMaxSize
		dh = int(jsRound(float64(dw) / (float64(sw) / float64(sh))))
	} else {
		dh = facMaxSize
		dw = int(jsRound(float64(dh) / (float64(sh) / float64(sw))))
	}
	if dw > sw || dh > sh || dw < facMinSize || dh < facMinSize {
		dw, dh = sw, sh
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)

	var red, green, blue, alpha float64
	count := 0
	for i := 0; i+3 < len(dst.Pix); i += 4 {
		a := float64(dst.Pix[i+3])
		count++
		if a == 0 {
			continue
		}
		// getImageData 返回未预乘的颜色，image.RGBA 是预乘的。
		r := float64(dst.Pix[i]) * 255 / a
		g := float64(dst.Pix[i+1]) * 255 / a
		bl := float64(dst.Pix[i+2]) * 255 / a
		red += r * r * a
		green += g * g * a
		blue += bl * bl * a
		alpha += a
	}
	if alpha == 0 {
		return [4]float64{}
	}
	return [4]float64{
		jsRound(math.Sqrt(red / alpha)),
		jsRound(math.Sqrt(green / alpha)),
		jsRound(math.Sqrt(blue / alpha)),
		jsRound(alpha / float64(count)),
	}
}

// jsRound 同 Math.round。
func jsRound(v float64) float64 { return math.Floor(v + 0.5) }
