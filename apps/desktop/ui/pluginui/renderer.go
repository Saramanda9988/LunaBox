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

// NativeSlots 允许宿主页面把原生交互区域（如启动栏、封面、返回按钮）嵌入插件描述树的 NativeSlot / HeroBanner 节点中。
type NativeSlots map[string]func(c *ui.Context, t *theme.Theme)

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

// RenderInstanceWithNatives 渲染插件实例描述树并解析其中的 NativeSlot / HeroBanner 宿主回调。
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
				components.Spinner(c, 16, theme.Primary500)
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
		barColor := theme.Primary500
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
		src := propStr(p, "src")
		w := propFloat(p, "width")
		h := propFloat(p, "height")
		rad := propFloat(p, "radius")
		pressID := eventID(n, "press")
		previewable := propBool(p, "previewable")
		transparent := propBool(p, "transparent")
		blur := propBool(p, "blur")
		hoverBorder := propBool(p, "hoverBorder")
		fit := ui.Cover
		if propStr(p, "fit") == "contain" {
			fit = ui.Contain
		}
		box := ui.Box(c.Key(key))
		if !transparent {
			box.Clip().Background(t.CoverBg)
		} else if rad > 0 {
			box.Clip()
		}
		if w > 0 {
			box.Width(w)
		} else {
			box.FillWidth()
		}
		if h > 0 {
			box.Height(h)
		} else if propStr(p, "height") == "full" {
			box.FillHeight()
		} else if ar := propFloat(p, "aspectRatio"); ar > 0 && w > 0 {
			box.Height(w / ar)
		} else {
			box.Height(160)
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
			if imgSrc := m.LoadImageSource(src); imgSrc != nil {
				img := ui.Image(c, imgSrc).Fill().Fit(fit)
				if blur {
					img.Blur(18)
				}
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
		name := assets.SafeIcon(propStr(p, "name"), "circle-medium")
		if name == "" {
			return ui.Element{}
		}
		sz := propFloat(p, "size")
		if sz <= 0 {
			sz = 16
		}
		col := mapToneColor(t, propStr(p, "tone"))
		return components.Icon(c, name, sz, col)

	case "NativeSlot":
		name := propStr(p, "name")
		if natives != nil {
			if fn, ok := natives[name]; ok && fn != nil {
				fn(c, t)
			}
		}
		return ui.Element{}

	case "HeroBanner":
		return m.renderHeroBanner(c, t, inst, key, p, n.Children, natives)

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
	px := propSpacing(p, "paddingX")
	py := propSpacing(p, "paddingY")
	if px > 0 || py > 0 {
		el.Padding(py, px)
	} else if pad := propSpacing(p, "padding"); pad > 0 {
		el.Padding(pad)
	}
	mt := propSpacing(p, "marginTop")
	mb := propSpacing(p, "marginBottom")
	if mt > 0 || mb > 0 {
		el.Margin(mt, 0, mb, 0)
	}
	switch propStr(p, "surface") {
	case "glass":
		bg := t.Pick(theme.White.Alpha(0.68), theme.Brand900.Alpha(0.72))
		border := t.Pick(theme.Brand300.Alpha(0.65), theme.White.Alpha(0.18))
		el.Background(bg)
		if propBool(p, "border") {
			el.Border(t.HL, border)
		}
	case "glass-card":
		bg := t.Pick(theme.White.Alpha(0.42), theme.Brand800.Alpha(0.48))
		border := t.Pick(theme.Brand200.Alpha(0.55), theme.White.Alpha(0.14))
		el.Background(bg)
		if propBool(p, "border") {
			el.Border(t.HL, border)
		}
	case "panel":
		bg, border := t.GlassPanel(t.PanelBg, t.PanelBorder)
		el.Background(bg)
		if propBool(p, "border") {
			el.Border(t.HL, border)
		}
	case "card":
		bg, border := t.GlassCard(t.CardBg, t.CardBorder)
		el.Background(bg)
		if propBool(p, "border") {
			el.Border(t.HL, border)
		}
	case "chip":
		el.Background(t.ChipBg)
		if propBool(p, "border") {
			el.Border(t.HL, t.Divider)
		}
	case "subtle":
		el.Background(t.Pick(theme.Brand150.Alpha(0.55), theme.White.Alpha(0.05)))
		if propBool(p, "border") {
			el.Border(t.HL, t.Divider.Alpha(0.5))
		}
	default:
		if propBool(p, "border") {
			el.Border(t.HL, t.PanelBorder)
		}
	}
	if propBool(p, "borderBottom") {
		el.BorderWidth(0, 0, t.HL, 0).BorderColor(t.Divider.Alpha(0.5))
	}
	if propBool(p, "borderLeft") {
		el.BorderWidth(0, 0, 0, t.HL).BorderColor(t.Divider.Alpha(0.5))
	}
	if op := propFloat(p, "opacity"); op > 0 && op < 1 {
		el.Opacity(op)
	}
	switch propStr(p, "radius") {
	case "sm":
		el.Radius(6).Clip()
	case "md":
		el.Radius(10).Clip()
	case "lg":
		el.Radius(14).Clip()
	case "full":
		el.Radius(999).Clip()
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
	if propBool(p, "wrap") {
		el.Wrap()
	}
	if propBool(p, "dragWindow") {
		el.DragWindow()
	}
	if pressID := events["press"]; pressID != "" {
		el.Cursor(ui.CursorPointer)
		el.OnClick(func() { m.dispatch(inst, pressID, nil) })
	}
	if ctxID := events["contextMenu"]; ctxID != "" && el.RightClicked() {
		m.dispatch(inst, ctxID, nil)
	}
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

// renderHeroBanner 渲染全出血沉浸式头图横幅（支持透明大徽标、标题/原名、图标元信息行、评分星级、无缝内嵌启动栏、截图缩略条与右侧悬浮封面）。
// 背景图与多层渐变遮罩均设置负 ZIndex，确保永远位于常规流前景内容之下，不会覆盖标题与封面。
func (m *Manager) renderHeroBanner(
	c *ui.Context,
	t *theme.Theme,
	inst *Instance,
	key string,
	p map[string]any,
	children []*plugin.UINode,
	natives NativeSlots,
) ui.Element {
	bannerH := propFloat(p, "height")
	if bannerH < 280 {
		bannerH = 440
	} else if bannerH > 600 {
		bannerH = 600
	}
	bannerSrc := propStr(p, "banner")
	if bannerSrc == "" {
		bannerSrc = propStr(p, "fallbackBanner")
	}
	logoSrc := propStr(p, "logo")
	title := propStr(p, "title")
	subtitle := propStr(p, "subtitle")
	blur := propBool(p, "blur")
	backSlot := propStr(p, "backSlot")
	coverSlot := propStr(p, "coverSlot")
	toolbarSlot := propStr(p, "toolbarSlot")

	type heroMetaItem struct {
		Icon string
		Text string
	}
	var metaItems []heroMetaItem
	if rawMeta, ok := p["meta"].([]any); ok {
		for _, item := range rawMeta {
			switch v := item.(type) {
			case map[string]any:
				if txt := strings.TrimSpace(strVal(v["text"])); txt != "" {
					metaItems = append(metaItems, heroMetaItem{
						Icon: assets.SafeIcon(strVal(v["icon"]), ""),
						Text: txt,
					})
				}
			default:
				if txt := strings.TrimSpace(strVal(item)); txt != "" {
					metaItems = append(metaItems, heroMetaItem{Text: txt})
				}
			}
		}
	} else if s := strings.TrimSpace(propStr(p, "meta")); s != "" {
		metaItems = append(metaItems, heroMetaItem{Text: s})
	}

	var ratingScore float64
	var ratingTotal int
	var ratingPlayed int
	if rm, ok := p["rating"].(map[string]any); ok {
		ratingScore = float64(propFloat(rm, "score"))
		ratingTotal = int(propFloat(rm, "total"))
		ratingPlayed = int(propFloat(rm, "played"))
	}

	// 页面底色：用于底部平滑渐隐，让头图无缝融入页面正文（无边框、无硬截断）
	fadeBg := t.AppBg
	if t.Glass {
		fadeBg = t.AppBg.Alpha(0.88)
	}
	scrimDark := t.Pick(theme.Brand900, t.AppBg)

	hero := ui.Box(c.Key(key)).FillWidth().MinHeight(bannerH).Clip()
	hero.LinearGradient(ui.LinearGradient{
		Angle: 135,
		Stops: []ui.GradientStop{
			{Color: t.Pick(theme.Brand700, theme.Brand800), At: 0},
			{Color: fadeBg, At: 1},
		},
	})
	hero.Children(func() {
		// 层 -3：高清原始头图（默认不虚化，仅当显式 blur=true 时模糊）
		if bannerSrc != "" {
			if bmp := m.loadBitmap(bannerSrc); bmp != nil {
				img := ui.Image(c.Key(key+"-bg"), bmp).Absolute().Top(0).Right(0).Bottom(0).Left(0).Fit(ui.Cover).PassThrough().ZIndex(-3)
				if blur {
					img.Blur(20)
				}
			}
		}
		// 层 -2：左侧横向暗色可读性梯度，保证左半区文字与徽标清晰，同时右半区头图主体保持通透
		ui.Box(c).Absolute().Top(0).Right(0).Bottom(0).Left(0).PassThrough().ZIndex(-2).LinearGradient(ui.LinearGradient{
			Angle: 90,
			Stops: []ui.GradientStop{
				{Color: scrimDark.Alpha(0.84), At: 0},
				{Color: scrimDark.Alpha(0.56), At: 0.42},
				{Color: scrimDark.Alpha(0.18), At: 0.76},
				{Color: scrimDark.Alpha(0.06), At: 1},
			},
		})
		// 层 -1：纵向融底梯度，顶部轻微压暗，中段通透展示头图，底部平滑过渡到页面底色
		ui.Box(c).Absolute().Top(0).Right(0).Bottom(0).Left(0).PassThrough().ZIndex(-1).LinearGradient(ui.LinearGradient{
			Angle: 180,
			Stops: []ui.GradientStop{
				{Color: theme.Black.Alpha(0.26), At: 0},
				{Color: theme.Black.Alpha(0.06), At: 0.30},
				{Color: t.Pick(scrimDark.Alpha(0.58), fadeBg.Alpha(0.55)), At: 0.68},
				{Color: t.Pick(scrimDark.Alpha(0.80), fadeBg.Alpha(0.92)), At: 0.90},
				{Color: fadeBg, At: 1},
			},
		})

		// 顶左浮动返回按钮（ZIndex 2 确保始终可点且不遮挡主视觉）
		if backSlot != "" && natives != nil {
			if fn, ok := natives[backSlot]; ok && fn != nil {
				pillBg := t.Pick(theme.White.Alpha(0.78), theme.Black.Alpha(0.36))
				pillBorder := t.Pick(theme.White.Alpha(0.45), theme.White.Alpha(0.14))
				ui.Box(c.Key(key+"-back-pill")).Absolute().Top(16).Left(32).ZIndex(2).
					Padding(4, 12, 4, 8).Radius(999).
					Background(pillBg).Border(t.HL, pillBorder).Children(func() {
					fn(c, t)
				})
			}
		}

		// 前景内容（默认 ZIndex 0，稳稳绘制在背景与渐变层之上）
		ui.Column(c).FillWidth().MinHeight(bannerH).Justify(ui.End).Padding(68, 32, 24, 32).Gap(16).Children(func() {
			ui.Row(c).FillWidth().Gap(32).AlignItems(ui.End).Children(func() {
				// 左侧主信息区
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(10).Children(func() {
					// 透明游戏徽标（按自然宽高比放大展示，贴左对齐）
					hasLogo := false
					if logoSrc != "" {
						if imgSrc := m.LoadImageSource(logoSrc); imgSrc != nil {
							hasLogo = true
							const maxLogoW float32 = 460
							const maxLogoH float32 = 132
							w, h := float32(340), float32(104)
							if bmp, ok := imgSrc.(*ui.Bitmap); ok {
								bw, bh := bmp.Size()
								if bw > 0 && bh > 0 {
									scale := min(maxLogoW/float32(bw), maxLogoH/float32(bh))
									if scale > 1 {
										scale = 1
									}
									w = float32(bw) * scale
									h = float32(bh) * scale
								}
							}
							ui.Box(c.Key(key+"-logo")).Width(w).Height(h).Children(func() {
								ui.Image(c, imgSrc).Fill().Fit(ui.Contain)
							})
						}
					}

					// 有徽标时标题退居精致副行，避免与徽标视觉抢戏；无徽标时用大气主标题
					if hasLogo {
						subLine := title
						if subtitle != "" && subtitle != title {
							subLine = title + "  ·  " + subtitle
						}
						if subLine != "" {
							components.Text(c, subLine, theme.TextBase, theme.WeightMedium, theme.White.Alpha(0.86)).Selectable()
						}
					} else {
						if title != "" {
							components.Text(c, title, theme.FontStep{Size: 34, Line: 42}, theme.WeightBold, theme.White).Selectable()
						}
						if subtitle != "" && subtitle != title {
							components.Text(c, subtitle, theme.TextSM, theme.WeightNormal, theme.White.Alpha(0.80)).Selectable()
						}
					}

					// 带图标的清爽元信息行（零胶囊边框，复刻 Hikarinagi 风格）
					if len(metaItems) > 0 {
						ui.Row(c).Wrap().Gap(16).AlignItems(ui.Center).Children(func() {
							for i, item := range metaItems {
								ui.Row(c.Key(fmt.Sprintf("%s-meta-%d", key, i))).Gap(5).AlignItems(ui.Center).Children(func() {
									if item.Icon != "" {
										components.Icon(c, item.Icon, 14, theme.White.Alpha(0.72))
									}
									components.Text(c, item.Text, theme.TextXS, theme.WeightNormal, theme.White.Alpha(0.90)).NoWrap()
								})
							}
						})
					}

					// 评分与星级行（有评分时才展示）
					if ratingScore > 0 {
						ui.Row(c.Key(key+"-rating")).Gap(12).AlignItems(ui.Center).Margin(2, 0, 2, 0).Children(func() {
							components.Text(c, fmt.Sprintf("%.1f", ratingScore), theme.FontStep{Size: 34, Line: 38}, theme.WeightBold, theme.White).NoWrap()
							ui.Column(c).Gap(2).Children(func() {
								ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
									stars := ratingScore / 2.0
									for i := 1; i <= 5; i++ {
										iconName := "star-outline"
										col := theme.White.Alpha(0.35)
										if stars >= float64(i)-0.25 {
											iconName = "star"
											col = theme.Warning400
										} else if stars >= float64(i)-0.75 {
											iconName = "star-half-full"
											col = theme.Warning400
										}
										if iconName = assets.SafeIcon(iconName, "star-outline"); iconName != "" {
											components.Icon(c, iconName, 16, col)
										}
									}
								})
								var statParts []string
								if ratingTotal > 0 {
									statParts = append(statParts, fmt.Sprintf("%d 人评分", ratingTotal))
								}
								if ratingPlayed > 0 {
									statParts = append(statParts, fmt.Sprintf("%d 人玩过", ratingPlayed))
								}
								if len(statParts) > 0 {
									components.Text(c, strings.Join(statParts, "   "), theme.TextXS, theme.WeightNormal, theme.White.Alpha(0.75)).NoWrap()
								}
							})
						})
					}

					// 原生启动栏无缝融入 Hero 内部（无硬边框与分隔条）
					if toolbarSlot != "" && natives != nil {
						if fn, ok := natives[toolbarSlot]; ok && fn != nil {
							ui.Box(c.Key(key+"-toolbar-inline")).FillWidth().Margin(6, 0, 0, 0).Children(func() {
								fn(c, t)
							})
						}
					}
				})

				// 右侧悬浮竖版封面（复用原生封面查看器与 NSFW 模糊）
				if coverSlot != "" && natives != nil {
					if fn, ok := natives[coverSlot]; ok && fn != nil {
						ui.Box(c.Key(key+"-cover-frame")).Shrink(0).Radius(10).
							Shadow(0, 14, 32, 0, theme.Black.Alpha(0.45)).
							Border(t.HL, theme.White.Alpha(0.18)).Clip().Children(func() {
							fn(c, t)
						})
					}
				}
			})

			// 底部扩展内容（例如 Hikarinagi 风格的横向游戏截图缩略预览条）
			if len(children) > 0 {
				ui.Row(c.Key(key+"-extra")).FillWidth().Wrap().Gap(10).AlignItems(ui.Center).Margin(4, 0, 0, 0).Children(func() {
					for i, child := range children {
						m.renderNode(c, t, inst, child, fmt.Sprintf("%s-hc-%d", key, i), natives)
					}
				})
			}
		})
	})
	return hero
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
		return theme.Primary500
	case "accent":
		return t.Pick(theme.Primary600, theme.Primary400)
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
