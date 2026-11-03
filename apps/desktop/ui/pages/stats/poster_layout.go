package stats

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"lunabox/backend/common/vo"
)

// 封面拼贴海报的几何，对应原版 utils/posterLayout.ts 与 utils/posterCrop.ts：与画布无关，预览与导出用同一份。
// 计算用 float64，与 JavaScript 的数字一致，TestPosterLayoutMatchesFrontend 拿原版算出的结果逐项对照。

const (
	posterWidth  = 1080 // POSTER_WIDTH
	posterHeight = 1440 // POSTER_HEIGHT
)

// posterTile 是 PosterTile：海报坐标里的封面框，lift 是前两名的“抬起”程度，rotation 是角度。
type posterTile struct {
	game                vo.PosterGame
	x, y, width, height float64
	lift, rotation      float64
}

// rankedPosterGames 对应 rankedPosterGames：去掉没有时长的游戏，同一游戏只留一个（位置取第一次出现、内容取最后一次，
// 同 Map 的 set），按时长降序、ID 升序排列。原版用 localeCompare 比较 ID，ID 是 ASCII 时与字节序相同。
func rankedPosterGames(games []vo.PosterGame) []vo.PosterGame {
	index := map[string]int{}
	var out []vo.PosterGame
	for _, g := range games {
		if g.TotalDuration <= 0 {
			continue
		}
		if i, ok := index[g.GameID]; ok {
			out[i] = g
			continue
		}
		index[g.GameID] = len(out)
		out = append(out, g)
	}
	slices.SortStableFunc(out, func(a, b vo.PosterGame) int {
		if a.TotalDuration != b.TotalDuration {
			return b.TotalDuration - a.TotalDuration
		}
		return strings.Compare(a.GameID, b.GameID)
	})
	return out
}

// posterHash 是 hash：FNV 式的 Math.imul 散列，逐个码点取第一个 UTF-16 码元（charCodeAt(0)）。
func posterHash(value string, seed int) float64 {
	result := int32(seed)
	for _, r := range value {
		code := r
		if r >= 0x10000 {
			code = 0xD800 + (r-0x10000)>>10
		} else if r == utf8.RuneError {
			code = 0xFFFD
		}
		result = (result ^ int32(code)) * 16777619
	}
	return float64(uint32(result)) / 4294967296
}

// posterPresets 是 6 个以内、时长不全相同时的固定排布：[x, y, width, height]。
var posterPresets = map[int][][4]float64{
	1: {{330, 350, 420, 620}},
	2: {{215, 390, 330, 500}, {555, 410, 310, 470}},
	3: {{382, 400, 316, 500}, {104, 470, 272, 360}, {704, 470, 272, 360}},
	4: {{382, 477, 316, 456}, {104, 525, 272, 360}, {704, 525, 272, 360}, {382, 311, 316, 160}},
	5: {{382, 477, 316, 456}, {104, 525, 272, 360}, {704, 525, 272, 360}, {382, 311, 316, 160}, {382, 939, 316, 160}},
	6: {{382, 477, 316, 456}, {104, 525, 272, 360}, {704, 525, 272, 360}, {382, 311, 316, 160}, {263, 939, 272, 160}, {541, 939, 272, 160}},
}

// posterShapes 是 shapes：每种面积（以 2×2 的格子块计）可以摆成的宽高。
func posterShapes(units int) [][2]int {
	switch units {
	case 12:
		return [][2]int{{6, 8}, {8, 6}}
	case 9:
		return [][2]int{{6, 6}}
	case 6:
		return [][2]int{{4, 6}, {6, 4}}
	case 4:
		return [][2]int{{4, 4}}
	case 2:
		return [][2]int{{2, 4}, {4, 2}}
	}
	return [][2]int{{2, 2}}
}

// buildPosterLayout 对应 buildPosterLayout：每个游戏都出现一次。6 个以内用固定排布（seed 为偶数时左右镜像）；
// 更多时按时长对数分配面积，在中央聚拢的椭圆网格里依次找离中心最近的空位，放不下就加两列重来。
func buildPosterLayout(input []vo.PosterGame, seed int) []posterTile {
	games := rankedPosterGames(input)
	if len(games) == 0 {
		return nil
	}
	tied := true
	for _, g := range games {
		if g.TotalDuration != games[0].TotalDuration {
			tied = false
			break
		}
	}
	tile := func(g vo.PosterGame, frame [4]float64, rank int) posterTile {
		t := posterTile{game: g, x: frame[0], y: frame[1], width: frame[2], height: frame[3]}
		if !tied {
			switch rank {
			case 0:
				t.lift, t.rotation = 1, -1.6
			case 1:
				t.lift, t.rotation = 0.6, 1.2
			}
		}
		return t
	}
	if len(games) <= 6 && (!tied || len(games) == 1) {
		out := make([]posterTile, len(games))
		for i, g := range games {
			frame := posterPresets[len(games)][i]
			if seed%2 == 0 {
				frame[0] = posterWidth - frame[0] - frame[2]
			}
			out[i] = tile(g, frame, i)
		}
		return out
	}

	minimum, maximum := float64(games[len(games)-1].TotalDuration), float64(games[0].TotalDuration)
	firstRanks := map[int]int{}
	for i, g := range games {
		if _, ok := firstRanks[g.TotalDuration]; !ok {
			firstRanks[g.TotalDuration] = i
		}
	}
	units := make([]int, len(games))
	cells := 0
	for i, g := range games {
		weight, podium := 0.0, 1
		if !tied {
			weight = (math.Log1p(float64(g.TotalDuration)) - math.Log1p(minimum)) / (math.Log1p(maximum) - math.Log1p(minimum))
			if r := firstRanks[g.TotalDuration]; r < 3 {
				podium = []int{12, 9, 6}[r]
			}
		}
		u := 1
		switch {
		case weight > 0.82:
			u = 4
		case weight > 0.45:
			u = 2
		}
		units[i] = max(podium, u)
		cells += units[i] * 4
	}

	columns := max(10, int(math.Ceil(math.Sqrt(float64(cells)*952/930)/2))*2)
	for {
		gap := 8.0
		switch {
		case len(games) > 80:
			gap = 3
		case len(games) > 24:
			gap = 5
		}
		gap = math.Min(gap, 952/float64(columns*5))
		cell := (952 - float64(columns-1)*gap) / float64(columns)
		rows := max(8, int(math.Floor((930+gap)/(cell+gap)/2))*2)
		grid := make([]bool, columns*rows)
		for y := 0; y < rows; y += 2 {
			for x := 0; x < columns; x += 2 {
				nx := math.Abs((float64(x+1)/float64(columns) - 0.5) * 2)
				ny := math.Abs((float64(y+1)/float64(rows) - 0.5) * 2)
				if math.Pow(nx, 1.5)+math.Pow(ny, 1.5) > 1.28 {
					for a := y; a < y+2; a++ {
						for b := x; b < x+2; b++ {
							grid[a*columns+b] = true
						}
					}
				}
			}
		}
		var result []posterTile
		for index, g := range games {
			var best [4]int
			found, score := false, math.Inf(1)
			for y := 0; y < rows; y += 2 {
				for x := 0; x < columns; x += 2 {
					for _, s := range posterShapes(units[index]) {
						w, h := s[0], s[1]
						if x+w > columns || y+h > rows || !posterFree(grid, columns, x, y, w, h) {
							continue
						}
						cx, cy := (float64(x)+float64(w)/2)/float64(columns)-0.5, (float64(y)+float64(h)/2)/float64(rows)-0.5
						value := (cx*cx + cy*cy) * 12
						if w > h && units[index] >= 6 {
							value += 0.12
						}
						value += posterHash(g.GameID+":"+strconv.Itoa(x)+":"+strconv.Itoa(y)+":"+strconv.Itoa(w), seed) * 0.035
						if value < score {
							score, best, found = value, [4]int{x, y, w, h}, true
						}
					}
				}
			}
			if !found {
				break
			}
			x, y, w, h := best[0], best[1], best[2], best[3]
			for a := y; a < y+h; a++ {
				for b := x; b < x+w; b++ {
					grid[a*columns+b] = true
				}
			}
			result = append(result, tile(g, [4]float64{
				64 + float64(x)*(cell+gap),
				252 + float64(y)*(cell+gap),
				float64(w)*cell + float64(w-1)*gap,
				float64(h)*cell + float64(h-1)*gap,
			}, index))
		}
		if len(result) == len(games) {
			return result
		}
		columns += 2
	}
}

func posterFree(grid []bool, columns, x, y, w, h int) bool {
	for a := y; a < y+h; a++ {
		for b := x; b < x+w; b++ {
			if grid[a*columns+b] {
				return false
			}
		}
	}
	return true
}

// posterCrop 是 PosterCrop：封面在框里的取景，x、y 是 0～1 的位置，zoom 是 1～3 的放大倍数。
type posterCrop struct{ x, y, zoom float64 }

var defaultCrop = posterCrop{0.5, 0.5, 1}

// normalizeCrop 对应 normalizeCrop；零值（没有调整过）当作默认取景。
func normalizeCrop(c posterCrop) posterCrop {
	if c == (posterCrop{}) {
		return defaultCrop
	}
	clamp := func(v, fallback, lo, hi float64) float64 {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			v = fallback
		}
		return math.Min(hi, math.Max(lo, v))
	}
	return posterCrop{clamp(c.x, 0.5, 0, 1), clamp(c.y, 0.5, 0, 1), clamp(c.zoom, 1, 1, 3)}
}

// posterCoverSource 对应 posterCoverSource：iw×ih 的封面铺满 width×height 的框（object-fit: cover）再按取景放大、平移后，
// 框里显示的那部分在封面上的位置。
func posterCoverSource(iw, ih, width, height float64, c posterCrop) (x, y, w, h float64) {
	c = normalizeCrop(c)
	scale := math.Max(width/iw, height/ih) * c.zoom
	w, h = width/scale, height/scale
	return (iw - w) * c.x, (ih - h) * c.y, w, h
}

// panPosterCrop 对应 panPosterCrop：在 width×height 的取景框里拖动 dx、dy 后的取景。
func panPosterCrop(start posterCrop, dx, dy, iw, ih, width, height float64) posterCrop {
	start = normalizeCrop(start)
	scale := math.Max(width/iw, height/ih) * start.zoom
	next := posterCrop{start.x, start.y, start.zoom}
	if iw*scale-width > 0.001 {
		next.x = start.x - dx/(iw*scale-width)
	}
	if ih*scale-height > 0.001 {
		next.y = start.y - dy/(ih*scale-height)
	}
	return normalizeCrop(next)
}

// zoomPosterCrop 对应 zoomPosterCrop：保持取景中心不动地缩放。
func zoomPosterCrop(start posterCrop, zoom, iw, ih, width, height float64) posterCrop {
	start = normalizeCrop(start)
	ox, oy, ow, oh := posterCoverSource(iw, ih, width, height, start)
	_, _, nw, nh := posterCoverSource(iw, ih, width, height, posterCrop{start.x, start.y, zoom})
	next := posterCrop{start.x, start.y, zoom}
	if iw-nw > 0.001 {
		next.x = (ox + ow/2 - nw/2) / (iw - nw)
	}
	if ih-nh > 0.001 {
		next.y = (oy + oh/2 - nh/2) / (ih - nh)
	}
	return normalizeCrop(next)
}

// hitPosterTile 对应海报画布的点击：按抬起程度从高到低，找 (px, py)（海报坐标）落在哪个旋转、放大后的封面里。
func hitPosterTile(tiles []posterTile, px, py float64) (posterTile, bool) {
	order := slices.Clone(tiles)
	slices.SortStableFunc(order, func(a, b posterTile) int {
		switch {
		case a.lift > b.lift:
			return -1
		case a.lift < b.lift:
			return 1
		}
		return 0
	})
	for _, t := range order {
		angle := -t.rotation * math.Pi / 180
		dx := px - t.x - t.width/2
		dy := py - t.y - t.height/2 + t.lift*7
		growth := 1 + t.lift*0.045
		x := (dx*math.Cos(angle)-dy*math.Sin(angle))/growth + t.width/2
		y := (dx*math.Sin(angle)+dy*math.Cos(angle))/growth + t.height/2
		if x >= 0 && x <= t.width && y >= 0 && y <= t.height {
			return t, true
		}
	}
	return posterTile{}, false
}
