package better

import (
	"slices"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// MetadataFieldSelectProps 是 MetadataFieldSelectModal 的属性。
type MetadataFieldSelectProps struct {
	Open        bool
	Title       string
	Description string
	ConfirmText string
	// Fields 是弹窗里勾选的字段（selectedFields），由调用方持有：打开弹窗时填入 InitialMetadataFields(initialFields)
	// （原版在 isOpen 变为 true 时重置），勾选与全选直接改它。
	Fields     *[]enums.MetadataUpdateField
	Submitting bool // isSubmitting：确认按钮禁用并显示旋转图标
	TopOffset  float32
	OnClose    func()
	OnConfirm  func(fields []enums.MetadataUpdateField)
}

// InitialMetadataFields 是 initialSelectedFields：initialFields 为空时用 DEFAULT_METADATA_UPDATE_FIELDS。返回副本。
func InitialMetadataFields(initial []enums.MetadataUpdateField) []enums.MetadataUpdateField {
	if len(initial) == 0 {
		return slices.Clone(enums.DefaultMetadataUpdateFields)
	}
	return slices.Clone(initial)
}

// metadataFieldText 是 t(`metadataUpdateFields.${field}.label`) 与 t(`metadataUpdateFields.${field}.hint`)。
func metadataFieldText(f enums.MetadataUpdateField) [2]string {
	return [2]string{
		locales.Key("metadataUpdateFields." + string(f) + ".label"),
		locales.Key("metadataUpdateFields." + string(f) + ".hint"),
	}
}

// MetadataFieldSelectModal 对应 components/modal/MetadataFieldSelectModal.tsx：max-w-2xl rounded-xl border p-6 shadow-xl，
// 标题与说明、右上角关闭；“更新全部字段”开关；sm:grid-cols-2 gap-3 的字段开关；取消与确认（cloud-sync 图标）。
// 详情页编辑（单个游戏）与设置页的批量更新元数据共用。原版是 components/modal 下的弹窗，放在这里是因为它由
// Better 系列控件组成（components 包不能引用 better）。
func MetadataFieldSelectModal(c *ui.Context, t *theme.Theme, key string, p MetadataFieldSelectProps) {
	all := enums.DefaultMetadataUpdateFields
	closeModal := func() {
		if p.OnClose != nil {
			p.OnClose()
		}
	}
	components.Modal(c, t, key, components.ModalProps{Open: p.Open, TopOffset: p.TopOffset, MaxWidth: 672}, func(pnl ui.Element) {
		fields := *p.Fields
		canConfirm := len(fields) > 0 && !p.Submitting
		components.ModalPanel(t, pnl).Children(func() {
			// flex items-start justify-between gap-4
			ui.Row(c).Gap(16).AlignItems(ui.Start).Justify(ui.SpaceBetween).Children(func() {
				// min-w-0 space-y-2：text-xl font-bold 的标题与 text-sm leading-relaxed 的说明
				ui.Column(c).MinWidth(0).Gap(8).Children(func() {
					components.Text(c, p.Title, theme.TextXL, theme.WeightBold, t.TextStrong)
					components.Text(c, p.Description, theme.FontStep{Size: 14, Line: 22.75}, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand400))
				})
				fieldModalCloseButton(c, t, "close", locales.CommonClose(), closeModal)
			})
			// mt-5 rounded-lg border border-brand-200 bg-brand-50 p-3 dark:border-brand-700 dark:bg-brand-700/40
			ui.Row(c.Key("all")).Margin(20, 0, 0, 0).Gap(16).Padding(12).Radius(8).AlignItems(ui.Center).Justify(ui.SpaceBetween).
				Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).Background(t.Pick(theme.Brand50, theme.Brand700.Alpha(0.4))).
				Children(func() {
					ui.Column(c).Gap(4).Children(func() {
						components.Text(c, locales.MetadataUpdateFieldsSelectAll(), theme.TextSM, theme.WeightSemibold, t.Pick(theme.Brand800, theme.Brand200))
						components.Text(c, locales.MetadataUpdateFieldsSelectedCount(strconv.Itoa(len(fields)), strconv.Itoa(len(all))), theme.TextXS,
							theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
					})
					Switch(c, t, "switch", len(fields) == len(all), false, func(on bool) {
						*p.Fields = nil
						if on {
							*p.Fields = slices.Clone(all)
						}
					})
				})
			// mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2
			cols := 1
			if winW, _ := c.Size(); winW >= 640 {
				cols = 2
			}
			ui.Grid(c.Key("fields")).Columns(cols).GapX(12).GapY(12).Margin(16, 0, 0, 0).Children(func() {
				for _, f := range all {
					metadataFieldToggle(c.Key(string(f)), t, metadataFieldText(f), slices.Contains(fields, f), func(on bool) {
						next := slices.DeleteFunc(slices.Clone(*p.Fields), func(x enums.MetadataUpdateField) bool { return x == f })
						if on {
							next = append(next, f)
						}
						*p.Fields = next
					})
				}
			})
			if len(fields) == 0 {
				// mt-3 text-xs text-error-600 dark:text-error-400
				components.TextMargin(c, locales.MetadataUpdateFieldsEmptyWarning(), theme.TextXS, theme.WeightNormal,
					t.Pick(theme.Error600, theme.Error400), 12, 0, 0, 0)
			}
			// mt-8 flex justify-end gap-3
			ui.Row(c).Margin(32, 0, 0, 0).Gap(12).Justify(ui.End).Children(func() {
				Button(c, t, "cancel", ButtonProps{Label: locales.CommonCancel()}).OnClick(closeModal)
				Button(c, t, "confirm", ButtonProps{
					Variant: ButtonPrimary, Icon: "cloud-sync", Label: p.ConfirmText, Disabled: !canConfirm, Loading: p.Submitting,
				}).OnClick(func() {
					if canConfirm && p.OnConfirm != nil {
						p.OnConfirm(slices.Clone(fields))
					}
				})
			})
		})
	})
}

// metadataFieldToggle：flex min-w-0 items-center justify-between gap-3 rounded-lg border p-3，左侧 label（点击也切换）是 text-sm
// font-medium 的名称与 text-xs leading-relaxed 的说明（space-y-1）。
func metadataFieldToggle(c *ui.Context, t *theme.Theme, text [2]string, checked bool, onChange func(bool)) {
	ui.Row(c).MinWidth(0).Gap(12).Padding(12).Radius(8).AlignItems(ui.Center).Justify(ui.SpaceBetween).
		Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
		lb := ui.ButtonBase(c.Key("label")).Column().Grow(1).Basis(0).MinWidth(0).Gap(4).AlignItems(ui.Start).
			FocusRing(false).Label(text[0])
		lb.OnClick(func() { onChange(!checked) }).Children(func() {
			components.Text(c, text[0], theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand800, theme.Brand200))
			components.Text(c, text[1], theme.FontStep{Size: 12, Line: 19.5}, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
		})
		Switch(c, t, "switch", checked, false, onChange)
	})
}

// fieldModalCloseButton 是右上角的关闭按钮：rounded-lg p-2 text-brand-500 hover:bg-brand-100 hover:text-brand-800
// dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-brand-100，i-mdi-close text-xl。
func fieldModalCloseButton(c *ui.Context, t *theme.Theme, key, label string, onClick func()) {
	btn := ui.ButtonBase(c.Key(key)).Padding(8).Radius(8).Center().Shrink(0).FocusRing(false).Label(label)
	color := t.Pick(theme.Brand500, theme.Brand400)
	if btn.Hovered() {
		btn.Background(t.Pick(theme.Brand100, theme.Brand700))
		color = t.Pick(theme.Brand800, theme.Brand100)
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond))
	btn.OnClick(onClick).Children(func() { components.Icon(c, "close", 20, color) })
}
