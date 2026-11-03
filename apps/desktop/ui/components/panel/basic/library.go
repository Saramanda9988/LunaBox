package basic

import (
	"log"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/service"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// selectDirectory 对应 SelectDirectory：打开原生的文件夹对话框，会阻塞，在 Env.Async 里调用。测试里替换掉，不弹对话框。
var selectDirectory = func(e *env.Env, title string) (string, error) {
	return e.Svc.AppConfig.SelectDirectory(title)
}

// libraryInput 对应游戏库目录的 BetterActionInput（className="text-sm"）：Enter 预览并应用，Esc 放弃输入；
// 右侧是选择文件夹与“扫描路径”（预览期间显示旋转图标），预览或应用期间整个输入框禁用。
func (s *State) libraryInput(c *ui.Context, t *theme.Theme, p panel.Settings) {
	value := p.Config.GameLibraryPath // gameLibraryInput = pendingGameLibraryInput ?? formData.game_library_path
	if s.libPending {
		value = s.libInput
	}
	busy := s.previewLoading || s.applying
	v := value
	in := better.ActionInput(c, t, "input", &v, better.InputProps{
		Placeholder: locales.SettingsBasicGameLibraryPathPlaceholder(),
		Disabled:    busy,
	},
		better.InputAction{Icon: "folder-open-outline", Label: locales.SettingsBasicSelectGameLibraryTitle(), OnClick: func() { s.selectLibraryPath(p) }},
		better.InputAction{Icon: "refresh", Label: locales.SettingsBasicLibraryChangeScanPaths(), Loading: s.previewLoading,
			OnClick: func() { s.requestLibraryChange(p, value, false) }},
	)
	in.FontSize(theme.TextSM.Size).FixedLineHeight(theme.TextSM.Line)
	if in.Changed() {
		s.libInput, s.libPending = v, true
		value = v
	}
	switch {
	case in.Submitted():
		s.requestLibraryChange(p, value, true)
	case in.Shortcut(0, ui.KeyEscape):
		s.libInput, s.libPending = "", false
	}
}

// requestLibraryChange 对应 requestGameLibraryPathChange：预览新目录会改动的记录。没有受影响的游戏时，
// saveWhenEmpty 且目录确实变了就直接保存新目录（不同步路径），否则只提示；有受影响的游戏时打开确认弹窗。
func (s *State) requestLibraryChange(p panel.Settings, newPath string, saveWhenEmpty bool) {
	s.previewLoading = true
	s.libGen++
	gen, e := s.libGen, p.Env
	e.Async(func() func() {
		preview, err := e.Svc.AppConfig.PreviewGameLibraryPathChange(newPath)
		return func() {
			if gen != s.libGen {
				return
			}
			if err != nil {
				log.Printf("settings: failed to preview game library path change: %v", err)
				s.previewLoading = false
				s.toast(env.Error(locales.SettingsBasicLibraryChangePreviewFailed()))
				return
			}
			if preview.AffectedGameCount > 0 {
				s.previewLoading = false
				s.showLibraryChange(preview)
				return
			}
			s.libPreview, s.libChanges = nil, nil
			changed := strings.TrimSpace(newPath) != strings.TrimSpace(p.Config.GameLibraryPath)
			if !saveWhenEmpty || !changed {
				s.previewLoading = false
				s.toast(env.Success(locales.SettingsBasicLibraryChangeNoAffectedRecords()))
				return
			}
			// 原版在应用结束前一直处于预览中（外层 finally 在 await 之后）。
			s.applying = true
			s.applyLibraryPath(p, preview.NewConfiguredPath, false, func(_ service.GameLibraryPathChangeResult, err error) {
				s.applying, s.previewLoading = false, false
				if err != nil {
					s.toast(env.Error(locales.SettingsBasicLibraryChangeApplyFailed()))
					return
				}
				s.libInput, s.libPending = "", false
				s.toast(env.Success(locales.SettingsBasicLibraryChangeChangeWithoutAffectedGamesSuccess()))
			})
		}
	})
}

// selectLibraryPath 对应 handleSelectGameLibraryPath：选择文件夹后填进输入框并预览。
func (s *State) selectLibraryPath(p panel.Settings) {
	e, title := p.Env, locales.SettingsBasicSelectGameLibraryTitle()
	e.Async(func() func() {
		path, err := selectDirectory(e, title)
		return func() {
			if err != nil {
				log.Printf("settings: failed to select game library path: %v", err)
				s.toast(env.Error(locales.SettingsBasicSelectGameLibraryFailed()))
				return
			}
			if path != "" {
				s.libInput, s.libPending = path, true
				s.requestLibraryChange(p, path, true)
			}
		}
	})
}

// applyLibraryPath 对应 store 的 applyGameLibraryPathChange：ApplyGameLibraryPathChange 保存新目录（syncPaths 时
// 同时改写游戏与下载任务里的路径），成功后生效的配置与草稿的 game_library_path 都改成新目录。原版之后还会刷新首页数据
// （fetchHomeData），首页还没有迁移。done 在主线程上执行。
func (s *State) applyLibraryPath(p panel.Settings, newPath string, syncPaths bool, done func(service.GameLibraryPathChangeResult, error)) {
	e := p.Env
	e.Async(func() func() {
		result, err := e.Svc.AppConfig.ApplyGameLibraryPathChange(newPath, syncPaths)
		return func() {
			if err != nil {
				log.Printf("settings: failed to apply game library path change: %v", err)
			} else {
				path := result.NewConfiguredPath
				p.PatchLive(func(cfg *appconf.AppConfig) { cfg.GameLibraryPath = path })
			}
			done(result, err)
		}
	})
}

// showLibraryChange 打开确认弹窗（setLibraryChangePreview）。
func (s *State) showLibraryChange(preview service.GameLibraryPathChangePreview) {
	s.libPreview = &preview
	s.libChanges = groupChangesByGame(preview.Changes)
	s.libScroll, s.libOuter = components.ScrollBar{}, components.ScrollBar{}
}

// closeLibraryChange 对应 handleCloseLibraryChange：关闭弹窗并放弃输入。
func (s *State) closeLibraryChange() {
	s.libPreview, s.libChanges = nil, nil
	s.libInput, s.libPending = "", false
}

// applyLibraryChange 对应 handleApplyLibraryChange：在弹窗里确认（syncPaths）或只改目录。
func (s *State) applyLibraryChange(p panel.Settings, syncPaths bool) {
	if s.libPreview == nil || s.applying {
		return
	}
	s.applying = true
	s.applyLibraryPath(p, s.libPreview.NewConfiguredPath, syncPaths, func(result service.GameLibraryPathChangeResult, err error) {
		s.applying = false
		if err != nil {
			s.toast(env.Error(locales.SettingsBasicLibraryChangeApplyFailed()))
			return
		}
		s.libInput, s.libPending = "", false
		s.libPreview, s.libChanges = nil, nil
		if syncPaths {
			s.toast(env.Success(locales.SettingsBasicLibraryChangeChangeSuccess(strconv.Itoa(result.UpdatedGameCount))))
		} else {
			s.toast(env.Success(locales.SettingsBasicLibraryChangeChangeWithoutSyncSuccess()))
		}
	})
}
