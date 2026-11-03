package cloudbackup

import (
	"time"
	"unicode/utf16"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// minPasswordLength 是备份密码的最短长度（PasswordInputModal 的 password.length >= 6）。
const minPasswordLength = 6

// passwordModalState 是 PasswordInputModal 的 isOpen 与它自己的 password、confirmPassword、showPassword。
type passwordModalState struct {
	open     bool
	password string
	confirm  string
	show     bool
}

// jsLength 同 JavaScript 的 string.length（UTF-16 码元数）。
func jsLength(s string) int { return len(utf16.Encode([]rune(s))) }

// reset 对应 handleSubmit、handleClose 里清空输入与 showPassword。
func (m *passwordModalState) reset() { m.password, m.confirm, m.show = "", "", false }

// passwordModal 对应 components/modal/PasswordInputModal.tsx（只有云备份面板用它，所以放在这个包里）：
// ModalPortal 里 absolute inset-0 z-50 bg-black/50 backdrop-blur-sm p-4 的遮罩（没有 Esc 与点击遮罩关闭），
// w-full max-w-md rounded-xl bg-white p-6 shadow-xl dark:bg-brand-800 border border-brand-200 dark:border-brand-700 的面板。
// onConfirm 收到密码与确认密码。
func passwordModal(c *ui.Context, t *theme.Theme, m *passwordModalState, onConfirm func(password, confirm string)) {
	closeModal := func() {
		m.open = false
		m.reset()
	}
	match := m.password == m.confirm
	canSubmit := jsLength(m.password) >= minPasswordLength && match
	submit := func() {
		if m.password == "" || m.password != m.confirm {
			return
		}
		pw, confirm := m.password, m.confirm
		closeModal()
		onConfirm(pw, confirm)
	}
	components.Modal(c, t, "password-modal", components.ModalProps{Open: m.open, TopOffset: panel.TopOffset}, func(pn ui.Element) {
		components.ModalPanel(t, pn).Children(func() {
			passwordModalHeader(c, t)
			// space-y-4 mb-6
			ui.Column(c).FillWidth().Gap(16).Margin(0, 0, 24, 0).Children(func() {
				ui.Column(c.Key("password")).FillWidth().Children(func() {
					modalLabel(c, t, locales.SettingsPasswordModalPasswordLabel())
					// relative：右侧 absolute right-2 top-1/2 -translate-y-1/2 的显示密码按钮
					ui.Box(c).FillWidth().Children(func() {
						passwordInput(c, t, "input", &m.password, locales.SettingsPasswordModalPasswordLabel(),
							locales.SettingsPasswordModalPasswordPlaceholder(), !m.show, true)
						ui.Row(c).Absolute().Top(0).Bottom(0).Right(8).AlignItems(ui.Center).PassThrough().Children(func() {
							eyeButton(c, t, m)
						})
					})
					if m.password != "" && jsLength(m.password) < minPasswordLength {
						modalError(c, t, locales.SettingsPasswordModalPasswordTooShort())
					}
				})
				ui.Column(c.Key("confirm")).FillWidth().Children(func() {
					modalLabel(c, t, locales.SettingsPasswordModalConfirmLabel())
					passwordInput(c, t, "input", &m.confirm, locales.SettingsPasswordModalConfirmLabel(),
						locales.SettingsPasswordModalConfirmPlaceholder(), !m.show, false)
					if m.confirm != "" && !match {
						modalError(c, t, locales.SettingsPasswordModalPasswordMismatch())
					}
				})
			})
			// flex justify-end gap-3
			ui.Row(c).FillWidth().Gap(12).Justify(ui.End).Children(func() {
				components.ModalTextButton(c, t, "cancel", locales.CommonCancel()).OnClick(closeModal)
				confirmButton(c, t, canSubmit).OnClick(submit)
			})
		})
	})
}

// passwordModalHeader：flex items-start gap-4 mb-6；p-2 rounded-full bg-warning-100 text-warning-600 dark:bg-warning-900/30
// dark:text-warning-400 的 i-mdi-lock-plus text-2xl；标题 text-xl font-bold text-brand-900 dark:text-white mb-2，
// 说明 text-brand-600 dark:text-brand-400 text-sm leading-relaxed space-y-2，第二段 font-semibold text-warning-600
// dark:text-warning-400。
func passwordModalHeader(c *ui.Context, t *theme.Theme) {
	relaxed := theme.FontStep{Size: 14, Line: 22.75}
	desc := t.Pick(theme.Brand600, theme.Brand400)
	ui.Row(c).FillWidth().Gap(16).AlignItems(ui.Start).Margin(0, 0, 24, 0).Children(func() {
		ui.Box(c).Padding(8).Radius(999).Shrink(0).Background(t.Pick(theme.Warning100, theme.Warning900.Alpha(0.3))).Children(func() {
			components.Icon(c, "lock-plus", 24, t.Pick(theme.Warning600, theme.Warning400))
		})
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			components.TextMargin(c, locales.SettingsPasswordModalTitle(), theme.TextXL, theme.WeightBold, t.TextStrong, 0, 0, 8, 0)
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				components.Text(c, locales.SettingsPasswordModalDesc1(), relaxed, theme.WeightNormal, desc)
				components.Text(c, locales.SettingsPasswordModalDesc2(), relaxed, theme.WeightSemibold, t.Pick(theme.Warning600, theme.Warning400))
				components.Text(c, locales.SettingsPasswordModalDesc3(), relaxed, theme.WeightNormal, desc)
			})
		})
	})
}

// modalLabel：block text-sm font-medium text-brand-700 dark:text-brand-300 mb-2。
func modalLabel(c *ui.Context, t *theme.Theme, s string) {
	components.TextMargin(c, s, theme.TextSM, theme.WeightMedium, t.SectionLabel, 0, 0, 8, 0)
}

// modalError：text-xs text-error-600 dark:text-error-400 mt-1。
func modalError(c *ui.Context, t *theme.Theme, s string) {
	components.TextMargin(c, s, theme.TextXS, theme.WeightNormal, t.Pick(theme.Error600, theme.Error400), 4, 0, 0, 0)
}

// passwordInput：w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none
// focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white（第一个输入框另有 pr-10 与 autoFocus）。
// 输入框继承 16px/24px 的字号与行高。
func passwordInput(c *ui.Context, t *theme.Theme, key string, value *string, label, placeholder string, hidden, first bool) {
	in := ui.TextInputBase(c.Key(key), value).FillWidth().Radius(6).
		FontSize(theme.TextBase.Size).FixedLineHeight(theme.TextBase.Line).TextColor(t.TextStrong).
		Background(t.Pick(theme.White, theme.Brand700)).Border(t.HL, t.Pick(theme.Brand300, theme.Brand600)).
		Placeholder(placeholder).Label(label).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
	if theme.UIFont != "" {
		in.Font(theme.UIFont)
	}
	if first {
		in.Padding(8, 40, 8, 12).AutoFocus()
	} else {
		in.Padding(8, 12)
	}
	if hidden {
		in.Password()
	}
	if in.Focused() {
		in.Shadow(0, 0, 0, 2, theme.Neutral500)
	}
}

// eyeButton 是显示 / 隐藏密码的按钮：text-brand-500 hover:text-brand-700 dark:text-brand-400 dark:hover:text-brand-200，
// i-mdi-eye / i-mdi-eye-off text-xl。原版没有无障碍名称，这里用 native.settingsCloudBackupShowPassword / HidePassword。
func eyeButton(c *ui.Context, t *theme.Theme, m *passwordModalState) {
	icon, label := "eye", locales.NativeSettingsCloudBackupShowPassword()
	if m.show {
		icon, label = "eye-off", locales.NativeSettingsCloudBackupHidePassword()
	}
	btn := ui.ButtonBase(c.Key("toggle-show")).FocusRing(false).Label(label)
	color := t.Pick(theme.Brand500, theme.Brand400)
	if btn.Hovered() {
		color = t.Pick(theme.Brand700, theme.Brand200)
	}
	btn.OnClick(func() { m.show = !m.show }).Children(func() {
		components.Icon(c, icon, 20, color)
	})
}

// confirmButton：px-4 py-2 text-sm font-medium text-white rounded-lg transition-colors bg-brand-600 hover:bg-brand-700
// shadow-sm shadow-brand-200 dark:shadow-none disabled:opacity-50 disabled:cursor-not-allowed。
func confirmButton(c *ui.Context, t *theme.Theme, enabled bool) ui.Element {
	btn := ui.ButtonBase(c.Key("confirm")).Padding(8, 16).Radius(8).FocusRing(false)
	bg := theme.Brand600
	if btn.Hovered() {
		bg = theme.Brand700
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
	if !t.Dark {
		btn.Shadow(0, 1, 2, 0, theme.Brand200)
	}
	if !enabled {
		components.Disable(btn, 0.5)
		btn.Cursor(ui.CursorNotAllowed)
	}
	btn.Children(func() {
		components.Text(c, locales.CommonConfirm(), theme.TextSM, theme.WeightMedium, theme.White).NoWrap()
	})
	return btn
}
