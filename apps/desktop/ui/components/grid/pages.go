package grid

import (
	"fmt"

	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/env"
)

// 与 routes/library.tsx、routes/category.tsx 相同的分页窗口。
const (
	PageSize          = 120
	windowBufferSize  = PageSize
	windowRequestSize = PageSize * 2
)

// Fetch 在后台读取 [offset, offset+limit) 的游戏与符合条件的总数。
type Fetch func(offset, limit int) (games []models.Game, total int, err error)

// Pages 是 VirtualGrid 的数据：按下标缓存的分页结果。查询条件变化后 Reload，可见范围有缺口时
// Ensure 请求覆盖它的窗口（getWindowRequest）。
type Pages struct {
	env   *env.Env
	fetch Fetch

	Games     map[int]*models.Game
	Total     int
	Loading   bool   // 查询条件变化后第一个窗口还没返回；旧数据保留到新结果返回
	Key       string // 当前查询条件
	LoadedKey string // 已返回结果的查询条件
	OnError   func() // 请求失败（页面显示提示）

	gen      int
	inflight map[string]bool
}

// NewPages 创建空的分页数据，Loading 为真直到第一次 Reload 返回。
func NewPages(e *env.Env) *Pages {
	return &Pages{env: e, Games: map[int]*models.Game{}, inflight: map[string]bool{}, Loading: true}
}

// Reload 用新的查询条件重新请求第一个窗口；fetch 里带着这次的条件，之后补缺口也用它。
func (p *Pages) Reload(key string, fetch Fetch) {
	p.gen++
	p.Key, p.fetch = key, fetch
	p.Loading = true
	p.inflight = map[string]bool{}
	p.load(0, windowRequestSize, true)
}

// Waiting 对应 isEmptyListWaiting：还没有任何游戏，且结果未返回（或查询条件还在变）。
func (p *Pages) Waiting(settling bool) bool {
	return p.Total == 0 && (p.Loading || settling || p.LoadedKey != p.Key)
}

// Ensure 在可见范围 [start, end] 有缺口时请求覆盖它的窗口，用作 VirtualGrid 的 OnRange。
func (p *Pages) Ensure(start, end int) {
	if p.Total == 0 || p.Loading {
		return
	}
	bufferedStart := max(0, start-windowBufferSize)
	offset := bufferedStart / PageSize * PageSize
	reqEnd := min(p.Total, offset+windowRequestSize, max(end+1, offset+PageSize))
	for i := offset; i < reqEnd; i++ {
		if p.Games[i] == nil {
			p.load(offset, max(1, reqEnd-offset), false)
			return
		}
	}
}

func (p *Pages) load(offset, limit int, replace bool) {
	key := fmt.Sprintf("%d:%d:%d", p.gen, offset, limit)
	if p.inflight[key] || p.fetch == nil {
		return
	}
	p.inflight[key] = true
	gen, fetch := p.gen, p.fetch
	p.env.Async(func() func() {
		games, total, err := fetch(offset, limit)
		return func() {
			delete(p.inflight, key)
			if gen != p.gen {
				return
			}
			if replace {
				p.Games = map[int]*models.Game{}
			}
			for i := range games {
				p.Games[offset+i] = &games[i]
			}
			if err == nil {
				p.Total = total
			} else if p.OnError != nil {
				p.OnError()
			}
			p.Loading = false
			p.LoadedKey = p.Key
		}
	})
}
