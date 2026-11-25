package api

import (
	"lunabox/backend/plugin"
)

// PluginService 是界面调用插件系统的后端接口（不暴露 moejs）。
type PluginService interface {
	List() []plugin.PluginInfo
	Enable(id string) error
	Disable(id string) error
	Reload(id string) error
	Reorder(ids []string) error
	OpenPluginDir(id string) error

	InspectZip(zipPath string) (plugin.ZipInstallPreview, error)
	InstallFromZip(zipPath string) (plugin.PluginInfo, error)
	Uninstall(id string, deleteData bool) error
	SelectPluginZip() (string, error)
	SelectDevDir() (string, error)
	AddDevPath(dir string) error
	RemoveDevPath(dir string) error
	DevPaths() []string

	// Started 报告启动时启用的插件是否已全部加载（之前读到的主题覆盖还不完整）。
	Started() bool
	Slots(slotName string) []plugin.SlotRegistration
	ThemeOverrides() []plugin.ThemeOverride
	Fonts() []plugin.FontRegistration
	Decorations() []plugin.DecorationRegistration

	GetConfigValues(pluginID string) map[string]any
	SetConfigValues(pluginID string, values map[string]any) error

	MountSlot(instanceID, pluginID, slotName, regID string, props any)
	// UpdateSlot 把变化后的 props 交给已挂载的插槽实例重渲染，插件内的 hook 状态保留。
	UpdateSlot(instanceID, pluginID, slotName, regID string, props any)
	UnmountSlot(instanceID, pluginID string)
	DispatchEvent(ev plugin.UIEvent)
	InvokeAction(pluginID, slotName, regID string, payload any)
	CheckActionWhen(pluginID, slotName, regID string, payload any) bool
	NotifyWindowEvent(pluginID, windowID, event string, payload map[string]any)

	SubscribeState(fn func()) func()
	SubscribeUIChanges(fn func()) func()
	SubscribeTreeUpdates(fn func(plugin.UITreeUpdate)) func()
	SubscribeActions(fn func(plugin.UIAction)) func()
	SubscribeWindowCommands(fn func(plugin.WindowCommand)) func()
}
