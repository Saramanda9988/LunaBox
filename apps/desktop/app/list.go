package app

import (
	"fmt"

	"lunabox/backend/appconf"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/host"
)

// listBackend 用真实服务实现 ui.Backend（游戏库的列表查询）。
type listBackend struct{ svc *host.Backend }

func (b listBackend) GetGames(req vo.GameListRequest) (vo.GameListResponse, error) {
	return b.svc.Game.GetGames(req)
}

func (b listBackend) SearchTagsInLibrary(query string) ([]string, error) {
	return b.svc.Tag.SearchTagsInLibrary(query)
}

func (b listBackend) FilterExistingTagNames(names []string) ([]string, error) {
	return b.svc.Tag.FilterExistingTagNames(names)
}

func (b listBackend) ListGameFilterPresets() ([]models.GameFilterPreset, error) {
	return b.svc.GameFilterPreset.ListGameFilterPresets()
}

func (b listBackend) Config() *appconf.AppConfig { return b.svc.Config }
func (b listBackend) DataDir() string            { return b.svc.DataDir }

// repeatedBackend 把列表虚拟放大 n 倍，用于压测大列表；封面相同，所以封面缓存不会随之膨胀。
type repeatedBackend struct {
	listBackend
	n int
}

func (r repeatedBackend) GetGames(req vo.GameListRequest) (vo.GameListResponse, error) {
	var base []models.Game
	page := req
	page.Limit = 240 // gamehelper.MaxGameListLimit
	for page.Offset = 0; ; page.Offset += page.Limit {
		resp, err := r.listBackend.GetGames(page)
		if err != nil {
			return resp, err
		}
		base = append(base, resp.Games...)
		if !resp.HasMore || len(resp.Games) == 0 {
			break
		}
	}
	if len(base) == 0 {
		return vo.GameListResponse{Limit: req.Limit, Offset: req.Offset}, nil
	}
	resp := vo.GameListResponse{Games: base}
	n := len(resp.Games)
	out := vo.GameListResponse{Limit: req.Limit, Offset: req.Offset, Total: n * r.n}
	for i := req.Offset; i < min(out.Total, req.Offset+req.Limit); i++ {
		g := resp.Games[i%n]
		if copyIndex := i / n; copyIndex > 0 {
			g.ID = fmt.Sprintf("%s#%d", g.ID, copyIndex)
			g.Name = fmt.Sprintf("%s #%d", g.Name, copyIndex)
		}
		out.Games = append(out.Games, g)
	}
	out.HasMore = req.Offset+len(out.Games) < out.Total
	return out, nil
}
