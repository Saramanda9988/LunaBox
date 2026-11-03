package importer

import (
	"log"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel/metadata"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// scanSteps 是 BatchImportModal 与 DragDropImportModal 共用的部分：扫描结果的预览、元数据匹配、导入与手动选择。
type scanSteps struct {
	env     *env.Env
	flow    *Flow
	step    string // preview、match、importing（以及各弹窗自己的 select、scan、processing、result）
	preview previewState
}

// sources 是 enabledMetadataSources 与由它得到的匹配设置。
func (s *scanSteps) sources() ([]enums.SourceType, matchSettings) {
	cfg := s.env.Config()
	enabled := metadata.EnabledSources(cfg)
	pref := preferredSource(cfg, enabled)
	label := locales.BatchImportModalPreferredSourceNone()
	if pref != "" {
		label = SourceLabel(pref)
	}
	return enabled, matchSettings{preferred: pref, preferredLabel: label, all: metadata.AllSources()}
}

// startMatch 对应 handleStartMatch：进入匹配步骤，完成或暂停后回到预览。
func (s *scanSteps) startMatch(c *ui.Context) {
	_, ms := s.sources()
	s.step = "match"
	s.flow.StartMatch(ms, func() { s.step = "preview" })
}

// importCandidates 对应 handleImport：导入期间显示 importing，结束后回到预览。
func (s *scanSteps) importCandidates(c *ui.Context, matchedOnly bool, onComplete func()) {
	s.step = "importing"
	back := func() { s.step = "preview" }
	s.flow.Import(c, matchedOnly, back, back, onComplete)
}

// setPreferredSource 对应 handlePreferredSourceChange：patchLiveConfig({ batch_import_preferred_source })。
func (s *scanSteps) setPreferredSource(v string) {
	if err := s.env.PatchConfig(func(cfg *appconf.AppConfig) { cfg.BatchImportPreferredSource = v }); err != nil {
		log.Printf("import: save preferred source: %v", err)
	}
}

// previewProps 返回两个弹窗相同的 ImportPreviewStep 属性。
func (s *scanSteps) previewProps(c *ui.Context, t *theme.Theme, onComplete func()) previewProps {
	enabled, ms := s.sources()
	header := locales.BatchImportModalExecutable()
	if getScanConfig(s.env.Config()).preset == "hierarchy_child" {
		header = locales.BatchImportModalGamePath()
	}
	return previewProps{
		executableHeader: header,
		onStartMatch:     func() { s.startMatch(c) },
		onImport:         func(matchedOnly bool) { s.importCandidates(c, matchedOnly, onComplete) },
		onManual:         func(i int) { s.flow.openManual(i, ms, enabled) },
		onOpenFolder:     func(path string) { s.flow.openFolder(c, path) },
		actionToolbar:    func() { s.sourceToolbar(c, t, enabled, ms.preferred) },
	}
}

// sourceToolbar 是 actionToolbar：h-11 的“优先来源”标签（rounded-l-lg border-r-0 bg-white/80 px-3 text-xs font-medium shadow-sm）
// 与 w-40 的 BetterSelect。
func (s *scanSteps) sourceToolbar(c *ui.Context, t *theme.Theme, enabled []enums.SourceType, preferred enums.SourceType) {
	opts := []better.SelectOption{{Label: locales.BatchImportModalPreferredSourceNone(), Value: ""}}
	for _, src := range enabled {
		opts = append(opts, better.SelectOption{Label: SourceLabel(src), Value: string(src)})
	}
	ui.Row(c.Key("preferred-source")).AlignItems(ui.Stretch).Children(func() {
		fg := t.Pick(theme.Brand600, theme.Brand300)
		ui.Row(c).Height(44).Padding(0, 12).Gap(6).AlignItems(ui.Center).Shrink(0).Radius(8, 0, 0, 8).
			Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).BorderWidth(t.HL, 0, t.HL, t.HL).
			Background(t.Pick(theme.White.Alpha(0.8), theme.Brand800.Alpha(0.6))).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05)).Children(func() {
			components.Icon(c, "database-search-outline", 16, theme.Brand400)
			components.Text(c, locales.BatchImportModalPreferredSourceLabel(), theme.TextXS, theme.WeightMedium, fg).NoWrap()
		})
		ui.Column(c).Width(160).Justify(ui.Center).Children(func() {
			// buttonClassName="h-11 rounded-lg py-0 text-sm shadow-sm sm:rounded-l-none"
			better.Select(c, t, "select", better.SelectProps{
				Value: string(preferred), Options: opts, OnChange: s.setPreferredSource, Font: theme.TextSM,
				Style: func(btn ui.Element) {
					btn.Height(44).Padding(0, 40, 0, 12).AlignItems(ui.Center).Radius(0, 8, 8, 0).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
				},
			})
		})
	})
}

// commonSteps 构建 match 与 importing 两个步骤；onStop 不为 nil 时匹配可以停止（DragDropImportModal）。
func (s *scanSteps) commonSteps(c *ui.Context, t *theme.Theme, onStop func()) {
	switch s.step {
	case "match":
		stop := ""
		if onStop != nil {
			stop = locales.CommonStop()
		}
		progressStep(c, t, theme.Neutral500, locales.BatchImportModalMatching(), locales.BatchImportModalMatchHint(), s.flow.Progress, stop, onStop)
	case "importing":
		loadingStep(c, t, theme.Success500, locales.BatchImportModalImporting(), "")
	}
}

// batchResultLabels 是两个弹窗结果步骤的文字。
func batchResultLabels() resultLabels {
	return resultLabels{
		success: locales.BatchImportModalResultSuccess(), skipped: locales.BatchImportModalResultSkipped(),
		failed: locales.BatchImportModalResultFailed(), skippedGames: locales.BatchImportModalSkippedGames(),
		failedGames: locales.BatchImportModalFailedGames(), complete: locales.CommonComplete(),
	}
}
