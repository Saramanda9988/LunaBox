// Package autobackup 对应原版 components/panel/AutoBackupSettingsPanel.tsx（设置页“自动备份”分区）：本地备份
// （退出时备份数据库、定时备份数据库、退出游戏时备份存档、保留数量）、云备份（自动上传、启动前同步存档、保留数量）
// 与云同步（开关、自动同步与间隔）。
package autobackup

import (
	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/components/panel/cloudbackup"
	"lunabox/desktop-mygo/ui/components/picker"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// State 是面板跨帧保留的状态，由设置页持有（面板没有自己的状态）。
type State struct{}

// View 构建面板：space-y-4 的三组设置，后两组是 border-t pt-4 的云备份与云同步。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Settings) {
	_ = s
	cfg := p.Config
	if cfg == nil {
		return
	}
	cloudEnabled := cfg.CloudBackupEnabled
	configured := cloudbackup.ProviderConfigured(cfg)
	change := p.Change
	ui.Column(c.Key("auto-backup-panel")).FillWidth().Gap(16).Children(func() {
		localSection(c, t, p)

		divided(c, t, "cloud-backup", func() {
			panel.GroupTitle(c, t, locales.SettingsAutoBackupCloudBackupSection())
			better.SettingSwitchRow(c, t, "auto-upload-db", better.SettingSwitchRowProps{
				Label: locales.SettingsAutoBackupAutoUploadDb(), Hint: locales.SettingsAutoBackupAutoUploadDbHint(),
				Checked: cfg.AutoUploadDBToCloud, Disabled: !cloudEnabled,
				OnChange: func(v bool) { change(func(cfg *appconf.AppConfig) { cfg.AutoUploadDBToCloud = v }) },
			})
			better.SettingSwitchRow(c, t, "auto-upload-save", better.SettingSwitchRowProps{
				Label: locales.SettingsAutoBackupAutoUploadSave(), Hint: locales.SettingsAutoBackupAutoUploadSaveHint(),
				Checked: cfg.AutoUploadSaveToCloud, Disabled: !cloudEnabled,
				OnChange: func(v bool) { change(func(cfg *appconf.AppConfig) { cfg.AutoUploadSaveToCloud = v }) },
			})
			better.SettingSwitchRow(c, t, "restore-before-launch", better.SettingSwitchRowProps{
				Label: locales.SettingsAutoBackupRestoreCloudSaveBeforeLaunch(), Hint: locales.SettingsAutoBackupRestoreCloudSaveBeforeLaunchHint(),
				Checked: cfg.AutoRestoreCloudSave, Disabled: !cloudEnabled || !configured,
				OnChange: func(v bool) { change(func(cfg *appconf.AppConfig) { cfg.AutoRestoreCloudSave = v }) },
			})
			// space-y-2 的两项保留数量
			ui.Column(c.Key("cloud-retention")).FillWidth().Gap(8).Children(func() {
				numberRow(c, t, "cloud-game-retention", locales.SettingsAutoBackupCloudGameRetention(), cloudEnabled, func() {
					hint(c, t, locales.SettingsAutoBackupCloudGameRetentionHint())
					if !cloudEnabled {
						warning(c, t, locales.SettingsAutoBackupCloudServiceRequiredHint())
					}
				}, or(cfg.CloudBackupRetention, 5), better.NumberInputProps{
					Min: better.Bound(1), Max: better.Bound(100), Size: better.NumberInputSM, Disabled: !cloudEnabled,
					OnChange: func(v int) { change(func(cfg *appconf.AppConfig) { cfg.CloudBackupRetention = v }) },
				})
				numberRow(c, t, "cloud-db-retention", locales.SettingsAutoBackupCloudDbRetention(), cloudEnabled, func() {
					hint(c, t, locales.SettingsAutoBackupCloudDbRetentionHint())
				}, or(cfg.CloudDBBackupRetention, 5), better.NumberInputProps{
					Min: better.Bound(1), Max: better.Bound(100), Size: better.NumberInputSM, Disabled: !cloudEnabled,
					OnChange: func(v int) { change(func(cfg *appconf.AppConfig) { cfg.CloudDBBackupRetention = v }) },
				})
			})
		})

		divided(c, t, "cloud-sync", func() {
			panel.GroupTitle(c, t, locales.SettingsAutoBackupCloudSyncSection())
			better.SettingSwitchRow(c, t, "cloud-sync", better.SettingSwitchRowProps{
				Label: locales.SettingsCloudBackupSyncEnableLabel(), Hint: locales.SettingsCloudBackupSyncEnableHint(),
				Checked: cfg.CloudSyncEnabled, Disabled: !cloudEnabled,
				// 关闭云同步时一并关闭自动云同步。
				OnChange: func(v bool) {
					change(func(cfg *appconf.AppConfig) {
						cfg.CloudSyncEnabled = v
						cfg.AutoCloudSyncEnabled = v && cfg.AutoCloudSyncEnabled
					})
				},
			})
			better.SettingSwitchRow(c, t, "auto-cloud-sync", better.SettingSwitchRowProps{
				Label: locales.SettingsCloudBackupAutoSyncEnableLabel(), Hint: locales.SettingsCloudBackupAutoSyncEnableHint(),
				Checked: cfg.AutoCloudSyncEnabled, Disabled: !cloudEnabled || !cfg.CloudSyncEnabled,
				OnChange: func(v bool) { change(func(cfg *appconf.AppConfig) { cfg.AutoCloudSyncEnabled = v }) },
			})
			intervalActive := cloudEnabled && cfg.CloudSyncEnabled && cfg.AutoCloudSyncEnabled
			ui.Column(c.Key("sync-interval-group")).FillWidth().Gap(8).Children(func() {
				numberRow(c, t, "sync-interval", locales.SettingsCloudBackupSyncIntervalLabel(), intervalActive, func() {
					hint(c, t, locales.SettingsCloudBackupSyncIntervalHint())
					switch {
					case !cloudEnabled:
						warning(c, t, locales.SettingsAutoBackupCloudServiceRequiredHint())
					case !configured:
						warning(c, t, locales.SettingsAutoBackupCloudSyncConfigRequiredHint())
					}
					if cloudEnabled && cfg.CloudSyncEnabled && !cfg.AutoCloudSyncEnabled {
						hint(c, t, locales.SettingsAutoBackupCloudSyncManualRecommendedHint())
					}
				}, cloudbackup.SyncIntervalSeconds(cfg), better.NumberInputProps{
					Min: better.Bound(15), Step: 5, Unit: locales.SettingsCloudBackupSyncIntervalUnit(), Size: better.NumberInputSM,
					Disabled: !intervalActive,
					OnChange: func(v int) { change(func(cfg *appconf.AppConfig) { cfg.CloudSyncIntervalSec = v }) },
				})
			})
		})
	})
}

// localSection 是第一组（space-y-4）：标题、退出时备份数据库、定时备份数据库（开启时是缩进的定时方式与间隔或时间）、
// 退出游戏时备份存档、本地保留数量。
func localSection(c *ui.Context, t *theme.Theme, p panel.Settings) {
	cfg, change := p.Config, p.Change
	ui.Column(c.Key("local")).FillWidth().Gap(16).Children(func() {
		panel.GroupTitle(c, t, locales.SettingsAutoBackupLocalSection())
		better.SettingSwitchRow(c, t, "auto-backup-db", better.SettingSwitchRowProps{
			Label: locales.SettingsAutoBackupBackupDbOnExit(), Hint: locales.SettingsAutoBackupBackupDbOnExitHint(),
			Checked:  cfg.AutoBackupDB,
			OnChange: func(v bool) { change(func(cfg *appconf.AppConfig) { cfg.AutoBackupDB = v }) },
		})
		better.SettingSwitchRow(c, t, "scheduled-db-backup", better.SettingSwitchRowProps{
			Label: locales.SettingsAutoBackupScheduledDbBackup(), Hint: locales.SettingsAutoBackupScheduledDbBackupHint(),
			Checked:  cfg.ScheduledDBBackupEnabled,
			OnChange: func(v bool) { change(func(cfg *appconf.AppConfig) { cfg.ScheduledDBBackupEnabled = v }) },
		})
		if cfg.ScheduledDBBackupEnabled {
			schedule(c, t, p)
		}
		better.SettingSwitchRow(c, t, "auto-backup-save", better.SettingSwitchRowProps{
			Label: locales.SettingsAutoBackupBackupSaveOnExit(), Hint: locales.SettingsAutoBackupBackupSaveOnExitHint(),
			Checked:  cfg.AutoBackupGameSave,
			OnChange: func(v bool) { change(func(cfg *appconf.AppConfig) { cfg.AutoBackupGameSave = v }) },
		})
		// space-y-4 的两项本地保留数量
		ui.Column(c.Key("local-retention")).FillWidth().Gap(16).Children(func() {
			numberRow(c, t, "local-game-retention", locales.SettingsAutoBackupLocalGameRetention(), true, func() {
				hint(c, t, locales.SettingsAutoBackupLocalGameRetentionHint())
			}, or(cfg.LocalBackupRetention, 10), better.NumberInputProps{
				Min: better.Bound(0), Size: better.NumberInputSM,
				OnChange: func(v int) { change(func(cfg *appconf.AppConfig) { cfg.LocalBackupRetention = v }) },
			})
			numberRow(c, t, "local-db-retention", locales.SettingsAutoBackupLocalDbRetention(), true, func() {
				hint(c, t, locales.SettingsAutoBackupLocalDbRetentionHint())
			}, max(1, cfg.LocalDBBackupRetention), better.NumberInputProps{
				Min: better.Bound(1), Size: better.NumberInputSM,
				OnChange: func(v int) { change(func(cfg *appconf.AppConfig) { cfg.LocalDBBackupRetention = max(1, v) }) },
			})
		})
	})
}

// schedule 是定时备份的设置：space-y-4 border-l-2 border-brand-200 pl-4 dark:border-brand-700，定时方式（w-44 的 BetterSelect），
// 每日时是备份时间（w-36 的 BetterTimeWheelInput），否则是备份间隔（15 到 10080 分钟，步长 15）。
func schedule(c *ui.Context, t *theme.Theme, p panel.Settings) {
	cfg, change := p.Config, p.Change
	mode := cfg.ScheduledDBBackupMode
	if mode == "" {
		mode = appconf.ScheduledDBBackupModeInterval
	}
	ui.Column(c.Key("schedule")).FillWidth().Gap(16).Padding(0, 0, 0, 16).
		BorderWidth(0, 0, 0, 2).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
		row(c, t, "schedule-mode", locales.SettingsAutoBackupScheduleMode(), true, func() {
			hint(c, t, locales.SettingsAutoBackupScheduleModeHint())
		}, func() {
			ui.Box(c.Key("select")).Width(176).Shrink(0).Children(func() {
				better.Select(c, t, "select", better.SelectProps{
					Value: mode,
					Options: []better.SelectOption{
						{Value: appconf.ScheduledDBBackupModeInterval, Label: locales.SettingsAutoBackupScheduleModeInterval()},
						{Value: appconf.ScheduledDBBackupModeDaily, Label: locales.SettingsAutoBackupScheduleModeDaily()},
					},
					OnChange: func(v string) { change(func(cfg *appconf.AppConfig) { cfg.ScheduledDBBackupMode = v }) },
				})
			})
		})
		if mode == appconf.ScheduledDBBackupModeDaily {
			row(c, t, "schedule-time", locales.SettingsAutoBackupScheduleDailyTime(), true, func() {
				hint(c, t, locales.SettingsAutoBackupScheduleDailyTimeHint())
			}, func() {
				value := cfg.ScheduledDBBackupTime
				if value == "" {
					value = appconf.DefaultScheduledDBBackupTime
				}
				ui.Box(c.Key("time")).Width(144).Shrink(0).Children(func() {
					picker.TimeWheelInput(c, t, "input", picker.TimeWheelInputProps{
						Value:       value,
						ActionLabel: locales.SettingsAutoBackupScheduleDailyTime(),
						OnChange:    func(v string) { change(func(cfg *appconf.AppConfig) { cfg.ScheduledDBBackupTime = v }) },
					})
				})
			})
			return
		}
		numberRow(c, t, "schedule-interval", locales.SettingsAutoBackupScheduleInterval(), true, func() {
			hint(c, t, locales.SettingsAutoBackupScheduleIntervalHint())
		}, or(cfg.ScheduledDBBackupIntervalMinutes, appconf.DefaultScheduledDBBackupIntervalMinutes), better.NumberInputProps{
			Min:  better.Bound(appconf.MinScheduledDBBackupIntervalMinutes),
			Max:  better.Bound(appconf.MaxScheduledDBBackupIntervalMinutes),
			Step: 15, Unit: locales.SettingsAutoBackupMinutesUnit(), Size: better.NumberInputSM,
			OnChange: func(v int) { change(func(cfg *appconf.AppConfig) { cfg.ScheduledDBBackupIntervalMinutes = v }) },
		})
	})
}

// divided：space-y-4 border-t border-brand-200 pt-4 dark:border-brand-700。
func divided(c *ui.Context, t *theme.Theme, key string, content func()) {
	ui.Column(c.Key(key)).FillWidth().Gap(16).Padding(16, 0, 0, 0).
		BorderWidth(t.HL, 0, 0, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(content)
}

// row 是 flex items-center justify-between gap-4 的一行：左侧 flex-1 space-y-2 的 label（text-sm font-medium，
// active 为假时 text-brand-400 dark:text-brand-500）与提示，右侧 shrink-0 的控件。
func row(c *ui.Context, t *theme.Theme, key, label string, active bool, hints, control func()) {
	color := t.SectionLabel
	if !active {
		color = t.Pick(theme.Brand400, theme.Brand500)
	}
	ui.Row(c.Key(key)).FillWidth().Gap(16).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(8).Children(func() {
			components.Text(c, label, theme.TextSM, theme.WeightMedium, color)
			hints()
		})
		control()
	})
}

// numberRow 是右侧为 size="sm" 的 BetterNumberInput 的一行。
func numberRow(c *ui.Context, t *theme.Theme, key, label string, active bool, hints func(), value int, np better.NumberInputProps) {
	np.Label = label
	row(c, t, key, label, active, hints, func() { better.NumberInput(c, t, "number", value, np) })
}

// hint：text-xs text-brand-500 dark:text-brand-400。
func hint(c *ui.Context, t *theme.Theme, s string) {
	components.Text(c, s, theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
}

// warning：text-xs text-warning-600 dark:text-warning-400。
func warning(c *ui.Context, t *theme.Theme, s string) {
	components.Text(c, s, theme.TextXS, theme.WeightNormal, t.Pick(theme.Warning600, theme.Warning400))
}

// or 同 JavaScript 的 value || fallback（0 时取默认值）。
func or(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}
