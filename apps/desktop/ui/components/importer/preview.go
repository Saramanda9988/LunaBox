package importer

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// previewState 是 ImportPreviewStep 自己的状态：两个列筛选与跳过列表弹窗。
type previewState struct {
	status    string // statusFilter：""、pending、matched、not_found、error
	selection string // selectionFilter：""、selected、unselected
	skipped   bool   // showSkippedModal
}

// previewProps 是 BatchImportModal、DragDropImportModal 传给 ImportPreviewStep 的属性（labels 与 theme 两边相同的部分
// 写在这里）。
type previewProps struct {
	executableHeader string // 层级模式下是“游戏路径”
	leftAction       string
	onLeft           func()
	onStartMatch     func()
	onImport         func(matchedOnly bool)
	onManual         func(index int)
	onOpenFolder     func(path string)
	actionToolbar    func() // 选择优先来源
}

// indexed 是 IndexedCandidate。
type indexed struct {
	c     *Candidate
	index int
}

// matchesStatus 对应 matchesStatusFilter；matchesSelection 对应 matchesSelectionFilter。
func matchesStatus(c *Candidate, filter string) bool {
	switch filter {
	case "":
		return true
	case "matched":
		return c.MatchStatus == MatchMatched || c.MatchStatus == MatchManual
	}
	return string(c.MatchStatus) == filter
}

func matchesSelection(c *Candidate, filter string) bool {
	switch filter {
	case "selected":
		return c.Selected
	case "unselected":
		return !c.Selected
	}
	return true
}

// pathLabel 是路径的最后一段。
func pathLabel(p string) string {
	parts := strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) == 0 {
		return p
	}
	return parts[len(parts)-1]
}

// statusPill 是匹配状态的胶囊：inline-flex items-center rounded-full px-2 py-1 text-xs，图标 mr-1。
func statusPill(c *ui.Context, t *theme.Theme, s MatchStatus) {
	icon, label, bg, fg := "clock-outline", locales.BatchImportModalStatusPending(), t.Pick(theme.Gray100, theme.Gray900.Alpha(0.3)), t.Pick(theme.Gray700, theme.Gray400)
	switch s {
	case MatchMatched, MatchManual:
		icon, label, bg, fg = "check-circle", locales.BatchImportModalStatusMatched(), t.Pick(theme.Success100, theme.Success900.Alpha(0.3)), t.Pick(theme.Success700, theme.Success400)
	case MatchNotFound:
		icon, label, bg, fg = "alert-circle", locales.BatchImportModalStatusNotFound(), t.Pick(theme.Orange100, theme.Orange900.Alpha(0.3)), t.Pick(theme.Orange700, theme.Orange400)
	case MatchError:
		icon, label, bg, fg = "close-circle", locales.BatchImportModalStatusError(), t.Pick(theme.Error100, theme.Error900.Alpha(0.3)), t.Pick(theme.Error700, theme.Error400)
	}
	ui.Row(c.Key("pill")).Padding(4, 8).Radius(999).AlignItems(ui.Center).Background(bg).Children(func() {
		components.Icon(c, icon, 12, fg).Margin(0, 4, 0, 0)
		components.Text(c, label, theme.TextXS, theme.WeightNormal, fg).NoWrap()
	})
}

// preview 对应 ImportPreviewStep（space-y-4）：统计卡片、暂停提示、跳过的已有游戏、筛选提示、候选项表格与底部按钮。
func (f *Flow) preview(c *ui.Context, t *theme.Theme, s *previewState, p previewProps) {
	matched, notFound, pending, errs := f.counts()
	selectedCount, matchedSelected := 0, 0
	for i := range f.Candidates {
		if f.Candidates[i].Selected {
			selectedCount++
			if f.Candidates[i].matched() {
				matchedSelected++
			}
		}
	}
	var rows []indexed
	for i := range f.Candidates {
		cand := &f.Candidates[i]
		if matchesStatus(cand, s.status) && matchesSelection(cand, s.selection) {
			rows = append(rows, indexed{cand, i})
		}
	}
	filtering := s.status != "" || s.selection != ""

	ui.Column(c.Key("preview-step")).Gap(16).Children(func() {
		if len(f.Candidates) > 0 || len(f.Skipped) > 0 {
			cards := []statCard{
				{value: len(f.Candidates), labelStr: locales.BatchImportModalDetected(), bg: t.Pick(theme.Neutral50, theme.Neutral900.Alpha(0.2)),
					valueFg: t.Pick(theme.Neutral600, theme.Neutral400), labelFg: t.Pick(theme.Neutral700, theme.Neutral300)},
				{value: matched, labelStr: locales.BatchImportModalMatched(), bg: t.Pick(theme.Success50, theme.Success900.Alpha(0.2)),
					valueFg: t.Pick(theme.Success600, theme.Success400), labelFg: t.Pick(theme.Success700, theme.Success300)},
			}
			if notFound > 0 {
				cards = append(cards, statCard{value: notFound, labelStr: locales.BatchImportModalNotMatched(), bg: t.Pick(theme.Orange50, theme.Orange900.Alpha(0.2)),
					valueFg: t.Pick(theme.Orange600, theme.Orange400), labelFg: t.Pick(theme.Orange700, theme.Orange300)})
			}
			if pending > 0 {
				cards = append(cards, statCard{value: pending, labelStr: locales.BatchImportModalPending(), bg: t.Pick(theme.Gray50, theme.Gray900.Alpha(0.2)),
					valueFg: t.Pick(theme.Gray600, theme.Gray400), labelFg: t.Pick(theme.Gray700, theme.Gray300)})
			}
			statCards(c, cards)
		}
		if f.Pause != "" {
			// flex items-start gap-2 rounded-lg border border-warning-300 bg-warning-50 px-4 py-3 text-sm text-warning-800
			fg := t.Pick(theme.Warning800, theme.Warning200)
			ui.Row(c.Key("pause")).Gap(8).AlignItems(ui.Start).Padding(12, 16).Radius(8).
				Border(t.HL, t.Pick(theme.Warning300, theme.Warning700)).Background(t.Pick(theme.Warning50, theme.Warning900.Alpha(0.25))).Children(func() {
				components.Icon(c, "pause-circle-outline", 18, fg).Margin(2, 0, 0, 0)
				components.Text(c, f.Pause, theme.TextSM, theme.WeightNormal, fg).Grow(1).Basis(0).MinWidth(0)
			})
		}
		if len(f.Skipped) > 0 {
			skippedBanner(c, t, len(f.Skipped), func() { s.skipped = true })
		}
		if filtering && len(f.Candidates) > 0 {
			filterBanner(c, t, len(rows), len(f.Candidates), func() { s.status, s.selection = "", "" })
		}
		f.previewTable(c, t, s, p, rows)
		f.previewFooter(c, t, p, selectedCount, matchedSelected, pending+errs > 0, errs > 0)
	})
	skippedModal(c, t, f.Skipped, &s.skipped)
}

// skippedBanner：flex items-center justify-between gap-3 rounded-lg border border-yellow-200 bg-yellow-50 px-4 py-3。
func skippedBanner(c *ui.Context, t *theme.Theme, count int, onView func()) {
	ui.Row(c.Key("skipped")).Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).Padding(12, 16).Radius(8).
		Border(t.HL, t.Pick(theme.Yellow200, theme.Yellow800.Alpha(0.8))).Background(t.Pick(theme.Yellow50, theme.Yellow900.Alpha(0.2))).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Grow(1).Basis(0).MinWidth(0).Children(func() {
			components.Icon(c, "alert-circle-outline", 20, t.Pick(theme.Yellow600, theme.Yellow300))
			ui.Column(c).MinWidth(0).Grow(1).Children(func() {
				components.Text(c, locales.BatchImportModalSkippedExistingSummary(strconv.Itoa(count)), theme.TextSM, theme.WeightMedium, t.Pick(theme.Yellow800, theme.Yellow200)).SingleLine()
				components.TextMargin(c, locales.BatchImportModalSkippedExistingDetails(), theme.TextXS, theme.WeightNormal,
					t.Pick(theme.Yellow700.Alpha(0.8), theme.Yellow300.Alpha(0.8)), 2, 0, 0, 0).SingleLine()
			})
		})
		// inline-flex shrink-0 gap-1.5 rounded-lg border border-yellow-300 bg-white/70 px-3 py-1.5 text-xs font-medium
		btn := ui.ButtonBase(c.Key("view")).Shrink(0).Gap(6).Padding(6, 12).Radius(8).AlignItems(ui.Center).FocusRing(false).
			Border(t.HL, t.Pick(theme.Yellow300, theme.Yellow700))
		bg := t.Pick(theme.White.Alpha(0.7), theme.Yellow950.Alpha(0.3))
		if btn.Hovered() {
			bg = t.Pick(theme.Yellow100, theme.Yellow900.Alpha(0.4))
		}
		fg := t.Pick(theme.Yellow800, theme.Yellow200)
		btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(onView).Children(func() {
			components.Icon(c, "format-list-bulleted", 16, fg)
			components.Text(c, locales.BatchImportModalSkippedExistingView(), theme.TextXS, theme.WeightMedium, fg).NoWrap()
		})
	})
}

// filterBanner：rounded-lg border border-brand-200 bg-brand-50/70 px-3 py-2 text-xs，右侧“清除筛选”。
func filterBanner(c *ui.Context, t *theme.Theme, visible, total int, onClear func()) {
	fg := t.Pick(theme.Brand600, theme.Brand300)
	ui.Row(c.Key("filtered")).Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).Padding(8, 12).Radius(8).
		Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).Background(t.Pick(theme.Brand50.Alpha(0.7), theme.Brand800.Alpha(0.5))).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).MinWidth(0).Children(func() {
			components.Icon(c, "filter", 12, theme.Success500)
			components.Text(c, locales.BatchImportModalFilterCount(strconv.Itoa(visible), strconv.Itoa(total)), theme.TextXS, theme.WeightNormal, fg).SingleLine()
		})
		btn := ui.ButtonBase(c.Key("clear")).Shrink(0).Gap(4).Padding(4, 8).Radius(6).AlignItems(ui.Center).FocusRing(false).
			Border(t.HL, t.Pick(theme.Brand200, theme.Brand600))
		bg := t.Pick(theme.White.Alpha(0.7), theme.Brand900.Alpha(0.4))
		if btn.Hovered() {
			bg = t.Pick(theme.Brand100, theme.Brand700)
		}
		bfg := t.Pick(theme.Brand700, theme.Brand200)
		btn.Background(bg).OnClick(onClear).Children(func() {
			components.Icon(c, "filter-off-outline", 14, bfg)
			components.Text(c, locales.BatchImportModalFilterClear(), theme.TextXS, theme.WeightMedium, bfg).NoWrap()
		})
	})
}

func (f *Flow) previewTable(c *ui.Context, t *theme.Theme, s *previewState, p previewProps, rows []indexed) {
	visibleSelected := 0
	for _, r := range rows {
		if r.c.Selected {
			visibleSelected++
		}
	}
	empty := locales.BatchImportModalNoFolderDetected()
	if (s.status != "" || s.selection != "") && len(f.Candidates) > 0 {
		empty = locales.BatchImportModalFilterEmptyFiltered()
	}
	cols := []better.DataTableColumn[indexed]{
		{
			Key: "selected", FixedWidth: 64,
			HeaderCell: func() {
				components.Checkbox(c, "all", components.CheckboxProps{
					Checked: len(rows) > 0 && visibleSelected == len(rows), Indeterminate: visibleSelected > 0 && visibleSelected < len(rows),
					Disabled: len(rows) == 0, Label: locales.BatchImportModalDetected(),
					OnChange: func(on bool) {
						idx := make([]int, len(rows))
						for i, r := range rows {
							idx[i] = r.index
						}
						f.toggleAll(on, idx)
					},
				})
			},
			Filter: &better.ColumnFilter{
				Value: s.selection, OnChange: func(v string) { s.selection = v },
				Title: locales.BatchImportModalFilterBySelection(), AllLabel: locales.BatchImportModalFilterAll(),
				Options: []better.ColumnFilterOption{
					{Value: "selected", Label: locales.BatchImportModalFilterSelected(), Icon: "checkbox-marked", IconColor: theme.Success500},
					{Value: "unselected", Label: locales.BatchImportModalFilterUnselected(), Icon: "checkbox-blank-outline", IconColor: theme.Brand400},
				},
			},
			Cell: func(r indexed, _ int) {
				components.Checkbox(c, "check", components.CheckboxProps{
					Checked: r.c.Selected, Label: r.c.SearchName, OnChange: func(bool) { f.toggle(r.index) },
				})
			},
		},
		{Key: "searchName", Header: locales.BatchImportModalSearchName(), Width: 0.34, Cell: func(r indexed, _ int) { f.nameCell(c, t, r) }},
		{Key: "executable", Header: p.executableHeader, Width: 0.30, Cell: func(r indexed, _ int) { f.exeCell(c, t, r) }},
		{
			Key: "status", Header: locales.BatchImportModalMatchStatus(), FixedWidth: 128, Center: true,
			Filter: &better.ColumnFilter{
				Value: s.status, OnChange: func(v string) { s.status = v },
				Title: locales.BatchImportModalFilterByStatus(), AllLabel: locales.BatchImportModalFilterAll(), Pill: true, AlignEnd: true,
				Options: []better.ColumnFilterOption{
					{Value: "pending", Label: locales.BatchImportModalStatusPending(), Icon: "clock-outline",
						PillBg: t.Pick(theme.Gray100, theme.Gray900.Alpha(0.3)), PillText: t.Pick(theme.Gray700, theme.Gray400)},
					{Value: "matched", Label: locales.BatchImportModalStatusMatched(), Icon: "check-circle",
						PillBg: t.Pick(theme.Success100, theme.Success900.Alpha(0.3)), PillText: t.Pick(theme.Success700, theme.Success400)},
					{Value: "not_found", Label: locales.BatchImportModalStatusNotFound(), Icon: "alert-circle",
						PillBg: t.Pick(theme.Orange100, theme.Orange900.Alpha(0.3)), PillText: t.Pick(theme.Orange700, theme.Orange400)},
					{Value: "error", Label: locales.BatchImportModalStatusError(), Icon: "close-circle",
						PillBg: t.Pick(theme.Error100, theme.Error900.Alpha(0.3)), PillText: t.Pick(theme.Error700, theme.Error400)},
				},
			},
			Cell: func(r indexed, _ int) { statusPill(c, t, r.c.MatchStatus) },
		},
		{
			Key: "action", Header: locales.CommonAction(), FixedWidth: 80, Center: true,
			Cell: func(r indexed, _ int) {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					if p.onOpenFolder != nil {
						dir := r.c.GameDirectory
						if dir == "" {
							dir = r.c.FolderPath
						}
						rowIconButton(c, "folder", "folder-open-outline", locales.BatchImportModalOpenFolder(), func() { p.onOpenFolder(dir) })
					}
					rowIconButton(c, "manual", "pencil", locales.BatchImportModalManualSelect(), func() { p.onManual(r.index) })
				})
			},
		},
	}
	better.DataTable(c, t, "candidates", better.DataTableProps[indexed]{
		Rows:      rows,
		Columns:   cols,
		MaxHeight: 400,
		RowKey: func(r indexed, i int) string {
			return fmt.Sprintf("%s-%s-%d-%d", r.c.FolderPath, r.c.SelectedExe, r.index, i)
		},
		Empty: func() { components.Text(c, empty, theme.TextSM, theme.WeightNormal, t.TextFaint) },
		RowOpacity: func(r indexed, _ int) float32 {
			if r.c.Selected {
				return 1
			}
			return 0.5
		},
	})
}

// nameCell：可编辑的搜索名（w-full border-b border-transparent bg-transparent text-sm，悬停 border-brand-300，
// 聚焦 border-neutral-500），下面是匹配到的名称、已有元数据的提示与匹配失败的原因（text-xs）。
func (f *Flow) nameCell(c *ui.Context, t *theme.Theme, r indexed) {
	cand := r.c
	ui.Column(c).Grow(1).MinWidth(0).Children(func() {
		v := cand.SearchName
		in := ui.TextInputBase(c.Key("name"), &v).FillWidth().FontSize(theme.TextSM.Size).FixedLineHeight(theme.TextSM.Line).
			TextColor(t.TextStrong).Label(cand.SearchName)
		if theme.UIFont != "" {
			in.Font(theme.UIFont)
		}
		border := ui.Transparent
		switch {
		case in.Focused():
			border = theme.Neutral500
		case in.Hovered():
			border = theme.Brand300
		}
		in.BorderWidth(0, 0, 1, 0).BorderColor(border)
		if in.Changed() {
			f.setSearchName(r.index, v)
		}
		if g := cand.MatchedGame; g != nil {
			ui.Row(c.Key("matched")).Margin(4, 0, 0, 0).Gap(4).AlignItems(ui.Center).MinWidth(0).Children(func() {
				components.Text(c, g.Name, theme.TextXS, theme.WeightNormal, t.Pick(theme.Success600, theme.Success400)).SingleLine().Shrink(1)
				components.Text(c, "("+string(cand.MatchSource)+")", theme.TextXS, theme.WeightNormal, theme.Brand400).NoWrap().Shrink(0)
			})
			if cand.DuplicateName != "" {
				fg := t.Pick(theme.Yellow700, theme.Yellow300)
				ui.Row(c.Key("duplicate")).Margin(4, 0, 0, 0).Gap(4).AlignItems(ui.Center).MinWidth(0).Children(func() {
					components.Icon(c, "alert-circle-outline", 12, fg)
					components.Text(c, locales.BatchImportModalMetadataExists(cand.DuplicateName), theme.TextXS, theme.WeightNormal, fg).SingleLine().Shrink(1)
				})
			}
		}
		if cand.MatchError != "" {
			fg := t.Pick(theme.Orange600, theme.Orange300)
			if cand.MatchStatus == MatchError {
				fg = t.Pick(theme.Error600, theme.Error300)
			}
			ui.Row(c.Key("error")).Margin(4, 0, 0, 0).Gap(4).AlignItems(ui.Center).MinWidth(0).Children(func() {
				components.Icon(c, "alert-circle-outline", 12, fg)
				components.Text(c, cand.MatchError, theme.TextXS, theme.WeightNormal, fg).SingleLine().Shrink(1)
			})
		}
	})
}

// exeCell：有多个可执行文件时是 BetterSelect（选项是文件名），否则是 truncate text-sm text-brand-500 的文件名。
func (f *Flow) exeCell(c *ui.Context, t *theme.Theme, r indexed) {
	cand := r.c
	if len(cand.Executables) > 1 {
		opts := make([]better.SelectOption, len(cand.Executables))
		for i, exe := range cand.Executables {
			opts[i] = better.SelectOption{Label: pathLabel(exe), Value: exe}
		}
		ui.Box(c).Grow(1).MinWidth(0).Children(func() {
			better.Select(c, t, "exe", better.SelectProps{
				Value: cand.SelectedExe, Options: opts,
				OnChange: func(v string) { f.Candidates[r.index].SelectedExe = v },
			})
		})
		return
	}
	p := cand.SelectedExe
	if p == "" {
		p = cand.FolderPath
	}
	components.Text(c, pathLabel(p), theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400)).SingleLine().Grow(1).MinWidth(0)
}

// rowIconButton：inline-flex h-8 w-8 rounded-md text-neutral-500 hover:text-neutral-700 的图标按钮（text-lg）。
func rowIconButton(c *ui.Context, key, icon, label string, onClick func()) {
	btn := ui.ButtonBase(c.Key(key)).Size(32, 32).Radius(6).Center().Shrink(0).FocusRing(false).Label(label)
	fg := theme.Neutral500
	if btn.Hovered() {
		fg = theme.Neutral700
	}
	btn.OnClick(onClick).Children(func() { components.Icon(c, icon, 18, fg) })
}

// previewFooter：左边是“重新选择”（或“取消”），右边依次是优先来源、开始（继续）匹配、只导入已匹配的与导入。
// 按钮都是 h-11 rounded-lg px-4 text-sm font-medium。
func (f *Flow) previewFooter(c *ui.Context, t *theme.Theme, p previewProps, selected, matchedSelected int, canMatch, hasErrors bool) {
	ui.Row(c.Key("footer")).Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		ui.Box(c).Shrink(0).Children(func() {
			if p.onLeft != nil && p.leftAction != "" {
				outlineButton(c, t, "left", p.leftAction, false, 16, 0).Height(44).OnClick(p.onLeft)
			}
		})
		// 右侧是 flex-wrap 的两组：优先来源，以及开始匹配、只导入已匹配的与导入（这一组不换行）
		ui.Row(c).Gap(12).Wrap().AlignItems(ui.Center).Justify(ui.End).Grow(1).Basis(0).MinWidth(0).Children(func() {
			if p.actionToolbar != nil && canMatch {
				p.actionToolbar()
			}
			ui.Row(c.Key("buttons")).Gap(12).AlignItems(ui.Center).Children(func() {
				if canMatch {
					label := locales.BatchImportModalStartMatching()
					if hasErrors {
						label = locales.BatchImportModalContinueMatching()
					}
					solidButton(c, "match", label, theme.Neutral600, theme.Neutral700, false, false, 16, 0).Height(44).OnClick(p.onStartMatch)
				}
				if matchedSelected > 0 && matchedSelected < selected {
					// border border-success-300 bg-success-50 px-3 text-success-700 hover:bg-success-100
					btn := ui.ButtonBase(c.Key("import-matched")).Height(44).Padding(0, 12).Gap(6).Radius(8).Center().FocusRing(false).
						Border(t.HL, t.Pick(theme.Success300, theme.Success700))
					bg := t.Pick(theme.Success50, theme.Success900.Alpha(0.25))
					if btn.Hovered() {
						bg = t.Pick(theme.Success100, theme.Success900.Alpha(0.4))
					}
					fg := t.Pick(theme.Success700, theme.Success300)
					btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(func() { p.onImport(true) }).Children(func() {
						components.Icon(c, "check-circle-outline", 16, fg)
						components.Text(c, locales.BatchImportModalImportMatchedCount(strconv.Itoa(matchedSelected)), theme.TextSM, theme.WeightMedium, fg).NoWrap()
					})
				}
				imp := solidButton(c, "import", locales.BatchImportModalImportCount(strconv.Itoa(selected)), theme.Success600, theme.Success700, selected == 0, false, 16, 0).Height(44)
				if selected > 0 {
					imp.OnClick(func() { p.onImport(false) })
				}
			})
		})
	})
}

// openFolder 对应 handleOpenFolder：在文件管理器里打开候选项的目录。
func (f *Flow) openFolder(c *ui.Context, path string) {
	svc := f.env.Svc.Game
	f.env.Async(func() func() {
		err := svc.OpenLocalPath(path)
		return func() {
			if err != nil {
				f.env.Toast(c, env.Error(locales.BatchImportModalToastOpenFolderFailed()))
			}
		}
	})
}
