// Package plugins 是独立的插件管理页面（路由 /plugins 与 /plugins/{id}），位于侧边栏「收藏」下方（见 docs/plugin-system.md §11）。
package plugins

import (
	"slices"

	"lunabox/backend/plugin"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/pluginui"
)

// Page 是插件列表与插件详情页的状态。
type Page struct {
	env     *env.Env
	plugins *pluginui.Manager

	scroll       components.ScrollBar
	detailScroll components.ScrollBar

	busy bool

	// zip 安装确认弹窗
	installPreview *plugin.ZipInstallPreview
	installing     bool

	// 卸载确认弹窗
	uninstallTarget     *plugin.PluginInfo
	deleteData          bool
	uninstallBackToList bool
}

// New 创建插件管理页。
func New(e *env.Env) *Page {
	return &Page{env: e}
}

// SetPluginUI 注入插件界面渲染器（用于详情页渲染插件设置表单/插槽与图标异步解码）。
func (p *Page) SetPluginUI(mgr *pluginui.Manager) {
	p.plugins = mgr
}

// Open 进入插件列表页时重置滚动位置。
func (p *Page) Open() {
	p.scroll = components.ScrollBar{}
}

// OpenDetail 进入插件详情页时重置滚动位置。
func (p *Page) OpenDetail(_ string) {
	p.detailScroll = components.ScrollBar{}
}

// DetailTitle 返回插件详情页的窗口标题。
func (p *Page) DetailTitle(id string) string {
	if _, _, info, ok := p.findPlugin(id); ok && info.Name != "" {
		return info.Name
	}
	return locales.NativeSideBarPlugins()
}

func (p *Page) findPlugin(id string) ([]plugin.PluginInfo, int, plugin.PluginInfo, bool) {
	if p.env == nil || p.env.Svc == nil || p.env.Svc.Plugin == nil {
		return nil, -1, plugin.PluginInfo{}, false
	}
	list := p.env.Svc.Plugin.List()
	for i, info := range list {
		if info.ID == id {
			return list, i, info, true
		}
	}
	return list, -1, plugin.PluginInfo{}, false
}

func extractIDs(list []plugin.PluginInfo) []string {
	ids := make([]string, len(list))
	for i, it := range list {
		ids[i] = it.ID
	}
	return slices.Clone(ids)
}
