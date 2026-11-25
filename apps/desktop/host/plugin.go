package host

import (
	"context"
	"encoding/json"
	"log"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"lunabox/backend/plugin"
	"lunabox/desktop/api"
)

// PluginBridge 实现 api.PluginService，并维护开发插件目录的 mygo.WatchFiles 热重载与 mygo/audio 音频播放。
type PluginBridge struct {
	mgr   *plugin.Manager
	audio *mygoAudioPlayer

	mu       sync.Mutex
	watchers map[string]context.CancelFunc
	unsubDev func()
}

func NewPluginBridge(mgr *plugin.Manager) *PluginBridge {
	ap := newMygoAudioPlayer()
	b := &PluginBridge{
		mgr:      mgr,
		audio:    ap,
		watchers: make(map[string]context.CancelFunc),
	}
	if mgr != nil {
		mgr.SetAudioPlayer(ap)
		b.unsubDev = mgr.SubscribeDevDirs(b.syncDevWatchers)
		b.syncDevWatchers(mgr.DevPaths())
	}
	return b
}

// Close 停止所有开发目录文件监听与插件音频播放。
func (b *PluginBridge) Close() {
	if b.audio != nil {
		b.audio.Stop("")
	}
	if b.unsubDev != nil {
		b.unsubDev()
		b.unsubDev = nil
	}
	b.mu.Lock()
	for _, cancel := range b.watchers {
		cancel()
	}
	b.watchers = make(map[string]context.CancelFunc)
	b.mu.Unlock()
}

func (b *PluginBridge) syncDevWatchers(dirs []string) {
	desired := make(map[string]struct{}, len(dirs))
	for _, d := range dirs {
		clean := filepath.Clean(strings.TrimSpace(d))
		if clean != "" {
			desired[clean] = struct{}{}
		}
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	for dir, cancel := range b.watchers {
		if _, keep := desired[dir]; !keep {
			cancel()
			delete(b.watchers, dir)
		}
	}
	for dir := range desired {
		if _, exists := b.watchers[dir]; exists {
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		b.watchers[dir] = cancel
		go b.watchDevDir(ctx, dir)
	}
}

func (b *PluginBridge) watchDevDir(ctx context.Context, dir string) {
	watcher, err := mygo.WatchFiles(ctx, dir, mygo.FileWatchOptions{Recursive: true})
	if err != nil {
		log.Printf("plugin watch %s failed: %v", dir, err)
		return
	}
	defer watcher.Close()

	var (
		timerMu sync.Mutex
		timer   *time.Timer
	)
	trigger := func() {
		timerMu.Lock()
		if timer != nil {
			timer.Reset(200 * time.Millisecond)
		} else {
			timer = time.AfterFunc(200*time.Millisecond, func() {
				if ctx.Err() == nil {
					b.mgr.ReloadByDir(dir)
				}
			})
		}
		timerMu.Unlock()
	}
	defer func() {
		timerMu.Lock()
		if timer != nil {
			timer.Stop()
		}
		timerMu.Unlock()
	}()

	for {
		events, err := watcher.Next(ctx)
		if err != nil {
			return
		}
		shouldReload := false
		for _, ev := range events {
			name := filepath.ToSlash(ev.Path)
			if strings.HasPrefix(name, "node_modules/") || strings.HasPrefix(name, ".git/") {
				continue
			}
			shouldReload = true
			break
		}
		if shouldReload {
			trigger()
		}
	}
}

func (b *PluginBridge) List() []plugin.PluginInfo                           { return b.mgr.List() }
func (b *PluginBridge) Enable(id string) error                              { return b.mgr.Enable(id) }
func (b *PluginBridge) Disable(id string) error                             { return b.mgr.Disable(id) }
func (b *PluginBridge) Reload(id string) error                              { return b.mgr.Reload(id) }
func (b *PluginBridge) Reorder(ids []string) error                          { return b.mgr.Reorder(ids) }
func (b *PluginBridge) OpenPluginDir(id string) error                       { return b.mgr.OpenPluginDir(id) }
func (b *PluginBridge) InspectZip(p string) (plugin.ZipInstallPreview, error) { return b.mgr.InspectZip(p) }
func (b *PluginBridge) InstallFromZip(p string) (plugin.PluginInfo, error)  { return b.mgr.InstallFromZip(p) }
func (b *PluginBridge) Uninstall(id string, deleteData bool) error          { return b.mgr.Uninstall(id, deleteData) }
func (b *PluginBridge) SelectPluginZip() (string, error)                    { return b.mgr.SelectPluginZip() }
func (b *PluginBridge) SelectDevDir() (string, error)                       { return b.mgr.SelectDevDir() }
func (b *PluginBridge) AddDevPath(dir string) error                         { return b.mgr.AddDevPath(dir) }
func (b *PluginBridge) RemoveDevPath(dir string) error                      { return b.mgr.RemoveDevPath(dir) }
func (b *PluginBridge) DevPaths() []string                                  { return slices.Clone(b.mgr.DevPaths()) }
func (b *PluginBridge) Started() bool                                       { return b.mgr.Started() }
func (b *PluginBridge) Slots(slotName string) []plugin.SlotRegistration     { return b.mgr.UI().Slots(slotName) }
func (b *PluginBridge) ThemeOverrides() []plugin.ThemeOverride              { return b.mgr.UI().ThemeOverrides() }
func (b *PluginBridge) Fonts() []plugin.FontRegistration                    { return b.mgr.UI().Fonts() }
func (b *PluginBridge) Decorations() []plugin.DecorationRegistration        { return b.mgr.UI().Decorations() }
func (b *PluginBridge) GetConfigValues(id string) map[string]any            { return b.mgr.GetConfigValues(id) }
func (b *PluginBridge) SetConfigValues(id string, v map[string]any) error   { return b.mgr.SetConfigValues(id, v) }
func (b *PluginBridge) MountSlot(inst, pid, slot, reg string, props any)    { b.mgr.MountSlot(inst, pid, slot, reg, props) }
func (b *PluginBridge) UnmountSlot(inst, pid string)                        { b.mgr.UnmountSlot(inst, pid) }
func (b *PluginBridge) UpdateSlot(inst, pid, slot, reg string, props any) {
	b.mgr.UpdateSlot(inst, pid, slot, reg, props)
}
func (b *PluginBridge) DispatchEvent(ev plugin.UIEvent)                     { b.mgr.DispatchEvent(ev) }
func (b *PluginBridge) InvokeAction(pid, slot, reg string, payload any)     { b.mgr.InvokeAction(pid, slot, reg, payload) }
func (b *PluginBridge) CheckActionWhen(pid, slot, reg string, payload any) bool {
	return b.mgr.CheckActionWhen(pid, slot, reg, payload)
}
func (b *PluginBridge) NotifyWindowEvent(pid, wid, ev string, payload map[string]any) {
	b.mgr.NotifyWindowEvent(pid, wid, ev, payload)
}
func (b *PluginBridge) ResolveDialog(pid, did string, result any) {
	var raw json.RawMessage
	if result != nil {
		raw, _ = json.Marshal(result)
	}
	b.mgr.ResolveDialog(pid, did, raw)
}
func (b *PluginBridge) SubscribeState(fn func()) func() { return b.mgr.SubscribeState(fn) }
func (b *PluginBridge) SubscribeUIChanges(fn func()) func() {
	return b.mgr.UI().SubscribeChanges(fn)
}
func (b *PluginBridge) SubscribeTreeUpdates(fn func(plugin.UITreeUpdate)) func() {
	return b.mgr.UI().SubscribeTreeUpdates(fn)
}
func (b *PluginBridge) SubscribeActions(fn func(plugin.UIAction)) func() {
	return b.mgr.UI().SubscribeActions(fn)
}
func (b *PluginBridge) SubscribeWindowCommands(fn func(plugin.WindowCommand)) func() {
	return b.mgr.UI().SubscribeWindowCommands(fn)
}

var _ api.PluginService = (*PluginBridge)(nil)
