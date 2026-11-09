package edit

import (
	"strconv"
	"strings"

	"lunabox/backend/common/enums"
	"lunabox/backend/models"
	"lunabox/desktop/ui/metasource"
)

// sourceLabel 对应 sourceOptions 的 label（与 importFlow.ts 的 sourceLabel 相同）；未知来源原样显示。
func sourceLabel(source enums.SourceType) string { return metasource.Label(source) }

// resultSourceLabel 是 importFlow.ts 的 sourceLabel：未知来源显示为 Steam。
func resultSourceLabel(source enums.SourceType) string {
	if !metasource.Known(source) {
		return "Steam"
	}
	return metasource.Label(source)
}

// SourceURL 是 sourceURL，给安装确认弹窗（InstallConfirmModal）等面板以外的地方用。
func SourceURL(source enums.SourceType, id, erogameScapeBase string) string {
	return sourceURL(source, id, erogameScapeBase)
}

// sourceURL 对应 utils/metadataSources.ts 的 getMetadataSourceURL。
func sourceURL(source enums.SourceType, id, erogameScapeBase string) string {
	return metasource.URL(source, id, erogameScapeBase)
}

// defaultSource 对应 defaultMetadataSource*：默认来源的类型与 ID（来源列表里没有时用游戏上的字段）。
func defaultSource(g *models.Game) (enums.SourceType, string) {
	for _, s := range g.MetadataSources {
		if s.SourceType == g.SourceType {
			return s.SourceType, s.SourceID
		}
	}
	return g.SourceType, g.SourceID
}

// configured 对应 configuredSourceTypes。
func configured(g *models.Game, source enums.SourceType) bool {
	for _, s := range g.MetadataSources {
		if s.SourceType == source {
			return true
		}
	}
	return false
}

// isRemoteCoverURL 对应 isRemoteCoverURL：http(s) 且不是 wails.localhost 的本地地址。
func isRemoteCoverURL(u string) bool {
	u = strings.ToLower(strings.TrimSpace(u))
	return (strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")) && !strings.Contains(u, "wails.localhost")
}

// executableDisplayPath 对应 getExecutableDisplayPath：可执行文件在游戏目录下时显示相对路径。
func executableDisplayPath(path, dir string) string {
	dir = strings.TrimRight(strings.ReplaceAll(dir, "/", `\`), `\`)
	prefix := dir + `\`
	if dir != "" && strings.HasPrefix(strings.ToLower(strings.ReplaceAll(path, "/", `\`)), strings.ToLower(prefix)) {
		return path[len(prefix):]
	}
	return path
}

// resolveExecutablePath 对应 resolveExecutablePath：相对路径拼到游戏目录下。
func resolveExecutablePath(path, dir string) string {
	if path == "" || dir == "" || isAbsWindowsPath(path) {
		return path
	}
	dir = strings.TrimRight(dir, `\/`)
	sep := "/"
	if strings.Contains(dir, `\`) {
		sep = `\`
	}
	return dir + sep + strings.TrimLeft(path, `\/`)
}

// isAbsWindowsPath 是 /^(?:[a-z]:[\\/]|[\\/]{2})/i。
func isAbsWindowsPath(p string) bool {
	if len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		ch := p[0] | 0x20
		return ch >= 'a' && ch <= 'z'
	}
	return len(p) >= 2 && (p[0] == '\\' || p[0] == '/') && (p[1] == '\\' || p[1] == '/')
}

// gameWithUpdatedMetadataSourceID 对应 routes/game.tsx 的同名函数：自动保存来源 ID 后同步到页面里的游戏。
func gameWithUpdatedMetadataSourceID(g models.Game, source enums.SourceType, id string) models.Game {
	if g.SourceType == "" || g.SourceType == enums.Local || g.SourceType == source {
		g.SourceType, g.SourceID = source, id
	}
	sources := make([]models.GameMetadataSource, len(g.MetadataSources))
	for i, s := range g.MetadataSources {
		if s.SourceType == source {
			s.SourceID = id
		}
		sources[i] = s
	}
	g.MetadataSources = sources
	return g
}

func parseFloat(s string) float32 {
	f, _ := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(s), "px"), 32)
	return float32(f)
}
