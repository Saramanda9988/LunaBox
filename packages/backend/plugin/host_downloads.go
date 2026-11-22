package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/MoeForge/moejs"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/backend/service"
)

func guessArchiveFormat(fileName string) string {
	lower := strings.ToLower(strings.TrimSpace(fileName))
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return "tar.gz"
	case strings.HasSuffix(lower, ".tar.bz2"), strings.HasSuffix(lower, ".tbz2"):
		return "tar.bz2"
	case strings.HasSuffix(lower, ".tar.xz"), strings.HasSuffix(lower, ".txz"):
		return "tar.xz"
	case strings.HasSuffix(lower, ".tar.zst"), strings.HasSuffix(lower, ".tzst"):
		return "tar.zst"
	case strings.HasSuffix(lower, ".7z"):
		return "7z"
	case strings.HasSuffix(lower, ".zip"):
		return "zip"
	case strings.HasSuffix(lower, ".rar"):
		return "rar"
	case strings.HasSuffix(lower, ".tar"):
		return "tar"
	default:
		return "none"
	}
}

func (p *Plugin) buildDownloadsAPI() map[string]any {
	return map[string]any{
		"requestInstall": p.asyncNative("downloads.requestInstall", func(_ context.Context, args []json.RawMessage) (any, error) {
			if p.manager.deps.Download == nil {
				return nil, fmt.Errorf("DownloadService 未初始化")
			}
			var input struct {
				Name            string            `json:"name"`
				Title           string            `json:"title"`
				DownloadURL     string            `json:"downloadUrl"`
				URL             string            `json:"url"`
				FileName        string            `json:"fileName"`
				ArchiveFormat   string            `json:"archiveFormat"`
				StartupPath     string            `json:"startupPath"`
				InstallSubdir   string            `json:"installSubdir"`
				StripTopLevel   bool              `json:"stripTopLevel"`
				Headers         map[string]string `json:"headers"`
				ArchivePassword string            `json:"archivePassword"`
				CoverURL        string            `json:"coverUrl"`
				Source          string            `json:"source"`
				SourceID        string            `json:"sourceId"`
				Size            int64             `json:"size"`
				Silent          bool              `json:"silent"`
			}
			if len(args) > 0 {
				if err := json.Unmarshal(args[0], &input); err != nil {
					return nil, err
				}
			}
			dlURL := strings.TrimSpace(input.DownloadURL)
			if dlURL == "" {
				dlURL = strings.TrimSpace(input.URL)
			}
			if dlURL == "" {
				return nil, fmt.Errorf("downloadUrl 不能为空")
			}
			title := strings.TrimSpace(input.Name)
			if title == "" {
				title = strings.TrimSpace(input.Title)
			}
			fileName := strings.TrimSpace(input.FileName)
			if fileName == "" {
				if u, err := url.Parse(dlURL); err == nil {
					base := path.Base(u.Path)
					if base != "" && base != "/" && base != "." {
						fileName = base
					}
				}
			}
			if fileName == "" {
				fileName = "resource.zip"
			}
			archiveFmt := strings.TrimSpace(input.ArchiveFormat)
			if archiveFmt == "" {
				archiveFmt = guessArchiveFormat(fileName)
			}
			size := input.Size
			if size <= 0 {
				size = 1 // 兼容未提前声明大小的站点直链
			}
			req := vo.InstallRequest{
				URL:             dlURL,
				FileName:        fileName,
				ArchiveFormat:   archiveFmt,
				StartupPath:     input.StartupPath,
				InstallSubdir:   input.InstallSubdir,
				StripTopLevel:   input.StripTopLevel,
				Title:           title,
				DownloadSource:  p.Manifest.DisplayName(p.manager.currentLanguage()),
				MetaSource:      input.Source,
				MetaID:          input.SourceID,
				Size:            size,
				ExpiresAt:       time.Now().Add(24 * time.Hour).Unix(),
				Headers:         input.Headers,
				ArchivePassword: input.ArchivePassword,
				CoverURL:        input.CoverURL,
			}

			if input.Silent && p.manager.deps.Config != nil && strings.TrimSpace(p.manager.deps.Config.GameLibraryPath) != "" {
				taskID, err := p.manager.deps.Download.StartDownload(req)
				if err != nil {
					return nil, err
				}
				return map[string]any{"taskId": taskID, "queued": true}, nil
			}
			p.manager.deps.Download.RequestInstall(req)
			return map[string]any{"pending": true}, nil
		}),
		"list": p.asyncNative("downloads.list", func(_ context.Context, _ []json.RawMessage) (any, error) {
			if p.manager.deps.Download == nil {
				return nil, fmt.Errorf("DownloadService 未初始化")
			}
			return p.manager.deps.Download.GetDownloadTasks(), nil
		}),
		"getImportStates": p.asyncNative("downloads.getImportStates", func(_ context.Context, args []json.RawMessage) (any, error) {
			if p.manager.deps.Download == nil {
				return nil, fmt.Errorf("DownloadService 未初始化")
			}
			var reqs []vo.DownloadImportStateRequest
			if len(args) > 0 {
				if err := json.Unmarshal(args[0], &reqs); err != nil {
					return nil, err
				}
			}
			return p.manager.deps.Download.CheckDownloadImportStates(reqs)
		}),
		"onProgress": p.safeNative("downloads.onProgress", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			if p.manager.deps.Download == nil {
				return moejs.Undefined(), throwJSError(r, "DownloadService 未初始化")
			}
			fn := moejs.Arg(args, 0)
			if !fn.IsObject() {
				return moejs.Undefined(), throwJSError(r, "downloads.onProgress 需要函数参数")
			}
			unsub := p.manager.deps.Download.SubscribeProgress(func(ev service.DownloadProgressEvent) {
				raw, err := json.Marshal(ev)
				if err != nil {
					return
				}
				p.Post(func(rt *moejs.Runtime) {
					v, err := rt.ParseJSON(raw)
					if err != nil {
						return
					}
					if _, err := rt.CallFunction(fn, moejs.Undefined(), v); err != nil {
						p.recordError(err)
					}
				})
			})
			dispose := p.scope.Effect(unsub)
			return p.rt.Function("dispose", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				dispose()
				return moejs.Undefined(), nil
			}), nil
		}),
		"onImported": p.safeNative("downloads.onImported", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			if p.manager.deps.Download == nil {
				return moejs.Undefined(), throwJSError(r, "DownloadService 未初始化")
			}
			fn := moejs.Arg(args, 0)
			if !fn.IsObject() {
				return moejs.Undefined(), throwJSError(r, "downloads.onImported 需要函数参数")
			}
			unsub := p.manager.deps.Download.SubscribeImported(func(taskID string) {
				p.Post(func(rt *moejs.Runtime) {
					if _, err := rt.CallFunction(fn, moejs.Undefined(), moejs.String(taskID)); err != nil {
						p.recordError(err)
					}
				})
			})
			dispose := p.scope.Effect(unsub)
			return p.rt.Function("dispose", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				dispose()
				return moejs.Undefined(), nil
			}), nil
		}),
	}
}

func (p *Plugin) buildImportAPI() map[string]any {
	return map[string]any{
		"openModal": p.safeNative("import.openModal", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			var payload json.RawMessage
			if len(args) > 0 && !args[0].IsUndefined() {
				if args[0].IsString() {
					b, _ := json.Marshal(map[string]any{"path": args[0].String()})
					payload = b
				} else if b, err := p.rt.AppendJSON(nil, args[0]); err == nil {
					payload = b
				}
			}
			p.manager.ui.EmitAction(UIAction{
				Type:     "openImportModal",
				PluginID: p.ID,
				Props:    payload,
			})
			return moejs.Undefined(), nil
		}),
		"fromPath": p.asyncNative("import.fromPath", func(_ context.Context, args []json.RawMessage) (any, error) {
			if p.manager.deps.Import == nil {
				return nil, fmt.Errorf("ImportService 未初始化")
			}
			var req struct {
				Path        string                `json:"path"`
				ExePath     string                `json:"exePath"`
				Name        string                `json:"name"`
				Source      string                `json:"source"`
				SourceID    string                `json:"sourceId"`
				Metadata    *pluginMetadataResult `json:"metadata"`
				CategoryIDs []string              `json:"categoryIds"`
			}
			if len(args) > 0 {
				if err := json.Unmarshal(args[0], &req); err != nil {
					return nil, err
				}
			}
			targetPath := strings.TrimSpace(req.Path)
			if targetPath == "" {
				return nil, fmt.Errorf("path 不能为空")
			}
			exePath := strings.TrimSpace(req.ExePath)
			if exePath == "" && p.manager.deps.Game != nil {
				if resolved, err := p.manager.deps.Game.ResolveExecutablePathForImport(targetPath); err == nil && resolved != "" {
					exePath = resolved
				}
			}
			if exePath == "" {
				exePath = targetPath
			}
			folderName := filepath.Base(targetPath)
			searchName := strings.TrimSpace(req.Name)
			if searchName == "" {
				searchName = folderName
			}
			cand := vo.BatchImportCandidate{
				FolderPath:    targetPath,
				GameDirectory: targetPath,
				FolderName:    folderName,
				Executables:   []string{exePath},
				SelectedExe:   exePath,
				SearchName:    searchName,
				SourceType:    enums.SourceType(req.Source),
				SourceID:      req.SourceID,
				IsSelected:    true,
				ImportStatus:  "new",
			}
			if req.Metadata != nil {
				m := req.Metadata.toBackend(enums.SourceType(req.Source))
				g := m.Game
				if g.Name == "" {
					g.Name = searchName
				}
				cand.MatchedGame = &g
				cand.MatchedTags = m.Tags
				cand.MatchSource = g.SourceType
				cand.MatchStatus = "matched"
			} else if searchName != "" {
				g := models.Game{Name: searchName, SourceType: enums.SourceType(req.Source), SourceID: req.SourceID}
				cand.MatchedGame = &g
			}
			res, err := p.manager.deps.Import.BatchImportGames([]vo.BatchImportCandidate{cand})
			if err != nil {
				return nil, err
			}
			if len(req.CategoryIDs) > 0 && p.manager.deps.Category != nil && p.manager.deps.Game != nil {
				if list, lerr := p.manager.deps.Game.GetGames(vo.GameListRequest{Limit: 10, SearchQuery: searchName}); lerr == nil {
					for _, g := range list.Games {
						if strings.EqualFold(filepath.Clean(g.Path), filepath.Clean(exePath)) {
							_ = p.manager.deps.Category.AddGamesToCategories([]string{g.ID}, req.CategoryIDs)
							break
						}
					}
				}
			}
			return res, nil
		}),
	}
}
