package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MoeForge/moejs"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/hooks"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
)

type gamePatch struct {
	Name           *string          `json:"name,omitempty"`
	Aliases        *[]string        `json:"aliases,omitempty"`
	Company        *string          `json:"company,omitempty"`
	Summary        *string          `json:"summary,omitempty"`
	Rating         *float64         `json:"rating,omitempty"`
	ReleaseDate    *string          `json:"release_date,omitempty"`
	Status         *string          `json:"status,omitempty"`
	CoverURL       *string          `json:"cover_url,omitempty"`
	IsNSFW         *bool            `json:"is_nsfw,omitempty"`
	MetadataLocked *bool            `json:"metadata_locked,omitempty"`
	SourceType     *enums.SourceType `json:"source_type,omitempty"`
	SourceID       *string          `json:"source_id,omitempty"`
}

func applyGamePatch(g *models.Game, patch gamePatch) {
	if patch.Name != nil {
		g.Name = *patch.Name
	}
	if patch.Aliases != nil {
		g.Aliases = *patch.Aliases
	}
	if patch.Company != nil {
		g.Company = *patch.Company
	}
	if patch.Summary != nil {
		g.Summary = *patch.Summary
	}
	if patch.Rating != nil {
		g.Rating = *patch.Rating
	}
	if patch.ReleaseDate != nil {
		g.ReleaseDate = *patch.ReleaseDate
	}
	if patch.Status != nil {
		g.Status = enums.GameStatus(*patch.Status)
	}
	if patch.CoverURL != nil {
		g.CoverURL = *patch.CoverURL
	}
	if patch.IsNSFW != nil {
		g.IsNSFW = *patch.IsNSFW
	}
	if patch.MetadataLocked != nil {
		g.MetadataLocked = *patch.MetadataLocked
	}
	if patch.SourceType != nil && patch.SourceID != nil {
		g.SourceType = *patch.SourceType
		g.SourceID = *patch.SourceID
	}
}

func (p *Plugin) buildGamesAPI() map[string]any {
	return map[string]any{
		"list": p.asyncNative("games.list", func(_ context.Context, args []json.RawMessage) (any, error) {
			if p.manager.deps.Game == nil {
				return nil, fmt.Errorf("GameService 未初始化")
			}
			req := vo.GameListRequest{Limit: 100}
			if len(args) > 0 && len(args[0]) > 0 {
				var query struct {
					vo.GameListRequest
					Query string `json:"query,omitempty"`
				}
				if err := json.Unmarshal(args[0], &query); err == nil {
					req = query.GameListRequest
					if req.SearchQuery == "" && query.Query != "" {
						req.SearchQuery = query.Query
					}
					if req.Limit <= 0 {
						req.Limit = 100
					}
				}
			}
			return p.manager.deps.Game.GetGames(req)
		}),
		"get": p.asyncNative("games.get", func(_ context.Context, args []json.RawMessage) (any, error) {
			if p.manager.deps.Game == nil {
				return nil, fmt.Errorf("GameService 未初始化")
			}
			var id string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &id)
			}
			return p.manager.deps.Game.GetGameByID(id)
		}),
		"findBySource": p.asyncNative("games.findBySource", func(_ context.Context, args []json.RawMessage) (any, error) {
			if p.manager.deps.Game == nil {
				return nil, fmt.Errorf("GameService 未初始化")
			}
			var source, sourceID string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &source)
			}
			if len(args) > 1 {
				_ = json.Unmarshal(args[1], &sourceID)
			}
			source = strings.ToLower(strings.TrimSpace(source))
			sourceID = strings.TrimSpace(sourceID)
			if source == "" || sourceID == "" {
				return nil, nil
			}
			srcType := enums.SourceType(source)
			resp, err := p.manager.deps.Game.GetGames(vo.GameListRequest{
				Limit:          5000,
				MetadataSource: &srcType,
			})
			if err != nil {
				return nil, err
			}
			for _, g := range resp.Games {
				if strings.EqualFold(string(g.SourceType), source) && strings.EqualFold(strings.TrimSpace(g.SourceID), sourceID) {
					return g, nil
				}
				for _, ms := range g.MetadataSources {
					if strings.EqualFold(string(ms.SourceType), source) && strings.EqualFold(strings.TrimSpace(ms.SourceID), sourceID) {
						return g, nil
					}
				}
			}
			return nil, nil
		}),
		"update": p.asyncNative("games.update", func(_ context.Context, args []json.RawMessage) (any, error) {
			if p.manager.deps.Game == nil {
				return nil, fmt.Errorf("GameService 未初始化")
			}
			var id string
			var patch gamePatch
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &id)
			}
			if len(args) > 1 {
				if err := json.Unmarshal(args[1], &patch); err != nil {
					return nil, err
				}
			}
			game, err := p.manager.deps.Game.GetGameByID(id)
			if err != nil {
				return nil, err
			}
			applyGamePatch(&game, patch)
			if err := p.manager.deps.Game.UpdateGame(game); err != nil {
				return nil, err
			}
			return game, nil
		}),
		"batchUpdate": p.asyncNative("games.batchUpdate", func(_ context.Context, args []json.RawMessage) (any, error) {
			if p.manager.deps.Game == nil {
				return nil, fmt.Errorf("GameService 未初始化")
			}
			type batchItem struct {
				ID    string    `json:"id"`
				Patch gamePatch `json:"patch"`
			}
			var items []batchItem
			if len(args) > 0 {
				if err := json.Unmarshal(args[0], &items); err != nil {
					return nil, err
				}
			}
			updated := 0
			for _, item := range items {
				game, err := p.manager.deps.Game.GetGameByID(item.ID)
				if err != nil {
					continue
				}
				applyGamePatch(&game, item.Patch)
				if err := p.manager.deps.Game.UpdateGame(game); err == nil {
					updated++
				}
			}
			return map[string]int{"updated": updated}, nil
		}),
		"setTags": p.asyncNative("games.setTags", func(_ context.Context, args []json.RawMessage) (any, error) {
			if p.manager.deps.Tag == nil {
				return nil, fmt.Errorf("TagService 未初始化")
			}
			var gameID string
			var tags []string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &gameID)
			}
			if len(args) > 1 {
				_ = json.Unmarshal(args[1], &tags)
			}
			existing, err := p.manager.deps.Tag.GetTagsByGame(gameID)
			if err != nil {
				return nil, err
			}
			existingMap := make(map[string]string, len(existing))
			for _, t := range existing {
				existingMap[strings.ToLower(strings.TrimSpace(t.Name))] = t.ID
			}
			desired := make(map[string]string, len(tags))
			for _, name := range tags {
				trimmed := strings.TrimSpace(name)
				if trimmed == "" {
					continue
				}
				desired[strings.ToLower(trimmed)] = trimmed
			}
			var toDelete []string
			for _, t := range existing {
				if t.Source == "user" {
					if _, keep := desired[strings.ToLower(strings.TrimSpace(t.Name))]; !keep {
						toDelete = append(toDelete, t.ID)
					}
				}
			}
			if len(toDelete) > 0 {
				_ = p.manager.deps.Tag.DeleteTags(toDelete)
			}
			for key, name := range desired {
				if _, ok := existingMap[key]; !ok {
					_ = p.manager.deps.Tag.AddUserTag(gameID, name)
				}
			}
			return p.manager.deps.Tag.GetTagsByGame(gameID)
		}),
		"start": p.asyncNative("games.start", func(_ context.Context, args []json.RawMessage) (any, error) {
			if p.manager.deps.Start == nil {
				return nil, fmt.Errorf("StartService 未初始化")
			}
			var gameID string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &gameID)
			}
			return p.manager.deps.Start.StartGameWithTracking(gameID)
		}),
		"onChanged": p.safeNative("games.onChanged", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			fn := moejs.Arg(args, 0)
			if !fn.IsObject() {
				return moejs.Undefined(), throwJSError(r, "games.onChanged 需要函数参数")
			}
			d1 := p.scope.Effect(p.manager.hooks.add(p, hooks.EventGameAdded, fn, false))
			d2 := p.scope.Effect(p.manager.hooks.add(p, hooks.EventGameUpdated, fn, false))
			d3 := p.scope.Effect(p.manager.hooks.add(p, hooks.EventGameDeleted, fn, false))
			return p.rt.Function("dispose", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				d1()
				d2()
				d3()
				return moejs.Undefined(), nil
			}), nil
		}),
	}
}

func (p *Plugin) buildCategoriesAPI() map[string]any {
	return map[string]any{
		"list": p.asyncNative("categories.list", func(_ context.Context, _ []json.RawMessage) (any, error) {
			if p.manager.deps.Category == nil {
				return nil, fmt.Errorf("CategoryService 未初始化")
			}
			return p.manager.deps.Category.GetCategories()
		}),
		"create": p.asyncNative("categories.create", func(_ context.Context, args []json.RawMessage) (any, error) {
			if p.manager.deps.Category == nil {
				return nil, fmt.Errorf("CategoryService 未初始化")
			}
			var input struct {
				Name  string `json:"name"`
				Emoji string `json:"emoji"`
			}
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &input)
			}
			if err := p.manager.deps.Category.AddCategory(input.Name, input.Emoji); err != nil {
				return nil, err
			}
			cats, err := p.manager.deps.Category.GetCategories()
			if err != nil {
				return nil, err
			}
			for i := len(cats) - 1; i >= 0; i-- {
				if cats[i].Name == input.Name {
					return cats[i], nil
				}
			}
			return nil, nil
		}),
		"update": p.asyncNative("categories.update", func(_ context.Context, args []json.RawMessage) (any, error) {
			if p.manager.deps.Category == nil {
				return nil, fmt.Errorf("CategoryService 未初始化")
			}
			var id string
			var patch struct {
				Name  *string `json:"name"`
				Emoji *string `json:"emoji"`
			}
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &id)
			}
			if len(args) > 1 {
				_ = json.Unmarshal(args[1], &patch)
			}
			cur, err := p.manager.deps.Category.GetCategoryByID(id)
			if err != nil {
				return nil, err
			}
			name, emoji := cur.Name, cur.Emoji
			if patch.Name != nil {
				name = *patch.Name
			}
			if patch.Emoji != nil {
				emoji = *patch.Emoji
			}
			return nil, p.manager.deps.Category.UpdateCategory(id, name, emoji)
		}),
		"delete": p.asyncNative("categories.delete", func(_ context.Context, args []json.RawMessage) (any, error) {
			if p.manager.deps.Category == nil {
				return nil, fmt.Errorf("CategoryService 未初始化")
			}
			var id string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &id)
			}
			return nil, p.manager.deps.Category.DeleteCategory(id)
		}),
		"addGames": p.asyncNative("categories.addGames", func(_ context.Context, args []json.RawMessage) (any, error) {
			if p.manager.deps.Category == nil {
				return nil, fmt.Errorf("CategoryService 未初始化")
			}
			var catID string
			var gameIDs []string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &catID)
			}
			if len(args) > 1 {
				_ = json.Unmarshal(args[1], &gameIDs)
			}
			return nil, p.manager.deps.Category.AddGamesToCategories(gameIDs, []string{catID})
		}),
		"removeGames": p.asyncNative("categories.removeGames", func(_ context.Context, args []json.RawMessage) (any, error) {
			if p.manager.deps.Category == nil {
				return nil, fmt.Errorf("CategoryService 未初始化")
			}
			var catID string
			var gameIDs []string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &catID)
			}
			if len(args) > 1 {
				_ = json.Unmarshal(args[1], &gameIDs)
			}
			return nil, p.manager.deps.Category.RemoveGamesFromCategory(gameIDs, catID)
		}),
	}
}
