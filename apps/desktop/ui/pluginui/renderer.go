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

// NativeSlots 允许宿主页面把原生交互区域（如启动栏、封面、返回按钮）嵌入插件描述树的 NativeSlot 节点中。
// props 为插件在 <NativeSlot name="..." ... /> 上附带的可选提示参数（例如 width、maxWidth）。
type NativeSlots map[string]func(c *ui.Context, t *theme.Theme, props map[string]any)

// RenderSlot 挂载并渲染一个插件插槽实例。
func (m *Manager) RenderSlot(c *ui.Context, t *theme.Theme, slotName, pluginID, regID, scopeKey string, props any) {
	m.RenderSlotWithNatives(c, t, slotName, pluginID, regID, scopeKey, props, nil)
}

// RenderSlotWithNatives 挂载并渲染一个插件插槽实例，并向其提供可嵌入的宿主原生槽位；返回实例是否已有可渲染内容（树或错误）。
func (m *Manager) RenderSlotWithNatives(c *ui.Context, t *theme.Theme, slotName, pluginID, regID, scopeKey string, props any, natives NativeSlots) bool {
	if m == nil {
		return false
	}
	inst := m.EnsureMounted(slotName, pluginID, regID, scopeKey, props)
	m.RenderInstanceWithNatives(c, t, inst, natives)
	return inst != nil && (inst.Tree != nil || inst.Error != "")
}

// RenderInstance 渲染一个插槽实例当前缓存的描述树，并在出错时展示统一的错误边界占位。
func (m *Manager) RenderInstance(c *ui.Context, t *theme.Theme, inst *Instance) {
	m.RenderInstanceWithNatives(c, t, inst, nil)
}

// RenderInstanceWithNatives 渲染插件实例描述树并解析其中的 NativeSlot 宿主回调。
func (m *Manager) RenderInstanceWithNatives(c *ui.Context, t *theme.Theme, inst *Instance, natives NativeSlots) {
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
	m.renderNode(c, t, inst, inst.Tree, "root", natives)

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

func (m *Manager) renderNode(c *ui.Context, t *theme.Theme, inst *Instance, n *plugin.UINode, fallbackKey string, natives NativeSlots) ui.Element {
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
		m.applyLayoutProps(el, t, p, n.Events, inst)
		el.Children(func() {
			for i, child := range n.Children {
				m.renderNode(c, t, inst, child, fmt.Sprintf("%s-%d", key, i), natives)
			}
		})
		return el

	case "Row":
		el := ui.Row(c.Key(key))
		m.applyLayoutProps(el, t, p, n.Events, inst)
		el.Children(func() {
			for i, child := range n.Children {
				m.renderNode(c, t, inst, child, fmt.Sprintf("%s-%d", key, i), natives)
			}
		})
		return el

	case "Box":
		if propBool(p, "springPress") {
			return m.renderSpringPressBox(c, t, inst, key, p, n.Events, n.Children, natives)
		}
		el := ui.Box(c.Key(key))
		m.applyLayoutProps(el, t, p, n.Events, inst)
		el.Children(func() {
			for i, child := range n.Children {
				m.renderNode(c, t, inst, child, fmt.Sprintf("%s-%d", key, i), natives)
			}
		})
		return el

	case "Stack":
		el := ui.Box(c.Key(key))
		m.applyLayoutProps(el, t, p, n.Events, inst)
		el.Children(func() {
			for i, child := range n.Children {
				m.renderNode(c, t, inst, child, fmt.Sprintf("%s-%d", key, i), natives)
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
						m.renderNode(c, t, inst, child, fmt.Sprintf("%s-%d", key, i), natives)
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
							m.renderNode(c, t, inst, child, fmt.Sprintf("%s-%d", key, start+j), natives)
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
		step := mapFontStepProp(p)
		weight := mapFontWeight(propStr(p, "weight"))
		col := mapToneColor(t, propStr(p, "tone"))
		if customCol, ok := resolveColor(t, p["color"]); ok {
			col = customCol
		}
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
		if lbl := propStr(p, "label"); lbl != "" {
			el.Label(lbl)
		}
		applyMarginProps(el, p)
		return el

	case "Markdown":
		return m.renderMarkdown(c, t, inst, key, propStr(p, "content"), eventID(n, "linkPress"), propBool(p, "streaming"))

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
						btn.BorderWidth(0, 0, 2, 0).BorderColor(t.Accent)
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
		card := ui.Column(c.Key(key)).Radius(12).Clip().Background(bg).Border(t.HL, border)
		if w := propFloat(p, "width"); w > 0 {
			card.Width(w)
		} else {
			card.FillWidth()
		}
		if h := propFloat(p, "height"); h > 0 {
			card.Height(h)
		} else if propStr(p, "height") == "full" {
			card.FillHeight()
		}
		if propBool(p, "dragWindow") {
			card.DragWindow()
		}
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
					m.renderNode(c, t, inst, child, fmt.Sprintf("%s-%d", key, i), natives)
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
				m.renderNode(c, t, inst, child, fmt.Sprintf("%s-%d", key, i), natives)
			}
		})

	case "Progress":
		lbl := propStr(p, "label")
		if propBool(p, "indeterminate") {
			return ui.Row(c.Key(key)).Gap(8).AlignItems(ui.Center).Children(func() {
				components.Spinner(c, 16, t.Accent)
				if lbl != "" {
					components.Text(c, lbl, theme.TextSM, theme.WeightNormal, t.TextMuted)
				}
			})
		}
		val := float64(propFloat(p, "value"))
		if val > 1 {
			val = val / 100
		}
		if val < 0 {
			val = 0
		}
		barColor := t.Accent
		switch propStr(p, "tone") {
		case "success":
			barColor = theme.Success500
		case "warning":
			barColor = theme.Warning500
		case "danger":
			barColor = theme.Error500
		}
		return ui.Column(c.Key(key)).FillWidth().Gap(4).Children(func() {
			if lbl != "" {
				components.Text(c, lbl, theme.TextXS, theme.WeightMedium, t.TextMuted)
			}
			components.ProgressBar(c, t, val, barColor)
		})

	case "Image":
		return m.renderImage(c, t, inst, key, n)

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
			childEl = m.renderNode(c, t, inst, n.Children[0], key+"-child", natives)
		}
		if tip := propStr(p, "content"); tip != "" && childEl.Valid() {
			better.Tooltip(c, t, childEl, tip)
		}
		return childEl

	case "Table":
		return m.renderTable(c, t, inst, key, p, eventID(n, "rowPress"))

	case "Divider":
		col := t.Divider
		if propBool(p, "subtle") {
			col = t.Divider.Alpha(0.45)
		}
		el := ui.Box(c.Key(key)).FillWidth().Height(t.HL).Background(col)
		if my := propSpacing(p, "marginY"); my > 0 {
			el.Margin(my, 0)
		}
		return el

	case "Spacer":
		return ui.Spacer(c)

	case "Icon":
		fallback := "circle-medium"
		if _, hasFb := p["fallback"]; hasFb {
			fallback = propStr(p, "fallback")
		}
		name := assets.SafeIcon(propStr(p, "name"), fallback)
		if name == "" {
			return ui.Element{}
		}
		sz := propFloat(p, "size")
		if sz <= 0 {
			sz = 16
		}
		col := mapToneColor(t, propStr(p, "tone"))
		if customCol, ok := resolveColor(t, p["color"]); ok {
			col = customCol
		}
		return components.Icon(c, name, sz, col)

	case "NativeSlot":
		name := propStr(p, "name")
		if natives != nil {
			if fn, ok := natives[name]; ok && fn != nil {
				fn(c, t, p)
			}
		}
		return ui.Element{}

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

func (m *Manager) applyLayoutProps(el ui.Element, t *theme.Theme, p map[string]any, events map[string]string, inst *Instance) {
	if gap := propSpacing(p, "gap"); gap > 0 {
		el.Gap(gap)
	}
	applyPaddingProps(el, p)
	applyMarginProps(el, p)

	disabled := propBool(p, "disabled")
	borderWidth := propBorderWidth(p, t.HL)
	customBorderCol, hasCustomBorderCol := resolveColor(t, p["borderColor"])
	var surfaceBorder ui.Color
	hasSurfaceBorder := false

	switch propStr(p, "surface") {
	case "glass":
		el.Background(t.Pick(theme.White.Alpha(0.68), theme.Brand900.Alpha(0.72)))
		surfaceBorder = t.Pick(theme.Brand300.Alpha(0.65), theme.White.Alpha(0.18))
		hasSurfaceBorder = true
	case "glass-card":
		el.Background(t.Pick(theme.White.Alpha(0.42), theme.Brand800.Alpha(0.48)))
		surfaceBorder = t.Pick(theme.Brand200.Alpha(0.55), theme.White.Alpha(0.14))
		hasSurfaceBorder = true
	case "panel":
		bg, border := t.GlassPanel(t.PanelBg, t.PanelBorder)
		el.Background(bg)
		surfaceBorder = border
		hasSurfaceBorder = true
	case "card":
		bg, border := t.GlassCard(t.CardBg, t.CardBorder)
		el.Background(bg)
		surfaceBorder = border
		hasSurfaceBorder = true
	case "chip":
		el.Background(t.ChipBg)
		surfaceBorder = t.Divider
		hasSurfaceBorder = true
	case "subtle":
		el.Background(t.Pick(theme.Brand150.Alpha(0.55), theme.White.Alpha(0.05)))
		surfaceBorder = t.Divider.Alpha(0.5)
		hasSurfaceBorder = true
	}

	if bg, ok := resolveColor(t, p["background"]); ok {
		el.Background(bg)
	}
	if hoverBg, ok := resolveColor(t, p["hoverBackground"]); ok {
		el.Transition(theme.ColorsTransition(120 * time.Millisecond))
		if !disabled && el.Hovered() {
			el.Background(hoverBg)
		}
	}
	if grad, ok := resolveGradient(t, p["gradient"]); ok {
		el.LinearGradient(grad)
	}

	if borderWidth > 0 {
		bc := t.PanelBorder
		if hasSurfaceBorder {
			bc = surfaceBorder
		}
		if hasCustomBorderCol {
			bc = customBorderCol
		}
		el.Border(borderWidth, bc)
	}
	if propBool(p, "borderBottom") {
		bc := t.Divider.Alpha(0.5)
		if hasCustomBorderCol {
			bc = customBorderCol
		}
		el.BorderWidth(0, 0, t.HL, 0).BorderColor(bc)
	}
	if propBool(p, "borderLeft") {
		bc := t.Divider.Alpha(0.5)
		if hasCustomBorderCol {
			bc = customBorderCol
		}
		el.BorderWidth(0, 0, 0, t.HL).BorderColor(bc)
	}

	if x, y, blur, spread, col, ok := resolveShadow(t, p["shadow"]); ok {
		el.Shadow(x, y, blur, spread, col)
	}
	if op := propFloat(p, "opacity"); op > 0 && op < 1 {
		el.Opacity(op)
	}

	if rad := propRadius(p, "radius"); rad > 0 {
		el.Radius(rad)
		if clipVal, hasClip := p["clip"]; !hasClip || clipVal != false {
			el.Clip()
		}
	}
	if propBool(p, "clip") {
		el.Clip()
	}

	if propStr(p, "position") == "absolute" {
		el.Absolute()
	}
	if _, hasInset := p["inset"]; hasInset {
		v := propFloat(p, "inset")
		el.Top(v).Right(v).Bottom(v).Left(v)
	}
	if _, ok := p["top"]; ok {
		el.Top(propFloat(p, "top"))
	}
	if _, ok := p["right"]; ok {
		el.Right(propFloat(p, "right"))
	}
	if _, ok := p["bottom"]; ok {
		el.Bottom(propFloat(p, "bottom"))
	}
	if _, ok := p["left"]; ok {
		el.Left(propFloat(p, "left"))
	}
	if _, ok := p["zIndex"]; ok {
		el.ZIndex(int(propFloat(p, "zIndex")))
	}
	if propStr(p, "pointerEvents") == "none" {
		el.PassThrough()
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
	switch propStr(p, "alignSelf") {
	case "start":
		el.AlignSelf(ui.Start)
	case "center":
		el.AlignSelf(ui.Center)
	case "end":
		el.AlignSelf(ui.End)
	case "stretch":
		el.AlignSelf(ui.Stretch)
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
	if _, hasShrink := p["shrink"]; hasShrink {
		el.Shrink(propFloat(p, "shrink"))
	}
	if _, hasBasis := p["basis"]; hasBasis {
		el.Basis(propFloat(p, "basis"))
	}
	if mw := propFloat(p, "minWidth"); mw > 0 || p["minWidth"] != nil {
		el.MinWidth(mw)
	}
	if mxw := propFloat(p, "maxWidth"); mxw > 0 {
		el.MaxWidth(mxw)
	}
	if mh := propFloat(p, "minHeight"); mh > 0 || p["minHeight"] != nil {
		el.MinHeight(mh)
	}
	if mxh := propFloat(p, "maxHeight"); mxh > 0 {
		el.MaxHeight(mxh)
	}
	if propBool(p, "wrap") {
		el.Wrap()
	}
	if propBool(p, "dragWindow") {
		el.DragWindow()
	}
	if lbl := propStr(p, "label"); lbl != "" {
		el.Label(lbl)
	}
	if pressID := events["press"]; pressID != "" && !disabled {
		el.Cursor(ui.CursorPointer)
		el.OnClick(func() { m.dispatch(inst, pressID, nil) })
	}
	if ctxID := events["contextMenu"]; ctxID != "" && !disabled && el.RightClicked() {
		m.dispatch(inst, ctxID, nil)
	}
}

func applyPaddingProps(el ui.Element, p map[string]any) {
	if p == nil {
		return
	}
	_, hasT := p["paddingTop"]
	_, hasR := p["paddingRight"]
	_, hasB := p["paddingBottom"]
	_, hasL := p["paddingLeft"]
	if hasT || hasR || hasB || hasL {
		base := propSpacing(p, "padding")
		px := propSpacing(p, "paddingX")
		py := propSpacing(p, "paddingY")
		if px == 0 {
			px = base
		}
		if py == 0 {
			py = base
		}
		pt, pr, pb, pl := py, px, py, px
		if hasT {
			pt = propSpacing(p, "paddingTop")
		}
		if hasR {
			pr = propSpacing(p, "paddingRight")
		}
		if hasB {
			pb = propSpacing(p, "paddingBottom")
		}
		if hasL {
			pl = propSpacing(p, "paddingLeft")
		}
		el.Padding(pt, pr, pb, pl)
		return
	}
	px := propSpacing(p, "paddingX")
	py := propSpacing(p, "paddingY")
	if px > 0 || py > 0 {
		el.Padding(py, px)
	} else if pad := propSpacing(p, "padding"); pad > 0 {
		el.Padding(pad)
	}
}

func applyMarginProps(el ui.Element, p map[string]any) {
	if p == nil {
		return
	}
	centerX := propBool(p, "centerX") || propStr(p, "marginX") == "auto"
	_, hasM := p["margin"]
	_, hasMX := p["marginX"]
	_, hasMY := p["marginY"]
	_, hasMT := p["marginTop"]
	_, hasMR := p["marginRight"]
	_, hasMB := p["marginBottom"]
	_, hasML := p["marginLeft"]
	if !centerX && !hasM && !hasMX && !hasMY && !hasMT && !hasMR && !hasMB && !hasML {
		return
	}
	base := propSpacing(p, "margin")
	mx := propSpacing(p, "marginX")
	my := propSpacing(p, "marginY")
	if !hasMX {
		mx = base
	}
	if !hasMY {
		my = base
	}
	mt, mr, mb, ml := my, mx, my, mx
	if hasMT {
		mt = propSpacing(p, "marginTop")
	}
	if hasMR {
		mr = propSpacing(p, "marginRight")
	}
	if hasMB {
		mb = propSpacing(p, "marginBottom")
	}
	if hasML {
		ml = propSpacing(p, "marginLeft")
	}
	if centerX {
		mr = ui.Auto
		ml = ui.Auto
	}
	el.Margin(mt, mr, mb, ml)
}

var springSquishEase = theme.CubicBezier(0.34, 1.56, 0.64, 1)

// renderSpringPressBox 为支持按压 Q 弹（springPress: true）的容器提供原生帧率弹簧压扁动画与 pressIn/pressOut 事件。
// 外层使用稳定尺寸的 ButtonBase 承接命中测试，内层围绕底边中心 (0.5, 1.0) 做弹性形变，避免按压边缘抖动。
func (m *Manager) renderSpringPressBox(
	c *ui.Context,
	t *theme.Theme,
	inst *Instance,
	key string,
	p map[string]any,
	events map[string]string,
	children []*plugin.UINode,
	natives NativeSlots,
) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).FocusRing(false).Cursor(ui.CursorPointer)
	m.applyLayoutProps(btn, t, p, events, inst)
	if events["press"] == "" {
		btn.OnClick(func() {})
	}

	pressed := btn.Pressed()
	wasPressed := ui.Local(btn, "spring-was-pressed", func() bool { return false })
	bounceUntil := ui.Local(btn, "spring-until", func() time.Time { return time.Time{} })

	if pressed && !*wasPressed {
		*wasPressed = true
		*bounceUntil = c.Now().Add(95 * time.Millisecond)
		c.After(95 * time.Millisecond)
		if id := events["pressIn"]; id != "" {
			m.dispatch(inst, id, nil)
		}
	} else if !pressed && *wasPressed {
		*wasPressed = false
		if id := events["pressOut"]; id != "" {
			m.dispatch(inst, id, nil)
		}
	}

	squished := pressed
	if !squished && !bounceUntil.IsZero() {
		if rem := bounceUntil.Sub(c.Now()); rem > 0 {
			squished = true
			c.After(rem)
		} else {
			*bounceUntil = time.Time{}
		}
	}

	squishX := propFloat(p, "squishX")
	if squishX <= 0 {
		squishX = 1.05
	}
	squishY := propFloat(p, "squishY")
	if squishY <= 0 {
		squishY = 0.88
	}
	targetSX, targetSY := float32(1.0), float32(1.0)
	if squished {
		targetSX, targetSY = squishX, squishY
	}
	sx := btn.AnimateWith("spring-sx", targetSX, 220*time.Millisecond, springSquishEase)
	sy := btn.AnimateWith("spring-sy", targetSY, 220*time.Millisecond, springSquishEase)
	if propBool(p, "flipX") {
		sx = -sx
	}

	btn.Children(func() {
		ui.Box(c.Key(key+"-spring-body")).Fill().TransformOrigin(0.5, 1.0).Scale(sx, sy).Children(func() {
			for i, child := range children {
				m.renderNode(c, t, inst, child, fmt.Sprintf("%s-%d", key, i), natives)
			}
		})
	})
	return btn
}

// renderImage 渲染通用图片原语：支持回退图（fallbackSrc）、按自然宽高比适配 maxWidth/maxHeight、
// 绝对定位铺底（position="absolute" + inset）、高斯模糊（blur）、点击预览（previewable）以及 onLoad/onError 去重回调。
func (m *Manager) renderImage(
	c *ui.Context,
	t *theme.Theme,
	inst *Instance,
	key string,
	n *plugin.UINode,
) ui.Element {
	p := n.Props
	src := propStr(p, "src")
	fallbackSrc := propStr(p, "fallbackSrc")
	if src == "" && fallbackSrc != "" {
		src = fallbackSrc
		fallbackSrc = ""
	}
	w := propFloat(p, "width")
	h := propFloat(p, "height")
	maxW := propFloat(p, "maxWidth")
	maxH := propFloat(p, "maxHeight")
	rad := propRadius(p, "radius")
	pressID := eventID(n, "press")
	loadID := eventID(n, "load")
	errorID := eventID(n, "error")
	previewable := propBool(p, "previewable")
	transparent := propBool(p, "transparent")
	hoverBorder := propBool(p, "hoverBorder")
	hideWhenEmpty := propBool(p, "hideWhenEmpty")
	blurRadius := propBlur(p, "blur")
	fit := ui.Cover
	if propStr(p, "fit") == "contain" {
		fit = ui.Contain
	}

	primarySrc := m.LoadImageSource(src)
	imgSrc := primarySrc
	if imgSrc == nil && fallbackSrc != "" && fallbackSrc != src {
		imgSrc = m.LoadImageSource(fallbackSrc)
	}

	if src != "" {
		if primarySrc != nil && loadID != "" {
			token := "ok:" + src
			if inst.ImageNotified[key] != token {
				inst.ImageNotified[key] = token
				var iw, ih int
				if bmp, ok := primarySrc.(*ui.Bitmap); ok && bmp != nil {
					iw, ih = bmp.Size()
				}
				m.dispatch(inst, loadID, map[string]any{"src": src, "width": iw, "height": ih})
			}
		} else if primarySrc == nil && errorID != "" && m.isImageFailed(src) {
			token := "err:" + src
			if inst.ImageNotified[key] != token {
				inst.ImageNotified[key] = token
				m.dispatch(inst, errorID, map[string]any{"src": src})
			}
		}
	}

	if hideWhenEmpty && imgSrc == nil {
		return ui.Element{}
	}

	isAbsolute := propStr(p, "position") == "absolute"
	box := ui.Box(c.Key(key))

	// 未显式指定 width/height 但给了 maxWidth/maxHeight 时，按自然宽高比等比缩放（不放大超过原图）
	naturalFit := w <= 0 && h <= 0 && (maxW > 0 || maxH > 0) && !isAbsolute &&
		propStr(p, "width") != "full" && propStr(p, "height") != "full"
	if naturalFit {
		if maxW <= 0 {
			maxW = 460
		}
		if maxH <= 0 {
			maxH = 160
		}
		w, h = maxW*0.75, maxH*0.75
		if bmp, ok := imgSrc.(*ui.Bitmap); ok && bmp != nil {
			bw, bh := bmp.Size()
			if bw > 0 && bh > 0 {
				scale := min(maxW/float32(bw), maxH/float32(bh))
				if scale > 1 {
					scale = 1
				}
				w = float32(bw) * scale
				h = float32(bh) * scale
			}
		}
	}

	m.applyLayoutProps(box, t, p, nil, inst)

	if !transparent {
		box.Clip().Background(t.CoverBg)
	} else if rad > 0 {
		box.Clip()
	}

	if !isAbsolute {
		if w > 0 {
			box.Width(w)
		} else if propStr(p, "width") == "full" || !naturalFit {
			box.FillWidth()
		}
		if h > 0 {
			box.Height(h)
		} else if propStr(p, "height") == "full" {
			box.FillHeight()
		} else if ar := propFloat(p, "aspectRatio"); ar > 0 && w > 0 {
			box.Height(w / ar)
		} else if !naturalFit {
			box.Height(160)
		}
	}

	if rad > 0 {
		box.Radius(rad)
	}
	if previewable || pressID != "" {
		box.Cursor(ui.CursorPointer)
		if hoverBorder {
			if box.Hovered() {
				box.Border(1.5, theme.White.Alpha(0.85))
			} else {
				box.Border(t.HL, theme.White.Alpha(0.22))
			}
		}
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
		if imgSrc != nil {
			img := ui.Image(c, imgSrc).Fill().Fit(fit)
			if blurRadius > 0 {
				img.Blur(blurRadius)
			}
		}
	})
	return box
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

// ─────────────────────────── 主题令牌映射（见 §9.8） ───────────────────────────

func propSpacing(p map[string]any, key string) float32 {
	if p == nil {
		return 0
	}
	v, ok := p[key]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case string:
		return mapSpacing(x)
	case float64:
		return float32(x)
	case int:
		return float32(x)
	case int64:
		return float32(x)
	default:
		return 0
	}
}

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
	case "2xl":
		return theme.FontStep{Size: 24, Line: 32}
	case "3xl":
		return theme.FontStep{Size: 30, Line: 38}
	case "4xl":
		return theme.FontStep{Size: 38, Line: 44}
	default:
		return theme.TextBase
	}
}

func mapFontStepProp(p map[string]any) theme.FontStep {
	if p == nil {
		return theme.TextBase
	}
	if sz := propFloat(p, "size"); sz > 0 {
		line := propFloat(p, "lineHeight")
		if line <= 0 {
			line = sz * 1.25
		}
		return theme.FontStep{Size: sz, Line: line}
	}
	step := mapFontStep(propStr(p, "size"))
	if line := propFloat(p, "lineHeight"); line > 0 {
		step.Line = line
	}
	return step
}

func propRadius(p map[string]any, key string) float32 {
	if p == nil {
		return 0
	}
	switch x := p[key].(type) {
	case float64:
		if x > 0 {
			return float32(x)
		}
	case float32:
		if x > 0 {
			return x
		}
	case int:
		if x > 0 {
			return float32(x)
		}
	case string:
		switch x {
		case "sm":
			return 6
		case "md":
			return 10
		case "lg":
			return 14
		case "full":
			return 999
		}
	}
	return 0
}

func propBlur(p map[string]any, key string) float32 {
	if p == nil {
		return 0
	}
	switch x := p[key].(type) {
	case bool:
		if x {
			return 18
		}
	case float64:
		if x > 0 {
			return float32(x)
		}
	case float32:
		if x > 0 {
			return x
		}
	case int:
		if x > 0 {
			return float32(x)
		}
	}
	return 0
}

func propBorderWidth(p map[string]any, hairline float32) float32 {
	if p == nil {
		return 0
	}
	switch x := p["border"].(type) {
	case bool:
		if x {
			return hairline
		}
	case float64:
		if x > 0 {
			return float32(x)
		}
	case float32:
		if x > 0 {
			return x
		}
	case int:
		if x > 0 {
			return float32(x)
		}
	}
	return 0
}

// resolveColor 解析通用颜色值，支持：
// - 语义色板令牌（"appBg"、"fadeBg"、"scrim"、"white"、"black"、"primary"、"warning400" 等）
// - 可选透明度后缀（"black@0.42"、"scrim@0.84"、"#1a1b26@0.8"）
// - 十六进制色值（"#rgb" / "#rgba" / "#rrggbb" / "#rrggbbaa"）
// - 明暗主题自适应对象（{ light: ColorValue, dark: ColorValue }）或 { color/token, alpha }
func resolveColor(t *theme.Theme, raw any) (ui.Color, bool) {
	if raw == nil {
		return ui.Color{}, false
	}
	switch v := raw.(type) {
	case map[string]any:
		if _, hasLight := v["light"]; hasLight || v["dark"] != nil {
			if t.Dark {
				if col, ok := resolveColor(t, v["dark"]); ok {
					return col, true
				}
				return resolveColor(t, v["light"])
			}
			if col, ok := resolveColor(t, v["light"]); ok {
				return col, true
			}
			return resolveColor(t, v["dark"])
		}
		baseVal := v["color"]
		if baseVal == nil {
			baseVal = v["token"]
		}
		col, ok := resolveColor(t, baseVal)
		if !ok {
			return ui.Color{}, false
		}
		if _, hasAlpha := v["alpha"]; hasAlpha {
			a := propFloat(v, "alpha")
			if a < 0 {
				a = 0
			} else if a > 1 {
				a = 1
			}
			col = col.Alpha(a)
		}
		return col, true
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return ui.Color{}, false
		}
		alpha := float32(-1)
		if base, aStr, hasAt := strings.Cut(s, "@"); hasAt {
			s = strings.TrimSpace(base)
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(aStr), 32); err == nil {
				if parsed < 0 {
					parsed = 0
				} else if parsed > 1 {
					parsed = 1
				}
				alpha = float32(parsed)
			}
		}
		col, ok := resolveColorToken(t, s)
		if !ok {
			return ui.Color{}, false
		}
		if alpha >= 0 {
			col = col.Alpha(alpha)
		}
		return col, true
	default:
		return ui.Color{}, false
	}
}

func resolveColorToken(t *theme.Theme, token string) (ui.Color, bool) {
	if strings.HasPrefix(token, "#") {
		hex := token[1:]
		if len(hex) == 3 || len(hex) == 4 || len(hex) == 6 || len(hex) == 8 {
			return ui.Hex(token), true
		}
		return ui.Color{}, false
	}
	switch token {
	case "transparent":
		return ui.Transparent, true
	case "white":
		return theme.White, true
	case "black":
		return theme.Black, true
	case "appBg":
		return t.AppBg, true
	case "fadeBg":
		if t.Glass {
			return t.AppBg.Alpha(0.88), true
		}
		return t.AppBg, true
	case "scrim":
		return t.Pick(theme.Brand900, t.AppBg), true
	case "panelBg":
		return t.PanelBg, true
	case "panelBorder":
		return t.PanelBorder, true
	case "cardBg":
		return t.CardBg, true
	case "cardBorder":
		return t.CardBorder, true
	case "chipBg":
		return t.ChipBg, true
	case "divider":
		return t.Divider, true
	case "text":
		return t.Text, true
	case "textStrong":
		return t.TextStrong, true
	case "textMuted":
		return t.TextMuted, true
	case "textFaint":
		return t.TextFaint, true
	case "primary", "brand":
		return t.Accent, true
	case "accent", "accentText":
		return t.AccentText, true
	case "accentSoft":
		return t.AccentSoft, true
	case "accentSoftText":
		return t.AccentSoftText, true
	case "success":
		return theme.Success500, true
	case "warning":
		return theme.Warning500, true
	case "warning400":
		return theme.Warning400, true
	case "danger":
		return theme.Error500, true
	case "brand700":
		return theme.Brand700, true
	case "brand800":
		return theme.Brand800, true
	case "brand900":
		return theme.Brand900, true
	default:
		return ui.Color{}, false
	}
}

func resolveGradient(t *theme.Theme, raw any) (ui.LinearGradient, bool) {
	mp, ok := raw.(map[string]any)
	if !ok || mp == nil {
		return ui.LinearGradient{}, false
	}
	rawStops, ok := mp["stops"].([]any)
	if !ok || len(rawStops) < 2 {
		return ui.LinearGradient{}, false
	}
	stops := make([]ui.GradientStop, 0, len(rawStops))
	for i, item := range rawStops {
		sm, ok := item.(map[string]any)
		if !ok {
			continue
		}
		col, ok := resolveColor(t, sm["color"])
		if !ok {
			continue
		}
		at := propFloat(sm, "at")
		if _, hasAt := sm["at"]; !hasAt && len(rawStops) > 1 {
			at = float32(i) / float32(len(rawStops)-1)
		}
		stops = append(stops, ui.GradientStop{Color: col, At: at})
	}
	if len(stops) < 2 {
		return ui.LinearGradient{}, false
	}
	angle := propFloat(mp, "angle")
	if _, hasAngle := mp["angle"]; !hasAngle {
		angle = 180
	}
	return ui.LinearGradient{Angle: angle, Stops: stops}, true
}

func resolveShadow(t *theme.Theme, raw any) (x, y, blur, spread float32, col ui.Color, ok bool) {
	if raw == nil {
		return 0, 0, 0, 0, ui.Color{}, false
	}
	switch v := raw.(type) {
	case string:
		switch v {
		case "sm":
			return 0, 2, 8, 0, theme.Black.Alpha(0.18), true
		case "md":
			return 0, 6, 18, 0, theme.Black.Alpha(0.28), true
		case "lg":
			return 0, 14, 32, 0, theme.Black.Alpha(0.45), true
		}
	case map[string]any:
		blur = propFloat(v, "blur")
		if blur <= 0 {
			return 0, 0, 0, 0, ui.Color{}, false
		}
		x = propFloat(v, "x")
		y = propFloat(v, "y")
		spread = propFloat(v, "spread")
		c, hasCol := resolveColor(t, v["color"])
		if !hasCol {
			c = theme.Black.Alpha(0.35)
		}
		return x, y, blur, spread, c, true
	}
	return 0, 0, 0, 0, ui.Color{}, false
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
	case "faint":
		return t.TextFaint
	case "strong":
		return t.TextStrong
	case "brand":
		return t.Accent
	case "accent":
		return t.AccentText
	case "success":
		return theme.Success500
	case "warning":
		return theme.Warning500
	case "danger":
		return theme.Error500
	case "light":
		return theme.White
	case "light-muted":
		return theme.White.Alpha(0.82)
	default:
		return t.Text
	}
}

func mapBadgeColors(t *theme.Theme, tone string) (bg, fg ui.Color) {
	switch tone {
	case "brand":
		return t.AccentSoft, t.AccentSoftText
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
