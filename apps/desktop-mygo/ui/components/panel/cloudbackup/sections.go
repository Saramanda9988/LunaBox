package cloudbackup

import (
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/components/panel/backup"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// monoFont 是 font-mono 在 Windows 的 Chromium 里实际生效的字体（ui-monospace、SFMono-Regular、Menlo、Monaco 都没有）。
const monoFont = "Consolas"

// oneDriveDocsURL 是 Client ID 提示里的文档链接。
const oneDriveDocsURL = "https://box.lunarain.site/configuration/onedrive-cloud-backup.html"

// hintColor：text-brand-500 dark:text-brand-400。
func hintColor(t *theme.Theme) ui.Color { return t.Pick(theme.Brand500, theme.Brand400) }

// warningHint：text-xs text-warning-600 dark:text-warning-400。
func warningHint(c *ui.Context, t *theme.Theme, s string) {
	components.Text(c, s, theme.TextXS, theme.WeightNormal, t.Pick(theme.Warning600, theme.Warning400))
}

// backupPassword 是“备份密码”（S3、OneDrive、WebDAV 才有）：space-y-2 的标题，已设置时是遮住的密码与用户 ID，
// 否则是设置按钮与两行提示。
func (s *State) backupPassword(c *ui.Context, t *theme.Theme, p panel.Settings) {
	ui.Column(c.Key("backup-password")).FillWidth().Gap(8).Children(func() {
		panel.FieldLabel(c, t, locales.SettingsCloudBackupBackupPasswordLabel())
		if id := p.Config.BackupUserID; id != "" {
			ui.Column(c.Key("set")).FillWidth().Gap(8).Children(func() {
				// glass-panel rounded-md border border-brand-300 px-3 py-2 text-brand-600 dark:border-brand-600 dark:text-brand-300
				bg, border := t.GlassPanel(ui.Transparent, t.Pick(theme.Brand300, theme.Brand600))
				ui.Row(c).FillWidth().Padding(8, 12).Radius(6).Background(bg).Border(t.HL, border).Children(func() {
					components.Text(c, "********", theme.TextBase, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand300)) // i18n:keep：遮住的密码
				})
				// text-xs 里的 span text-success-600 dark:text-success-400：“✓ 密码已设置，用户ID: ” + 前 8 位 + "..."
				components.Text(c, locales.SettingsCloudBackupPasswordSet()+" "+prefix(id, 8)+"...", theme.TextXS, theme.WeightNormal,
					t.Pick(theme.Success600, theme.Success400))
			})
			return
		}
		ui.Column(c.Key("unset")).FillWidth().Gap(8).Children(func() {
			setPasswordButton(c, t).OnClick(func() { s.modal.open = true })
			components.Text(c, locales.SettingsCloudBackupPasswordForIdHint(), theme.TextXS, theme.WeightNormal, hintColor(t))
			warningHint(c, t, locales.SettingsCloudBackupPasswordWarning())
		})
	})
}

// setPasswordButton：glass-btn-neutral flex w-full items-center justify-center gap-2 rounded-md bg-brand-600 px-4 py-2
// text-white transition-colors hover:bg-brand-700，i-mdi-lock-plus text-lg。
func setPasswordButton(c *ui.Context, t *theme.Theme) ui.Element {
	btn := ui.ButtonBase(c.Key("set-password")).FillWidth().Padding(8, 16).Radius(6).Gap(8).Center().FocusRing(false)
	g := glassButton(t, btn, theme.BtnColors{Bg: theme.Brand600, Hover: theme.Brand700, Text: theme.White}, false)
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
		components.Icon(c, "lock-plus", 18, g.Text)
		components.Text(c, locales.SettingsCloudBackupSetPasswordBtn(), theme.TextBase, theme.WeightNormal, g.Text).NoWrap()
	})
	return btn
}

// card：glass-card space-y-4 rounded-lg bg-brand-100 p-4 dark:bg-brand-800（各提供商的配置卡片），第一项是
// text-sm font-semibold 的标题。
func card(c *ui.Context, t *theme.Theme, key, title string, content func()) {
	bg, border := t.GlassCard(t.Pick(theme.Brand100, theme.Brand800), ui.Transparent)
	e := ui.Column(c.Key(key)).FillWidth().Gap(16).Padding(16).Radius(8).Background(bg)
	if t.Glass {
		e.Border(t.HL, border)
	}
	e.Children(func() {
		panel.GroupTitle(c, t, title)
		content()
	})
}

// textField 是 space-y-2 的 label 与 BetterInput；below 在输入框下面加提示。
func textField(c *ui.Context, t *theme.Theme, key, label, value string, ip better.InputProps, mono bool, onChange func(v string), below func()) {
	ui.Column(c.Key(key)).FillWidth().Gap(8).Children(func() {
		panel.FieldLabel(c, t, label)
		if ip.Label == "" {
			ip.Label = label
		}
		v := value
		in := better.Input(c, t, "input", &v, ip)
		if mono {
			in.Font(monoFont)
		}
		if in.Changed() {
			onChange(v)
		}
		if below != nil {
			below()
		}
	})
}

// twoColumns：grid grid-cols-2 gap-4。
func twoColumns(c *ui.Context, left, right func()) {
	ui.Row(c).FillWidth().Gap(16).AlignItems(ui.Start).Children(func() {
		ui.Column(c.Key("left")).Grow(1).Basis(0).MinWidth(0).Children(left)
		ui.Column(c.Key("right")).Grow(1).Basis(0).MinWidth(0).Children(right)
	})
}

// set 返回改草稿里一个字段的 onChange（handleChange 的默认分支）。
func set(p panel.Settings, field func(cfg *appconf.AppConfig) *string) func(string) {
	return func(v string) { p.Change(func(cfg *appconf.AppConfig) { *field(cfg) = v }) }
}

// testButton 是卡片右下角的“测试连接”（flex justify-end），测试中显示“测试中...”并禁用。
func testButton(c *ui.Context, t *theme.Theme, busy bool, onClick func()) {
	ui.Row(c.Key("test-row")).FillWidth().Justify(ui.End).Children(func() {
		label := locales.SettingsCloudBackupTestConnection()
		if busy {
			label = locales.SettingsCloudBackupTesting()
		}
		secondaryButton(c, t, "test", label, "", false, busy).OnClick(onClick)
	})
}

// s3Card 是 S3 配置：Endpoint、Region 与 Bucket（两列）、Access Key、Secret Key 与测试连接。
func (s *State) s3Card(c *ui.Context, t *theme.Theme, p panel.Settings) {
	cfg := p.Config
	card(c, t, "s3", locales.SettingsCloudBackupS3Section(), func() {
		textField(c, t, "s3-endpoint", "S3 Endpoint", cfg.S3Endpoint, better.InputProps{Placeholder: "https://s3.example.com"}, false, // i18n:keep：原版写死的技术名词与示例
			set(p, func(cfg *appconf.AppConfig) *string { return &cfg.S3Endpoint }), nil)
		twoColumns(c, func() {
			textField(c, t, "s3-region", "Region", cfg.S3Region, better.InputProps{Placeholder: "us-east-1"}, false, // i18n:keep：原版写死的技术名词与示例
				set(p, func(cfg *appconf.AppConfig) *string { return &cfg.S3Region }), nil)
		}, func() {
			textField(c, t, "s3-bucket", "Bucket", cfg.S3Bucket, better.InputProps{Placeholder: "lunabox-backup"}, false, // i18n:keep：原版写死的技术名词与示例
				set(p, func(cfg *appconf.AppConfig) *string { return &cfg.S3Bucket }), nil)
		})
		textField(c, t, "s3-access-key", "Access Key", cfg.S3AccessKey, better.InputProps{}, false, // i18n:keep：原版写死的技术名词
			set(p, func(cfg *appconf.AppConfig) *string { return &cfg.S3AccessKey }), nil)
		textField(c, t, "s3-secret-key", "Secret Key", cfg.S3SecretKey, better.InputProps{Password: true}, false, // i18n:keep：原版写死的技术名词
			set(p, func(cfg *appconf.AppConfig) *string { return &cfg.S3SecretKey }), nil)
		testButton(c, t, s.testingS3, func() {
			s.testConnection(p, &s.testingS3, p.Env.Svc.Backup.TestS3Connection,
				locales.SettingsCloudBackupToastS3TestSuccess, locales.SettingsCloudBackupToastS3TestFailed)
		})
	})
}

// webdavCard 是 WebDAV 配置：URL 与提示、用户名与密码（两列）、测试连接。
func (s *State) webdavCard(c *ui.Context, t *theme.Theme, p panel.Settings) {
	cfg := p.Config
	card(c, t, "webdav", locales.SettingsCloudBackupWebdavSection(), func() {
		textField(c, t, "webdav-url", "URL", cfg.WebDAVURL, better.InputProps{Placeholder: "https://dav.example.com/remote.php/dav/files/user"}, false, // i18n:keep：原版写死的技术名词与示例
			set(p, func(cfg *appconf.AppConfig) *string { return &cfg.WebDAVURL }), func() {
				components.Text(c, locales.SettingsCloudBackupWebdavUrlHint(), theme.TextXS, theme.WeightNormal, hintColor(t))
			})
		twoColumns(c, func() {
			textField(c, t, "webdav-username", locales.SettingsCloudBackupWebdavUsername(), cfg.WebDAVUsername, better.InputProps{}, false,
				set(p, func(cfg *appconf.AppConfig) *string { return &cfg.WebDAVUsername }), nil)
		}, func() {
			textField(c, t, "webdav-password", locales.SettingsCloudBackupWebdavPassword(), cfg.WebDAVPassword, better.InputProps{Password: true}, false,
				set(p, func(cfg *appconf.AppConfig) *string { return &cfg.WebDAVPassword }), nil)
		})
		testButton(c, t, s.testingWebDAV, func() {
			s.testConnection(p, &s.testingWebDAV, p.Env.Svc.Backup.TestWebDAVConnection,
				locales.SettingsCloudBackupToastWebdavTestSuccess, locales.SettingsCloudBackupToastWebdavTestFailed)
		})
	})
}

// oneDriveCard 是 OneDrive 配置：注意事项、Client ID、授权状态，已授权时还有测试连接。
func (s *State) oneDriveCard(c *ui.Context, t *theme.Theme, p panel.Settings) {
	cfg := p.Config
	card(c, t, "onedrive", locales.SettingsCloudBackupOneDriveSection(), func() {
		oneDriveNote(c, t)
		// handleChange 的 onedrive_client_id 分支：Client ID（去掉首尾空白后）变了时清除旧授权。
		textField(c, t, "onedrive-client-id", "Client ID", cfg.OneDriveClientID, better.InputProps{Placeholder: "your-app-client-id"}, true, // i18n:keep：原版写死的技术名词与示例
			func(v string) {
				prev := strings.TrimSpace(p.Config.OneDriveClientID)
				p.Change(func(cfg *appconf.AppConfig) {
					cfg.OneDriveClientID = v
					if prev != strings.TrimSpace(v) {
						cfg.OneDriveRefreshToken = ""
					}
				})
			}, func() {
				clientIDHint(c, t)
				warningHint(c, t, locales.SettingsCloudBackupClientIdChangeHint())
			})
		ui.Column(c.Key("auth")).FillWidth().Gap(8).Children(func() {
			panel.FieldLabel(c, t, locales.SettingsCloudBackupAuthStatusLabel())
			if cfg.OneDriveRefreshToken != "" {
				// flex items-center gap-2：flex items-center gap-1 text-success-600 dark:text-success-400 的“已授权”与取消授权
				ui.Row(c.Key("authorized")).Gap(8).AlignItems(ui.Center).Children(func() {
					green := t.Pick(theme.Success600, theme.Success400)
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						components.Icon(c, "check-circle", 18, green)
						components.Text(c, locales.SettingsCloudBackupAuthorized(), theme.TextBase, theme.WeightNormal, green).NoWrap()
					})
					revokeOneDriveButton(c, t).OnClick(func() {
						p.Change(func(cfg *appconf.AppConfig) { cfg.OneDriveRefreshToken = "" })
					})
				})
				return
			}
			ui.Column(c.Key("unauthorized")).FillWidth().Gap(12).Children(func() {
				hasClientID := strings.TrimSpace(cfg.OneDriveClientID) != ""
				label, icon := locales.SettingsCloudBackupAuthOneDriveBtn(), "microsoft"
				if s.authorizingOneDrive {
					label = locales.SettingsCloudBackupWaitingAuth()
				}
				darkButton(c, t, "auth-onedrive", label, icon, s.authorizingOneDrive, s.authorizingOneDrive || !hasClientID).
					OnClick(func() { s.oneDriveAuth(p) })
				if s.authorizingOneDrive {
					components.Text(c, locales.SettingsCloudBackupAuthWaitHint(), theme.TextXS, theme.WeightNormal, hintColor(t))
				}
			})
		})
		if cfg.OneDriveRefreshToken != "" {
			testButton(c, t, s.testingOneDrive, func() {
				s.testConnection(p, &s.testingOneDrive, p.Env.Svc.Backup.TestOneDriveConnection,
					locales.SettingsCloudBackupToastOneDriveTestSuccess, locales.SettingsCloudBackupToastOneDriveTestFailed)
			})
		}
	})
}

// oneDriveNote 是注意事项：rounded-md border border-brand-300 bg-brand-100 p-3 dark:border-brand-600 dark:bg-brand-700，
// flex items-start gap-2；i-mdi-information-outline mt-0.5 text-lg text-warning-500 dark:text-brand-400；
// space-y-1 text-xs text-brand-600 dark:text-brand-400 的 font-medium 标题与 list-disc list-inside space-y-0.5 pl-2 的列表。
func oneDriveNote(c *ui.Context, t *theme.Theme) {
	ui.Row(c.Key("note")).FillWidth().Padding(12).Radius(6).Gap(8).AlignItems(ui.Start).
		Border(t.HL, t.Pick(theme.Brand300, theme.Brand600)).Background(t.Pick(theme.Brand100, theme.Brand700)).Children(func() {
		components.Icon(c, "information-outline", 18, t.Pick(theme.Warning500, theme.Brand400)).Margin(2, 0, 0, 0)
		color := t.Pick(theme.Brand600, theme.Brand400)
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4).Children(func() {
			components.Text(c, locales.SettingsCloudBackupOneDriveNote(), theme.TextXS, theme.WeightMedium, color)
			ui.Column(c).FillWidth().Padding(0, 0, 0, 8).Gap(2).Children(func() {
				for _, item := range []func() string{
					locales.SettingsCloudBackupOneDriveNoteItem1, locales.SettingsCloudBackupOneDriveNoteItem2,
					locales.SettingsCloudBackupOneDriveNoteItem3, locales.SettingsCloudBackupOneDriveNoteItem4,
					locales.SettingsCloudBackupOneDriveNoteItem5,
				} {
					// list-inside 的圆点与文字在同一行里，折行的文字从圆点下面开始。
					components.Text(c, "• "+item(), theme.TextXS, theme.WeightNormal, color) // i18n:keep：列表圆点
				}
			})
		})
	})
}

// clientIDHint：text-xs text-brand-500 dark:text-brand-400 的一段话，中间是 underline hover:text-brand-600
// dark:hover:text-brand-300 的文档链接（在浏览器里打开）。
func clientIDHint(c *ui.Context, t *theme.Theme) {
	color := hintColor(t)
	hintText := locales.SettingsCloudBackupClientIdHint() + " " + locales.SettingsCloudBackupClientIdHintLink() + locales.SettingsCloudBackupClientIdHint2()
	d := components.LineOverflow(ui.Span{Text: hintText, Font: theme.UIFont, Size: theme.TextXS.Size, Weight: theme.WeightNormal}, theme.TextXS.Line)
	rt := ui.RichText(c.Key("client-id-hint")).FontSize(theme.TextXS.Size).FixedLineHeight(theme.TextXS.Line).
		FontWeight(theme.WeightNormal).TextColor(color).Margin(-d, 0, -d, 0)
	if theme.UIFont != "" {
		rt.Font(theme.UIFont)
	}
	rt.Children(func() {
		ui.Text(c, locales.SettingsCloudBackupClientIdHint()+" ")
		link := ui.Link(c, locales.SettingsCloudBackupClientIdHintLink(), oneDriveDocsURL).Underline()
		linkColor := color
		if link.Hovered() {
			linkColor = t.Pick(theme.Brand600, theme.Brand300)
		}
		link.TextColor(linkColor)
		ui.Text(c, locales.SettingsCloudBackupClientIdHint2())
	})
}

// revokeOneDriveButton：rounded px-2 py-1 text-xs text-error-600 hover:bg-error-100 dark:hover:bg-error-900。
func revokeOneDriveButton(c *ui.Context, t *theme.Theme) ui.Element {
	btn := ui.ButtonBase(c.Key("revoke-onedrive")).Padding(4, 8).Radius(4).FocusRing(false)
	if btn.Hovered() {
		btn.Background(t.Pick(theme.Error100, theme.Error900))
	}
	btn.Children(func() {
		components.Text(c, locales.SettingsCloudBackupRevokeAuth(), theme.TextXS, theme.WeightNormal, theme.Error600).NoWrap()
	})
	return btn
}

// umbraCard 是 Umbra 配置：授权状态（已授权时是用户信息卡片、错误提示与操作按钮），以及高级选项里的服务地址。
func (s *State) umbraCard(c *ui.Context, t *theme.Theme, p panel.Settings) {
	cfg := p.Config
	card(c, t, "umbra", locales.SettingsCloudBackupUmbraSection(), func() {
		ui.Column(c.Key("auth")).FillWidth().Gap(8).Children(func() {
			panel.FieldLabel(c, t, locales.SettingsCloudBackupAuthStatusLabel())
			if cfg.UmbraAuthenticated {
				s.umbraAuthorized(c, t, p)
			} else {
				s.umbraUnauthorized(c, t, p)
			}
		})
		s.advancedOptions(c, t, p)
	})
}

// umbraAuthorized 是已授权的 Umbra：space-y-3 的用户信息卡片（加载中或已读到时）、读取失败提示与
// flex flex-wrap justify-end gap-2 的刷新、测试连接、取消授权。
func (s *State) umbraAuthorized(c *ui.Context, t *theme.Theme, p panel.Settings) {
	ui.Column(c.Key("authorized")).FillWidth().Gap(12).Children(func() {
		if s.loadingProfile || s.profile != nil {
			s.profileCard(c, t)
		}
		if s.profileError != "" {
			s.profileErrorBox(c, t, p)
		}
		ui.Row(c.Key("actions")).FillWidth().Wrap().Justify(ui.End).Gap(8).Children(func() {
			secondaryButton(c, t, "refresh", locales.SettingsCloudBackupRefresh(), "refresh", s.loadingProfile, s.loadingProfile).
				OnClick(func() { s.loadProfile(p.Env, *p.Config) })
			label := locales.SettingsCloudBackupTestConnection()
			if s.testingUmbra {
				label = locales.SettingsCloudBackupTesting()
			}
			secondaryButton(c, t, "test", label, "", false, s.testingUmbra).OnClick(func() {
				s.testConnection(p, &s.testingUmbra, p.Env.Svc.Backup.TestUmbraConnection,
					locales.SettingsCloudBackupToastUmbraTestSuccess, locales.SettingsCloudBackupToastUmbraTestFailed)
			})
			label = locales.SettingsCloudBackupRevokeAuth()
			if s.revokingUmbra {
				label = locales.SettingsCloudBackupWaitingAuth()
			}
			dangerTextButton(c, t, "revoke", label, s.revokingUmbra).OnClick(func() { s.umbraLogout(p) })
		})
	})
}

// umbraUnauthorized 是未授权的 Umbra：space-y-3 的 flex items-center gap-2 授权按钮（等待中另有取消等待），
// 等待中再加一行提示。
func (s *State) umbraUnauthorized(c *ui.Context, t *theme.Theme, p panel.Settings) {
	ui.Column(c.Key("unauthorized")).FillWidth().Gap(12).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			label := locales.SettingsCloudBackupAuthUmbraBtn()
			if s.authorizingUmbra {
				label = locales.SettingsCloudBackupWaitingAuth()
			}
			darkButton(c, t, "auth-umbra", label, "shield-key-outline", s.authorizingUmbra, s.authorizingUmbra).
				OnClick(func() { s.umbraAuth(p) })
			if s.authorizingUmbra {
				dangerTextButton(c, t, "cancel-auth", locales.SettingsCloudBackupCancelAuth(), s.cancellingUmbra).
					OnClick(func() { s.cancelUmbraAuth(p) })
			}
		})
		if s.authorizingUmbra {
			components.Text(c, locales.SettingsCloudBackupAuthWaitHint(), theme.TextXS, theme.WeightNormal, hintColor(t))
		}
	})
}

// profileCard：glass-panel overflow-hidden rounded-lg border border-brand-200 dark:border-brand-700。还没读到时是
// flex items-center gap-3 px-4 py-5 text-sm 的加载提示；读到后是头像、用户名与“已授权”徽标一行，
// 下面 border-t px-4 py-3 space-y-2 的存储空间（用量、进度条与剩余）。
func (s *State) profileCard(c *ui.Context, t *theme.Theme) {
	bg, border := t.GlassPanel(ui.Transparent, t.Pick(theme.Brand200, theme.Brand700)) // glass-panel
	ui.Column(c.Key("profile")).FillWidth().Radius(8).Clip().Background(bg).Border(t.HL, border).Children(func() {
		pr := s.profile
		if pr == nil {
			ui.Row(c.Key("loading")).FillWidth().Padding(20, 16).Gap(12).AlignItems(ui.Center).Children(func() {
				components.Spinner(c, 20, hintColor(t))
				components.Text(c, locales.SettingsCloudBackupUmbraProfileLoading(), theme.TextSM, theme.WeightNormal, hintColor(t))
			})
			return
		}
		ui.Row(c.Key("user")).FillWidth().Padding(12, 16).Gap(12).AlignItems(ui.Center).Children(func() {
			avatar(c, pr.Username)
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
				components.Text(c, pr.Username, theme.TextSM, theme.WeightSemibold, t.Pick(theme.Brand800, theme.Brand200)).SingleLine()
			})
			authorizedBadge(c, t)
		})
		ui.Column(c.Key("storage")).FillWidth().Padding(12, 16).Gap(8).
			BorderWidth(t.HL, 0, 0, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				components.Text(c, locales.SettingsCloudBackupUmbraStorage(), theme.TextXS, theme.WeightMedium, t.SectionLabel).NoWrap()
				components.Text(c, fileSize(pr.UsedBytes)+" / "+fileSize(pr.QuotaBytes), theme.TextXS, theme.WeightNormal, hintColor(t)).NoWrap()
			})
			storageBar(c, t, storagePercent(pr.UsedBytes, pr.QuotaBytes))
			ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
				components.Text(c, locales.SettingsCloudBackupUmbraStorageAvailable(fileSize(pr.AvailableBytes)), theme.TextXS,
					theme.WeightNormal, hintColor(t))
			})
		})
	})
}

// fileSize 是 utils/size.ts 的 formatFileSize（与备份面板共用 backup.FormatFileSize）。
func fileSize(v uint64) string { return backup.FormatFileSize(int64(min(v, math.MaxInt64))) }

// storagePercent 同 umbraStoragePercent：已用 / 配额，最多 100。
func storagePercent(used, quota uint64) float32 {
	if quota == 0 {
		return 0
	}
	return float32(min(100, float64(used)/float64(quota)*100))
}

// avatar：flex h-10 w-10 flex-none items-center justify-center rounded-full bg-neutral-600 text-lg font-semibold
// text-white shadow-sm，用户名的第一个字（大写），没有时是 i-mdi-account。
func avatar(c *ui.Context, username string) {
	ui.Box(c.Key("avatar")).Size(40, 40).Shrink(0).Radius(999).Center().Background(theme.Neutral600).
		Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05)).Children(func() {
		name := strings.TrimSpace(username)
		if r, _ := utf8.DecodeRuneInString(name); name != "" {
			components.Text(c, string(unicode.ToUpper(r)), theme.TextLG, theme.WeightSemibold, theme.White)
			return
		}
		components.Icon(c, "account", 18, theme.White)
	})
}

// authorizedBadge：flex flex-none items-center gap-1 rounded-full bg-success-100 px-2 py-1 text-xs font-medium
// text-success-700 dark:bg-success-900 dark:text-success-300，i-mdi-check-decagram。
func authorizedBadge(c *ui.Context, t *theme.Theme) {
	fg := t.Pick(theme.Success700, theme.Success300)
	ui.Row(c.Key("badge")).Shrink(0).Padding(4, 8).Gap(4).Radius(999).AlignItems(ui.Center).
		Background(t.Pick(theme.Success100, theme.Success900)).Children(func() {
		components.Icon(c, "check-decagram", 12, fg)
		components.Text(c, locales.SettingsCloudBackupAuthorized(), theme.TextXS, theme.WeightMedium, fg).NoWrap()
	})
}

// storageBar：role="progressbar" 的 h-1.5 overflow-hidden rounded-full bg-brand-200 dark:bg-brand-700，里面
// h-full rounded-full bg-neutral-600 dark:bg-neutral-400 transition-[width] duration-300 的已用部分。
func storageBar(c *ui.Context, t *theme.Theme, percent float32) {
	bar := ui.Row(c.Key("storage-bar")).FillWidth().Height(6).Radius(999).Clip().Background(t.Pick(theme.Brand200, theme.Brand700)).
		Role(ui.RoleProgress).Label(locales.SettingsCloudBackupUmbraStorage()).Range(0, 100, math.Round(float64(percent)))
	w := bar.AnimateWith("width", percent, 300*time.Millisecond, theme.EaseTailwind)
	bar.Children(func() {
		ui.Box(c.Key("fill")).WidthPercent(w).Height(6).Shrink(0).Radius(999).Background(t.Pick(theme.Neutral600, theme.Neutral400))
	})
}

// profileErrorBox：flex items-start justify-between gap-3 rounded-md border border-error-200 bg-error-50 px-3 py-2 text-xs
// text-error-700 dark:border-error-800 dark:bg-error-900 dark:text-error-300；左侧 i-mdi-alert-circle-outline mt-0.5 与
// 错误，右侧 flex-none rounded px-2 py-1 font-medium hover:bg-error-100 disabled:opacity-50 dark:hover:bg-error-800 的重试。
func (s *State) profileErrorBox(c *ui.Context, t *theme.Theme, p panel.Settings) {
	fg := t.Pick(theme.Error700, theme.Error300)
	ui.Row(c.Key("profile-error")).FillWidth().Padding(8, 12).Gap(12).Radius(6).AlignItems(ui.Start).Justify(ui.SpaceBetween).
		Border(t.HL, t.Pick(theme.Error200, theme.Error800)).
		Background(t.IfGlass(t.Pick(theme.Error50, theme.Error900), theme.Error900.Alpha(0.2), theme.Error900.Alpha(0.2))).Children(func() { // data-glass:bg-error-900/20
		ui.Row(c).Shrink(1).MinWidth(0).Gap(8).AlignItems(ui.Start).Children(func() {
			components.Icon(c, "alert-circle-outline", 12, fg).Margin(2, 0, 0, 0)
			ui.Column(c).Shrink(1).MinWidth(0).Children(func() {
				components.Text(c, locales.SettingsCloudBackupUmbraProfileLoadFailed(s.profileError), theme.TextXS, theme.WeightNormal, fg)
			})
		})
		btn := ui.ButtonBase(c.Key("retry")).Shrink(0).Padding(4, 8).Radius(4).FocusRing(false)
		if btn.Hovered() && !s.loadingProfile {
			btn.Background(t.Pick(theme.Error100, theme.Error800))
		}
		if s.loadingProfile {
			components.Disable(btn, 0.5)
		}
		btn.OnClick(func() { s.loadProfile(p.Env, *p.Config) }).Children(func() {
			components.Text(c, locales.SettingsCloudBackupRetry(), theme.TextXS, theme.WeightMedium, fg).NoWrap()
		})
	})
}

// advancedOptions 是 <details> 的高级选项：glass-panel group overflow-hidden rounded-lg border border-brand-200
// dark:border-brand-700；summary 是 flex items-center justify-between gap-3 px-3 py-2.5 text-sm font-medium
// transition-colors hover:bg-brand-200/60 dark:hover:bg-brand-700/60 focus-visible:ring-2 ring-neutral-500，左侧
// i-mdi-tune-variant，右侧 i-mdi-chevron-down（展开时 rotate-180，duration-200）；内容是 space-y-2 border-t px-3 py-3 的
// 服务地址。展开状态同 <details> 一样不受控，挂在元素上（切换提供商后收起）。
func (s *State) advancedOptions(c *ui.Context, t *theme.Theme, p panel.Settings) {
	bg, border := t.GlassPanel(ui.Transparent, t.Pick(theme.Brand200, theme.Brand700)) // glass-panel
	box := ui.Column(c.Key("advanced")).FillWidth().Radius(8).Clip().Background(bg).Border(t.HL, border)
	open := ui.Local(box, "open", func() bool { return false })
	box.Children(func() {
		sum := ui.ButtonBase(c.Key("summary")).FillWidth().Padding(10, 12).Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).
			FocusRing(false).Label(locales.SettingsCloudBackupAdvancedOptions())
		if sum.Hovered() {
			sum.Background(t.Pick(theme.Brand200.Alpha(0.6), theme.Brand700.Alpha(0.6)))
		}
		if sum.FocusVisible() {
			sum.Shadow(0, 0, 0, 2, theme.Neutral500)
		}
		rot := sum.AnimateWith("chevron", 180*theme.Bool(*open), 200*time.Millisecond, theme.EaseTailwind)
		sum.Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(func() { *open = !*open }).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				components.Icon(c, "tune-variant", 14, t.SectionLabel)
				components.Text(c, locales.SettingsCloudBackupAdvancedOptions(), theme.TextSM, theme.WeightMedium, t.SectionLabel).NoWrap()
			})
			components.Icon(c, "chevron-down", 14, t.SectionLabel).Rotate(rot)
		})
		if !*open {
			return
		}
		ui.Column(c.Key("content")).FillWidth().Padding(12).Gap(8).
			BorderWidth(t.HL, 0, 0, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
			panel.FieldLabel(c, t, locales.SettingsCloudBackupUmbraBaseURL())
			v := p.Config.UmbraBaseURL
			in := better.Input(c, t, "umbra-base-url", &v, better.InputProps{
				Placeholder: "https://umbra.example.com", // i18n:keep：原版写死的示例地址
				Label:       locales.SettingsCloudBackupUmbraBaseURL(),
			})
			in.Font(monoFont)
			// handleChange 的 umbra_base_url 分支：作废用户信息，改了地址需要重新授权。
			if in.Changed() {
				s.clearProfile()
				p.Change(func(cfg *appconf.AppConfig) {
					cfg.UmbraBaseURL = v
					cfg.UmbraAuthenticated = false
				})
			}
			components.Text(c, locales.SettingsCloudBackupUmbraBaseURLHint(), theme.TextXS, theme.WeightNormal, hintColor(t))
		})
	})
}

// secondaryButton：glass-btn-neutral rounded-md bg-brand-100 px-3 py-1.5 text-sm text-brand-700 hover:bg-brand-200
// disabled:opacity-50 dark:bg-brand-700 dark:text-brand-300 dark:hover:bg-brand-600（测试连接；刷新另有 flex items-center
// gap-1.5 与图标，加载中是 i-mdi-loading animate-spin）。
func secondaryButton(c *ui.Context, t *theme.Theme, key, label, icon string, spin, disabled bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Shrink(0).Padding(6, 12).Radius(6).Gap(6).AlignItems(ui.Center).FocusRing(false)
	g := glassButton(t, btn, theme.BtnColors{Bg: t.Pick(theme.Brand100, theme.Brand700), Hover: t.Pick(theme.Brand200, theme.Brand600),
		Text: t.Pick(theme.Brand700, theme.Brand300)}, disabled)
	if disabled {
		components.Disable(btn, 0.5)
	}
	color := g.Text
	btn.Children(func() {
		switch {
		case spin:
			components.Spinner(c, 14, color)
		case icon != "":
			components.Icon(c, icon, 14, color)
		}
		components.Text(c, label, theme.TextSM, theme.WeightNormal, color).NoWrap()
	})
	return btn
}

// darkButton：glass-btn-neutral flex items-center gap-2 rounded-md bg-neutral-600 px-3 py-1.5 text-sm text-white
// hover:bg-neutral-700 disabled:opacity-50（授权按钮），等待中图标换成 i-mdi-loading animate-spin。
func darkButton(c *ui.Context, t *theme.Theme, key, label, icon string, spin, disabled bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Shrink(0).AlignSelf(ui.Start).Padding(6, 12).Radius(6).Gap(8).AlignItems(ui.Center).FocusRing(false)
	g := glassButton(t, btn, theme.BtnColors{Bg: theme.Neutral600, Hover: theme.Neutral700, Text: theme.White}, disabled)
	if disabled {
		components.Disable(btn, 0.5)
	}
	btn.Children(func() {
		if spin {
			components.Spinner(c, 14, g.Text)
		} else {
			components.Icon(c, icon, 14, g.Text)
		}
		components.Text(c, label, theme.TextSM, theme.WeightNormal, g.Text).NoWrap()
	})
	return btn
}

// glassButton 给 glass-btn-neutral 的按钮上色：平时是 normal（悬停时 Hover），玻璃模式下换成玻璃按钮的底色、边框与文字色。
// 返回文字颜色所在的那组颜色。
func glassButton(t *theme.Theme, btn ui.Element, normal theme.BtnColors, disabled bool) theme.BtnColors {
	g := t.GlassButton(theme.GlassNeutral, normal)
	bg := g.Bg
	if btn.Hovered() && !disabled {
		bg = g.Hover
	}
	btn.Background(bg)
	if t.Glass {
		btn.Border(t.HL, g.Border)
	}
	return g
}

// dangerTextButton：rounded-md px-3 py-1.5 text-sm text-error-600 hover:bg-error-100 disabled:opacity-50
// dark:text-error-400 dark:hover:bg-error-900（取消授权、取消等待）。
func dangerTextButton(c *ui.Context, t *theme.Theme, key, label string, disabled bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Shrink(0).Padding(6, 12).Radius(6).FocusRing(false)
	if btn.Hovered() && !disabled {
		btn.Background(t.Pick(theme.Error100, theme.Error900))
	}
	if disabled {
		components.Disable(btn, 0.5)
	}
	btn.Children(func() {
		components.Text(c, label, theme.TextSM, theme.WeightNormal, t.Pick(theme.Error600, theme.Error400)).NoWrap()
	})
	return btn
}
