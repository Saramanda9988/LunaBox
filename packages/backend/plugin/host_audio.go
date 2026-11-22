package plugin

import (
	"encoding/json"
	"strings"

	"github.com/MoeForge/moejs"
)

// AudioPlayer 是由桌面宿主（apps/desktop/host 基于 mygo/audio）注入的音频播放接口，
// 保持 packages/backend 不直接依赖 GUI 框架。
type AudioPlayer interface {
	Play(path string, volume float64)
	Stop(path string)
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (m *Manager) getAudioPlayer() AudioPlayer {
	m.mu.RLock()
	ap := m.audioPlayer
	m.mu.RUnlock()
	return ap
}

func (p *Plugin) buildAudioAPI() map[string]any {
	return map[string]any{
		"play": p.safeNative("audio.play", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			src := strings.TrimSpace(argString(args, 0))
			if src == "" {
				return moejs.Undefined(), nil
			}
			volume := 1.0
			if len(args) > 1 {
				if args[1].IsNumber() {
					volume = args[1].AsNumber()
				} else if args[1].IsObject() {
					if raw, err := p.rt.AppendJSON(nil, args[1]); err == nil {
						var opts struct {
							Volume *float64 `json:"volume"`
						}
						if json.Unmarshal(raw, &opts) == nil && opts.Volume != nil {
							volume = *opts.Volume
						}
					}
				}
			}
			if ap := p.manager.getAudioPlayer(); ap != nil {
				ap.Play(p.resolvePluginAssetPath(src), clampFloat(volume, 0, 1))
			}
			return moejs.Undefined(), nil
		}),
		"stop": p.safeNative("audio.stop", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			src := strings.TrimSpace(argString(args, 0))
			if src != "" {
				src = p.resolvePluginAssetPath(src)
			}
			if ap := p.manager.getAudioPlayer(); ap != nil {
				ap.Stop(src)
			}
			return moejs.Undefined(), nil
		}),
	}
}
