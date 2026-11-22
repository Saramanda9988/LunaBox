package pluginui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/plugin"
	"lunabox/desktop/ui/assets"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

// RenderSlot 挂载并渲染一个插件插槽实例。
func (m *Manager) RenderSlot(c *ui.Context, t *theme.Theme, slotName, pluginID, regID, scopeKey string, props any) {
	if m == nil {
		return
	}
	inst := m.EnsureMounted(slotName, pluginID, regID, scopeKey, props)
	m.RenderInstance(c, t, inst)
}

// RenderInstance 渲染一个插槽实例当前缓存的描述树，并在出错时展示统一的错误边界占位。
func (m *Manager) RenderInstance(c *ui.Context, t *theme.Theme, inst *Instance) {
	if m == nil || inst == nil {
		return
	}
	// 处理输入框防抖派发（全部使用帧时钟 c.Now() 与 c.After()，符合 frontend.md）
	m.flushDebouncedInputs(c, inst)

	if inst.Error != "" {
		m.renderErrorBoundary(c, t, inst)
		return
	}
	if inst.Tree == nil {
		return
	}
	m.renderNode(c, t, inst, inst.Tree, "root")

	if inst.PreviewOpen && inst.PreviewSrc != "" {
		if bmp := m.loadBitmap(inst.PreviewSrc); bmp != nil {
			better.ImageViewer(c, t, "plugin-img-preview-"+inst.ID, better.ImageViewerProps{
				Open:      &inst.PreviewOpen,
				Image:     bmp,
				TopOffset: env.TopbarHeight,
			})
		}
	}
}

func (m *Manager) renderErrorBoundary(c *ui.Context, t *theme.Theme, inst *Instance) {
	name := m.pluginName(inst.PluginID)
	bg := t.Pick(theme.Error50, theme.Error900.Alpha(0.25))
	border := t.Pick(theme.Error200, theme.Error700)
	fg := t.Pick(theme.Error600, theme.Error400)
	ui.Column(c.Key("plugin-err-"+inst.ID)).FillWidth().Padding(12).Gap(6).Radius(10).
		Background(bg).Border(t.HL, border).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			components.Icon(c, "alert-circle-outline", 18, fg)
			components.Text(c, locales.NativePluginsUiError(name), theme.TextSM, theme.WeightSemibold, fg)
		})
		if m.DevMode && inst.Error != "" {
			components.Text(c, inst.Error, theme.TextXS, theme.WeightNormal, t.TextMuted)
		}
	})
}

func (m *Manager) flushDebouncedInputs(c *ui.Context, inst *Instance) {
	now := c.Now()
	for _, st := range inst.Inputs {
		if !st.Dirty || st.PendingEvent == "" {
			continue
		}
		if st.DirtyAt.IsZero() {
			st.DirtyAt = now
		}
		wait := inputDebounce - now.Sub(st.DirtyAt)
		if wait > 0 {
			c.After(wait)
			continue
		}
		st.Dirty = false
		st.DirtyAt = time.Time{}
		m.dispatch(inst, st.PendingEvent, st.Value)
	}
}

func (m *Manager) getInputState(inst *Instance, key string, incoming string) *InputState {
	st, ok := inst.Inputs[key]
	if !ok {
		st = &InputState{Value: incoming, LastSent: incoming}
		inst.Inputs[key] = st
		return st
	}
	// 插件主动下发了不同于上次的值时，覆盖本地；相同时保留本地正在输入的内容（见 §9.7）
	if incoming != st.LastSent {
		st.Value = incoming
		st.LastSent = incoming
		st.Dirty = false
		st.DirtyAt = time.Time{}
	}
	return st
}

func (m *Manager) getScrollBar(inst *Instance, key string) *components.ScrollBar {
	sb, ok := inst.Scrolls[key]
	if !ok {
		sb = &components.ScrollBar{}
		inst.Scrolls[key] = sb
	}
	return sb
}

func (m *Manager) getCollapsibleState(inst *Instance, key string, defaultOpen bool) *bool {
	open, ok := inst.Collapsibles[key]
	if !ok {
		v := defaultOpen
		open = &v
		inst.Collapsibles[key] = open
	}
	return open
}

func (m *Manager) renderNode(c *ui.Context, t *theme.Theme, inst *Instance, n *plugin.UINode, fallbackKey string) ui.Element {
	if n == nil {
		return ui.Element{}
	}
	key := n.Key
	if key == "" {
		key = fallbackKey
	}
	p := n.Props

	switch n.Type {
	case "Column":
		el := ui.Column(c.Key(key))
		m.applyLayoutProps(el, p, n.Events, inst)
		el.Children(func() {
			for i, child := range n.Children {
				m.renderNode(c, t, inst, child, fmt.Sprintf("%s-%d", key, i))
			}
		})
		return el

	case "Row":
		el := ui.Row(c.Key(key))
		m.applyLayoutProps(el, p, n.Events, inst)
		el.Children(func() {
			for i, child := range n.Children {
				m.renderNode(c, t, inst, child, fmt.Sprintf("%s-%d", key, i))
			}
		})
		return el

	case "Box":
		el := ui.Box(c.Key(key))
		m.applyLayoutProps(el, p, n.Events, inst)
		el.Children(func() {
			for i, child := range n.Children {
				m.renderNode(c, t, inst, child, fmt.Sprintf("%s-%d", key, i))
			}
		})
		return el

	case "ScrollArea":
		sb := m.getScrollBar(inst, key)
		pad := mapSpacing(propStr(p, "padding"))
		h := propFloat(p, "height")
		autoBottom := propBool(p, "autoScrollBottom")
		var outer ui.Element
		components.ScrollArea(c, t, key, sb, func(w ui.Element) {
			outer = w
			w.FillWidth()
			if h > 0 {
				w.Height(h)
			} else if g := propFloat(p, "grow"); g > 0 {
				w.Grow(g).Basis(0)
			} else {
				w.Fill()
			}
		}, func(sc ui.Element) {
			if pad > 0 {
				sc.Padding(pad)
			}
			sc.Children(func() {
				ui.Column(c).FillWidth().Gap(mapSpacing(propStr(p, "gap"))).Children(func() {
					for i, child := range n.Children {
						m.renderNode(c, t, inst, child, fmt.Sprintf("%s-%d", key, i))
					}
				})
			})
		})
		if autoBottom && sb.State.MaxY > 0 {
			sb.State.Y = sb.State.MaxY
		}
		if reachHandler := eventID(n, "reachBottom", "ReachBottom"); reachHandler != "" {
			if sb.State.MaxY > 0 && sb.State.Y >= sb.State.MaxY-24 {
				lastMax := ui.Local(outer, "reach-max", func() float32 { return -1 })
				if *lastMax != sb.State.MaxY {
					*lastMax = sb.State.MaxY
					m.dispatch(inst, reachHandler, nil)
				}
			}
		}
		return outer

	case "Grid":
		gap := mapSpacing(propStr(p, "gap"))
		if gap == 0 {
			gap = 12
		}
		cols := int(propFloat(p, "cols"))
		minColW := propFloat(p, "minColWidth")
		if cols <= 0 {
			if minColW <= 0 {
				minColW = 220
			}
			avail := m.env.MainWidth - 64
			if avail < minColW {
				avail = minColW
			}
			cols = max(1, int((avail+gap)/(minColW+gap)))
		}
		el := ui.Column(c.Key(key)).FillWidth().Gap(gap)
		el.Children(func() {
			for start := 0; start < len(n.Children); start += cols {
				end := min(start+cols, len(n.Children))
				rowChildren := n.Children[start:end]
				ui.Row(c.Key(fmt.Sprintf("%s-r%d", key, start))).FillWidth().Gap(gap).AlignItems(ui.Stretch).Children(func() {
					for j, child := range rowChildren {
						ui.Column(c.Key(fmt.Sprintf("%s-c%d", key, start+j))).Grow(1).Basis(0).MinWidth(0).Children(func() {
							m.renderNode(c, t, inst, child, fmt.Sprintf("%s-%d", key, start+j))
						})
					}
					for pad := len(rowChildren); pad < cols; pad++ {
						ui.Box(c.Key(fmt.Sprintf("%s-pad%d", key, start+pad))).Grow(1).Basis(0)
					}
				})
			}
		})
		return el

	case "Text":
		txt := propStr(p, "text")
		step := mapFontStep(propStr(p, "size"))
		weight := mapFontWeight(propStr(p, "weight"))
		col := mapToneColor(t, propStr(p, "tone"))
		el := components.Text(c, txt, step, weight, col)
		if wrapVal, hasWrap := p["wrap"]; hasWrap {
			if b, ok := wrapVal.(bool); ok && !b {
				el.NoWrap()
			}
		}
		if maxLines := int(propFloat(p, "maxLines")); maxLines == 1 {
			el.SingleLine()
		}
		if propBool(p, "selectable") {
			el.Selectable()
		}
		return el

	case "Markdown":
		return m.renderMarkdown(c, t, inst, key, propStr(p, "content"), eventID(n, "linkPress"))

	case "CodeBlock":
		return m.renderCodeBlock(c, t, key, propStr(p, "code"), propStr(p, "language"), propBool(p, "copyable"))

	case "Button":
		pressID := eventID(n, "press", "click")
		ic := propStr(p, "icon")
		if ic != "" {
			ic = assets.SafeIcon(ic, "")
		}
		return better.Button(c, t, key, better.ButtonProps{
			Label:    propStr(p, "label"),
			Icon:     ic,
			Variant:  mapButtonVariant(propStr(p, "variant")),
			Size:     mapButtonSize(propStr(p, "size")),
			Disabled: propBool(p, "disabled"),
			Loading:  propBool(p, "loading"),
			OnClick: func() {
				if pressID != "" {
					m.dispatch(inst, pressID, nil)
				}
			},
		})

	case "IconButton":
		pressID := eventID(n, "press", "click")
		icon := assets.SafeIcon(propStr(p, "icon"), "dots-horizontal")
		pad := float32(8)
		if propStr(p, "size") == "sm" {
			pad = 6
		}
		btn := better.IconButton(c, t, key, icon, pad, propBool(p, "active"), !propBool(p, "disabled"))
		if lbl := propStr(p, "label"); lbl != "" {
			btn.Label(lbl)
		}
		if pressID != "" {
			btn.OnClick(func() { m.dispatch(inst, pressID, nil) })
		}
		return btn

	case "Input":
		st := m.getInputState(inst, key, propStr(p, "value"))
		changeID := eventID(n, "change")
		submitID := eventID(n, "submit")
		in := better.Input(c, t, key, &st.Value, better.InputProps{
			Placeholder: propStr(p, "placeholder"),
			Disabled:    propBool(p, "disabled"),
			Password:    propBool(p, "password"),
		})
		if in.Changed() && changeID != "" {
			st.Dirty = true
			st.DirtyAt = c.Now()
			st.PendingEvent = changeID
			c.After(inputDebounce)
		}
		if in.Submitted() && submitID != "" {
			st.Dirty = false
			st.DirtyAt = time.Time{}
			m.dispatch(inst, submitID, st.Value)
		}
		return in

	case "Textarea":
		st := m.getInputState(inst, key, propStr(p, "value"))
		changeID := eventID(n, "change")
		submitID := eventID(n, "submit")
		rows := int(propFloat(p, "rows"))
		if propBool(p, "autoResize") {
			lines := strings.Count(st.Value, "\n") + 1
			if lines > rows {
				rows = min(lines, 10)
			}
		}
		ta := better.Textarea(c, t, key, &st.Value, better.TextareaProps{
			Placeholder: propStr(p, "placeholder"),
			Rows:        rows,
			Disabled:    propBool(p, "disabled"),
		})
		if ta.Changed() && changeID != "" {
			st.Dirty = true
			st.DirtyAt = c.Now()
			st.PendingEvent = changeID
			c.After(inputDebounce)
		}
		if ta.Focused() && submitID != "" && c.Shortcut(ui.Cmd, ui.KeyEnter) {
			st.Dirty = false
			st.DirtyAt = time.Time{}
			m.dispatch(inst, submitID, st.Value)
		}
		return ta

	case "Switch":
		changeID := eventID(n, "change")
		better.Switch(c, t, key, propBool(p, "checked"), propBool(p, "disabled"), func(v bool) {
			if changeID != "" {
				m.dispatch(inst, changeID, v)
			}
		})
		return ui.Element{}

	case "Checkbox":
		changeID := eventID(n, "change")
		lbl := propStr(p, "label")
		return ui.Row(c.Key(key)).Gap(8).AlignItems(ui.Center).Children(func() {
			components.Checkbox(c, key+"-box", components.CheckboxProps{
				Checked:  propBool(p, "checked"),
				Disabled: propBool(p, "disabled"),
				Label:    lbl,
				OnChange: func(v bool) {
					if changeID != "" {
						m.dispatch(inst, changeID, v)
					}
				},
			})
			if lbl != "" {
				components.Text(c, lbl, theme.TextSM, theme.WeightNormal, t.Text)
			}
		})

	case "Select":
		changeID := eventID(n, "change")
		var opts []better.SelectOption
		if rawOpts, ok := p["options"].([]any); ok {
			for _, item := range rawOpts {
				if mp, ok := item.(map[string]any); ok {
					opts = append(opts, better.SelectOption{
						Value: strVal(mp["value"]),
						Label: strVal(mp["label"]),
					})
				}
			}
		}
		better.Select(c, t, key, better.SelectProps{
			Value:       propStr(p, "value"),
			Options:     opts,
			Placeholder: propStr(p, "placeholder"),
			Disabled:    propBool(p, "disabled"),
			OnChange: func(v string) {
				if changeID != "" {
					m.dispatch(inst, changeID, v)
				}
			},
		})
		return ui.Element{}

	case "NumberInput":
		changeID := eventID(n, "change")
		val := int(math.Round(float64(propFloat(p, "value"))))
		np := better.NumberInputProps{
			Step:     int(propFloat(p, "step")),
			Disabled: propBool(p, "disabled"),
			OnChange: func(v int) {
				if changeID != "" {
					m.dispatch(inst, changeID, v)
				}
			},
		}
		if _, ok := p["min"]; ok {
			np.Min = better.Bound(int(propFloat(p, "min")))
		}
		if _, ok := p["max"]; ok {
			np.Max = better.Bound(int(propFloat(p, "max")))
		}
		return better.NumberInput(c, t, key, val, np)

	case "Tabs":
		changeID := eventID(n, "change")
		cur := propStr(p, "value")
		return ui.Row(c.Key(key)).FillWidth().Gap(4).BorderWidth(0, 0, t.HL, 0).BorderColor(t.Divider).Children(func() {
			if rawItems, ok := p["items"].([]any); ok {
				for _, item := range rawItems {
					mp, ok := item.(map[string]any)
					if !ok {
						continue
					}
					tk := strVal(mp["key"])
					lbl := strVal(mp["label"])
					badge := strVal(mp["badge"])
					active := tk == cur
					col := t.TextMuted
					if active {
						col = t.TextStrong
					}
					btn := ui.ButtonBase(c.Key(key+"-tab-"+tk)).Padding(8, 12).Gap(6).AlignItems(ui.Center).FocusRing(false)
					if active {
						btn.BorderWidth(0, 0, 2, 0).BorderColor(theme.Primary500)
					} else if btn.Hovered() {
						col = t.Text
					}
					tabKey := tk
					btn.OnClick(func() {
						if changeID != "" {
							m.dispatch(inst, changeID, tabKey)
						}
					}).Children(func() {
						components.Text(c, lbl, theme.TextSM, theme.WeightMedium, col).NoWrap()
						if badge != "" {
							ui.Box(c).Padding(1, 6).Radius(999).Background(t.ChipBg).Children(func() {
								components.Text(c, badge, theme.TextXS, theme.WeightMedium, t.ChipText)
							})
						}
					})
				}
			}
		})

	case "Badge":
		pressID := eventID(n, "press")
		closeID := eventID(n, "close")
		lbl := propStr(p, "label")
		tone := propStr(p, "tone")
		bg, fg := mapBadgeColors(t, tone)
		el := ui.Row(c.Key(key)).Padding(2, 8).Radius(999).Gap(4).AlignItems(ui.Center).Background(bg)
		if pressID != "" {
			el.OnClick(func() { m.dispatch(inst, pressID, nil) })
		}
		el.Children(func() {
			if ic := assets.SafeIcon(propStr(p, "icon"), ""); ic != "" {
				components.Icon(c, ic, 14, fg)
			}
			components.Text(c, lbl, theme.TextXS, theme.WeightMedium, fg).NoWrap()
			if propBool(p, "closable") && closeID != "" {
				ui.ButtonBase(c.Key(key+"-close")).FocusRing(false).OnClick(func() {
					m.dispatch(inst, closeID, nil)
				}).Children(func() {
					components.Icon(c, "close", 12, fg)
				})
			}
		})
		return el

	case "Card":
		pressID := eventID(n, "press")
		pad := mapSpacing(propStr(p, "padding"))
		if pad == 0 {
			pad = 12
		}
		bg, border := t.GlassCard(t.CardBg, t.CardBorder)
		card := ui.Column(c.Key(key)).FillWidth().Radius(12).Clip().Background(bg).Border(t.HL, border)
		if propBool(p, "hoverable") && card.Hovered() {
			card.Border(t.HL, t.CardBorderHover)
		}
		if pressID != "" {
			card.OnClick(func() { m.dispatch(inst, pressID, nil) })
		}
		card.Children(func() {
			if cover := propStr(p, "cover"); cover != "" {
				if bmp := m.loadBitmap(cover); bmp != nil {
					ui.Image(c, bmp).FillWidth().Height(160).Fit(ui.Cover)
				} else {
					ui.Box(c).FillWidth().Height(160).Background(t.CoverBg)
				}
			}
			ui.Column(c).FillWidth().Padding(pad).Gap(8).Children(func() {
				if title := propStr(p, "title"); title != "" {
					components.Text(c, title, theme.TextBase, theme.WeightSemibold, t.TextStrong)
				}
				for i, child := range n.Children {
					m.renderNode(c, t, inst, child, fmt.Sprintf("%s-%d", key, i))
				}
			})
		})
		return card

	case "Collapsible":
		open := m.getCollapsibleState(inst, key, propBool(p, "defaultOpen"))
		icon := assets.SafeIcon(propStr(p, "icon"), "")
		if icon == "" {
			switch propStr(p, "status") {
			case "running":
				icon = "loading"
			case "done":
				icon = "check-circle-outline"
			case "error":
				icon = "alert-circle-outline"
			default:
				icon = "chevron-right"
			}
		}
		return components.CollapsibleSection(c, t, key, components.CollapsibleSectionProps{
			Title: propStr(p, "title"),
			Icon:  icon,
			Open:  open,
		}, func() {
			for i, child := range n.Children {
				m.renderNode(c, t, inst, child, fmt.Sprintf("%s-%d", key, i))
			}
		})

	case "Progress":
		lbl := propStr(p, "label")
		if propBool(p, "indeterminate") {
			return ui.Row(c.Key(key)).Gap(8).AlignItems(ui.Center).Children(func() {
				components.Spinner(c, 16, theme.Primary500)
				if lbl != "" {
					components.Text(c, lbl, theme.TextSM, theme.WeightNormal, t.TextMuted)
				}
			})
		}
		return ui.Column(c.Key(key)).FillWidth().Gap(4).Children(func() {
			if lbl != "" {
				components.Text(c, lbl, theme.TextXS, theme.WeightMedium, t.TextMuted)
			}
			components.ProgressBar(c, t, float64(propFloat(p, "value")), theme.Primary500)
		})

	case "Image":
		src := propStr(p, "src")
		w := propFloat(p, "width")
		h := propFloat(p, "height")
		rad := propFloat(p, "radius")
		pressID := eventID(n, "press")
		previewable := propBool(p, "previewable")
		fit := ui.Cover
		if propStr(p, "fit") == "contain" {
			fit = ui.Contain
		}
		box := ui.Box(c.Key(key)).Clip().Background(t.CoverBg)
		if w > 0 {
			box.Width(w)
		} else {
			box.FillWidth()
		}
		if h > 0 {
			box.Height(h)
		} else if ar := propFloat(p, "aspectRatio"); ar > 0 && w > 0 {
			box.Height(w / ar)
		} else {
			box.Height(160)
		}
		if rad > 0 {
			box.Radius(rad)
		}
		if previewable || pressID != "" {
			box.OnClick(func() {
				if previewable && src != "" {
					inst.PreviewSrc = src
					inst.PreviewOpen = true
				}
				if pressID != "" {
					m.dispatch(inst, pressID, src)
				}
			})
		}
		box.Children(func() {
			if bmp := m.loadBitmap(src); bmp != nil {
				ui.Image(c, bmp).Fill().Fit(fit)
			}
		})
		return box

	case "Avatar":
		sz := propFloat(p, "size")
		if sz <= 0 {
			sz = 32
		}
		src := propStr(p, "src")
		fb := propStr(p, "fallback")
		box := ui.Box(c.Key(key)).Size(sz, sz).Radius(sz / 2).Clip().Center().Background(t.ChipBg)
		box.Children(func() {
			if bmp := m.loadBitmap(src); bmp != nil {
				ui.Image(c, bmp).Fill().Fit(ui.Cover)
			} else if fb != "" {
				components.Text(c, fb, theme.TextXS, theme.WeightSemibold, t.TextStrong)
			}
		})
		return box

	case "Empty":
		icon := assets.SafeIcon(propStr(p, "icon"), "package-variant")
		return ui.Column(c.Key(key)).FillWidth().Padding(24).Gap(8).Center().Children(func() {
			components.Icon(c, icon, 36, t.TextFaint)
			if title := propStr(p, "title"); title != "" {
				components.Text(c, title, theme.TextBase, theme.WeightMedium, t.TextMuted)
			}
			if desc := propStr(p, "description"); desc != "" {
				components.Text(c, desc, theme.TextSM, theme.WeightNormal, t.TextFaint)
			}
		})

	case "Skeleton":
		lines := int(propFloat(p, "lines"))
		if lines <= 0 {
			lines = 3
		}
		h := propFloat(p, "height")
		if h <= 0 {
			h = 16
		}
		return ui.Column(c.Key(key)).FillWidth().Gap(8).Children(func() {
			for i := 0; i < lines; i++ {
				ui.Box(c.Key(fmt.Sprintf("%s-sk%d", key, i))).FillWidth().Height(h).Radius(6).Background(t.ChipBg)
			}
		})

	case "Tooltip":
		var childEl ui.Element
		if len(n.Children) > 0 {
			childEl = m.renderNode(c, t, inst, n.Children[0], key+"-child")
		}
		if tip := propStr(p, "content"); tip != "" && childEl.Valid() {
			better.Tooltip(c, t, childEl, tip)
		}
		return childEl

	case "Table":
		return m.renderTable(c, t, inst, key, p, eventID(n, "rowPress"))

	case "Divider":
		return ui.Box(c.Key(key)).FillWidth().Height(t.HL).Background(t.Divider)

	case "Spacer":
		return ui.Spacer(c)

	default:
		// 未知组件：开发模式下画红色占位并写明类型，正式版跳过（见 §9.6）
		if !m.DevMode {
			return ui.Element{}
		}
		return ui.Box(c.Key(key)).Padding(6, 10).Radius(6).
			Background(theme.Error500.Alpha(0.15)).Border(t.HL, theme.Error500).Children(func() {
			components.Text(c, locales.NativePluginsUnknownComponent(n.Type), theme.TextXS, theme.WeightMedium, theme.Error500)
		})
	}
}

func (m *Manager) applyLayoutProps(el ui.Element, p map[string]any, events map[string]string, inst *Instance) {
	if gap := mapSpacing(propStr(p, "gap")); gap > 0 {
		el.Gap(gap)
	}
	if pad := mapSpacing(propStr(p, "padding")); pad > 0 {
		el.Padding(pad)
	}
	switch propStr(p, "align") {
	case "start":
		el.AlignItems(ui.Start)
	case "center":
		el.AlignItems(ui.Center)
	case "end":
		el.AlignItems(ui.End)
	case "stretch":
		el.AlignItems(ui.Stretch)
	}
	switch propStr(p, "justify") {
	case "start":
		el.Justify(ui.Start)
	case "center":
		el.Justify(ui.Center)
	case "end":
		el.Justify(ui.End)
	case "between", "space-between":
		el.Justify(ui.SpaceBetween)
	}
	if w := propFloat(p, "width"); w > 0 {
		el.Width(w)
	} else if propStr(p, "width") == "full" {
		el.FillWidth()
	}
	if h := propFloat(p, "height"); h > 0 {
		el.Height(h)
	} else if propStr(p, "height") == "full" {
		el.FillHeight()
	}
	if g := propFloat(p, "grow"); g > 0 {
		el.Grow(g)
	}
	if propBool(p, "dragWindow") {
		el.DragWindow()
	}
	if pressID := events["press"]; pressID != "" {
		el.OnClick(func() { m.dispatch(inst, pressID, nil) })
	}
	if ctxID := events["contextMenu"]; ctxID != "" && el.RightClicked() {
		m.dispatch(inst, ctxID, nil)
	}
}

func (m *Manager) renderTable(c *ui.Context, t *theme.Theme, inst *Instance, key string, p map[string]any, rowPressID string) ui.Element {
	type colDef struct {
		Key   string
		Label string
	}
	var cols []colDef
	if rawCols, ok := p["columns"].([]any); ok {
		for _, item := range rawCols {
			if mp, ok := item.(map[string]any); ok {
				cols = append(cols, colDef{Key: strVal(mp["key"]), Label: strVal(mp["label"])})
			} else if s, ok := item.(string); ok {
				cols = append(cols, colDef{Key: s, Label: s})
			}
		}
	}
	rawRows, _ := p["rows"].([]any)
	bg, border := t.GlassPanel(t.PanelBg, t.PanelBorder)
	return ui.Column(c.Key(key)).FillWidth().Radius(10).Clip().Background(bg).Border(t.HL, border).Children(func() {
		if len(cols) > 0 {
			ui.Row(c.Key(key+"-hdr")).FillWidth().Padding(8, 12).Gap(8).
				Background(t.ChipBg).BorderWidth(0, 0, t.HL, 0).BorderColor(t.Divider).Children(func() {
				for _, col := range cols {
					ui.Box(c.Key(key+"-h-"+col.Key)).Grow(1).Basis(0).MinWidth(0).Children(func() {
						components.Text(c, col.Label, theme.TextXS, theme.WeightSemibold, t.TextMuted).SingleLine()
					})
				}
			})
		}
		for i, rawRow := range rawRows {
			rowMap, _ := rawRow.(map[string]any)
			rIdx := i
			row := ui.Row(c.Key(fmt.Sprintf("%s-r%d", key, i))).FillWidth().Padding(8, 12).Gap(8).
				BorderWidth(0, 0, t.HL, 0).BorderColor(t.Divider)
			if rowPressID != "" {
				if row.Hovered() {
					row.Background(t.MenuHover)
				}
				row.OnClick(func() { m.dispatch(inst, rowPressID, map[string]any{"index": rIdx, "row": rowMap}) })
			}
			row.Children(func() {
				for _, col := range cols {
					val := ""
					if rowMap != nil {
						val = strVal(rowMap[col.Key])
					}
					ui.Box(c.Key(fmt.Sprintf("%s-r%d-%s", key, i, col.Key))).Grow(1).Basis(0).MinWidth(0).Children(func() {
						components.Text(c, val, theme.TextSM, theme.WeightNormal, t.Text).SingleLine()
					})
				}
			})
		}
	})
}

func (m *Manager) renderCodeBlock(c *ui.Context, t *theme.Theme, key, code, lang string, copyable bool) ui.Element {
	services := c.Services()
	bg := t.Pick(theme.Brand100, theme.Brand900)
	border := t.Divider
	return ui.Column(c.Key(key)).FillWidth().Radius(8).Clip().Background(bg).Border(t.HL, border).Children(func() {
		if lang != "" || copyable {
			ui.Row(c).FillWidth().Padding(4, 10).AlignItems(ui.Center).Justify(ui.SpaceBetween).
				BorderWidth(0, 0, t.HL, 0).BorderColor(border).Children(func() {
				components.Text(c, lang, theme.TextXS, theme.WeightMedium, t.TextMuted)
				if copyable {
					better.Button(c, t, key+"-copy", better.ButtonProps{
						Label:   locales.NativePluginsCopyCode(),
						Variant: better.ButtonGhost,
						Size:    better.ButtonSM,
						OnClick: func() {
							services.WriteClipboard(code)
							m.env.Toast(c, env.Success(locales.NativePluginsCopied()))
						},
					})
				}
			})
		}
		ui.Box(c).FillWidth().Padding(10).Children(func() {
			el := components.Text(c, code, theme.TextXS, theme.WeightNormal, t.TextStrong).Selectable()
			if mf := theme.ActiveMonoFont(); mf != "" {
				el.Font(mf)
			}
		})
	})
}

// renderMarkdown 将结构化 Markdown 内容解析为原生组件树（标题、列表、代码块、引用、分隔线、图片、段落）。
func (m *Manager) renderMarkdown(c *ui.Context, t *theme.Theme, inst *Instance, key, content, linkPressID string) ui.Element {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	return ui.Column(c.Key(key)).FillWidth().Gap(8).Children(func() {
		i := 0
		blockIdx := 0
		for i < len(lines) {
			line := lines[i]
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				i++
				continue
			}
			blockIdx++
			bKey := fmt.Sprintf("%s-md%d", key, blockIdx)

			// 围栏代码块 ```
			if strings.HasPrefix(trimmed, "```") {
				lang := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
				i++
				var codeLines []string
				for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
					codeLines = append(codeLines, lines[i])
					i++
				}
				if i < len(lines) {
					i++
				}
				m.renderCodeBlock(c, t, bKey, strings.Join(codeLines, "\n"), lang, true)
				continue
			}
			// 标题 # / ## / ###
			if strings.HasPrefix(trimmed, "#") {
				level := 0
				for level < len(trimmed) && trimmed[level] == '#' {
					level++
				}
				if level <= 6 && level < len(trimmed) && trimmed[level] == ' ' {
					title := strings.TrimSpace(trimmed[level+1:])
					step := theme.TextBase
					switch level {
					case 1:
						step = theme.TextXL
					case 2:
						step = theme.TextLG
					}
					components.Text(c, title, step, theme.WeightBold, t.TextStrong).Selectable()
					i++
					continue
				}
			}
			// 分隔线
			if trimmed == "---" || trimmed == "***" {
				ui.Box(c.Key(bKey)).FillWidth().Height(t.HL).Background(t.Divider)
				i++
				continue
			}
			// 引用块 >
			if strings.HasPrefix(trimmed, ">") {
				var quoteLines []string
				for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">") {
					quoteLines = append(quoteLines, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), ">")))
					i++
				}
				ui.Box(c.Key(bKey)).FillWidth().Padding(6, 12).BorderWidth(0, 0, 0, 3).
					BorderColor(theme.Primary400).Background(t.ChipBg).Children(func() {
					components.Text(c, strings.Join(quoteLines, "\n"), theme.TextSM, theme.WeightNormal, t.TextMuted).Selectable()
				})
				continue
			}
			// 图片 ![alt](url)
			if strings.HasPrefix(trimmed, "![") && strings.Contains(trimmed, "](") && strings.HasSuffix(trimmed, ")") {
				openIdx := strings.Index(trimmed, "](")
				imgURL := trimmed[openIdx+2 : len(trimmed)-1]
				if bmp := m.loadBitmap(imgURL); bmp != nil {
					ui.Image(c, bmp).FillWidth().Height(200).Radius(8).Fit(ui.Contain)
				}
				i++
				continue
			}
			// 无序列表
			if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
				ui.Row(c.Key(bKey)).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
					components.Text(c, "•", theme.TextSM, theme.WeightBold, t.TextMuted)
					components.Text(c, strings.TrimSpace(trimmed[2:]), theme.TextSM, theme.WeightNormal, t.Text).Selectable().Grow(1)
				})
				i++
				continue
			}
			// 普通段落（附带简单链接点击提取）
			if linkPressID != "" && strings.Contains(trimmed, "](") {
				if openText := strings.Index(trimmed, "["); openText >= 0 {
					if mid := strings.Index(trimmed[openText:], "]("); mid > 0 {
						end := strings.Index(trimmed[openText+mid+2:], ")")
						if end > 0 {
							linkText := trimmed[openText+1 : openText+mid]
							href := trimmed[openText+mid+2 : openText+mid+2+end]
							ui.Row(c.Key(bKey)).Gap(4).Children(func() {
								if openText > 0 {
									components.Text(c, trimmed[:openText], theme.TextSM, theme.WeightNormal, t.Text)
								}
								ui.ButtonBase(c.Key(bKey+"-link")).FocusRing(false).OnClick(func() {
									m.dispatch(inst, linkPressID, href)
								}).Children(func() {
									components.Text(c, linkText, theme.TextSM, theme.WeightMedium, theme.Primary500)
								})
							})
							i++
							continue
						}
					}
				}
			}
			components.Text(c, line, theme.TextSM, theme.WeightNormal, t.Text).Selectable()
			i++
		}
	})
}

// ─────────────────────────── 主题令牌映射（见 §9.8） ───────────────────────────

func mapSpacing(v string) float32 {
	switch v {
	case "none":
		return 0
	case "xs":
		return 4
	case "sm":
		return 8
	case "md":
		return 12
	case "lg":
		return 16
	case "xl":
		return 24
	default:
		return 0
	}
}

func mapFontStep(v string) theme.FontStep {
	switch v {
	case "xs":
		return theme.TextXS
	case "sm":
		return theme.TextSM
	case "lg":
		return theme.TextLG
	case "xl":
		return theme.TextXL
	default:
		return theme.TextBase
	}
}

func mapFontWeight(v string) int {
	switch v {
	case "medium":
		return theme.WeightMedium
	case "semibold":
		return theme.WeightSemibold
	case "bold":
		return theme.WeightBold
	default:
		return theme.WeightNormal
	}
}

func mapToneColor(t *theme.Theme, tone string) ui.Color {
	switch tone {
	case "muted":
		return t.TextMuted
	case "strong":
		return t.TextStrong
	case "brand":
		return theme.Primary500
	case "success":
		return theme.Success500
	case "warning":
		return theme.Warning500
	case "danger":
		return theme.Error500
	default:
		return t.Text
	}
}

func mapBadgeColors(t *theme.Theme, tone string) (bg, fg ui.Color) {
	switch tone {
	case "brand":
		return t.Pick(theme.Primary100, theme.Primary900), t.Pick(theme.Primary700, theme.Primary300)
	case "success":
		return t.Pick(theme.Success100, theme.Success900), t.Pick(theme.Success700, theme.Success300)
	case "warning":
		return t.Pick(theme.Warning100, theme.Warning900), t.Pick(theme.Warning700, theme.Warning300)
	case "danger":
		return t.Pick(theme.Error100, theme.Error900), t.Pick(theme.Error700, theme.Error300)
	default:
		return t.ChipBg, t.ChipText
	}
}

func mapButtonVariant(v string) better.ButtonVariant {
	switch v {
	case "primary":
		return better.ButtonPrimary
	case "danger":
		return better.ButtonDanger
	case "ghost":
		return better.ButtonGhost
	default:
		return better.ButtonSecondary
	}
}

func mapButtonSize(v string) better.ButtonSize {
	switch v {
	case "sm":
		return better.ButtonSM
	case "lg":
		return better.ButtonLG
	default:
		return better.ButtonMD
	}
}

func propStr(p map[string]any, key string) string {
	if p == nil {
		return ""
	}
	return strVal(p[key])
}

func strVal(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		return fmt.Sprint(x)
	}
}

func propFloat(p map[string]any, key string) float32 {
	if p == nil {
		return 0
	}
	switch x := p[key].(type) {
	case float64:
		return float32(x)
	case int:
		return float32(x)
	case float32:
		return x
	default:
		return 0
	}
}

func propBool(p map[string]any, key string) bool {
	if p == nil {
		return false
	}
	b, _ := p[key].(bool)
	return b
}

func eventID(n *plugin.UINode, names ...string) string {
	if n == nil || len(n.Events) == 0 {
		return ""
	}
	for _, name := range names {
		if id := n.Events[name]; id != "" {
			return id
		}
	}
	return ""
}
