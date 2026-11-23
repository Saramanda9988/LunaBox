package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/MoeForge/moejs"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/backend/utils/metadata"
)

const metadataCallTimeout = 15 * time.Second

// pluginMetadataResult 与文档 §8 的 MetadataResult JSON 形状对齐。
type pluginMetadataResult struct {
	Game models.Game `json:"game"`
	Tags []struct {
		Name      string  `json:"name"`
		Source    string  `json:"source,omitempty"`
		Weight    float64 `json:"weight,omitempty"`
		IsSpoiler bool    `json:"is_spoiler,omitempty"`
	} `json:"tags,omitempty"`
}

func (r pluginMetadataResult) toBackend(source enums.SourceType) metadata.MetadataResult {
	out := metadata.MetadataResult{
		Game: r.Game,
	}
	if out.Game.SourceType == "" {
		out.Game.SourceType = source
	}
	if len(r.Tags) > 0 {
		out.Tags = make([]metadata.TagItem, 0, len(r.Tags))
		for _, t := range r.Tags {
			if strings.TrimSpace(t.Name) == "" {
				continue
			}
			src := t.Source
			if src == "" {
				src = string(source)
			}
			out.Tags = append(out.Tags, metadata.TagItem{
				Name:      strings.TrimSpace(t.Name),
				Source:    src,
				Weight:    t.Weight,
				IsSpoiler: t.IsSpoiler,
			})
		}
	}
	return out
}

// pluginExternalSource 实现 metadata.ExternalSource 接口，桥接宿主元数据调用与插件线程。
type pluginExternalSource struct {
	p           *Plugin
	meta        metadata.ExternalSourceMeta
	linkFn      moejs.Value
	normalizeFn moejs.Value
	fetchFn     moejs.Value
	searchFn    moejs.Value
	batchFn     moejs.Value
}

func (s *pluginExternalSource) Meta() metadata.ExternalSourceMeta { return s.meta }

func (s *pluginExternalSource) NormalizeID(input string) (string, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", false
	}
	if !s.normalizeFn.IsObject() {
		return input, true
	}
	ctx, cancel := context.WithTimeout(s.p.scope.Context(), 2*time.Second)
	defer cancel()
	raw, err := s.p.Call(ctx, s.normalizeFn, input)
	if err != nil || len(raw) == 0 || string(raw) == "null" {
		return "", false
	}
	var out string
	if err := json.Unmarshal(raw, &out); err != nil || strings.TrimSpace(out) == "" {
		return "", false
	}
	return strings.TrimSpace(out), true
}

func (s *pluginExternalSource) Link(sourceID string) string {
	if !s.linkFn.IsObject() || strings.TrimSpace(sourceID) == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(s.p.scope.Context(), 2*time.Second)
	defer cancel()
	raw, err := s.p.Call(ctx, s.linkFn, sourceID)
	if err != nil || len(raw) == 0 {
		return ""
	}
	var out string
	if json.Unmarshal(raw, &out) == nil {
		return strings.TrimSpace(out)
	}
	return ""
}

func (s *pluginExternalSource) FetchMetadata(id string, _ string) (metadata.MetadataResult, error) {
	return s.FetchContext(context.Background(), id, s.p.manager.currentLanguage())
}

func (s *pluginExternalSource) FetchMetadataByName(name string, _ string) (metadata.MetadataResult, error) {
	items, err := s.SearchContext(context.Background(), name, s.p.manager.currentLanguage(), 1)
	if err != nil {
		return metadata.MetadataResult{}, err
	}
	if len(items) == 0 {
		return metadata.MetadataResult{}, fmt.Errorf("未找到结果")
	}
	return items[0], nil
}

func (s *pluginExternalSource) FetchMetadataCandidatesByName(name string, _ string) ([]metadata.MetadataResult, error) {
	return s.SearchContext(context.Background(), name, s.p.manager.currentLanguage(), 5)
}

func (s *pluginExternalSource) FetchMetadataBatch(ids []string, _ string) (map[string]metadata.MetadataResult, error) {
	return s.FetchBatchContext(context.Background(), ids, s.p.manager.currentLanguage())
}

func (s *pluginExternalSource) FetchContext(ctx context.Context, id string, language string) (metadata.MetadataResult, error) {
	if !s.fetchFn.IsObject() {
		return metadata.MetadataResult{}, fmt.Errorf("插件元数据源 %s 未实现 fetch", s.meta.Source)
	}
	cctx, cancel := context.WithTimeout(ctx, metadataCallTimeout)
	defer cancel()
	raw, err := s.p.Call(cctx, s.fetchFn, id, map[string]any{"language": language})
	if err != nil {
		return metadata.MetadataResult{}, err
	}
	var parsed pluginMetadataResult
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return metadata.MetadataResult{}, fmt.Errorf("解析插件元数据源返回失败: %w", err)
	}
	if parsed.Game.SourceID == "" {
		parsed.Game.SourceID = id
	}
	return parsed.toBackend(s.meta.Source), nil
}

func (s *pluginExternalSource) SearchContext(ctx context.Context, name string, language string, limit int) ([]metadata.MetadataResult, error) {
	if !s.searchFn.IsObject() {
		return nil, fmt.Errorf("插件元数据源 %s 未实现 search", s.meta.Source)
	}
	if limit <= 0 {
		limit = 5
	}
	cctx, cancel := context.WithTimeout(ctx, metadataCallTimeout)
	defer cancel()
	raw, err := s.p.Call(cctx, s.searchFn, name, map[string]any{"language": language, "limit": limit})
	if err != nil {
		return nil, err
	}
	var parsed []pluginMetadataResult
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("解析插件元数据搜索结果失败: %w", err)
	}
	out := make([]metadata.MetadataResult, 0, len(parsed))
	for _, item := range parsed {
		out = append(out, item.toBackend(s.meta.Source))
	}
	return out, nil
}

func (s *pluginExternalSource) FetchBatchContext(ctx context.Context, ids []string, language string) (map[string]metadata.MetadataResult, error) {
	out := make(map[string]metadata.MetadataResult, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	if s.batchFn.IsObject() {
		cctx, cancel := context.WithTimeout(ctx, metadataCallTimeout*2)
		defer cancel()
		raw, err := s.p.Call(cctx, s.batchFn, ids, map[string]any{"language": language})
		if err != nil {
			return nil, err
		}
		var parsed map[string]pluginMetadataResult
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, err
		}
		for id, item := range parsed {
			if item.Game.SourceID == "" {
				item.Game.SourceID = id
			}
			out[id] = item.toBackend(s.meta.Source)
		}
		return out, nil
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		if res, err := s.FetchContext(ctx, id, language); err == nil {
			out[id] = res
		}
	}
	return out, nil
}

func (p *Plugin) buildMetadataAPI() map[string]any {
	return map[string]any{
		"registerSource": p.safeNative("metadata.registerSource", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			def := moejs.Arg(args, 0)
			if !def.IsObject() {
				return moejs.Undefined(), throwJSError(r, "metadata.registerSource 需要配置对象")
			}
			idVal, _ := p.rt.Get(def, "id")
			id := strings.ToLower(strings.TrimSpace(idVal.String()))
			sourceType := enums.SourceType(fmt.Sprintf("plugin:%s/%s", p.ID, id))
			if !metadata.IsValidPluginSource(string(sourceType)) {
				return moejs.Undefined(), throwJSError(r, fmt.Sprintf("无效的元数据源 id: %q", id))
			}

			names := make(map[string]string)
			if nameVal, err := p.rt.Get(def, "name"); err == nil && !nameVal.IsUndefined() {
				if nameVal.IsString() {
					names["en-US"] = nameVal.String()
				} else if nameVal.IsObject() {
					if raw, merr := p.rt.AppendJSON(nil, nameVal); merr == nil {
						_ = json.Unmarshal(raw, &names)
					}
				}
			}
			iconPath := ""
			if iconVal, err := p.rt.Get(def, "icon"); err == nil && iconVal.IsString() {
				iconRel := strings.TrimSpace(iconVal.String())
				if iconRel != "" {
					if strings.HasPrefix(iconRel, "http://") || strings.HasPrefix(iconRel, "https://") || filepath.IsAbs(iconRel) {
						iconPath = iconRel
					} else {
						iconPath = filepath.Join(p.Dir, filepath.FromSlash(iconRel))
					}
				}
			}
			perSec := 0.0
			if rlVal, err := p.rt.Get(def, "rateLimit"); err == nil && rlVal.IsObject() {
				if ps, err := p.rt.Get(rlVal, "perSecond"); err == nil && ps.IsNumber() {
					perSec = ps.AsNumber()
				}
			}

			ext := &pluginExternalSource{
				p: p,
				meta: metadata.ExternalSourceMeta{
					Source:    sourceType,
					PluginID:  p.ID,
					ID:        id,
					Names:     names,
					IconPath:  iconPath,
					PerSecond: perSec,
				},
			}
			if fn, err := p.rt.Get(def, "link"); err == nil && fn.IsObject() {
				ext.linkFn = fn
			}
			if fn, err := p.rt.Get(def, "normalizeId"); err == nil && fn.IsObject() {
				ext.normalizeFn = fn
			}
			if fn, err := p.rt.Get(def, "fetch"); err == nil && fn.IsObject() {
				ext.fetchFn = fn
			}
			if fn, err := p.rt.Get(def, "search"); err == nil && fn.IsObject() {
				ext.searchFn = fn
			}
			if fn, err := p.rt.Get(def, "fetchBatch"); err == nil && fn.IsObject() {
				ext.batchFn = fn
			}

			dispose := p.scope.Effect(metadata.RegisterExternal(sourceType, ext))
			return p.rt.Function("dispose", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				dispose()
				return moejs.Undefined(), nil
			}), nil
		}),
		"search": p.asyncNative("metadata.search", func(ctx context.Context, args []json.RawMessage) (any, error) {
			if p.manager.deps.Game == nil {
				return nil, fmt.Errorf("GameService 未初始化")
			}
			var name string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &name)
			}
			return p.manager.deps.Game.FetchMetadataByNameContext(ctx, name)
		}),
		"fetch": p.asyncNative("metadata.fetch", func(ctx context.Context, args []json.RawMessage) (any, error) {
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
			return p.manager.deps.Game.FetchMetadataFromWebContext(ctx, vo.MetadataRequest{
				Source: enums.SourceType(source),
				ID:     sourceID,
			})
		}),
		"getAuthToken": p.asyncNative("metadata.getAuthToken", func(ctx context.Context, args []json.RawMessage) (any, error) {
			var source string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &source)
			}
			switch strings.ToLower(strings.TrimSpace(source)) {
			case "hikarinagi":
				if p.manager.deps.Hikarinagi == nil {
					return "", nil
				}
				token, err := p.manager.deps.Hikarinagi.GetCatalogAccessToken(ctx)
				if err != nil {
					return "", nil
				}
				return token, nil
			default:
				return "", nil
			}
		}),
	}
}
