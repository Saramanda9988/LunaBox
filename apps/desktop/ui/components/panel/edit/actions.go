package edit

import (
	"encoding/base64"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
)

// 本文件是 routes/game.tsx 传给 GameEditPanel 的处理函数，以及面板里调用服务的部分。服务调用都在后台执行
// （Env.Async），结果回到主线程后再改状态；打开文件对话框的服务会一直等到用户选完。

// selectExecutable 对应 handleSelectExecutable。
func (e *editor) selectExecutable(c *ui.Context) {
	svc, s, id, cur := e.p.Env.Svc, e.s, e.g.ID, e.g.Path
	e.p.Env.Async(func() func() {
		path, err := svc.Game.SelectGameExecutable(cur)
		return func() {
			if err != nil {
				e.toast(c, env.Error(locales.GameToastSelectExecutableFailed()))
				return
			}
			if path != "" {
				s.patch(id, func(g *models.Game) { g.Path = path })
			}
		}
	})
}

// selectGameDirectory 对应 handleSelectGameDirectory：对话框从当前游戏目录（或启动路径）打开。
func (e *editor) selectGameDirectory(c *ui.Context) {
	svc, s, id := e.p.Env.Svc, e.s, e.g.ID
	cur := e.g.GameDirectory
	if cur == "" {
		cur = e.g.Path
	}
	e.p.Env.Async(func() func() {
		path, err := svc.Game.SelectGameDirectory(cur)
		return func() {
			if err != nil {
				e.toast(c, env.Error(locales.GameToastSelectGameDirFailed()))
				return
			}
			if path != "" {
				s.patch(id, func(g *models.Game) { g.GameDirectory = path })
			}
		}
	})
}

// selectSaveDirectory 对应 handleSelectSaveDirectory。
func (e *editor) selectSaveDirectory(c *ui.Context, root string) {
	svc, s, id := e.p.Env.Svc, e.s, e.g.ID
	e.p.Env.Async(func() func() {
		path, err := svc.Game.SelectSaveDirectory(root)
		return func() {
			if err != nil {
				e.toast(c, env.Error(locales.GameToastSelectSaveDirFailed()))
				return
			}
			if path != "" {
				s.patch(id, func(g *models.Game) { g.SavePath = path })
			}
		}
	})
}

// selectSaveFile 对应 handleSelectSaveFile。
func (e *editor) selectSaveFile(c *ui.Context, root string) {
	svc, s, id := e.p.Env.Svc, e.s, e.g.ID
	e.p.Env.Async(func() func() {
		path, err := svc.Game.SelectSaveFile(root)
		return func() {
			if err != nil {
				e.toast(c, env.Error(locales.GameToastSelectSaveFileFailed()))
				return
			}
			if path != "" {
				s.patch(id, func(g *models.Game) { g.SavePath = path })
			}
		}
	})
}

// openPath 对应 OpenLocalPath：目录直接打开，文件在资源管理器中选中。
func (e *editor) openPath(c *ui.Context, path string) {
	if path == "" {
		return
	}
	svc := e.p.Env.Svc
	e.p.Env.Async(func() func() {
		err := svc.Game.OpenLocalPath(path)
		return func() {
			if err != nil {
				e.toast(c, env.Error(locales.GameEditOpenPathFailed()))
			}
		}
	})
}

// coverChanged 在服务换了封面之后丢掉封面缓存（本地封面换图后地址不变），并让页面重新读取游戏。
func (e *editor) coverChanged(c *ui.Context, old models.Game, newURL string) {
	if cc := e.p.Env.Covers; cc != nil {
		cc.Forget(&old)
		cc.Forget(&models.Game{CoverURL: newURL})
	}
	e.p.OnSaved()
}

// selectCoverImage 对应 handleSelectCoverImage：选图后服务把图片存进 covers 目录并写入数据库。
func (e *editor) selectCoverImage(c *ui.Context) {
	svc, s, old := e.p.Env.Svc, e.s, e.g
	e.p.Env.Async(func() func() {
		url, err := svc.Game.SelectCoverImage(old.ID)
		return func() {
			if err != nil {
				e.toast(c, env.Error(locales.GameToastSelectCoverFailed()))
				return
			}
			if url != "" && s.gameID == old.ID {
				e.coverChanged(c, old, url)
			}
		}
	})
}

// downloadCover 对应 handleDownloadCover：把远程封面下载为本地封面。
func (e *editor) downloadCover(c *ui.Context, remote string) {
	if !isRemoteCoverURL(remote) || e.s.downloadingCover {
		return
	}
	svc, s, old := e.p.Env.Svc, e.s, e.g
	s.downloadingCover = true
	e.p.Env.Async(func() func() {
		url, err := svc.Game.DownloadCoverImage(old.ID, remote)
		return func() {
			if s.gameID == old.ID {
				s.downloadingCover = false
			}
			if err != nil {
				e.toast(c, env.Error(locales.GameEditDownloadCoverFailed()))
				return
			}
			if url != "" && s.gameID == old.ID {
				e.coverChanged(c, old, url)
			}
			e.toast(c, env.Success(locales.GameEditDownloadCoverSuccess()))
		}
	})
}

// pasteCover 对应 handleCoverPaste：剪贴板里有图片时接管粘贴，存为本地封面；只有文字时照常粘贴。
func (e *editor) pasteCover(c *ui.Context, _ string) bool {
	if e.p.Env.Win == nil {
		return false // 没有窗口（测试）时读不了系统剪贴板
	}
	png := mygo.Clipboard.ReadImage()
	if len(png) == 0 {
		return false
	}
	e.importCoverPNG(c, png)
	return true
}

// importCoverPNG 对应 importCoverDataURL：以 data URL 交给 SaveCoverImageDataURL。
func (e *editor) importCoverPNG(c *ui.Context, png []byte) {
	svc, s, old := e.p.Env.Svc, e.s, e.g
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	e.p.Env.Async(func() func() {
		url, err := svc.Game.SaveCoverImageDataURL(old.ID, dataURL)
		return func() {
			if err != nil {
				e.toast(c, env.Error(locales.GameEditImportFromClipboardFailed()))
				return
			}
			if url != "" && s.gameID == old.ID {
				e.coverChanged(c, old, url)
			}
			e.toast(c, env.Success(locales.GameEditImportFromClipboardSuccess()))
		}
	})
}

// deleteGame 对应 confirmDeleteGame：删除后回到游戏库。
func (e *editor) deleteGame(c *ui.Context) {
	svc, id, onDeleted := e.p.Env.Svc, e.g.ID, e.p.OnDeleted
	e.p.Env.Async(func() func() {
		err := svc.Game.DeleteGame(id)
		return func() {
			if err != nil {
				e.toast(c, env.Error(locales.GameToastDeleteFailed()))
				return
			}
			e.toast(c, env.Success(locales.GameToastDeleteSuccess()))
			if onDeleted != nil {
				onDeleted()
			}
		}
	})
}

// updateFromRemote 对应 handleUpdateFromRemote：按勾选的字段从默认来源更新。
func (e *editor) updateFromRemote(c *ui.Context, fields []enums.MetadataUpdateField) {
	s, svc, old := e.s, e.p.Env.Svc, e.g
	if len(fields) == 0 {
		fields = enums.DefaultMetadataUpdateFields
	}
	s.lastFields = slices.Clone(fields)
	s.fieldsOpen = false
	s.updating = true
	e.p.Env.Async(func() func() {
		err := svc.Game.UpdateGameFromRemoteWithFields(old.ID, fields)
		return func() {
			if s.gameID == old.ID {
				s.updating = false
			}
			if err != nil {
				e.toast(c, env.Error(locales.GameToastUpdateRemoteFailed(err.Error())))
				return
			}
			if s.gameID == old.ID {
				e.coverChanged(c, old, "") // 页面重新读取游戏与标签，封面也重新加载
			}
			e.toast(c, env.Success(locales.GameToastUpdateRemoteSuccess()))
		}
	})
}

// sourceAction 对应 runSourceAction：busySource 标出正在操作的按钮，失败时提示。work 在后台执行，after 在成功后回到主线程执行。
func (e *editor) sourceAction(c *ui.Context, key string, work func() error, after func()) {
	s, id := e.s, e.g.ID
	s.busySource = key
	e.p.Env.Async(func() func() {
		err := work()
		return func() {
			if s.gameID != id {
				return
			}
			s.busySource = ""
			if err != nil {
				e.toast(c, env.Error(locales.GameEditSourceOperationFailed(err.Error())))
				return
			}
			after()
		}
	})
}

// addSource 是“添加来源”：handleUpsertMetadataSource，成功后清空 ID，并把下拉框换到下一个还没配置的来源。
func (e *editor) addSource(c *ui.Context) {
	s, svc, g := e.s, e.p.Env.Svc, e.g
	source, sourceID := s.sourceDraftType, s.sourceDraftID
	e.sourceAction(c, "add", func() error { return svc.Game.UpsertGameMetadataSource(g.ID, source, sourceID) }, func() {
		e.p.OnSaved()
		e.toast(c, env.Success(locales.GameEditSourceSaved()))
		s.sourceDraftID = ""
		for _, t := range metadataSourceTypes {
			if t != source && !configured(&g, t) {
				s.sourceDraftType = t
				break
			}
		}
	})
}

// deleteSource 对应删除按钮：取消这个来源待保存的 ID，handleDeleteMetadataSource，再丢掉输入框里的修改。
func (e *editor) deleteSource(c *ui.Context, source enums.SourceType) {
	s, svc, id := e.s, e.p.Env.Svc, e.g.ID
	delete(s.sourceSaves, source)
	e.sourceAction(c, string(source)+":delete", func() error { return svc.Game.DeleteGameMetadataSource(id, source) }, func() {
		e.p.OnSaved()
		e.toast(c, env.Success(locales.GameEditSourceDeleted()))
		delete(s.sourceEdits, source)
	})
}

// setDefaultSource 对应 handleSetDefaultMetadataSource。
func (e *editor) setDefaultSource(c *ui.Context, source enums.SourceType) {
	svc, id := e.p.Env.Svc, e.g.ID
	e.sourceAction(c, string(source)+":default", func() error { return svc.Game.SetDefaultMetadataSource(id, source) }, func() {
		e.p.OnSaved()
		e.toast(c, env.Success(locales.GameEditDefaultSourceSaved()))
	})
}

// scheduleSourceSave 对应 scheduleMetadataSourceAutoSave：输入停止 500ms 后保存，清空时不保存。
func (e *editor) scheduleSourceSave(c *ui.Context, source enums.SourceType, id string) {
	delete(e.s.sourceSaves, source)
	if id = strings.TrimSpace(id); id != "" {
		e.s.sourceSaves[source] = pendingSave{id: id, due: c.Now().Add(sourceSaveDelay)}
	}
}

// tickSourceSaves 保存到期的来源 ID（原版的 setTimeout），还没到期时安排下一帧。
func (e *editor) tickSourceSaves(c *ui.Context) {
	now := c.Now()
	var wait time.Duration
	for source, p := range e.s.sourceSaves {
		if d := p.due.Sub(now); d > 0 {
			if wait == 0 || d < wait {
				wait = d
			}
			continue
		}
		delete(e.s.sourceSaves, source)
		e.persistSourceID(source, p.id)
	}
	if wait > 0 {
		c.After(wait)
	}
}

// flushSourceSaves 立即保存全部待保存的来源 ID（原版在面板卸载时补存）。
func (e *editor) flushSourceSaves() {
	for source, p := range e.s.sourceSaves {
		e.persistSourceID(source, p.id)
	}
	clear(e.s.sourceSaves)
}

// persistSourceID 对应 persistMetadataSourceID / handleAutoSaveMetadataSource。原版保存后只把 ID 合并进页面里的游戏，
// 这里让页面重新读取（抽屉打开时其他字段不会有未保存的修改）。
func (e *editor) persistSourceID(source enums.SourceType, sourceID string) {
	svc, s, id, onSaved, ev := e.p.Env.Svc, e.s, e.g.ID, e.p.OnSaved, e.p.Env
	ev.Async(func() func() {
		err := svc.Game.UpsertGameMetadataSource(id, source, sourceID)
		return func() {
			if err != nil {
				ev.Notify(env.Error(locales.GameEditSourceOperationFailed(err.Error())))
				return
			}
			if s.gameID == id {
				onSaved()
			}
		}
	})
}

// searchByName 对应 searchMetadataByName + handleSearchMetadataByName：先保存待保存的来源 ID（失败则不搜索），
// 再按当前名称搜索全部来源；打开结果弹窗并关上抽屉。
func (e *editor) searchByName(c *ui.Context, mode searchMode) {
	s, svc, id := e.s, e.p.Env.Svc, e.g.ID
	if s.searching {
		return
	}
	pending := make(map[enums.SourceType]string, len(s.sourceSaves))
	for source, p := range s.sourceSaves {
		pending[source] = p.id
	}
	clear(s.sourceSaves)
	name := strings.TrimSpace(e.g.Name)
	s.searching = true
	e.p.Env.Async(func() func() {
		var saveErr error
		for source, sourceID := range pending {
			if err := svc.Game.UpsertGameMetadataSource(id, source, sourceID); err != nil && saveErr == nil {
				saveErr = err
			}
		}
		var results []vo.GameMetadataFromWebVO
		var err error
		if saveErr == nil {
			results, err = svc.Game.FetchMetadataByName(name)
		}
		return func() {
			if s.gameID != id {
				return
			}
			s.searching = false
			if len(pending) > 0 {
				e.p.OnSaved()
			}
			switch {
			case saveErr != nil:
				e.toast(c, env.Error(locales.GameEditSourceOperationFailed(saveErr.Error())))
			case err != nil:
				e.toast(c, env.Error(locales.GameEditMetadataSearchFailed(err.Error())))
			default:
				s.results = results
				s.searchMode = mode
				s.resultsUI.Reset()
				s.searchOpen = true
				s.drawerOpen = false
			}
		}
	})
}

// selectSearchResult 对应 handleSelectMetadataSearchResult：每个来源只能保留一个结果；把保留的结果都关联到游戏，
// 选中的来源设为默认（“更新元数据”时先从它更新游戏信息）。
func (e *editor) selectSearchResult(c *ui.Context, result vo.GameMetadataFromWebVO) {
	s, svc, old := e.s, e.p.Env.Svc, e.g
	if strings.TrimSpace(result.Game.SourceID) == "" || s.applying {
		return
	}
	type link struct {
		source enums.SourceType
		id     string
	}
	var links []link
	seen := map[enums.SourceType]bool{}
	for _, item := range s.results {
		itemID := strings.TrimSpace(item.Game.SourceID)
		if itemID == "" {
			continue
		}
		if seen[item.Source] {
			e.toast(c, env.Error(locales.AddGameModalToastDuplicateSource(resultSourceLabel(item.Source))))
			return
		}
		seen[item.Source] = true
		links = append(links, link{item.Source, itemID})
	}
	if len(links) == 0 {
		return
	}
	mode := s.searchMode
	s.applying = true
	e.p.Env.Async(func() func() {
		err := func() error {
			for _, l := range links {
				if err := svc.Game.UpsertGameMetadataSource(old.ID, l.source, l.id); err != nil {
					return err
				}
			}
			if mode == searchUpdateMetadata {
				if err := svc.Game.UpdateGameFromRemoteBySource(old.ID, result.Source); err != nil {
					return err
				}
			}
			return svc.Game.SetDefaultMetadataSource(old.ID, result.Source)
		}()
		return func() {
			if s.gameID != old.ID {
				return
			}
			s.applying = false
			if err != nil {
				if mode == searchUpdateMetadata {
					e.toast(c, env.Error(locales.GameEditMetadataSearchApplyFailed(err.Error())))
				} else {
					e.toast(c, env.Error(locales.GameEditMetadataSearchSourceApplyFailed(err.Error())))
				}
				return
			}
			if mode == searchUpdateMetadata {
				e.coverChanged(c, old, "")
			} else {
				e.p.OnSaved()
			}
			s.searchOpen = false
			if mode == searchUpdateMetadata {
				e.toast(c, env.Success(locales.GameEditMetadataSearchApplySuccess()))
			} else {
				e.toast(c, env.Success(locales.GameEditMetadataSearchSourceApplySuccess()))
			}
		}
	})
}
