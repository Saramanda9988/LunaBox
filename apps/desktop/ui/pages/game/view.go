package game

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/models"
	"lunabox/backend/utils/apputils"
	"lunabox/desktop-mygo/ui/assets"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel/backup"
	"lunabox/desktop-mygo/ui/components/panel/edit"
	"lunabox/desktop-mygo/ui/components/panel/launch"
	"lunabox/desktop-mygo/ui/components/panel/progress"
	"lunabox/desktop-mygo/ui/components/panel/review"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/tagname"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

const (
	pagePadding = 32  // p-8
	coverW      = 240 // grid-cols-[15rem_minmax(0,1fr)]
	headerGap   = 24  // gap-6
	toolbarGap  = 12  // gap-3
)

// View 构建页面。滚动容器是原版的 <main>（overflow-auto）：内容超出时才出现 9px 的滚动条槽。
func (p *Page) View(c *ui.Context) {
	t := p.env.Theme
	if wait := p.autoSave(c.Now()); wait > 0 {
		c.After(wait)
	}
	if p.failed != nil {
		p.env.Toast(c, env.Error(p.failed()))
		p.failed = nil
	}
	switch {
	case p.loading && p.game == nil:
		if wait := skeletonDelay - c.Now().Sub(p.openedAt); wait > 0 {
			c.After(wait)
			return
		}
		p.skeleton(c, t)
		return
	case p.game == nil:
		p.notFound(c, t)
		return
	}

	gutter := float32(0)
	if p.scroll.Overflowing() {
		gutter = t.Gutter()
	}
	contentW := p.env.MainWidth - 2*pagePadding - gutter
	components.ScrollArea(c, t, "game-scroll", &p.scroll, func(w ui.Element) { w.Fill() }, func(sc ui.Element) {
		sc.Padding(pagePadding, pagePadding+gutter, pagePadding, pagePadding)
		sc.Children(func() {
			// space-y-8 max-w-8xl mx-auto（max-w-8xl 在 UnoCSS 里没有定义，不限宽）
			ui.Column(c).Width(contentW).Margin(0, ui.Auto).Gap(32).Children(func() {
				p.backButton(c, t)
				p.header(c, t, contentW)
				p.tabBar(c, t)
				p.tabContent(c, t)
			})
		})
	})
	p.launcher.Modals(c, t, p.panel())
	p.coverViewer(c, t)
	p.categoryView(c, t)
	p.guideView(c, t)
	p.statsModals(c, t)
	p.addSessionView(c, t)
}

// openViewer 打开封面大图，并在后台读取原尺寸的图片（封面缓存里只有卡片用的小图）。
func (p *Page) openViewer() {
	p.viewerOpen = true
	if p.fullCover != nil || p.game == nil {
		return
	}
	g, gen := *p.game, p.gen
	p.env.Async(func() func() {
		bmp, err := p.env.Covers.LoadFull(&g, 2400)
		return func() {
			if gen == p.gen && err == nil {
				p.fullCover = bmp
			}
		}
	})
}

// coverViewer 是封面的 BetterImageViewer，“另存为”对应 handleSaveCoverImage（ExportCoverImage 弹出保存对话框）。
func (p *Page) coverViewer(c *ui.Context, t *theme.Theme) {
	g := p.game
	better.ImageViewer(c, t, "cover-viewer", better.ImageViewerProps{
		Open:      &p.viewerOpen,
		Title:     g.Name,
		Image:     p.fullCover,
		Preview:   p.env.Covers.Get(g),
		TopOffset: env.TopbarHeight,
		OnSave: func() {
			id := g.ID
			p.env.Async(func() func() {
				_, err := p.env.Svc.Game.ExportCoverImage(id)
				return func() {
					if err != nil {
						p.env.Toast(c, env.Error(locales.GameToastSaveFailed(err.Error())))
					}
				}
			})
		},
	})
}

// tabContent 是当前标签页：游戏统计在本包，其余在 ui/components/panel 的子包。
func (p *Page) tabContent(c *ui.Context, t *theme.Theme) {
	switch p.tab {
	case "stats":
		p.statsView(c, t)
	case "edit":
		edit.View(c, t, &p.editPanel, p.panel())
	case "launch":
		launch.View(c, t, &p.launchPanel, p.panel())
	case "backup":
		backup.View(c, t, &p.backupPanel, p.panel())
	case "progress":
		progress.View(c, t, &p.progress, p.panel())
	case "review":
		review.View(c, t, &p.review, p.panel())
	}
}

// backButton：flex items-center rounded-md text-brand-750 hover:text-brand-900，i-mdi-arrow-left text-2xl mr-1。
func (p *Page) backButton(c *ui.Context, t *theme.Theme) {
	btn := ui.ButtonBase(c.Key("back")).AlignSelf(ui.Start).AlignItems(ui.Center).Radius(6).FocusRing(false)
	color := t.Pick(theme.Brand750, theme.Brand400)
	if btn.Hovered() {
		color = t.Pick(theme.Brand900, theme.Brand200)
	}
	color = btn.AnimateColor("text", color, theme.Tween(150*time.Millisecond))
	btn.OnClick(func() { p.env.Router.Back() }).Children(func() {
		components.Icon(c, "arrow-left", 24, color).Margin(0, 4, 0, 0)
		components.Text(c, locales.CommonBack(), theme.TextBase, theme.WeightNormal, color).NoWrap()
	})
}

// header：grid-cols-[15rem_minmax(0,1fr)] items-stretch gap-6，左侧封面在 min-h-64 的格子里垂直居中。
func (p *Page) header(c *ui.Context, t *theme.Theme, contentW float32) {
	g := p.game
	rightW := contentW - coverW - headerGap
	ui.Row(c.Key("header")).Gap(headerGap).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(coverW).MinHeight(256).Justify(ui.Center).Shrink(0).Children(func() {
			p.cover(c, t, g)
		})
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(16).Children(func() {
			ui.Column(c).Gap(12).Children(func() {
				components.Text(c, g.Name, theme.Text4XL, theme.WeightBold, t.TextStrong)
				p.toolbar(c, t, rightW)
			})
			p.infoGrid(c, t)
			p.summary(c, t)
			components.GameTags(c, t, components.GameTagsProps{
				Tags:      p.tags,
				Translate: tagname.Enabled(p.env.Config()),
				Editor:    &p.tagEditor,
				OnClick:   func(tag models.GameTag) { p.links.FilterByTag(tag.Name) },
				OnDelete:  func(tag models.GameTag) { p.deleteTag(c, tag) },
				OnAdd:     func(name string) { p.addTag(c, name) },
			})
		})
	})
}

// addTag 对应 GameTags 的 handleAddTag：AddUserTag 后重新读取标签并收起输入框。
func (p *Page) addTag(c *ui.Context, name string) {
	id, gen := p.id, p.gen
	p.env.Async(func() func() {
		err := p.env.Svc.Tag.AddUserTag(id, name)
		var tags []models.GameTag
		if err == nil {
			tags, err = p.env.Svc.Tag.GetTagsByGame(id)
		}
		return func() {
			if gen != p.gen {
				return
			}
			if err != nil {
				p.env.Toast(c, env.Error(locales.TagsAddFailed()))
				return
			}
			p.tags, p.tagEditor = tags, components.TagEditor{}
		}
	})
}

// deleteTag 对应 handleDeleteTag：删除自动获取的标签时提示它可能在更新元数据后回来。
func (p *Page) deleteTag(c *ui.Context, tag models.GameTag) {
	gen := p.gen
	p.env.Async(func() func() {
		err := p.env.Svc.Tag.DeleteTag(tag.ID)
		return func() {
			if err != nil {
				p.env.Toast(c, env.Error(locales.TagsDeleteFailed()))
				return
			}
			if gen == p.gen {
				p.tags = slices.DeleteFunc(slices.Clone(p.tags), func(t models.GameTag) bool { return t.ID == tag.ID })
			}
			if tag.Source != "user" {
				p.env.Toast(c, env.Success(locales.TagsDeleteScrapedHint()))
			}
		}
	})
}

// cover 是 GameCoverImage（w-full h-auto rounded-lg），点击打开大图；没有封面时显示“没有封面”。
func (p *Page) cover(c *ui.Context, t *theme.Theme, g *models.Game) {
	bmp := p.env.Covers.Get(g)
	switch {
	case bmp != nil:
		w, h := bmp.Size()
		// cursor-zoom-in：MyGo 没有放大镜光标，用手形。
		btn := ui.ButtonBase(c.Key("cover")).Width(coverW).Height(coverW * float32(h) / float32(max(1, w))).
			Radius(8).Clip().FocusRing(false).Cursor(ui.CursorPointer).Label(locales.GameImageViewerOpen(g.Name))
		btn.OnClick(func() { p.openViewer() }).Children(func() {
			img := ui.Image(c, bmp).Fill().Fit(ui.Cover)
			if g.IsNSFW && blurNSFW(p.env) {
				components.CoverBlur(c, btn, img, true)
			}
		})
	case p.env.Covers.Missing(g):
		ui.Box(c).FillWidth().Grow(1).MinHeight(256).Center().Children(func() {
			components.Text(c, locales.GameNoCover(), theme.TextBase, theme.WeightNormal, theme.Brand400)
		})
	}
}

// toolbar：relative flex flex-wrap items-center gap-3。放不下全部状态按钮时（requiredWidth + 4 > clientWidth）
// 改成状态下拉菜单；宽度按文字量出来，不用等布局。
func (p *Page) toolbar(c *ui.Context, t *theme.Theme, width float32) {
	g := p.game
	status := currentStatus(g)
	links := sourceLinks(g, erogameScapeBase(p.env))
	options := launchOptions(p.launcher, g)
	mode := p.launcher.EffectiveMode(p.launchMode, g) // selectedLaunchMode
	option := launchOption(options, mode)

	launchW := 2*t.HL + 10 + 16 + 4 + textWidth(c, option.Label) + 10 + 2*t.HL + 6 + 16 + 6
	statusW := float32(len(components.GameStatuses)-1)*(40+6) + 12 + 16 + 6 + textWidth(c, components.GameStatusLabel(status)) + 12
	actionsW := float32(1 + toolbarGap + 3*32 + 3*6 + 32)
	if len(links) > 1 {
		actionsW += 48 - 32
	}
	compact := launchW+statusW+actionsW+2*toolbarGap+4 > width

	ui.Row(c.Key("toolbar")).Wrap().GapX(toolbarGap).GapY(toolbarGap).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Gap(toolbarGap).AlignItems(ui.Center).Shrink(0).Children(func() {
			running, ending := p.launcher.Status(g.ID)
			label, icon := option.Label, option.Icon
			switch {
			case ending:
				label = locales.PlayingIslandEnding()
			case running:
				label, icon = locales.HomeGaming(), "gamepad-variant"
			}
			better.SplitButton(c, t, "launch", better.SplitButtonProps{
				Label:       label,
				Icon:        icon,
				Options:     options,
				SelectedKey: string(mode),
				MenuTitle:   locales.GameCardLaunchMode(),
				Disabled:    running,
				Loading:     ending,
				OnClick:     func() { p.launcher.Start(c, p.panel(), mode) },
				OnSelect:    func(key string) { p.launchMode = enums.LaunchMode(key) },
			})
			if compact {
				p.statusMenu(c, t, status)
			} else {
				p.statusButtons(c, t, status)
			}
		})
		ui.Row(c).Gap(toolbarGap).AlignItems(ui.Center).Shrink(0).Children(func() {
			ui.Box(c).Size(1, 24).Background(t.Divider)
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				p.sourceButton(c, t, links)
				dir := g.GameDirectory
				if dir == "" {
					dir = g.Path
				}
				actionButton(c, t, "explorer", "folder-open-outline", locales.GameEditOpenInExplorer(), dir == "", func() { p.openPath(c, dir) })
				guideIcon := "text-box-search-outline"
				if p.guide.loading {
					guideIcon = "loading"
				}
				actionButton(c, t, "guide", guideIcon, locales.GameEditOpenGuide(), dir == "" || p.guide.loading, func() { p.openGuides(c) })
				actionButton(c, t, "category", "folder-plus-outline", locales.AddToCategoryTitle(), false, func() { p.openCategories(c) })
			})
		})
	})
}

// blurNSFW 对应 config.blur_nsfw_game_covers !== false。
func blurNSFW(e *env.Env) bool {
	cfg := e.Config()
	return cfg == nil || cfg.BlurNSFWGameCovers
}

func erogameScapeBase(e *env.Env) string {
	if cfg := e.Config(); cfg != nil {
		return cfg.ErogameScapeBaseURL
	}
	return ""
}

// openPath 对应 OpenLocalPath：目录直接打开，文件在资源管理器中选中。
func (p *Page) openPath(c *ui.Context, path string) {
	p.env.Async(func() func() {
		err := apputils.OpenFileOrFolder(path)
		return func() {
			if err != nil {
				p.env.Toast(c, env.Error(locales.GameEditOpenPathFailed()))
			}
		}
	})
}

// statusButtons：flex gap-1.5 的状态按钮，只有当前状态显示文字；当前状态带 ring-2 ring-offset-1 ring-brand-400。
func (p *Page) statusButtons(c *ui.Context, t *theme.Theme, current enums.GameStatus) {
	ui.Row(c.Key("statuses")).Gap(6).Shrink(0).Children(func() {
		for _, s := range components.GameStatuses {
			label := components.GameStatusLabel(s)
			active := s == current
			btn := ui.ButtonBase(c.Key(string(s))).Height(32).Padding(0, 12).Radius(999).Gap(6).
				AlignItems(ui.Center).FocusRing(false).Label(label)
			icon, bg, fg := components.GameStatusStyle(t, s)
			if active {
				// 阴影按添加顺序绘制、后加的盖在上面（与 CSS 相反）：先画 ring，再画 ring-offset 盖住内侧 1px。
				btn.Shadow(0, 0, 0, 3, theme.Brand400).Shadow(0, 0, 0, 1, t.Pick(theme.White, theme.Brand900))
			} else {
				bg, fg = t.Pick(theme.Brand150, theme.Brand700), t.Pick(theme.Brand500, theme.Brand400)
				if btn.Hovered() {
					bg = t.Pick(theme.Brand200, theme.Brand600)
				}
			}
			btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
			btn.OnClick(func() { p.setStatus(c, s) }).Children(func() {
				components.Icon(c, icon, 16, fg)
				if active {
					components.Text(c, label, theme.TextXS, theme.WeightMedium, fg).NoWrap()
				}
			})
			better.Tooltip(c, t, btn, label)
		}
	})
}

// statusMenu 是窄屏时的状态下拉菜单（openOnHover align="start"）：触发器 glass-btn-neutral h-8 rounded-full px-3
// ring-1 ring-inset，玻璃模式下 glass-btn-neutral 的底色、文字与边框盖过状态配色。
func (p *Page) statusMenu(c *ui.Context, t *theme.Theme, current enums.GameStatus) {
	label := components.GameStatusLabel(current)
	icon, bg, fg := components.GameStatusStyle(t, current)
	trigger := ui.ButtonBase(c.Key("status-menu")).Height(32).Padding(0, 12).Radius(999).Gap(6).AlignItems(ui.Center).
		InsetShadow(0, 0, 0, 1, theme.Brand400.Alpha(0.5)).FocusRing(false).Label(locales.CommonStatus() + ": " + label)
	if t.Glass {
		g := t.GlassButton(theme.GlassNeutral, theme.BtnColors{})
		bg, fg = g.Bg, g.Text
		if trigger.Hovered() {
			bg = g.Hover
		}
		trigger.Border(t.HL, g.Border)
	}
	trigger.Background(bg)
	trigger.Children(func() {
		components.Icon(c, icon, 16, fg)
		components.Text(c, label, theme.TextXS, theme.WeightMedium, fg).NoWrap()
		components.Icon(c, "chevron-down", 14, fg)
	})
	items := make([]better.MenuItem, 0, len(components.GameStatuses))
	for _, s := range components.GameStatuses {
		icon, bg, fg := components.GameStatusStyle(t, s)
		items = append(items, better.MenuItem{
			Key: string(s), Label: components.GameStatusLabel(s), Icon: icon,
			Pill: true, PillBg: bg, PillText: fg,
			OnClick: func() { p.setStatus(c, s) },
		})
	}
	better.DropdownMenu(c, t, trigger, better.MenuProps{
		Open: &p.statusOpen, Items: items, Title: locales.CommonStatus(), SelectedKey: string(current),
		AlignStart: true, OpenOnHover: true,
	})
}

// setStatus 对应 handleStatusChange：先改页面里的状态，再 BatchUpdateStatus 并读回保存后的状态。
func (p *Page) setStatus(c *ui.Context, s enums.GameStatus) {
	if p.game == nil || currentStatus(p.game) == s {
		return
	}
	prev := p.game.Status
	p.game.Status = s
	id, gen := p.game.ID, p.gen
	p.env.Async(func() func() {
		err := p.env.Svc.Game.BatchUpdateStatus([]string{id}, string(s))
		var saved models.Game
		if err == nil {
			saved, err = p.env.Svc.Game.GetGameByID(id)
		}
		return func() {
			if err != nil {
				if gen == p.gen && p.game != nil {
					p.game.Status = prev
				}
				p.env.Toast(c, env.Error(locales.GameToastStatusUpdateFailed()))
				return
			}
			if gen == p.gen && p.game != nil {
				p.game.Status, p.game.UpdatedAt = saved.Status, saved.UpdatedAt
			}
			if p.links.LibraryChanged != nil {
				p.links.LibraryChanged()
			}
			p.env.Toast(c, env.Success(locales.GameToastStatusUpdated()))
		}
	})
}

// sourceButton 是“打开数据源网页”：只有一个来源时是圆形按钮，多个来源时带下拉箭头并弹出来源列表。
func (p *Page) sourceButton(c *ui.Context, t *theme.Theme, links []metadataSource) {
	if len(links) <= 1 {
		url := ""
		if len(links) == 1 {
			url = links[0].url
		}
		actionButton(c, t, "source", "open-in-new", locales.GameEditOpenSourcePage(), url == "", func() { c.OpenURL(url) })
		return
	}
	// flex h-8 items-center gap-0.5 rounded-full px-2
	btn := actionBase(c, t, "source", false).Padding(0, 8).Gap(2).Label(locales.GameEditChooseSourcePage())
	fg := actionColor(t, btn.Hovered())
	btn.Children(func() {
		components.Icon(c, "open-in-new", 16, fg)
		components.Icon(c, "chevron-down", 14, fg)
	})
	better.Tooltip(c, t, btn, locales.GameEditChooseSourcePage())
	items := make([]better.MenuItem, 0, len(links))
	for i, l := range links {
		desc := locales.GameEditSourceEntry(l.id)
		if i == 0 && l.source == p.game.SourceType {
			desc = locales.GameEditDefaultSourceEntry(l.id)
		}
		items = append(items, better.MenuItem{
			Key: string(l.source), Label: sourceLabel(l.source), Description: desc,
			Image:   assets.Provider(string(l.source), true), // getMetadataSourceIcon(source, "compact")
			OnClick: func() { c.OpenURL(l.url) },
		})
	}
	better.DropdownMenu(c, t, btn, better.MenuProps{Items: items, Title: locales.GameEditOpenSourcePage(), MinWidth: 240, AlignStart: true})
}

// actionButton：h-8 w-8 rounded-full bg-brand-150 text-brand-500 的图标按钮，disabled:opacity-45。
func actionButton(c *ui.Context, t *theme.Theme, key, icon, label string, disabled bool, onClick func()) {
	btn := actionBase(c, t, key, disabled).Width(32).Label(label)
	fg := actionColor(t, btn.Hovered() && !disabled)
	if !disabled {
		btn.OnClick(onClick)
	}
	btn.Children(func() {
		if icon == "loading" { // i-mdi-loading animate-spin
			components.Spinner(c, 16, fg)
		} else {
			components.Icon(c, icon, 16, fg)
		}
	})
	better.Tooltip(c, t, btn, label)
}

func actionBase(c *ui.Context, t *theme.Theme, key string, disabled bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Height(32).Radius(999).AlignItems(ui.Center).Justify(ui.Center).FocusRing(false)
	bg := t.Pick(theme.Brand150, theme.Brand700)
	if btn.Hovered() && !disabled {
		bg = t.Pick(theme.Brand200, theme.Brand600)
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
	if disabled {
		components.Disable(btn, 0.45)
	}
	return btn
}

func actionColor(t *theme.Theme, hovered bool) ui.Color {
	if hovered {
		return t.Pick(theme.Brand900, theme.Brand100)
	}
	return t.Pick(theme.Brand500, theme.Brand400)
}

// infoGrid：grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4 text-sm（断点按窗口宽度）。
func (p *Page) infoGrid(c *ui.Context, t *theme.Theme) {
	g := p.game
	winW, _ := c.Size()
	cols := 2
	switch {
	case winW >= 1024:
		cols = 5
	case winW >= 768:
		cols = 3
	}
	color := t.Pick(theme.Brand750, theme.Brand400)
	rating, release := "-", strings.TrimSpace(g.ReleaseDate)
	if g.Rating > 0 {
		rating = fmt.Sprintf("%.1f / 10", g.Rating)
	}
	if release == "" {
		release = "-"
	}
	created := strings.ReplaceAll(timefmt.Date(g.CreatedAt, timefmt.Location(p.env.Config())), "/", "-")

	cell := func(label string, value func()) {
		ui.Column(c).MinWidth(0).Children(func() {
			components.TextMargin(c, label, theme.TextSM, theme.WeightSemibold, color, 0, 0, 4, 0)
			value()
		})
	}
	text := func(s string) func() {
		return func() { components.Text(c, s, theme.TextSM, theme.WeightNormal, color) }
	}
	ui.Grid(c.Key("info")).Columns(cols).GapX(16).GapY(16).Children(func() {
		cell(locales.GameDataSource(), text(metadataSourceText(g)))
		cell(locales.GameDeveloper(), func() {
			company := strings.TrimSpace(g.Company)
			if company == "" {
				components.Text(c, "-", theme.TextSM, theme.WeightNormal, color)
				return
			}
			btn := ui.ButtonBase(c.Key("company")).AlignSelf(ui.Start).Justify(ui.Start).FocusRing(false)
			btn.OnClick(func() { p.links.SearchLibrary(company) }).Children(func() {
				components.Text(c, g.Company, theme.TextSM, theme.WeightNormal, color)
			})
		})
		cell(locales.CommonCreatedAt(), text(created))
		cell(locales.GameRating(), text(rating))
		cell(locales.GameReleaseDate(), text(release))
	})
}

// summary：font-semibold mb-2 的“简介”，正文 text-sm leading-relaxed whitespace-pre-wrap max-h-60 overflow-y-auto pr-2。
func (p *Page) summary(c *ui.Context, t *theme.Theme) {
	text := p.game.Summary
	if text == "" {
		text = locales.GameNoSummary()
	}
	ui.Column(c.Key("summary")).MinWidth(0).Children(func() {
		components.TextMargin(c, locales.GameSummary(), theme.TextBase, theme.WeightSemibold, t.TextStrong, 0, 0, 8, 0)
		ui.Scroll(c.Key("summary-scroll")).MaxHeight(240).Children(func() {
			components.Text(c, text, theme.FontStep{Size: 14, Line: 22.75}, theme.WeightNormal, t.Pick(theme.Brand750, theme.Brand400)).
				Margin(0, 8, 0, 0)
		})
	})
}

// tabBar：border-b 下的 nav（-mb-px space-x-8），按钮 py-4 px-1 border-b-2 text-sm font-medium。
func (p *Page) tabBar(c *ui.Context, t *theme.Theme) {
	bar := ui.Row(c.Key("tabs")).Gap(32)
	hl, line := t.HL, t.Divider
	// 底边框画在标签下面，选中标签的 2px 下划线盖住它（-mb-px）。
	bar.Draw(func(pt *ui.Painter, r ui.Rect) {
		pt.Fill(ui.Rect{X: r.X, Y: r.Y + r.H - hl, W: r.W, H: hl}, line, 0)
	})
	bar.Children(func() {
		for _, tab := range tabs {
			label := tab.label()
			btn := ui.ButtonBase(c.Key(tab.key)).Column().Padding(16, 0, 0, 0).FocusRing(false).Label(label)
			active := tab.key == p.tab
			color, border := t.Pick(theme.Brand700, theme.Brand400), ui.Transparent
			switch {
			case active:
				color, border = t.Pick(theme.Brand700, theme.Neutral400), theme.Neutral500
			case btn.Hovered():
				color, border = t.Pick(theme.Brand750, theme.Brand300), theme.Brand300
			}
			btn.OnClick(func() { p.tab = tab.key }).Children(func() {
				components.TextMargin(c, label, theme.TextSM, theme.WeightMedium, color, 0, 4, 16, 4).NoWrap()
				ui.Box(c).Height(2).FillWidth().Background(border)
			})
		}
	})
}

// skeleton 对应 GameDetailSkeleton（animate-pulse），玻璃模式下各块是 data-glass:bg-white/2 dark:bg-black/2。
func (p *Page) skeleton(c *ui.Context, t *theme.Theme) {
	bg := t.IfGlass(t.Pick(theme.Brand200, theme.Brand800), theme.White.Alpha(0.02), theme.Black.Alpha(0.02))
	contentW := p.env.MainWidth - 2*pagePadding
	wrap := ui.Column(c.Key("skeleton")).Fill().Clip().Padding(pagePadding).PassThrough()
	wrap.Opacity(components.Pulse(wrap)).Children(func() {
		ui.Column(c).Width(contentW).Margin(0, ui.Auto).Gap(32).Children(func() {
			ui.Box(c).Size(96, 32).Radius(6).Background(bg)
			ui.Row(c).Gap(24).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(240, 320).Radius(8).Background(bg).Shrink(0)
				ui.Column(c).Grow(1).Gap(16).Children(func() {
					ui.Box(c).Height(48).WidthPercent(75).Radius(8).Background(bg)
					ui.Grid(c).Columns(4).GapX(16).Children(func() {
						for range 4 {
							ui.Box(c).Height(48).Radius(6).Background(bg)
						}
					})
					ui.Box(c).Height(128).Radius(8).Background(bg)
				})
			})
			tabs := ui.Row(c).Gap(32)
			hl, line := t.HL, t.Divider
			tabs.Draw(func(pt *ui.Painter, r ui.Rect) {
				pt.Fill(ui.Rect{X: r.X, Y: r.Y + r.H - hl, W: r.W, H: hl}, line, 0)
			})
			tabs.Children(func() {
				for range 3 {
					ui.Box(c).Size(80, 40).Radius(6, 6, 0, 0).Background(bg)
				}
			})
			ui.Grid(c).Columns(3).GapX(24).Children(func() {
				for range 3 {
					ui.Box(c).Height(96).Radius(8).Background(bg)
				}
			})
			ui.Box(c).Height(320).Radius(8).Background(bg)
		})
	})
}

// notFound：未找到游戏时的提示与“返回库”。
func (p *Page) notFound(c *ui.Context, t *theme.Theme) {
	ui.Column(c.Key("not-found")).Fill().Center().Gap(16).Children(func() {
		components.Icon(c, "gamepad-variant-outline", 60, theme.Brand500)
		components.Text(c, locales.GameNotFound(), theme.TextXL, theme.WeightNormal, theme.Brand500)
		// text-neutral-600 hover:underline
		btn := ui.ButtonBase(c.Key("back-to-library")).FocusRing(false)
		btn.OnClick(func() { p.env.Router.Push(env.PathLibrary) }).Children(func() {
			text := components.Text(c, locales.GameBackToLibrary(), theme.TextBase, theme.WeightNormal, theme.Neutral600)
			if btn.Hovered() {
				text.Underline()
			}
		})
	})
}

func textWidth(c *ui.Context, s string) float32 {
	w, _ := c.MeasureText(0, ui.Span{Text: s, Font: theme.UIFont, Size: theme.TextXS.Size, Weight: theme.WeightMedium})
	return w
}
