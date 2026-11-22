package host

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egoist/mygo/audio"
	_ "github.com/egoist/mygo/audio/mp3"
	_ "github.com/egoist/mygo/audio/vorbis"
)

const maxCachedSounds = 64

var audioLatencyOnce sync.Once

// mygoAudioPlayer 基于 mygo/audio 实现 plugin.AudioPlayer，支持 WAV / MP3 / Ogg Vorbis 内存解码缓存与低延迟混音播放。
type mygoAudioPlayer struct {
	mu     sync.Mutex
	sounds map[string]*audio.Sound
	order  []string
	active map[*audio.Player]string // player -> normalized path
}

func newMygoAudioPlayer() *mygoAudioPlayer {
	return &mygoAudioPlayer{
		sounds: make(map[string]*audio.Sound),
		active: make(map[*audio.Player]string),
	}
}

func normalizeAudioPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return filepath.Clean(path)
}

func (a *mygoAudioPlayer) Play(path string, volume float64) {
	if volume <= 0 {
		return
	}
	if volume > 1 {
		volume = 1
	}
	absPath := normalizeAudioPath(path)
	if absPath == "" {
		return
	}
	audioLatencyOnce.Do(func() {
		audio.SetLatency(audio.LatencyInteractive)
	})

	a.mu.Lock()
	snd, ok := a.sounds[absPath]
	a.mu.Unlock()
	if ok {
		a.startPlayer(absPath, snd, volume)
		return
	}

	// 首次加载异步读取并解码，不阻塞调用方或界面线程
	go func() {
		data, err := os.ReadFile(absPath)
		if err != nil {
			log.Printf("plugin audio: read %s: %v", absPath, err)
			return
		}
		decoded, err := audio.DecodeSound(data)
		if err != nil {
			log.Printf("plugin audio: decode %s: %v", absPath, err)
			return
		}
		a.mu.Lock()
		if _, exists := a.sounds[absPath]; !exists {
			if len(a.order) >= maxCachedSounds {
				evict := a.order[0]
				a.order = a.order[1:]
				delete(a.sounds, evict)
			}
			a.sounds[absPath] = decoded
			a.order = append(a.order, absPath)
		} else {
			decoded = a.sounds[absPath]
		}
		a.mu.Unlock()
		a.startPlayer(absPath, decoded, volume)
	}()
}

func (a *mygoAudioPlayer) startPlayer(absPath string, snd *audio.Sound, volume float64) {
	if snd == nil {
		return
	}
	p := snd.NewPlayer()
	p.SetVolume(volume)
	a.mu.Lock()
	a.active[p] = absPath
	a.mu.Unlock()
	p.OnEnd(func() {
		a.mu.Lock()
		delete(a.active, p)
		a.mu.Unlock()
	})
	p.Play()
}

func (a *mygoAudioPlayer) Stop(path string) {
	target := normalizeAudioPath(path)
	a.mu.Lock()
	var toStop []*audio.Player
	for p, pPath := range a.active {
		if target == "" || strings.EqualFold(pPath, target) {
			toStop = append(toStop, p)
			delete(a.active, p)
		}
	}
	a.mu.Unlock()
	for _, p := range toStop {
		p.Stop()
	}
}
