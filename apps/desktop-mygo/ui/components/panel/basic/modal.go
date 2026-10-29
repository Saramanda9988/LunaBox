package basic

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"lunabox/backend/service"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// 以下对应原版 components/modal/GameLibraryPathChangeModal.tsx。

// maxVisibleGames 对应 MAX_VISIBLE_GAMES。
const maxVisibleGames = 200

// monoFont 是 font-mono 在 Windows 上实际生效的字体（ui-monospace、SFMono-Regular、Menlo、Monaco 都没有）。
const monoFont = "Consolas"

// gamePathChange 对应 GamePathChangePreview：一个游戏的代表性路径变化。
type gamePathChange struct {
	key          string
	name         string
	oldPath      string
	newPath      string
	targetExists bool
	priority     int // representativePriority
}

// fieldPriority 对应 fieldPriority：游戏目录优先，其次是可执行文件、存档、下载文件。
func fieldPriority(field string) int {
	switch field {
	case "game_directory":
		return 0
	case "path":
		return 1
	case "save_path":
		return 2
	case "file_path":
		return 3
	}
	return 4
}

// groupChangesByGame 对应 groupChangesByGame：只取游戏记录，每个游戏一行，路径取优先级最高的字段，
// 所有字段的目标都存在才算存在。顺序是游戏第一次出现的顺序（Map 的插入顺序）。
func groupChangesByGame(changes []service.GameLibraryPathChangeItem) []gamePathChange {
	var out []gamePathChange
	index := map[string]int{}
	for _, ch := range changes {
		if ch.RecordType != "game" {
			continue
		}
		key := "game:" + ch.RecordID
		priority := fieldPriority(ch.Field)
		i, ok := index[key]
		if !ok {
			index[key] = len(out)
			out = append(out, gamePathChange{key: key, name: ch.RecordName, oldPath: ch.OldPath, newPath: ch.NewPath,
				targetExists: ch.TargetExists, priority: priority})
			continue
		}
		g := &out[i]
		g.targetExists = g.targetExists && ch.TargetExists
		if priority < g.priority {
			g.oldPath, g.newPath, g.priority = ch.OldPath, ch.NewPath, priority
		}
	}
	return out
}

// hasDirectoryPrefix 对应 hasDirectoryPrefix：不区分大小写的前缀，且在路径分隔符处断开。
func hasDirectoryPrefix(path, prefix string) bool {
	if prefix == "" || len(path) < len(prefix) || !strings.EqualFold(path[:len(prefix)], prefix) {
		return false
	}
	rest := path[len(prefix):]
	return rest == "" || rest[0] == '\\' || rest[0] == '/'
}

// splitRebasedPath 对应 splitRebasedPath：新旧路径都在各自的游戏库目录下、余下部分相同时，只标出游戏库目录的变化。
func splitRebasedPath(oldPath, newPath, oldLibrary, newLibrary string) (oldPrefix, newPrefix, suffix string) {
	if hasDirectoryPrefix(oldPath, oldLibrary) && hasDirectoryPrefix(newPath, newLibrary) {
		oldSuffix, newSuffix := oldPath[len(oldLibrary):], newPath[len(newLibrary):]
		if strings.EqualFold(oldSuffix, newSuffix) {
			return oldLibrary, newLibrary, newSuffix
		}
	}
	return oldPath, newPath, ""
}

// libraryModal 对应 GameLibraryPathChangeModal：preview 为空时不显示。遮罩 absolute inset-0 z-50 bg-black/45 p-4
// backdrop-blur-[3px] sm:p-6，面板 w-full max-w-3xl rounded-2xl border border-brand-200 bg-white/98 shadow-2xl
// shadow-black/20 backdrop-blur-20 dark:border-brand-700 dark:bg-brand-800/98。应用期间 Esc 与点击遮罩不关闭。
func (s *State) libraryModal(c *ui.Context, t *theme.Theme, p panel.Settings) {
	pv := s.libPreview
	if pv == nil {
		return
	}
	winW, winH := c.Size()
	sm := winW >= smBreakpoint
	closeModal := func() {
		if !s.applying {
			s.closeLibraryChange()
		}
	}
	title := locales.SettingsBasicLibraryChangeTitle()
	components.Overlay(c, t, func() {
		host := ui.Box(c.Key("library-change-modal")).Absolute().Top(panel.TopOffset).Left(0).Right(0).Bottom(0).Modal()
		host.OnShortcut(0, ui.KeyEscape, closeModal) // onKeyDownCapture：Escape 关闭
		host.Children(func() {
			backdrop := ui.ButtonBase(c.Key("backdrop")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
				Material(glass.Blur{Radius: 3}).FocusRing(false) // onMouseDown 在遮罩本身上时关闭（原版的遮罩没有无障碍名称）
			backdrop.OnClick(closeModal).Children(func() {
				ui.Box(c).Fill().Background(ui.RGBA(0, 0, 0, 0.45))
			})
			pad := float32(16)
			if sm {
				pad = 24
			}
			gutter := float32(0)
			if s.libOuter.Overflowing() {
				gutter = t.Gutter()
			}
			// 遮罩 overflow-y-auto > flex min-h-full items-center justify-center：弹窗比窗口还高时整个遮罩滚动。
			// 滚动区让指针穿过，点在弹窗外面时仍是遮罩（关闭）。
			components.ScrollArea(c, t, "scroll", &s.libOuter, func(w ui.Element) {
				w.Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough()
			}, func(sc ui.Element) {
				sc.PassThrough().Children(func() {
					ui.Column(c).FillWidth().MinHeight(max(0, winH-panel.TopOffset)).Center().Padding(pad, pad+gutter, pad, pad).PassThrough().Children(func() {
						dialog := ui.Column(c.Key("dialog")).FillWidth().MaxWidth(768).Radius(16).
							Background(t.Pick(theme.White.Alpha(0.98), theme.Brand800.Alpha(0.98))).
							Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).Material(glass.Blur{Radius: 20}).
							Shadow(0, 25, 50, -12, ui.RGBA(0, 0, 0, 0.2)).Role(ui.RoleDialog).Label(title)
						dialog.Children(func() {
							s.modalHeader(c, t, pv, sm, title, closeModal)
							s.modalBody(c, t, pv, sm, winH)
							s.modalFooter(c, t, p, pv, sm)
						})
					})
				})
			})
		})
	})
}

// modalHeader：border-b px-5 py-5 sm:px-6，flex items-start gap-4。左侧 h-11 w-11 rounded-xl border border-primary-200
// bg-primary-50 text-primary-600 dark:border-primary-800 dark:bg-primary-900/35 dark:text-primary-300 的
// i-mdi-folder-sync-outline text-2xl；标题 text-lg font-semibold，说明 mt-1 text-sm text-brand-500；右侧 h-9 w-9
// rounded-lg 的关闭按钮（autoFocus）。
func (s *State) modalHeader(c *ui.Context, t *theme.Theme, pv *service.GameLibraryPathChangePreview, sm bool, title string, closeModal func()) {
	px := float32(20)
	if sm {
		px = 24
	}
	ui.Row(c.Key("header")).FillWidth().Padding(20, px).Gap(16).AlignItems(ui.Start).
		BorderWidth(0, 0, t.HL, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
		ui.Box(c).Size(44, 44).Shrink(0).Center().Radius(12).
			Background(t.Pick(theme.Primary50, theme.Primary900.Alpha(0.35))).
			Border(t.HL, t.Pick(theme.Primary200, theme.Primary800)).Children(func() {
			components.Icon(c, "folder-sync-outline", 24, t.Pick(theme.Primary600, theme.Primary300))
		})
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			components.Text(c, title, theme.TextLG, theme.WeightSemibold, t.Pick(theme.Brand900, theme.White))
			components.TextMargin(c, locales.SettingsBasicLibraryChangeAffectedGamesDescription(strconv.Itoa(pv.AffectedGameCount)),
				theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400), 4, 0, 0, 0)
		})
		btn := ui.ButtonBase(c.Key("close")).Size(36, 36).Shrink(0).Center().Radius(8).FocusRing(false).AutoFocus().
			Label(locales.SettingsBasicLibraryChangeClose())
		color := t.Pick(theme.Brand500, theme.Brand400)
		if s.applying {
			components.Disable(btn, 0.5)
		} else {
			if btn.Hovered() {
				btn.Background(t.Pick(theme.Brand100, theme.Brand700))
				color = t.Pick(theme.Brand800, theme.White)
			}
			btn.OnClick(closeModal)
		}
		btn.Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
			components.Icon(c, "close", 20, color)
		})
	})
}

// modalBody：max-h-[58dvh] space-y-4 overflow-y-auto p-5 sm:p-6，依次是下载任务阻止、目标缺失、Steam 三条提示，
// 以及路径变化列表（或“没有受影响的记录”）。
func (s *State) modalBody(c *ui.Context, t *theme.Theme, pv *service.GameLibraryPathChangePreview, sm bool, winH float32) {
	pad := float32(20)
	if sm {
		pad = 24
	}
	missing := 0
	for _, ch := range s.libChanges {
		if !ch.targetExists {
			missing++
		}
	}
	gutter := float32(0)
	if s.libScroll.Overflowing() {
		gutter = t.Gutter()
	}
	components.ScrollArea(c, t, "body", &s.libScroll, func(w ui.Element) { w.FillWidth().MaxHeight(max(0, winH*0.58)) }, func(sc ui.Element) {
		sc.Padding(pad, pad+gutter, pad, pad).Children(func() {
			ui.Column(c.Key("content")).FillWidth().Gap(16).Children(func() {
				if n := pv.BlockingDownloadTaskCount; n > 0 {
					noticeBox(c, t, "blocked", "download-lock", locales.SettingsBasicLibraryChangeDownloadBlocked(strconv.Itoa(n)),
						t.Pick(theme.Error200, theme.Error800.Alpha(0.7)), t.Pick(theme.Error50, theme.Error900.Alpha(0.2)),
						t.Pick(theme.Error700, theme.Error300))
				}
				if missing > 0 {
					noticeBox(c, t, "missing", "folder-alert-outline", locales.SettingsBasicLibraryChangeMissingWarning(strconv.Itoa(missing)),
						t.Pick(theme.Warning200, theme.Warning800.Alpha(0.7)), t.Pick(theme.Warning50, theme.Warning900.Alpha(0.2)),
						t.Pick(theme.Warning800, theme.Warning300))
				}
				if n := pv.SteamGameCount; n > 0 {
					noticeBox(c, t, "steam", "steam", locales.SettingsBasicLibraryChangeSteamWarning(strconv.Itoa(n)),
						t.Pick(theme.Info200, theme.Info800.Alpha(0.7)), t.Pick(theme.Info50, theme.Info900.Alpha(0.2)),
						t.Pick(theme.Info800, theme.Info300))
				}
				if len(s.libChanges) > 0 {
					s.changeList(c, t, pv, sm)
				} else {
					emptyChanges(c, t)
				}
			})
		})
	})
}

// noticeBox：flex items-start gap-3 rounded-xl border px-4 py-3，图标 mt-0.5 shrink-0 text-xl，文字 text-sm leading-6。
func noticeBox(c *ui.Context, t *theme.Theme, key, icon, text string, border, bg, fg ui.Color) {
	ui.Row(c.Key(key)).FillWidth().Padding(12, 16).Gap(12).AlignItems(ui.Start).Radius(12).
		Background(bg).Border(t.HL, border).Children(func() {
		components.Icon(c, icon, 20, fg).Margin(2, 0, 0, 0)
		components.Text(c, text, theme.FontStep{Size: 14, Line: 24}, theme.WeightNormal, fg).Grow(1).Basis(0).MinWidth(0)
	})
}

// emptyChanges：flex items-center gap-3 rounded-xl border border-brand-200 px-4 py-4 text-brand-600 dark:border-brand-700
// dark:text-brand-300，i-mdi-check-circle-outline text-xl text-success-500 与 text-sm 的说明。
func emptyChanges(c *ui.Context, t *theme.Theme) {
	ui.Row(c.Key("empty")).FillWidth().Padding(16).Gap(12).AlignItems(ui.Center).Radius(12).
		Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
		components.Icon(c, "check-circle-outline", 20, theme.Success500)
		components.Text(c, locales.SettingsBasicLibraryChangeNoAffectedRecords(), theme.TextSM, theme.WeightNormal,
			t.Pick(theme.Brand600, theme.Brand300)).Grow(1).Basis(0).MinWidth(0)
	})
}

// changeList：section space-y-2，标题 text-sm font-semibold text-brand-800 dark:text-brand-200，列表 overflow-hidden
// rounded-xl border；每行 grid gap-2 px-3.5 py-3 sm:grid-cols-[9rem_minmax(0,1fr)_auto] sm:items-center，第二行起 border-t。
// 最多显示 200 个游戏，其余用一行 text-xs 说明。
func (s *State) changeList(c *ui.Context, t *theme.Theme, pv *service.GameLibraryPathChangePreview, sm bool) {
	visible := s.libChanges
	if len(visible) > maxVisibleGames {
		visible = visible[:maxVisibleGames]
	}
	hidden := len(s.libChanges) - len(visible)
	border := t.Pick(theme.Brand200, theme.Brand700)
	ui.Column(c.Key("changes")).FillWidth().Gap(8).Children(func() {
		components.Text(c, locales.SettingsBasicLibraryChangePreviewTitle(), theme.TextSM, theme.WeightSemibold, t.Pick(theme.Brand800, theme.Brand200))
		ui.Column(c.Key("list")).FillWidth().Radius(12).Clip().Border(t.HL, border).Children(func() {
			for i, ch := range visible {
				var row ui.Element
				if sm {
					row = ui.Row(c.Key(ch.key)).AlignItems(ui.Center)
				} else {
					row = ui.Column(c.Key(ch.key)).AlignItems(ui.Start)
				}
				row.FillWidth().Padding(12, 14).Gap(8)
				if i > 0 {
					row.BorderWidth(t.HL, 0, 0, 0).BorderColor(border)
				}
				row.Children(func() {
					name := ui.Column(c.Key("name")).MinWidth(0)
					if sm {
						name.Width(144).Shrink(0)
					} else {
						name.AlignSelf(ui.Stretch)
					}
					name.Children(func() {
						components.Text(c, ch.name, theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand800, theme.Brand200)).SingleLine()
					})
					diff := ui.Column(c.Key("diff")).MinWidth(0)
					if sm {
						diff.Grow(1).Basis(0)
					} else {
						diff.AlignSelf(ui.Stretch)
					}
					diff.Children(func() { pathPrefixDiff(c, t, ch, pv.OldLibraryPath, pv.NewLibraryPath) })
					statusBadge(c, t, ch.targetExists)
				})
			}
		})
		if hidden > 0 {
			components.Text(c, locales.SettingsBasicLibraryChangeMoreChanges(strconv.Itoa(hidden)), theme.TextXS, theme.WeightNormal,
				t.Pick(theme.Brand500, theme.Brand400))
		}
	})
}

// pathPrefixDiff 对应 PathPrefixDiff：一行 font-mono text-xs text-brand-800 dark:text-brand-200（超出时省略号），
// 旧前缀 rounded bg-error-100 text-error-700 line-through decoration-error-500/70（dark:bg-error-900/35 dark:text-error-300），
// mx-1.5 text-brand-400 的箭头，新前缀 rounded bg-success-100 text-success-700（dark:bg-success-900/35 dark:text-success-300），
// 最后是相同的后半段。title 与 aria-label 是“旧路径 → 新路径”。
// 原版前缀的 px-1 py-0.5 在行内文字里做不到，背景用带圆角、贴着字形的 Span 背景。
func pathPrefixDiff(c *ui.Context, t *theme.Theme, ch gamePathChange, oldLibrary, newLibrary string) {
	oldPrefix, newPrefix, suffix := splitRebasedPath(ch.oldPath, ch.newPath, oldLibrary, newLibrary)
	space := ui.Span{Text: "\uFFFC", ObjectWidth: 6, ObjectHeight: 1} // mx-1.5
	f := theme.TextXS
	full := ch.oldPath + " → " + ch.newPath
	d := components.LineOverflow(ui.Span{Text: full, Font: monoFont, Size: f.Size}, f.Line)
	ui.RichText(c,
		ui.Span{Text: oldPrefix, Background: t.Pick(theme.Error100, theme.Error900.Alpha(0.35)), Radius: 4,
			Color: t.Pick(theme.Error700, theme.Error300), Strikethrough: true, DecorationColor: theme.Error500.Alpha(0.7)},
		space,
		ui.Span{Text: "→", Color: theme.Brand400},
		space,
		ui.Span{Text: newPrefix, Background: t.Pick(theme.Success100, theme.Success900.Alpha(0.35)), Radius: 4,
			Color: t.Pick(theme.Success700, theme.Success300)},
		ui.Span{Text: suffix},
	).Font(monoFont).FontSize(f.Size).FixedLineHeight(f.Line).TextColor(t.Pick(theme.Brand800, theme.Brand200)).
		SingleLine().Margin(-d, 0, -d, 0).Label(full).Tooltip(full)
}

// statusBadge：inline-flex w-fit items-center gap-1 rounded-full px-2 py-1 text-[11px] font-medium，目标存在时
// bg-success-50 text-success-700 dark:bg-success-900/25 dark:text-success-300 的 i-mdi-check-circle，
// 否则 bg-warning-50 text-warning-700 dark:bg-warning-900/25 dark:text-warning-300 的 i-mdi-alert-circle-outline。
func statusBadge(c *ui.Context, t *theme.Theme, exists bool) {
	icon, bg, fg := "check-circle", t.Pick(theme.Success50, theme.Success900.Alpha(0.25)), t.Pick(theme.Success700, theme.Success300)
	if !exists {
		icon, bg, fg = "alert-circle-outline", t.Pick(theme.Warning50, theme.Warning900.Alpha(0.25)), t.Pick(theme.Warning700, theme.Warning300)
	}
	ui.Row(c.Key("status")).Shrink(0).Padding(4, 8).Gap(4).AlignItems(ui.Center).Radius(999).Background(bg).Children(func() {
		components.Icon(c, icon, 11, fg)
	})
}

// modalFooter：flex justify-end gap-2 border-t px-5 py-4 sm:px-6，“只改目录”（secondary，应用期间禁用）与
// “确认更改”（primary，有下载任务阻止时禁用，应用期间显示加载）。
func (s *State) modalFooter(c *ui.Context, t *theme.Theme, p panel.Settings, pv *service.GameLibraryPathChangePreview, sm bool) {
	px := float32(20)
	if sm {
		px = 24
	}
	ui.Row(c.Key("footer")).FillWidth().Padding(16, px).Gap(8).Justify(ui.End).
		BorderWidth(t.HL, 0, 0, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
		better.Button(c, t, "skip", better.ButtonProps{
			Variant: better.ButtonSecondary, Disabled: s.applying, Label: locales.SettingsBasicLibraryChangeSkipPathUpdate(),
		}).OnClick(func() { s.applyLibraryChange(p, false) })
		better.Button(c, t, "confirm", better.ButtonProps{
			Variant: better.ButtonPrimary, Disabled: pv.BlockingDownloadTaskCount > 0, Loading: s.applying, Icon: "check",
			Label: locales.SettingsBasicLibraryChangeConfirmChange(),
		}).OnClick(func() { s.applyLibraryChange(p, true) })
	})
}
