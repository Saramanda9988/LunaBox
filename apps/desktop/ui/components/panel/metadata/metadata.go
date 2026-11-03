// Package metadata 对应原版 components/panel/MetadataSettingsPanel.tsx（设置页“元数据”分区）：元数据来源的开关、
// Bangumi / VNDB 封面来源、ErogameScape 地址、Steam 竖版封面等选项，以及批量更新元数据、下载远程封面与补全游戏 ID
// 三个操作（含 MetadataFieldSelectModal、MetadataRefreshProgressModal、GameIDEnrichmentPreviewModal）。
package metadata

import (
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/service"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// defaultScrapedTagLimit 是 DEFAULT_SCRAPED_TAG_LIMIT。
const defaultScrapedTagLimit = 10

// progressEvent 是 GameService 批量更新元数据时发出的进度事件（原版 onWailsEvent 订阅的名字）。
const progressEvent = "metadata:refresh-progress"

// State 是面板跨帧保留的状态，由设置页持有（整个页面生命周期里保留，见 settings.panels）。
type State struct {
	subscribed bool // 已订阅 metadata:refresh-progress（Events 没有退订，只订阅一次）

	refreshing     bool // isRefreshing
	queueingCover  bool // isQueueingCoverDownload
	enriching      bool // isEnrichingIDs
	previewOpen    bool // isIDPreviewOpen
	previewLoading bool // isIDPreviewLoading
	preview        *service.GameIDEnrichmentPreview

	refreshOpen    bool                        // isRefreshModalOpen
	fieldsOpen     bool                        // isFieldModalOpen
	fieldDraft     []enums.MetadataUpdateField // MetadataFieldSelectModal 里勾选的字段（弹窗内部的 selectedFields）
	selectedFields []enums.MetadataUpdateField // selectedRefreshFields；nil 为 DEFAULT_METADATA_UPDATE_FIELDS
	progress       refreshProgress

	confirmOpen   bool                        // confirmConfig.isOpen
	confirmFields []enums.MetadataUpdateField // confirmConfig.onConfirm 要用的字段

	toasts []env.Notice // 待显示的提示（后台结果的 apply 拿不到 Context），下一次构建时显示
}

func (s *State) toast(n env.Notice) { s.toasts = append(s.toasts, n) }

// subscribe 对应挂载时的 useEffect：订阅 metadata:refresh-progress，把事件的内容作为进度。
func (s *State) subscribe(p panel.Settings) {
	if s.subscribed || p.Env == nil || p.Env.Svc == nil || p.Env.Svc.Events == nil {
		return
	}
	s.subscribed = true
	p.Env.Svc.Events.On(progressEvent, func(data any) {
		if pr, ok := progressFromEvent(data); ok {
			s.progress = pr
		}
	})
}

// View 构建面板：space-y-4 的选项列、mt-6 pt-6 的“批量更新”一组（在设置页 space-y-4 的列里 mt-6 被盖成 16），最后是弹窗。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Settings) {
	s.subscribe(p)
	for _, msg := range s.toasts {
		p.Toast(c, msg)
	}
	s.toasts = nil
	if p.Config == nil {
		return
	}
	winW, _ := c.Size()
	sm := winW >= 640

	ui.Column(c.Key("metadata-options")).FillWidth().Gap(16).Children(func() {
		panel.GroupTitle(c, t, locales.SettingsMetadataSourceTitle())
		s.sourceGrid(c, t, p)
		coverSources(c, t, p, sm)
		erogameScapeField(c, t, p)
		cfg := p.Config
		better.SettingSwitchRow(c, t, "steam-portrait-cover", better.SettingSwitchRowProps{
			Label: locales.SettingsMetadataSteamPortraitCover(), Hint: locales.SettingsMetadataSteamPortraitCoverHint(),
			Checked: cfg.SteamCoverOrientation != enums.SteamCoverOrientationLandscape,
			OnChange: func(on bool) {
				p.Change(func(cfg *appconf.AppConfig) {
					cfg.SteamCoverOrientation = enums.SteamCoverOrientationLandscape
					if on {
						cfg.SteamCoverOrientation = enums.SteamCoverOrientationPortrait
					}
				})
			},
		})
		better.SettingSwitchRow(c, t, "allow-duplicate-metadata-import", better.SettingSwitchRowProps{
			Label: locales.SettingsMetadataAllowDuplicateMetadataImport(), Hint: locales.SettingsMetadataAllowDuplicateMetadataImportHint(),
			Checked:  cfg.AllowDuplicateMetadataImport,
			OnChange: func(on bool) { p.Change(func(cfg *appconf.AppConfig) { cfg.AllowDuplicateMetadataImport = on }) },
		})
		better.SettingSwitchRow(c, t, "enable-tag-translation", better.SettingSwitchRowProps{
			Label: locales.SettingsMetadataEnableTagTranslation(), Hint: locales.SettingsMetadataEnableTagTranslationHint(),
			Checked:  cfg.EnableTagTranslation,
			OnChange: func(on bool) { p.Change(func(cfg *appconf.AppConfig) { cfg.EnableTagTranslation = on }) },
		})
		tagLimitRow(c, t, p)
	})

	s.refreshSection(c, t, p, sm)

	components.ConfirmModal(c, t, "metadata-refresh-confirm", components.ConfirmProps{
		Open:      &s.confirmOpen,
		Title:     locales.SettingsMetadataModalRefreshTitle(),
		Message:   locales.SettingsMetadataModalRefreshMessage(),
		Danger:    true,
		TopOffset: panel.TopOffset,
		OnConfirm: func() { s.runRefresh(p, nil, s.confirmFields) },
	})
	previewModal(c, t, previewModalProps{
		open: s.previewOpen, loading: s.previewLoading, applying: s.enriching, preview: s.preview,
		onClose: func() {
			// handleCloseIDPreview
			if !s.enriching {
				s.previewOpen, s.preview = false, nil
			}
		},
		onConfirm: func() { s.runEnrichment(p) },
	})
	better.MetadataFieldSelectModal(c, t, "metadata-refresh-fields", better.MetadataFieldSelectProps{
		Open:        s.fieldsOpen,
		Title:       locales.MetadataUpdateFieldsModalFullTitle(),
		Description: locales.MetadataUpdateFieldsModalFullDescription(),
		ConfirmText: locales.MetadataUpdateFieldsModalContinue(),
		Fields:      &s.fieldDraft,
		TopOffset:   panel.TopOffset,
		OnClose:     func() { s.fieldsOpen = false },
		OnConfirm: func(fields []enums.MetadataUpdateField) {
			// handleConfirmRefreshFields
			s.selectedFields = fields
			s.fieldsOpen = false
			s.confirmOpen, s.confirmFields = true, fields
		},
	})
	progressModal(c, t, progressModalProps{
		open: s.refreshOpen, progress: s.progress, refreshing: s.refreshing,
		onRetryFailed: func() { s.retryFailed(p) },
		onClose:       func() { s.refreshOpen = false },
	})
}

// sourceGrid：grid grid-cols-2 gap-3 的来源卡片。每张是 glass-panel flex items-center justify-between rounded-lg border
// border-brand-200 p-4 dark:border-brand-700：左侧 flex-1 space-y-2 是 logo 与名称（flex items-center gap-2 select-none，
// 名称 text-sm font-medium，点击切换）和 text-xs 的说明，右侧 BetterSwitch。
func (s *State) sourceGrid(c *ui.Context, t *theme.Theme, p panel.Settings) {
	selected := normalizeEnabledSources(p.Config.MetadataSources)
	ui.Grid(c.Key("sources")).Columns(2).GapX(12).GapY(12).FillWidth().Children(func() {
		for _, item := range sourceItems {
			checked := slices.Contains(selected, item.value)
			toggle := func(on bool) { s.toggleSource(p, selected, item.value, on) }
			label := item.label()
			bg, border := t.GlassPanel(ui.Transparent, t.Pick(theme.Brand200, theme.Brand700))
			ui.Row(c.Key(string(item.value))).MinWidth(0).Padding(16).Radius(8).AlignItems(ui.Center).Justify(ui.SpaceBetween).
				Background(bg).Border(t.HL, border).Children(func() {
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(8).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						if l := sourceLogo(item.value); l != nil {
							logoImage(c, t, l, 22)
						}
						ui.Box(c.Key("label")).MinWidth(0).OnClick(func() { toggle(!checked) }).Children(func() {
							components.Text(c, label, theme.TextSM, theme.WeightMedium, t.SectionLabel)
						})
					})
					panel.FieldHint(c, t, item.hint())
				})
				better.Switch(c, t, "switch", checked, false, toggle)
			})
		}
	})
}

// toggleSource 对应 handleToggleSource：打开时加到末尾；关闭时移除，最后一个不能关（toast.error）。
func (s *State) toggleSource(p panel.Settings, selected []enums.SourceType, source enums.SourceType, checked bool) {
	next := selected
	if checked {
		if !slices.Contains(selected, source) {
			next = append(slices.Clone(selected), source)
		}
	} else {
		next = slices.DeleteFunc(slices.Clone(selected), func(v enums.SourceType) bool { return v == source })
		if len(next) == 0 {
			s.toast(env.Error(locales.SettingsMetadataToastAtLeastOneSource()))
			return
		}
	}
	list := make([]string, len(next))
	for i, v := range next {
		list[i] = string(v)
	}
	p.Change(func(cfg *appconf.AppConfig) { cfg.MetadataSources = list })
}

// coverSources：grid grid-cols-1 gap-4 sm:grid-cols-2 的 Bangumi 与 VNDB 封面来源（space-y-2 的 label、BetterSelect 与提示）。
func coverSources(c *ui.Context, t *theme.Theme, p panel.Settings, sm bool) {
	options := []better.SelectOption{
		{Label: locales.SettingsMetadataCoverSourcesHikarinagi(), Value: string(enums.MetadataCoverSourceHikarinagi)},
		{Label: locales.SettingsMetadataCoverSourcesOriginal(), Value: string(enums.MetadataCoverSourceOriginal)},
	}
	value := func(v enums.MetadataCoverSource) string {
		if v == "" {
			return string(enums.MetadataCoverSourceHikarinagi)
		}
		return string(v)
	}
	// 两项的网格：sm 以上是两列等宽（用行排列，各占 flex-1），以下是一列。
	grid := ui.Row(c.Key("cover-sources")).FillWidth().Gap(16).AlignItems(ui.Start)
	if !sm {
		grid.Column().AlignItems(ui.Stretch)
	}
	grid.Children(func() {
		panel.Field(c, t, "bangumi-cover-source", locales.SettingsMetadataBangumiCoverSource(), locales.SettingsMetadataBangumiCoverSourceHint(), func() {
			better.Select(c, t, "select", better.SelectProps{
				Value: value(p.Config.BangumiCoverSource), Options: options,
				OnChange: func(v string) {
					p.Change(func(cfg *appconf.AppConfig) { cfg.BangumiCoverSource = enums.MetadataCoverSource(v) })
				},
			})
		}).Grow(1).Basis(0).MinWidth(0)
		panel.Field(c, t, "vndb-cover-source", locales.SettingsMetadataVndbCoverSource(), locales.SettingsMetadataVndbCoverSourceHint(), func() {
			better.Select(c, t, "select", better.SelectProps{
				Value: value(p.Config.VNDBCoverSource), Options: options,
				OnChange: func(v string) {
					p.Change(func(cfg *appconf.AppConfig) { cfg.VNDBCoverSource = enums.MetadataCoverSource(v) })
				},
			})
		}).Grow(1).Basis(0).MinWidth(0)
	})
}

// erogameScapeField：space-y-2 的 label、type="url" 的 BetterInput（每次输入都改草稿）与提示。
func erogameScapeField(c *ui.Context, t *theme.Theme, p panel.Settings) {
	label := locales.SettingsMetadataErogameScapeBaseURL()
	panel.Field(c, t, "erogamescape-base-url", label, locales.SettingsMetadataErogameScapeBaseURLHint(), func() {
		v := p.Config.ErogameScapeBaseURL
		if better.Input(c, t, "input", &v, better.InputProps{
			Placeholder: "https://erogamescape.org/~ap2/ero/toukei_kaiseki", Label: label,
		}).Changed() {
			p.Change(func(cfg *appconf.AppConfig) { cfg.ErogameScapeBaseURL = v })
		}
	})
}

// tagLimitRow 是“刮削标签数量上限”：flex items-center justify-between gap-4，左侧 flex-1 space-y-2 的 label 与提示，右侧
// flex shrink-0 flex-wrap items-center justify-end gap-3 的 BetterNumberInput（size="sm"，min 0，不限时禁用并显示 0）、
// “不限”label（点击切换）与 BetterSwitch。
func tagLimitRow(c *ui.Context, t *theme.Theme, p panel.Settings) {
	limit := max(-1, p.Config.ScrapedTagLimit)
	unlimited := limit < 0
	shown := limit
	if unlimited {
		shown = 0
	}
	setUnlimited := func(on bool) {
		p.Change(func(cfg *appconf.AppConfig) {
			cfg.ScrapedTagLimit = defaultScrapedTagLimit
			if on {
				cfg.ScrapedTagLimit = -1
			}
		})
	}
	label := locales.SettingsMetadataScrapedTagLimit()
	ui.Row(c.Key("scraped-tag-limit")).FillWidth().Gap(16).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(8).Children(func() {
			panel.FieldLabel(c, t, label)
			panel.FieldHint(c, t, locales.SettingsMetadataScrapedTagLimitHint())
		})
		ui.Row(c).Shrink(0).Wrap().Gap(12).AlignItems(ui.Center).Justify(ui.End).Children(func() {
			better.NumberInput(c, t, "input", shown, better.NumberInputProps{
				Min: better.Bound(0), Step: 1, Size: better.NumberInputSM, Disabled: unlimited, Label: label,
				OnChange: func(v int) {
					// handleTagLimitChange
					p.Change(func(cfg *appconf.AppConfig) { cfg.ScrapedTagLimit = max(0, v) })
				},
			})
			ui.Box(c.Key("unlimited-label")).OnClick(func() { setUnlimited(!unlimited) }).Children(func() {
				components.SectionLabel(c, t, locales.SettingsMetadataScrapedTagLimitUnlimited())
			})
			better.Switch(c, t, "unlimited", unlimited, false, setUnlimited)
		})
	})
}

// refreshSection 是“批量更新”一组：mt-6 border-brand-200 pt-6（没有 border-t，不画线）的标题、mt-2 的提示与 mt-4 flex
// flex-col gap-3 sm:flex-row sm:flex-wrap 的三个按钮（w-full justify-center sm:w-auto）。
func (s *State) refreshSection(c *ui.Context, t *theme.Theme, p panel.Settings, sm bool) {
	ui.Column(c.Key("metadata-refresh")).FillWidth().Padding(24, 0, 0, 0).Children(func() {
		panel.GroupTitle(c, t, locales.SettingsMetadataRefreshTitle())
		// mt-2 text-xs text-brand-500 dark:text-brand-400
		components.TextMargin(c, locales.SettingsMetadataRefreshHint(), theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400), 8, 0, 0, 0)
		buttons := ui.Row(c.Key("buttons")).FillWidth().Margin(16, 0, 0, 0).Gap(12)
		if sm {
			buttons.Wrap()
		} else {
			buttons.Column().AlignItems(ui.Stretch)
		}
		buttons.Children(func() {
			refreshLabel := locales.SettingsMetadataRefreshButton()
			if s.refreshing {
				refreshLabel = locales.SettingsMetadataRefreshing()
			}
			better.Button(c, t, "refresh", better.ButtonProps{
				Variant: better.ButtonPrimary, Icon: "database-refresh", Label: refreshLabel,
				Loading: s.refreshing, Disabled: s.queueingCover || s.enriching,
			}).OnClick(func() {
				// handleRefreshAllMetadata：先选字段（initialFields 是上次用过的字段）。
				if !s.refreshing {
					s.fieldDraft = better.InitialMetadataFields(s.selectedFields)
					s.fieldsOpen = true
				}
			})
			coverLabel := locales.SettingsMetadataDownloadCoverButton()
			if s.queueingCover {
				coverLabel = locales.SettingsMetadataDownloadCoverQueueing()
			}
			better.Button(c, t, "download-covers", better.ButtonProps{
				Icon: "image-move", Label: coverLabel, Loading: s.queueingCover, Disabled: s.refreshing || s.enriching,
			}).OnClick(func() { s.downloadRemoteCovers(p) })
			idLabel := locales.SettingsMetadataIdEnrichmentButton()
			switch {
			case s.enriching:
				idLabel = locales.SettingsMetadataIdEnriching()
			case s.previewLoading:
				idLabel = locales.SettingsMetadataIdPreviewLoading()
			}
			better.Button(c, t, "id-enrichment", better.ButtonProps{
				Icon: "database-plus-outline", Label: idLabel, Loading: s.enriching || s.previewLoading, Disabled: s.refreshing || s.queueingCover,
			}).OnClick(func() { s.openPreview(p) })
		})
	})
}

// runRefresh 对应 runMetadataRefresh：gameIDs 非空时只重试这些游戏。打开进度弹窗，后台调用服务，进度由事件更新；
// 完成后显示结果，没有失败时关闭弹窗。
func (s *State) runRefresh(p panel.Settings, gameIDs []string, fields []enums.MetadataUpdateField) {
	if s.refreshing {
		return
	}
	retry := slices.DeleteFunc(slices.Clone(gameIDs), func(id string) bool { return strings.TrimSpace(id) == "" })
	if len(fields) == 0 {
		fields = slices.Clone(enums.DefaultMetadataUpdateFields)
	}
	s.refreshing, s.refreshOpen = true, true
	status := "started"
	if len(retry) > 0 {
		status = "retrying"
	}
	s.progress = newRefreshProgress(status)
	svc := p.Env.Svc
	p.Env.Async(func() func() {
		var (
			result vo.MetadataRefreshResult
			err    error
		)
		if len(retry) > 0 {
			result, err = svc.Game.RefreshGamesMetadataWithFields(retry, fields)
		} else {
			result, err = svc.Game.RefreshAllGamesMetadataWithFields(fields)
		}
		return func() {
			s.refreshing = false
			if err != nil {
				s.toast(env.Error(locales.SettingsMetadataToastRefreshFailed(err.Error())))
				s.refreshOpen = false
				return
			}
			s.progress = progressFromResult(result, "done")
			s.toast(env.Success(locales.SettingsMetadataToastRefreshSuccess(strconv.Itoa(result.UpdatedGames), strconv.Itoa(result.FailedGames),
				strconv.Itoa(result.SkippedGames), strconv.Itoa(result.LockedGames))))
			if result.FailedGames == 0 {
				s.refreshOpen = false
			}
		}
	})
}

// retryFailed 对应 handleRetryFailedMetadata：用上次选的字段重试失败的游戏。
func (s *State) retryFailed(p panel.Settings) {
	if s.refreshing || len(s.progress.failedGameIDs) == 0 {
		return
	}
	s.runRefresh(p, s.progress.failedGameIDs, s.selectedFields)
}

// downloadRemoteCovers 对应 handleDownloadRemoteCovers：把远程封面加入下载管理；没有远程封面时任务 ID 为空。
func (s *State) downloadRemoteCovers(p panel.Settings) {
	if s.queueingCover {
		return
	}
	s.queueingCover = true
	svc := p.Env.Svc
	p.Env.Async(func() func() {
		taskID, err := svc.Game.StartRemoteCoverImageDownloadTask()
		return func() {
			s.queueingCover = false
			switch {
			case err != nil:
				s.toast(env.Error(locales.SettingsMetadataToastDownloadCoverFailed(err.Error())))
			case taskID == "":
				s.toast(env.Success(locales.SettingsMetadataToastNoRemoteCovers()))
			default:
				s.toast(env.Success(locales.SettingsMetadataToastDownloadCoverQueued()))
			}
		}
	})
}

// openPreview 对应 handleGameIDEnrichment：打开预览弹窗并在后台计算补全结果；失败时关闭弹窗并提示。
func (s *State) openPreview(p panel.Settings) {
	if s.enriching || s.previewLoading {
		return
	}
	s.previewOpen, s.previewLoading, s.preview = true, true, nil
	svc := p.Env.Svc
	p.Env.Async(func() func() {
		preview, err := svc.Game.PreviewLegacyGameMetadataSourceIDs()
		return func() {
			s.previewLoading = false
			if err != nil {
				s.previewOpen = false
				s.toast(env.Error(locales.SettingsMetadataToastIdPreviewFailed(err.Error())))
				return
			}
			s.preview = &preview
		}
	})
}

// runEnrichment 对应 runGameIDEnrichment：写入补全的来源，关闭弹窗并提示结果。
func (s *State) runEnrichment(p panel.Settings) {
	if s.enriching {
		return
	}
	s.enriching = true
	svc := p.Env.Svc
	p.Env.Async(func() func() {
		result, err := svc.Game.EnrichLegacyGameMetadataSourceIDs()
		return func() {
			s.enriching = false
			if err != nil {
				s.toast(env.Error(locales.SettingsMetadataToastIdEnrichmentFailed(err.Error())))
				return
			}
			s.previewOpen, s.preview = false, nil
			if result.AddedSources == 0 {
				s.toast(env.Success(locales.SettingsMetadataToastIdEnrichmentNoChanges(strconv.Itoa(result.ScannedGames), strconv.Itoa(result.UnmatchedGames))))
				return
			}
			s.toast(env.Success(locales.SettingsMetadataToastIdEnrichmentSuccess(strconv.Itoa(result.UpdatedGames), strconv.Itoa(result.AddedSources),
				strconv.Itoa(result.UnmatchedGames))))
		}
	})
}
