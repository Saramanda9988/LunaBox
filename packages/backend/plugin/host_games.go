package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/MoeForge/moejs"
	"github.com/google/uuid"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/hooks"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	launcherpkg "lunabox/backend/service/launcher"
	"lunabox/backend/utils/metadata"
)

// libraryChangedEvent 通知界面插件改动了游戏库（游戏库页与首页据此重新加载）。
const libraryChangedEvent = "plugin:library-changed"

// gameWritableFields 是插件可以写入的 Game 字段（JSON 名）。
// 系统维护的字段（id、时间戳、last_played_at、cover_source_url）静默忽略；
// metadata_sources 走 ctx.games.sources；Steam 关联字段由 Steam 集成维护，不开放。
var gameWritableFields = map[string]bool{
	"name":                 true,
	"aliases":              true,
	"cover_url":            true,
	"company":              true,
	"summary":              true,
	"rating":               true,
	"release_date":         true,
	"path":                 true,
	"game_directory":       true,
	"save_path":            true,
	"process_name":         true,
	"wine_runner":          true,
	"wine_args":            true,
	"wine_prefix":          true,
	"launch_mode":          true,
	"steam_launch_options": true,
	"status":               true,
	"source_type":          true,
	"source_id":            true,
	"use_locale_emulator":  true,
	"use_magpie":           true,
	"is_nsfw":              true,
	"metadata_locked":      true,
}

var gameReadOnlyFields = map[string]bool{
	"id":                true,
	"cover_source_url":  true,
	"metadata_sources":  true,
	"cached_at":         true,
	"created_at":        true,
	"updated_at":        true,
	"last_played_at":    true,
	"steam_launch_id":   true,
	"steam_launch_kind": true,
	"steam_user_id":     true,
}

// applyGamePatch 把 JSON 补丁写入游戏：只接受可写字段，只读字段忽略，未知字段报错（方便 AI 工具调用发现拼写错误）。
func applyGamePatch(g *models.Game, raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var patch map[string]json.RawMessage
	if err := json.Unmarshal(raw, &patch); err != nil {
		return fmt.Errorf("游戏补丁必须是对象: %w", err)
	}
	base, err := json.Marshal(g)
	if err != nil {
		return err
	}
	var merged map[string]json.RawMessage
	if err := json.Unmarshal(base, &merged); err != nil {
		return err
	}
	for k, v := range patch {
		switch {
		case gameWritableFields[k]:
			merged[k] = v
		case gameReadOnlyFields[k]:
		default:
			return fmt.Errorf("未知的游戏字段: %s", k)
		}
	}
	data, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	var next models.Game
	if err := json.Unmarshal(data, &next); err != nil {
		return fmt.Errorf("游戏补丁字段类型错误: %w", err)
	}
	*g = next
	return nil
}

// jsonArg 解析第 i 个参数；缺省时返回零值。
func jsonArg[T any](args []json.RawMessage, i int) (T, error) {
	var v T
	if i >= len(args) || len(args[i]) == 0 || string(args[i]) == "null" {
		return v, nil
	}
	if err := json.Unmarshal(args[i], &v); err != nil {
		return v, fmt.Errorf("参数 %d 格式错误: %w", i, err)
	}
	return v, nil
}

func requireID(id, what string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("%s不能为空", what)
	}
	return id, nil
}

// notifyLibraryChanged 让界面重新加载游戏库与首页。
func (p *Plugin) notifyLibraryChanged() {
	if rt := p.manager.deps.Runtime; rt != nil {
		rt.Emit(libraryChangedEvent)
	}
}

// gameCreateInput 是 ctx.games.create 的入参：Game 的可写字段，加上标签、元数据来源与收藏夹。
type gameCreateInput struct {
	Tags            []string `json:"tags"`
	MetadataSources []struct {
		Source   enums.SourceType `json:"source"`
		SourceID string           `json:"source_id"`
	} `json:"metadata_sources"`
	CategoryIDs []string `json:"category_ids"`
}

func (p *Plugin) createGame(raw json.RawMessage) (models.Game, error) {
	game := models.Game{}
	if len(raw) == 0 {
		return game, fmt.Errorf("缺少游戏信息")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return game, fmt.Errorf("游戏信息必须是对象: %w", err)
	}
	var extra gameCreateInput
	if err := json.Unmarshal(raw, &extra); err != nil {
		return game, err
	}
	for _, k := range []string{"tags", "metadata_sources", "category_ids"} {
		delete(fields, k)
	}
	patch, _ := json.Marshal(fields)
	if err := applyGamePatch(&game, patch); err != nil {
		return game, err
	}
	if strings.TrimSpace(game.Name) == "" {
		return game, fmt.Errorf("游戏名称不能为空")
	}
	game.ID = uuid.New().String()
	now := time.Now()
	for _, src := range extra.MetadataSources {
		if strings.TrimSpace(string(src.Source)) == "" || strings.TrimSpace(src.SourceID) == "" {
			continue
		}
		game.MetadataSources = append(game.MetadataSources, models.GameMetadataSource{
			GameID:     game.ID,
			SourceType: src.Source,
			SourceID:   strings.TrimSpace(src.SourceID),
			CachedAt:   now,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}
	tags := make([]metadata.TagItem, 0, len(extra.Tags))
	for _, name := range extra.Tags {
		if name = strings.TrimSpace(name); name != "" {
			tags = append(tags, metadata.TagItem{Name: name, Source: "user"})
		}
	}
	if err := p.manager.deps.Game.AddGameFromWebMetadata(vo.GameMetadataFromWebVO{
		Source: game.SourceType,
		Game:   game,
		Tags:   tags,
	}); err != nil {
		return game, err
	}
	if len(extra.CategoryIDs) > 0 && p.manager.deps.Category != nil {
		if err := p.manager.deps.Category.AddGamesToCategories([]string{game.ID}, extra.CategoryIDs); err != nil {
			return game, fmt.Errorf("游戏已添加，但加入收藏夹失败: %w", err)
		}
	}
	return p.manager.deps.Game.GetGameByID(game.ID)
}

func (p *Plugin) buildGamesAPI() map[string]any {
	gameSvc := func() error {
		if p.manager.deps.Game == nil {
			return fmt.Errorf("GameService 未初始化")
		}
		return nil
	}
	return map[string]any{
		"list": p.asyncNative("games.list", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := gameSvc(); err != nil {
				return nil, err
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
			if err := gameSvc(); err != nil {
				return nil, err
			}
			id, _ := jsonArg[string](args, 0)
			return p.manager.deps.Game.GetGameByID(id)
		}),
		"findBySource": p.asyncNative("games.findBySource", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := gameSvc(); err != nil {
				return nil, err
			}
			source, _ := jsonArg[string](args, 0)
			sourceID, _ := jsonArg[string](args, 1)
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
		"create": p.asyncNative("games.create", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := gameSvc(); err != nil {
				return nil, err
			}
			var raw json.RawMessage
			if len(args) > 0 {
				raw = args[0]
			}
			game, err := p.createGame(raw)
			if game.ID != "" {
				p.notifyLibraryChanged()
			}
			if err != nil {
				return nil, err
			}
			return game, nil
		}),
		"update": p.asyncNative("games.update", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := gameSvc(); err != nil {
				return nil, err
			}
			id, err := jsonArg[string](args, 0)
			if err != nil {
				return nil, err
			}
			game, err := p.manager.deps.Game.GetGameByID(id)
			if err != nil {
				return nil, err
			}
			if len(args) > 1 {
				if err := applyGamePatch(&game, args[1]); err != nil {
					return nil, err
				}
			}
			if err := p.manager.deps.Game.UpdateGame(game); err != nil {
				return nil, err
			}
			p.notifyLibraryChanged()
			return p.manager.deps.Game.GetGameByID(id)
		}),
		"batchUpdate": p.asyncNative("games.batchUpdate", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := gameSvc(); err != nil {
				return nil, err
			}
			type batchItem struct {
				ID    string          `json:"id"`
				Patch json.RawMessage `json:"patch"`
			}
			items, err := jsonArg[[]batchItem](args, 0)
			if err != nil {
				return nil, err
			}
			updated := 0
			var failures []map[string]string
			for _, item := range items {
				game, err := p.manager.deps.Game.GetGameByID(item.ID)
				if err == nil {
					err = applyGamePatch(&game, item.Patch)
				}
				if err == nil {
					err = p.manager.deps.Game.UpdateGame(game)
				}
				if err != nil {
					failures = append(failures, map[string]string{"id": item.ID, "error": err.Error()})
					continue
				}
				updated++
			}
			if updated > 0 {
				p.notifyLibraryChanged()
			}
			return map[string]any{"updated": updated, "failed": failures}, nil
		}),
		"delete": p.asyncNative("games.delete", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := gameSvc(); err != nil {
				return nil, err
			}
			id, _ := jsonArg[string](args, 0)
			if id, err := requireID(id, "游戏 ID"); err != nil {
				return nil, err
			} else if err := p.manager.deps.Game.DeleteGame(id); err != nil {
				return nil, err
			}
			p.notifyLibraryChanged()
			return nil, nil
		}),
		"batchDelete": p.asyncNative("games.batchDelete", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := gameSvc(); err != nil {
				return nil, err
			}
			ids, err := jsonArg[[]string](args, 0)
			if err != nil {
				return nil, err
			}
			ids = slices.DeleteFunc(ids, func(id string) bool { return strings.TrimSpace(id) == "" })
			if len(ids) == 0 {
				return map[string]int{"deleted": 0}, nil
			}
			if err := p.manager.deps.Game.DeleteGames(ids); err != nil {
				return nil, err
			}
			p.notifyLibraryChanged()
			return map[string]int{"deleted": len(ids)}, nil
		}),
		"setTags": p.asyncNative("games.setTags", func(_ context.Context, args []json.RawMessage) (any, error) {
			if p.manager.deps.Tag == nil {
				return nil, fmt.Errorf("TagService 未初始化")
			}
			gameID, _ := jsonArg[string](args, 0)
			tags, _ := jsonArg[[]string](args, 1)
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
			p.notifyLibraryChanged()
			return p.manager.deps.Tag.GetTagsByGame(gameID)
		}),
		"refreshMetadata": p.asyncNative("games.refreshMetadata", func(ctx context.Context, args []json.RawMessage) (any, error) {
			if err := gameSvc(); err != nil {
				return nil, err
			}
			id, _ := jsonArg[string](args, 0)
			opts, err := jsonArg[struct {
				Source enums.SourceType            `json:"source"`
				Fields []enums.MetadataUpdateField `json:"fields"`
			}](args, 1)
			if err != nil {
				return nil, err
			}
			svc := p.manager.deps.Game
			switch {
			case opts.Source != "":
				err = svc.UpdateGameFromRemoteBySourceContext(ctx, id, opts.Source)
			case len(opts.Fields) > 0:
				err = svc.UpdateGameFromRemoteWithFieldsContext(ctx, id, opts.Fields)
			default:
				err = svc.UpdateGameFromRemote(id)
			}
			if err != nil {
				return nil, err
			}
			p.notifyLibraryChanged()
			return svc.GetGameByID(id)
		}),
		"setCoverFromURL": p.asyncNative("games.setCoverFromURL", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := gameSvc(); err != nil {
				return nil, err
			}
			id, _ := jsonArg[string](args, 0)
			coverURL, _ := jsonArg[string](args, 1)
			localPath, err := p.manager.deps.Game.DownloadCoverImage(id, coverURL)
			if err != nil {
				return nil, err
			}
			p.notifyLibraryChanged()
			return localPath, nil
		}),
		"start": p.asyncNative("games.start", func(_ context.Context, args []json.RawMessage) (any, error) {
			if p.manager.deps.Start == nil {
				return nil, fmt.Errorf("StartService 未初始化")
			}
			gameID, _ := jsonArg[string](args, 0)
			if len(args) > 1 && len(args[1]) > 0 {
				opts, err := jsonArg[hooks.LaunchOptions](args, 1)
				if err != nil {
					return nil, err
				}
				return p.manager.deps.Start.StartGameWithOptions(gameID, launcherpkg.LaunchOptions{
					UseLocaleEmulator: opts.UseLocaleEmulator,
					UseMagpie:         opts.UseMagpie,
					RunAsAdmin:        opts.RunAsAdmin,
					WineRunner:        opts.WineRunner,
					WineArgs:          opts.WineArgs,
					WinePrefix:        opts.WinePrefix,
					UseSteam:          opts.UseSteam,
					UseCompatibility:  opts.UseCompatibility,
				})
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
		"tags":     p.buildGameTagsAPI(),
		"sources":  p.buildGameSourcesAPI(),
		"sessions": p.buildGameSessionsAPI(),
		"progress": p.buildGameProgressAPI(),
		"review":   p.buildGameReviewAPI(),
	}
}

// ─────────────────────────── ctx.games.tags ───────────────────────────

func (p *Plugin) buildGameTagsAPI() map[string]any {
	tagSvc := func() error {
		if p.manager.deps.Tag == nil {
			return fmt.Errorf("TagService 未初始化")
		}
		return nil
	}
	return map[string]any{
		"list": p.asyncNative("games.tags.list", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := tagSvc(); err != nil {
				return nil, err
			}
			gameID, _ := jsonArg[string](args, 0)
			return p.manager.deps.Tag.GetTagsByGame(gameID)
		}),
		"add": p.asyncNative("games.tags.add", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := tagSvc(); err != nil {
				return nil, err
			}
			gameID, _ := jsonArg[string](args, 0)
			name, _ := jsonArg[string](args, 1)
			if err := p.manager.deps.Tag.AddUserTag(gameID, strings.TrimSpace(name)); err != nil {
				return nil, err
			}
			p.notifyLibraryChanged()
			return p.manager.deps.Tag.GetTagsByGame(gameID)
		}),
		"remove": p.asyncNative("games.tags.remove", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := tagSvc(); err != nil {
				return nil, err
			}
			tagID, _ := jsonArg[string](args, 0)
			if err := p.manager.deps.Tag.DeleteTag(tagID); err != nil {
				return nil, err
			}
			p.notifyLibraryChanged()
			return nil, nil
		}),
		"search": p.asyncNative("games.tags.search", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := tagSvc(); err != nil {
				return nil, err
			}
			query, _ := jsonArg[string](args, 0)
			return p.manager.deps.Tag.SearchTagsInLibrary(query)
		}),
		"gameIds": p.asyncNative("games.tags.gameIds", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := tagSvc(); err != nil {
				return nil, err
			}
			name, _ := jsonArg[string](args, 0)
			return p.manager.deps.Tag.GetGameIDsByTag(name)
		}),
	}
}

// ─────────────────────────── ctx.games.sources ───────────────────────────

func (p *Plugin) buildGameSourcesAPI() map[string]any {
	gameSvc := func() error {
		if p.manager.deps.Game == nil {
			return fmt.Errorf("GameService 未初始化")
		}
		return nil
	}
	return map[string]any{
		"list": p.asyncNative("games.sources.list", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := gameSvc(); err != nil {
				return nil, err
			}
			gameID, _ := jsonArg[string](args, 0)
			return p.manager.deps.Game.GetGameMetadataSources(gameID)
		}),
		"upsert": p.asyncNative("games.sources.upsert", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := gameSvc(); err != nil {
				return nil, err
			}
			gameID, _ := jsonArg[string](args, 0)
			source, _ := jsonArg[string](args, 1)
			sourceID, _ := jsonArg[string](args, 2)
			if err := p.manager.deps.Game.UpsertGameMetadataSource(gameID, enums.SourceType(strings.TrimSpace(source)), strings.TrimSpace(sourceID)); err != nil {
				return nil, err
			}
			p.notifyLibraryChanged()
			return p.manager.deps.Game.GetGameMetadataSources(gameID)
		}),
		"setDefault": p.asyncNative("games.sources.setDefault", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := gameSvc(); err != nil {
				return nil, err
			}
			gameID, _ := jsonArg[string](args, 0)
			source, _ := jsonArg[string](args, 1)
			if err := p.manager.deps.Game.SetDefaultMetadataSource(gameID, enums.SourceType(strings.TrimSpace(source))); err != nil {
				return nil, err
			}
			p.notifyLibraryChanged()
			return nil, nil
		}),
		"remove": p.asyncNative("games.sources.remove", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := gameSvc(); err != nil {
				return nil, err
			}
			gameID, _ := jsonArg[string](args, 0)
			source, _ := jsonArg[string](args, 1)
			if err := p.manager.deps.Game.DeleteGameMetadataSource(gameID, enums.SourceType(strings.TrimSpace(source))); err != nil {
				return nil, err
			}
			p.notifyLibraryChanged()
			return nil, nil
		}),
	}
}

// ─────────────────────────── ctx.games.sessions ───────────────────────────

func (p *Plugin) buildGameSessionsAPI() map[string]any {
	sessionSvc := func() error {
		if p.manager.deps.Session == nil {
			return fmt.Errorf("SessionService 未初始化")
		}
		return nil
	}
	return map[string]any{
		"list": p.asyncNative("games.sessions.list", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := sessionSvc(); err != nil {
				return nil, err
			}
			gameID, _ := jsonArg[string](args, 0)
			return p.manager.deps.Session.GetPlaySessions(gameID)
		}),
		"add": p.asyncNative("games.sessions.add", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := sessionSvc(); err != nil {
				return nil, err
			}
			gameID, _ := jsonArg[string](args, 0)
			input, err := jsonArg[struct {
				StartTime time.Time `json:"start_time"`
				Duration  int       `json:"duration"`
			}](args, 1)
			if err != nil {
				return nil, err
			}
			if input.StartTime.IsZero() {
				return nil, fmt.Errorf("start_time 不能为空")
			}
			session, err := p.manager.deps.Session.AddPlaySession(gameID, input.StartTime, input.Duration)
			if err != nil {
				return nil, err
			}
			p.notifyLibraryChanged()
			return session, nil
		}),
		"update": p.asyncNative("games.sessions.update", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := sessionSvc(); err != nil {
				return nil, err
			}
			session, err := jsonArg[models.PlaySession](args, 0)
			if err != nil {
				return nil, err
			}
			if _, err := requireID(session.ID, "游玩记录 ID"); err != nil {
				return nil, err
			}
			if err := p.manager.deps.Session.UpdatePlaySession(session); err != nil {
				return nil, err
			}
			p.notifyLibraryChanged()
			return nil, nil
		}),
		"remove": p.asyncNative("games.sessions.remove", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := sessionSvc(); err != nil {
				return nil, err
			}
			sessionID, _ := jsonArg[string](args, 0)
			if err := p.manager.deps.Session.DeletePlaySession(sessionID); err != nil {
				return nil, err
			}
			p.notifyLibraryChanged()
			return nil, nil
		}),
	}
}

// ─────────────────────────── ctx.games.progress / review ───────────────────────────

func (p *Plugin) buildGameProgressAPI() map[string]any {
	progressSvc := func() error {
		if p.manager.deps.Progress == nil {
			return fmt.Errorf("GameProgressService 未初始化")
		}
		return nil
	}
	return map[string]any{
		"get": p.asyncNative("games.progress.get", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := progressSvc(); err != nil {
				return nil, err
			}
			gameID, _ := jsonArg[string](args, 0)
			return p.manager.deps.Progress.GetGameProgress(gameID)
		}),
		"list": p.asyncNative("games.progress.list", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := progressSvc(); err != nil {
				return nil, err
			}
			gameID, _ := jsonArg[string](args, 0)
			return p.manager.deps.Progress.ListGameProgresses(gameID)
		}),
		"save": p.asyncNative("games.progress.save", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := progressSvc(); err != nil {
				return nil, err
			}
			gp, err := jsonArg[models.GameProgress](args, 0)
			if err != nil {
				return nil, err
			}
			if _, err := requireID(gp.GameID, "game_id"); err != nil {
				return nil, err
			}
			return p.manager.deps.Progress.UpsertGameProgress(gp)
		}),
		"clear": p.asyncNative("games.progress.clear", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := progressSvc(); err != nil {
				return nil, err
			}
			gameID, _ := jsonArg[string](args, 0)
			return nil, p.manager.deps.Progress.DeleteGameProgress(gameID)
		}),
	}
}

func (p *Plugin) buildGameReviewAPI() map[string]any {
	reviewSvc := func() error {
		if p.manager.deps.Review == nil {
			return fmt.Errorf("GameReviewService 未初始化")
		}
		return nil
	}
	return map[string]any{
		"get": p.asyncNative("games.review.get", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := reviewSvc(); err != nil {
				return nil, err
			}
			gameID, _ := jsonArg[string](args, 0)
			return p.manager.deps.Review.GetGameReview(gameID)
		}),
		"save": p.asyncNative("games.review.save", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := reviewSvc(); err != nil {
				return nil, err
			}
			review, err := jsonArg[models.GameReview](args, 0)
			if err != nil {
				return nil, err
			}
			if _, err := requireID(review.GameID, "game_id"); err != nil {
				return nil, err
			}
			return p.manager.deps.Review.SaveGameReview(review)
		}),
	}
}

// ─────────────────────────── ctx.categories ───────────────────────────

func (p *Plugin) buildCategoriesAPI() map[string]any {
	catSvc := func() error {
		if p.manager.deps.Category == nil {
			return fmt.Errorf("CategoryService 未初始化")
		}
		return nil
	}
	return map[string]any{
		"list": p.asyncNative("categories.list", func(_ context.Context, _ []json.RawMessage) (any, error) {
			if err := catSvc(); err != nil {
				return nil, err
			}
			return p.manager.deps.Category.GetCategories()
		}),
		"get": p.asyncNative("categories.get", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := catSvc(); err != nil {
				return nil, err
			}
			id, _ := jsonArg[string](args, 0)
			return p.manager.deps.Category.GetCategoryByID(id)
		}),
		"listByGame": p.asyncNative("categories.listByGame", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := catSvc(); err != nil {
				return nil, err
			}
			gameID, _ := jsonArg[string](args, 0)
			return p.manager.deps.Category.GetCategoriesByGame(gameID)
		}),
		"games": p.asyncNative("categories.games", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := catSvc(); err != nil {
				return nil, err
			}
			id, _ := jsonArg[string](args, 0)
			return p.manager.deps.Category.GetGamesByCategory(id)
		}),
		"create": p.asyncNative("categories.create", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := catSvc(); err != nil {
				return nil, err
			}
			input, _ := jsonArg[struct {
				Name  string `json:"name"`
				Emoji string `json:"emoji"`
			}](args, 0)
			if err := p.manager.deps.Category.AddCategory(input.Name, input.Emoji); err != nil {
				return nil, err
			}
			p.notifyLibraryChanged()
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
			if err := catSvc(); err != nil {
				return nil, err
			}
			id, _ := jsonArg[string](args, 0)
			patch, _ := jsonArg[struct {
				Name  *string `json:"name"`
				Emoji *string `json:"emoji"`
			}](args, 1)
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
			if err := p.manager.deps.Category.UpdateCategory(id, name, emoji); err != nil {
				return nil, err
			}
			p.notifyLibraryChanged()
			return nil, nil
		}),
		"delete": p.asyncNative("categories.delete", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := catSvc(); err != nil {
				return nil, err
			}
			id, _ := jsonArg[string](args, 0)
			if err := p.manager.deps.Category.DeleteCategory(id); err != nil {
				return nil, err
			}
			p.notifyLibraryChanged()
			return nil, nil
		}),
		"addGames": p.asyncNative("categories.addGames", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := catSvc(); err != nil {
				return nil, err
			}
			catID, _ := jsonArg[string](args, 0)
			gameIDs, _ := jsonArg[[]string](args, 1)
			if err := p.manager.deps.Category.AddGamesToCategories(gameIDs, []string{catID}); err != nil {
				return nil, err
			}
			p.notifyLibraryChanged()
			return nil, nil
		}),
		"removeGames": p.asyncNative("categories.removeGames", func(_ context.Context, args []json.RawMessage) (any, error) {
			if err := catSvc(); err != nil {
				return nil, err
			}
			catID, _ := jsonArg[string](args, 0)
			gameIDs, _ := jsonArg[[]string](args, 1)
			if err := p.manager.deps.Category.RemoveGamesFromCategory(gameIDs, catID); err != nil {
				return nil, err
			}
			p.notifyLibraryChanged()
			return nil, nil
		}),
	}
}
