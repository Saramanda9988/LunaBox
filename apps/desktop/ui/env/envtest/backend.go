// Package envtest 提供测试用的内存后端。
package envtest

import (
	"fmt"
	"strings"
	"time"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
)

// Backend 在内存中实现 env.Backend，按名称搜索与状态筛选，其余条件忽略。
type Backend struct {
	Games []models.Game
	Tags  []string // 库里的标签名（SearchTagsInLibrary、FilterExistingTagNames）
	Cfg   appconf.AppConfig
	Reqs  []vo.GameListRequest

	Presets []models.GameFilterPreset // ListGameFilterPresets 的结果
}

// NewBackend 返回 n 个游戏、侧边栏展开、封面显示排序字段、显示标签译名（同默认配置）的后端。
func NewBackend(n int) *Backend {
	b := &Backend{Cfg: appconf.AppConfig{SidebarOpen: true, ShowSortFieldOnCover: true, EnableTagTranslation: true}}
	names := []string{"橘光柠水随想曲", "Summer Pockets", "星空鉄道とシロの旅"}
	for i := range n {
		b.Games = append(b.Games, models.Game{
			ID:        fmt.Sprint(i),
			Name:      names[i%len(names)],
			Company:   "ゆずソフト",
			CreatedAt: time.Date(2026, 9, 11, 12, 0, 0, 0, time.Local),
			Status:    []enums.GameStatus{enums.StatusNotStarted, enums.StatusCompleted}[i%2],
		})
	}
	return b
}

func (b *Backend) GetGames(req vo.GameListRequest) (vo.GameListResponse, error) {
	b.Reqs = append(b.Reqs, req)
	var match []models.Game
	for _, g := range b.Games {
		if req.SearchQuery != "" && !strings.Contains(strings.ToLower(g.Name), strings.ToLower(req.SearchQuery)) {
			continue
		}
		if req.Status != nil && (g.Status == *req.Status) == req.ExcludeStatus {
			continue
		}
		match = append(match, g)
	}
	resp := vo.GameListResponse{Total: len(match), Offset: req.Offset, Limit: req.Limit}
	if req.Offset < len(match) {
		resp.Games = match[req.Offset:min(len(match), req.Offset+req.Limit)]
	}
	return resp, nil
}

// SearchTagsInLibrary 返回名称包含 query 的标签（不区分大小写）。
func (b *Backend) SearchTagsInLibrary(query string) ([]string, error) {
	var out []string
	for _, tag := range b.Tags {
		if strings.Contains(strings.ToLower(tag), strings.ToLower(query)) {
			out = append(out, tag)
		}
	}
	return out, nil
}

// FilterExistingTagNames 返回 names 里库中已有的标签，保留输入顺序。
func (b *Backend) FilterExistingTagNames(names []string) ([]string, error) {
	var out []string
	for _, n := range names {
		for _, tag := range b.Tags {
			if n == tag {
				out = append(out, n)
				break
			}
		}
	}
	return out, nil
}

func (b *Backend) ListGameFilterPresets() ([]models.GameFilterPreset, error) {
	return append([]models.GameFilterPreset(nil), b.Presets...), nil
}
func (b *Backend) Config() *appconf.AppConfig { return &b.Cfg }
func (b *Backend) DataDir() string            { return "" }
