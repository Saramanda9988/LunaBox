// Package plugins 是独立的插件管理页面（路由 /plugins），位于侧边栏「收藏」下方（见 docs/plugin-system.md §11）。
package plugins

import (
	"slices"

	"lunabox/backend/plugin"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/env"
)

// Page 是插件管理页的状态。
type Page struct {
	env    *env.Env
	scroll components.ScrollBar

	busy bool

	// zip 安装确认弹窗
	installPreview *plugin.ZipInstallPreview
	installing     bool

	// 卸载确认弹窗
	uninstallTarget *plugin.PluginInfo
	deleteData      bool
}

// New 创建插件管理页。
func New(e *env.Env) *Page {
	return &Page{env: e}
}

// Open 进入页面时重置滚动位置。
func (p *Page) Open() {
	p.scroll = components.ScrollBar{}
}

func extractIDs(list []plugin.PluginInfo) []string {
	ids := make([]string, len(list))
	for i, it := range list {
		ids[i] = it.ID
	}
	return slices.Clone(ids)
}
