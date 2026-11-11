package components

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"

	"lunabox/desktop/ui/theme"
)

// warpShader 是图标区的动态背景：仿 Paper Shaders 的 Warp 效果（shaders.paper.design/warp）自己写的片元着色器。
// 坐标先被两层值噪声扭曲、再迭代叠加漩涡，得到流动的棋盘图案，最后在四种颜色之间柔和地混合；末尾加一点抖动，
// 避免渐变出现色带。
var warpShader = shader.MustCompile(`package main

var Time float
var Colors [4]vec4 // not premultiplied
var Scale float     // pattern units per DIP
var Proportion float
var Softness float
var Distortion float
var Swirl float
var SwirlIterations int
var ShapeFreq float // checks per pattern unit, in radians

func hash(p vec2) float {
	p = fract(p * vec2(123.34, 456.21))
	p += dot(p, p+45.32)
	return fract(p.x * p.y)
}

func valueNoise(p vec2) float {
	i := floor(p)
	f := fract(p)
	a := hash(i)
	b := hash(i + vec2(1, 0))
	c := hash(i + vec2(0, 1))
	d := hash(i + vec2(1, 1))
	u := f * f * (3 - 2*f)
	return mix(mix(a, b, u.x), mix(c, d, u.x), u.y)
}

func Fragment(dst vec4, pos vec2) vec4 {
	uv := (pos - 0.5*rectSize()) * Scale
	t := 0.0625 * (Time + 118)

	// Warp: noise gives the direction and strength.
	n1 := valueNoise(uv + t)
	n2 := valueNoise(uv*2 - t)
	angle := n1 * 6.28318530718
	uv += 4 * Distortion * n2 * vec2(cos(angle), sin(angle))

	// Swirl: each layer weaker than the last.
	for i := 1; i <= 20; i++ {
		if i <= SwirlIterations {
			fi := float(i)
			uv.x += Swirl / fi * cos(t + fi*1.5*uv.y)
			uv.y += Swirl / fi * cos(t + fi*uv.x)
		}
	}

	// Checks; Proportion shifts how much each color covers.
	f := ShapeFreq
	shape := 0.5 + 0.5*sin(uv.x*f)*sin(uv.y*f)
	shape = clamp(shape+(Proportion-0.5), 0, 1)

	// Blend the four colors in turn; Softness is the width of each transition.
	mixer := shape * 3
	col := Colors[0]
	for i := 1; i < 4; i++ {
		m := clamp(mixer-float(i-1), 0, 1)
		edge := 0.5 * clamp(Softness, 0.02, 1)
		m = smoothstep(0.5-edge, 0.5+edge, m)
		col = mix(col, Colors[i], m)
	}

	rgb := col.rgb + (hash(dst.xy*0.014)-0.5)/256
	return vec4(rgb*col.a, col.a)
}
`)

var warpStart = time.Now()

// warpColors 是四种颜色，对应 Warp 默认的 #121212、#9470ff、#121212、#8838ff：深色主题是近黑与亮、暗两种主题紫，
// 浅色主题是近白与两种淡紫。
func warpColors(t *theme.Theme) []float32 {
	cs := []ui.Color{theme.Brand900, theme.Primary500, theme.Brand900, theme.Primary700}
	if !t.Dark {
		cs = []ui.Color{theme.Brand50, theme.Primary300, theme.Brand50, theme.Primary400}
	}
	out := make([]float32, 0, 16)
	for _, c := range cs {
		out = append(out, float32(c.R)/255, float32(c.G)/255, float32(c.B)/255, float32(c.A)/255)
	}
	return out
}

// drawWarp 在 r 里画动态背景并请求下一次重画。没有 GPU 加速时什么也不画（着色器在 CPU 上逐像素解释执行，太慢），
// 露出元素自己的底色。
func drawWarp(p *ui.Painter, r ui.Rect, t *theme.Theme) {
	if !p.Accelerated() {
		return
	}
	warpShader.Draw(p, r, shader.Options{Uniforms: map[string]any{
		"Time":            float32(p.Now().Sub(warpStart).Seconds()),
		"Colors":          warpColors(t),
		"Scale":           float32(0.005), // 同 Warp 的 scale 1：每 DIP 0.005 个图案单位
		"Proportion":      float32(0.45),
		"Softness":        float32(1),
		"Distortion":      float32(0.25),
		"Swirl":           float32(0.6), // 比 Warp 默认的 0.8、10 层弱一些，少些细密的环纹
		"SwirlIterations": 8,
		"ShapeFreq":       float32(0.85), // 低频的棋盘：大块色域
	}})
	p.AnimationFrameRate(60) // 缓慢流动，60 帧足够平滑；只重画，不重建视图
}
