// Package importer 是添加与导入游戏，对应原版 components/ui/import（导入流程、预览、手动匹配等步骤）与
// components/modal 里的 AddGameModal、BatchImportModal、DragDropImportModal、GameImportModal、
// SkippedImportCandidatesModal。
package importer

import (
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/backend/service"
	"lunabox/backend/utils/metadata"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
)

// MatchStatus 是 ImportMatchStatus。
type MatchStatus string

const (
	MatchPending  MatchStatus = "pending"
	MatchMatched  MatchStatus = "matched"
	MatchNotFound MatchStatus = "not_found"
	MatchError    MatchStatus = "error"
	MatchManual   MatchStatus = "manual"
)

// Candidate 是 ImportCandidate：扫描到的一个游戏目录，以及它的元数据匹配结果。
type Candidate struct {
	FolderPath    string
	GameDirectory string
	FolderName    string
	Executables   []string
	SelectedExe   string
	SearchName    string
	SourceType    enums.SourceType
	SourceID      string
	SizeOnDisk    int64
	Selected      bool
	ImportStatus  string
	SkipReason    string
	ExistingName  string

	MatchedGame *models.Game
	MatchedTags []metadata.TagItem
	MatchSource enums.SourceType
	MatchStatus MatchStatus
	MatchError  string

	DuplicateID   string // metadataDuplicateExistingId
	DuplicateName string // metadataDuplicateExistingName
	AllMatches    []vo.GameMetadataFromWebVO
}

// matched 对应 isMatchedImportCandidate。
func (c *Candidate) matched() bool {
	return c.MatchedGame != nil && (c.MatchStatus == MatchMatched || c.MatchStatus == MatchManual)
}

// clearMatch 清空匹配结果与重复提示。
func (c *Candidate) clearMatch(status MatchStatus, reason string) {
	c.MatchedGame, c.MatchedTags, c.MatchSource = nil, nil, ""
	c.MatchStatus, c.MatchError = status, reason
	c.DuplicateID, c.DuplicateName = "", ""
}

// toCandidate 对应 toImportCandidate。
func toCandidate(c vo.BatchImportCandidate) Candidate {
	dir := c.GameDirectory
	if dir == "" {
		dir = c.FolderPath
	}
	status := c.ImportStatus
	if status == "" {
		status = "new"
	}
	return Candidate{
		FolderPath: c.FolderPath, GameDirectory: dir, FolderName: c.FolderName,
		Executables: c.Executables, SelectedExe: c.SelectedExe, SearchName: c.SearchName,
		SourceType: c.SourceType, SourceID: c.SourceID, SizeOnDisk: c.SizeOnDisk,
		Selected: true, ImportStatus: status, SkipReason: c.SkipReason, ExistingName: c.ExistingName,
		MatchStatus: MatchPending,
	}
}

// scanToCandidates 对应 scanResultToCandidates：跳过的候选项默认不选中。
func scanToCandidates(r vo.BatchImportScanResult) (cands, skipped []Candidate) {
	for _, c := range r.Candidates {
		cands = append(cands, toCandidate(c))
	}
	for _, c := range r.SkippedCandidates {
		s := toCandidate(c)
		s.Selected = false
		skipped = append(skipped, s)
	}
	return cands, skipped
}

// importRequest 对应 candidatesToImportRequest。
func importRequest(cands []Candidate, matchedOnly bool) []vo.BatchImportCandidate {
	var out []vo.BatchImportCandidate
	for _, c := range cands {
		if !shouldImport(c, matchedOnly) {
			continue
		}
		out = append(out, vo.BatchImportCandidate{
			FolderPath: c.FolderPath, GameDirectory: c.GameDirectory, FolderName: c.FolderName,
			Executables: c.Executables, SelectedExe: c.SelectedExe, SearchName: c.SearchName,
			SourceType: c.SourceType, SourceID: c.SourceID, SizeOnDisk: c.SizeOnDisk,
			IsSelected: c.Selected, MatchStatus: string(c.MatchStatus), ImportStatus: c.ImportStatus,
			SkipReason: c.SkipReason, ExistingName: c.ExistingName,
			MatchedGame: c.MatchedGame, MatchedTags: c.MatchedTags, MatchSource: c.MatchSource,
		})
	}
	return out
}

// shouldImport 对应 shouldImportCandidate。
func shouldImport(c Candidate, matchedOnly bool) bool {
	return c.Selected && (!matchedOnly || c.matched())
}

// 扫描设置，对应 importFlow.ts 的常量与 getImportScanConfig。
const (
	maxHierarchyDepth = 5
	// failurePauseThreshold 是 PREFERRED_SOURCE_FAILURE_PAUSE_THRESHOLD：连续失败这么多次后暂停匹配。
	failurePauseThreshold = 3
	// matchDelay 是两次匹配请求之间的等待（避免被元数据网站限流）。
	matchDelay = 1500 * time.Millisecond
)

// scanConfig 是 getImportScanConfig 的结果。
type scanConfig struct {
	preset         string // scan_parent、scan_library_child、hierarchy_child
	hierarchyDepth int
	options        vo.BatchImportScanOptions
}

func (s scanConfig) hierarchyLevel() int { return s.hierarchyDepth + 1 }

func getScanConfig(cfg *appconf.AppConfig) scanConfig {
	s := scanConfig{preset: "scan_parent"}
	if cfg != nil {
		switch cfg.BatchImportScanPreset {
		case "scan_parent", "scan_library_child", "hierarchy_child":
			s.preset = cfg.BatchImportScanPreset
		}
		s.hierarchyDepth = clampDepth(cfg.BatchImportHierarchyDepth)
	}
	s.options = vo.BatchImportScanOptions{ScanMode: "scan", ScanNameMode: "depth", HierarchyDepth: s.hierarchyDepth}
	if s.preset == "hierarchy_child" {
		s.options.ScanMode = "hierarchy"
	}
	if s.preset == "scan_parent" {
		s.options.ScanNameMode = "parent"
	}
	return s
}

func clampDepth(d int) int { return min(maxHierarchyDepth, max(0, d)) }

// preferredSource 对应 getPreferredSource：配置的优先来源已启用时用它，否则没有优先来源（""）。
func preferredSource(cfg *appconf.AppConfig, enabled []enums.SourceType) enums.SourceType {
	if cfg == nil || cfg.BatchImportPreferredSource == "" {
		return ""
	}
	s := enums.SourceType(cfg.BatchImportPreferredSource)
	if slices.Contains(enabled, s) {
		return s
	}
	return ""
}

// sourcePriority 对应 sourcePriorityOrder：优先来源排在最前，其余按默认顺序。
func sourcePriority(preferred enums.SourceType, all []enums.SourceType) []enums.SourceType {
	if preferred == "" {
		return all
	}
	out := []enums.SourceType{preferred}
	for _, s := range all {
		if s != preferred {
			out = append(out, s)
		}
	}
	return out
}

// pickBestMatch 对应 pickBestMatch：按来源优先级取第一个有游戏信息的结果。
func pickBestMatch(matches []vo.GameMetadataFromWebVO, preferred enums.SourceType, all []enums.SourceType) *vo.GameMetadataFromWebVO {
	for _, s := range sourcePriority(preferred, all) {
		for i := range matches {
			if matches[i].Source == s && matches[i].Game.Name != "" {
				return &matches[i]
			}
		}
	}
	return nil
}

// SourceLabel 对应 importFlow.ts 的 sourceLabel：未知来源显示为 Steam。
func SourceLabel(s enums.SourceType) string {
	switch s {
	case enums.Bangumi:
		return "Bangumi"
	case enums.VNDB:
		return "VNDB"
	case enums.Ymgal:
		return locales.GameEditSourceYmgal()
	case enums.DLsite:
		return locales.GameEditSourceDlsite()
	case enums.TouchGal:
		return locales.GameEditSourceTouchGal()
	case enums.Hikarinagi:
		return locales.GameEditSourceHikarinagi()
	case enums.Kungal:
		return locales.GameEditSourceKungal()
	case enums.ErogameScape:
		return locales.GameEditSourceErogameScape()
	}
	return "Steam"
}

// Progress 是 MatchProgressState。
type Progress struct {
	Current, Total int
	Name           string
}

// manualState 是 ImportManualSelectModal 的状态。
type manualState struct {
	open      bool
	index     int
	matches   []vo.GameMetadataFromWebVO
	searching bool
	id        string
	source    enums.SourceType
}

// Flow 对应 useImportFlow：候选项、元数据匹配、手动选择与批量导入。BatchImportModal 与 DragDropImportModal 共用。
type Flow struct {
	env *env.Env

	Candidates []Candidate
	Skipped    []Candidate
	Result     *service.ImportResult
	Progress   Progress
	Pause      string // matchPauseMessage

	abort  bool // abortMatchRef
	gen    int  // Reset 后之前的异步结果不再生效
	manual manualState

	// Delay 是两次匹配请求之间的等待，默认 matchDelay；测试里设为负数不等待。
	Delay time.Duration
}

func newFlow(e *env.Env) *Flow { return &Flow{env: e} }

// Reset 对应 resetImportFlow：停止匹配并清空。
func (f *Flow) Reset() {
	f.abort = true
	f.gen++
	f.Candidates, f.Skipped, f.Result, f.Pause = nil, nil, nil, ""
	f.Progress = Progress{}
	f.manual.open = false
}

// SetCandidates 换上扫描结果。
func (f *Flow) SetCandidates(cands, skipped []Candidate) { f.Candidates, f.Skipped = cands, skipped }

// Stop 对应 stopMatching。
func (f *Flow) Stop() { f.abort = true }

// counts 是 matchedCount、notFoundCount、pendingCount、errorCount（只算选中的）。
func (f *Flow) counts() (matched, notFound, pending, errs int) {
	for _, c := range f.Candidates {
		if !c.Selected {
			continue
		}
		switch c.MatchStatus {
		case MatchMatched, MatchManual:
			matched++
		case MatchNotFound:
			notFound++
		case MatchPending:
			pending++
		case MatchError:
			errs++
		}
	}
	return
}

// matchable 对应 matchableCount。
func (f *Flow) matchable() int {
	_, _, p, e := f.counts()
	return p + e
}

func shouldMatch(c Candidate) bool {
	return c.Selected && (c.MatchStatus == MatchPending || c.MatchStatus == MatchError)
}

// matchSettings 是匹配时用到的来源设置。
type matchSettings struct {
	preferred      enums.SourceType
	preferredLabel string
	all            []enums.SourceType
}

// StartMatch 对应 handleStartMatch：依次为选中且待匹配（或出错）的候选项查找元数据，每次之间等待 Delay。
// 优先来源连续失败或被限流时暂停并提示；全部完成后检查元数据重复。onDone 在完成或暂停后调用（被 Stop 打断时不调用）。
func (f *Flow) StartMatch(s matchSettings, onDone func()) {
	f.Pause, f.abort = "", false
	total := 0
	for _, cand := range f.Candidates {
		if shouldMatch(cand) {
			total++
		}
	}
	f.Progress = Progress{Total: total}
	m := &matcher{f: f, s: s, onDone: onDone, gen: f.gen}
	m.next(0)
}

// matcher 是一次匹配的进度：handleStartMatch 循环里的局部变量。
type matcher struct {
	f        *Flow
	s        matchSettings
	onDone   func()
	gen      int
	count    int // matchedCount
	failures int // consecutiveFetchFailures
}

func (m *matcher) live() bool { return m.gen == m.f.gen }

// next 从下标 i 开始找下一个要匹配的候选项。
func (m *matcher) next(i int) {
	f := m.f
	for ; i < len(f.Candidates); i++ {
		if f.abort {
			return
		}
		if shouldMatch(f.Candidates[i]) {
			break
		}
	}
	if f.abort {
		return
	}
	if i >= len(f.Candidates) {
		m.finish()
		return
	}
	m.count++
	name := f.Candidates[i].SearchName
	f.Progress.Current, f.Progress.Name = m.count, name
	svc := f.env.Svc.Import
	f.env.Async(func() func() {
		result, err := svc.FetchMetadataForCandidateWithPreference(name, m.s.preferred)
		return func() {
			if !m.live() || i >= len(f.Candidates) {
				return
			}
			if pause := m.apply(i, result, err); pause != "" {
				f.abort = true
				f.Pause = pause
				f.env.Notify(env.Error(pause))
				if m.onDone != nil {
					m.onDone()
				}
				return
			}
			if f.abort {
				return
			}
			delay := f.Delay
			if delay == 0 {
				delay = matchDelay
			}
			if delay < 0 {
				m.next(i + 1)
				return
			}
			f.env.Async(func() func() {
				time.Sleep(delay)
				return func() {
					if m.live() {
						m.next(i + 1)
					}
				}
			})
		}
	})
}

// apply 记下第 i 个候选项的匹配结果，返回暂停的原因（为空时继续）。
func (m *matcher) apply(i int, r vo.BatchImportMetadataMatchResult, err error) string {
	cand := &m.f.Candidates[i]
	pauseFor := func(reason string) string {
		return locales.BatchImportModalPreferredSourceConsecutiveFailurePause(m.s.preferredLabel, strconv.Itoa(failurePauseThreshold), reason)
	}
	if err != nil {
		log.Printf("import: match %s: %v", cand.SearchName, err)
		m.failures++
		cand.clearMatch(MatchError, err.Error())
		if m.failures >= failurePauseThreshold {
			return pauseFor(err.Error())
		}
		return ""
	}
	if m.s.preferred != "" && !r.PreferredMatched {
		reason := r.PreferredError
		if reason == "" {
			reason = locales.BatchImportModalNoMatchResult()
		}
		status := MatchError
		if r.PreferredNoResult {
			status = MatchNotFound
		}
		cand.clearMatch(status, reason)
		cand.AllMatches = r.Matches
		if r.PreferredNoResult {
			m.failures = 0
			return ""
		}
		m.failures++
		if r.PreferredRateLimited {
			return locales.BatchImportModalPreferredSourceRateLimitedPause(m.s.preferredLabel, reason)
		}
		if m.failures >= failurePauseThreshold {
			return pauseFor(reason)
		}
		return ""
	}
	m.failures = 0
	cand.AllMatches = r.Matches
	if best := pickBestMatch(r.Matches, m.s.preferred, m.s.all); best != nil {
		g := best.Game
		cand.clearMatch(MatchMatched, "")
		cand.MatchedGame, cand.MatchedTags, cand.MatchSource = &g, best.Tags, best.Source
	} else {
		cand.clearMatch(MatchNotFound, locales.BatchImportModalNoMatchResult())
	}
	return ""
}

// finish 在全部匹配完后检查元数据重复（applyMetadataDuplicateHints），再调用 onDone。
func (m *matcher) finish() {
	f := m.f
	f.applyDuplicateHints(nil, func() {
		if m.live() && m.onDone != nil {
			m.onDone()
		}
	})
}

// duplicateKey 对应 getMetadataDuplicateKey。
func duplicateKey(source enums.SourceType, id string) string {
	if source == "" || id == "" {
		return ""
	}
	return string(source) + "\x00" + strings.ToLower(strings.TrimSpace(id))
}

// candidateKeys 对应 getCandidateMetadataSourceKeys：匹配来源与游戏的全部元数据来源。
func candidateKeys(c *Candidate) []string {
	var keys []string
	add := func(source enums.SourceType, id string) {
		if k := duplicateKey(source, id); k != "" && !slices.Contains(keys, k) {
			keys = append(keys, k)
		}
	}
	if g := c.MatchedGame; g != nil {
		source := c.MatchSource
		if source == "" {
			source = g.SourceType
		}
		add(source, g.SourceID)
		for _, s := range g.MetadataSources {
			add(s.SourceType, s.SourceID)
		}
	}
	return keys
}

// applyDuplicateHints 对应 applyMetadataDuplicateHints：查询匹配到的元数据是否已经在库里，标在候选项上。
// only 不为 nil 时只检查这些下标。失败时保持原样（原版只打日志）。done 在主线程上调用。
func (f *Flow) applyDuplicateHints(only []int, done func()) {
	indexes := only
	if indexes == nil {
		for i := range f.Candidates {
			indexes = append(indexes, i)
		}
	}
	var reqs []vo.ImportMetadataDuplicateRequest
	seen := map[string]bool{}
	for _, i := range indexes {
		for _, k := range candidateKeys(&f.Candidates[i]) {
			if seen[k] {
				continue
			}
			seen[k] = true
			source, id, _ := strings.Cut(k, "\x00")
			reqs = append(reqs, vo.ImportMetadataDuplicateRequest{Source: enums.SourceType(source), SourceID: id})
		}
	}
	gen := f.gen
	set := func(results map[string]vo.ImportMetadataDuplicateResult) {
		for _, i := range indexes {
			c := &f.Candidates[i]
			c.DuplicateID, c.DuplicateName = "", ""
			for _, k := range candidateKeys(c) {
				if r, ok := results[k]; ok && r.Exists {
					c.DuplicateID, c.DuplicateName = r.ExistingID, r.ExistingName
					break
				}
			}
		}
	}
	if len(reqs) == 0 {
		set(nil)
		done()
		return
	}
	svc := f.env.Svc.Import
	f.env.Async(func() func() {
		results, err := svc.CheckImportMetadataDuplicates(reqs)
		return func() {
			if gen != f.gen {
				return
			}
			if err != nil {
				log.Printf("import: check metadata duplicates: %v", err)
			} else {
				byKey := map[string]vo.ImportMetadataDuplicateResult{}
				for _, r := range results {
					byKey[duplicateKey(r.Source, r.SourceID)] = r
				}
				set(byKey)
			}
			done()
		}
	})
}

// resultMessage 对应 findImportResultMessage：结果里与候选项名称（搜索名或匹配到的名称）对应的一条。
func resultMessage(c Candidate, names []string) string {
	var own []string
	if c.SearchName != "" {
		own = append(own, c.SearchName)
	}
	if c.MatchedGame != nil && c.MatchedGame.Name != "" {
		own = append(own, c.MatchedGame.Name)
	}
	for _, r := range names {
		for _, n := range own {
			if r == n || strings.HasPrefix(r, n+" (") {
				return r
			}
		}
	}
	return ""
}

// Import 对应 handleImport：导入选中的候选项（matchedOnly 时只导入已匹配的）。导入成功的从列表里移除，跳过与失败的
// 取消选中并显示原因。onDone 在成功后、onFailed 在请求失败后调用；有导入成功的游戏时提示并调用 onComplete。
func (f *Flow) Import(c *ui.Context, matchedOnly bool, onDone, onFailed, onComplete func()) {
	var submitted []int
	for i, cand := range f.Candidates {
		if shouldImport(cand, matchedOnly) {
			submitted = append(submitted, i)
		}
	}
	req := importRequest(f.Candidates, matchedOnly)
	svc, gen := f.env.Svc.Import, f.gen
	f.env.Async(func() func() {
		result, err := svc.BatchImportGames(req)
		return func() {
			if gen != f.gen {
				return
			}
			if err != nil {
				log.Printf("import: batch import: %v", err)
				f.env.Toast(c, env.Error(locales.BatchImportModalToastImportFailed()))
				onFailed()
				return
			}
			f.Result = &result
			f.applyImportResult(submitted, result)
			onDone()
			if result.Success > 0 {
				f.env.Toast(c, env.Success(locales.BatchImportModalToastImportSuccess(strconv.Itoa(result.Success))))
				if onComplete != nil {
					onComplete()
				}
			}
		}
	})
}

// applyImportResult 是 handleImport 里 setCandidates 的部分：跳过的、失败的留下并取消选中，其余按成功数量依次移除。
func (f *Flow) applyImportResult(submitted []int, r service.ImportResult) {
	isSubmitted := map[int]bool{}
	for _, i := range submitted {
		isSubmitted[i] = true
	}
	skipped, failed := map[int]string{}, map[int]string{}
	for _, i := range submitted {
		if msg := resultMessage(f.Candidates[i], r.SkippedNames); msg != "" {
			skipped[i] = msg
			continue
		}
		if msg := resultMessage(f.Candidates[i], r.FailedNames); msg != "" {
			failed[i] = msg
		}
	}
	remaining := r.Success
	out := f.Candidates[:0:0]
	for i, cand := range f.Candidates {
		if !isSubmitted[i] {
			out = append(out, cand)
			continue
		}
		if msg, ok := skipped[i]; ok {
			cand.Selected, cand.MatchError = false, msg
			out = append(out, cand)
			continue
		}
		if msg, ok := failed[i]; ok {
			cand.Selected, cand.MatchStatus, cand.MatchError = false, MatchError, msg
			out = append(out, cand)
			continue
		}
		if remaining > 0 {
			remaining--
			continue
		}
		cand.Selected = false
		out = append(out, cand)
	}
	f.Candidates = out
}

// toggle 对应 toggleCandidate。
func (f *Flow) toggle(i int) { f.Candidates[i].Selected = !f.Candidates[i].Selected }

// toggleAll 对应 toggleAllCandidates：只改 indexes 里的候选项（为 nil 时全部）。
func (f *Flow) toggleAll(checked bool, indexes []int) {
	for i := range f.Candidates {
		if indexes == nil || slices.Contains(indexes, i) {
			f.Candidates[i].Selected = checked
		}
	}
}

// setSearchName 对应 updateSearchName：改名后回到待匹配。
func (f *Flow) setSearchName(i int, name string) {
	c := &f.Candidates[i]
	c.SearchName = name
	c.clearMatch(MatchPending, "")
	f.Pause = ""
}

// openManual 对应 openManualSelect：显示自动匹配时得到的全部结果；没有时按名称重新搜索。
func (f *Flow) openManual(i int, s matchSettings, enabled []enums.SourceType) {
	m := &f.manual
	src := m.source
	if !slices.Contains(enabled, src) {
		src = enums.Bangumi
		if len(enabled) > 0 {
			src = enabled[0]
		}
	}
	*m = manualState{open: true, index: i, matches: f.Candidates[i].AllMatches, source: src}
	if len(m.matches) > 0 {
		return
	}
	m.searching = true
	name, svc, gen := f.Candidates[i].SearchName, f.env.Svc.Import, f.gen
	f.env.Async(func() func() {
		r, err := svc.FetchMetadataForCandidateWithPreference(name, s.preferred)
		return func() {
			if gen != f.gen || !m.open || m.index != i {
				return
			}
			m.searching = false
			if err != nil {
				log.Printf("import: manual search %s: %v", name, err)
				return
			}
			m.matches = r.Matches
		}
	})
}

// selectManual 对应 selectManualMatch：用选中的结果作为手动匹配，检查元数据重复后关闭。
func (f *Flow) selectManual(match vo.GameMetadataFromWebVO) {
	m := &f.manual
	if match.Game.Name == "" || !m.open || m.index >= len(f.Candidates) {
		return
	}
	i := m.index
	g := match.Game
	c := &f.Candidates[i]
	c.MatchedGame, c.MatchedTags, c.MatchSource = &g, match.Tags, match.Source
	c.MatchStatus, c.MatchError = MatchManual, ""
	f.applyDuplicateHints([]int{i}, func() { m.open = false })
}

// searchByID 对应 handleSearchById：按来源与 ID 取元数据，找到就选中它。
func (f *Flow) searchByID(c *ui.Context) {
	m := &f.manual
	id := m.id
	if id == "" || !m.open {
		return
	}
	m.searching = true
	svc, src, gen := f.env.Svc.Game, m.source, f.gen
	f.env.Async(func() func() {
		meta, err := svc.FetchMetadataFromWeb(vo.MetadataRequest{Source: src, ID: id})
		return func() {
			if gen != f.gen {
				return
			}
			m.searching = false
			switch {
			case err != nil:
				log.Printf("import: fetch by id %s/%s: %v", src, id, err)
				f.env.Toast(c, env.Error(locales.BatchImportModalToastFetchFailed()))
			case meta.Game.Name == "":
				f.env.Toast(c, env.Error(locales.BatchImportModalToastGameNotFound()))
			default:
				f.selectManual(meta)
			}
		}
	})
}

// skipMetadata 对应 handleSkipMetadata：不使用元数据导入（标成未找到）。
func (f *Flow) skipMetadata() {
	m := &f.manual
	if !m.open || m.index >= len(f.Candidates) {
		return
	}
	f.Candidates[m.index].clearMatch(MatchNotFound, "")
	m.open = false
}
