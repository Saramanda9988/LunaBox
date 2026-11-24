package pluginui

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"
	_ "github.com/gen2brain/avif"
	_ "golang.org/x/image/webp"

	"lunabox/backend/utils/httputils"
	"lunabox/backend/version"
	"lunabox/desktop/ui/assets"
)

const (
	pluginImageMaxDim     = 1600
	pluginImageMemBudget  = 160 << 20 // 内存中最多保留约 160MB 已解码位图（LRU 淘汰）
	pluginImageDiskBudget = 300 << 20 // 本地磁盘缓存上限 300MB（LRU 清理）
	pluginImageMaxBytes   = 24 << 20  // 单张远程图片最大 24MB
	pluginImageDecodeCap  = 4         // 并发图片解码上限，避免截图墙首次解码内存飙升
	pluginImageFailTTL    = 30 * time.Second
)

type imgCacheEntry struct {
	key   string
	bmp   *ui.Bitmap
	bytes int64
}

// imageDiskCache 把插件加载的远程图片（缩放后的位图与原始 SVG）缓存在系统缓存目录
// （Windows: %LOCALAPPDATA%\LunaBox\plugin-images，Linux: ~/.cache/LunaBox/plugin-images），
// 跨会话复用，避免每次重启或打开游戏详情页时重新走网络下载导致先黑再显示。
type imageDiskCache struct {
	dir     string
	writes  atomic.Int32
	pruneMu sync.Mutex
}

func openImageDiskCache() *imageDiskCache {
	base, err := os.UserCacheDir()
	if err != nil {
		return nil
	}
	return openImageDiskCacheAt(filepath.Join(base, "LunaBox", "plugin-images"))
}

func openImageDiskCacheAt(dir string) *imageDiskCache {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("pluginui: image disk cache: %v", err)
		return nil
	}
	return &imageDiskCache{dir: dir}
}

func (d *imageDiskCache) path(kind, src, ext string) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + src))
	return filepath.Join(d.dir, hex.EncodeToString(sum[:16])+ext)
}

func (d *imageDiskCache) loadBitmap(src string) (image.Image, bool) {
	if d == nil {
		return nil, false
	}
	p := d.path("bmp", src, ".img")
	f, err := os.Open(p)
	if err != nil {
		return nil, false
	}
	img, _, derr := image.Decode(f)
	_ = f.Close()
	if derr != nil || img == nil {
		_ = os.Remove(p)
		return nil, false
	}
	now := time.Now()
	_ = os.Chtimes(p, now, now)
	return img, true
}

func (d *imageDiskCache) storeBitmap(src string, img image.Image) {
	if d == nil || img == nil {
		return
	}
	p := d.path("bmp", src, ".img")
	f, err := os.CreateTemp(d.dir, filepath.Base(p)+".*.tmp")
	if err != nil {
		return
	}
	if o, ok := img.(interface{ Opaque() bool }); ok && o.Opaque() {
		err = jpeg.Encode(f, img, &jpeg.Options{Quality: 90})
	} else {
		err = (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(f, img)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), p)
	}
	if err != nil {
		_ = os.Remove(f.Name())
		log.Printf("pluginui: image disk cache store: %v", err)
		return
	}
	if d.writes.Add(1)%32 == 0 {
		d.prune(pluginImageDiskBudget)
	}
}

func (d *imageDiskCache) loadBytes(src string) ([]byte, bool) {
	if d == nil {
		return nil, false
	}
	p := d.path("svg", src, ".svg")
	data, err := os.ReadFile(p)
	if err != nil || len(data) == 0 {
		return nil, false
	}
	now := time.Now()
	_ = os.Chtimes(p, now, now)
	return data, true
}

func (d *imageDiskCache) storeBytes(src string, data []byte) {
	if d == nil || len(data) == 0 {
		return
	}
	p := d.path("svg", src, ".svg")
	f, err := os.CreateTemp(d.dir, filepath.Base(p)+".*.tmp")
	if err != nil {
		return
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), p)
	}
	if err != nil {
		_ = os.Remove(f.Name())
		return
	}
	if d.writes.Add(1)%32 == 0 {
		d.prune(pluginImageDiskBudget)
	}
}

func (d *imageDiskCache) prune(budget int64) {
	if d == nil || budget <= 0 {
		return
	}
	if !d.pruneMu.TryLock() {
		return
	}
	defer d.pruneMu.Unlock()

	entries, err := os.ReadDir(d.dir)
	if err != nil {
		return
	}
	type fileInfo struct {
		path string
		size int64
		used time.Time
	}
	var files []fileInfo
	var total int64
	for _, e := range entries {
		info, ierr := e.Info()
		if ierr != nil || !info.Mode().IsRegular() {
			continue
		}
		p := filepath.Join(d.dir, e.Name())
		if filepath.Ext(e.Name()) == ".tmp" {
			if time.Since(info.ModTime()) > time.Hour {
				_ = os.Remove(p)
			}
			continue
		}
		files = append(files, fileInfo{path: p, size: info.Size(), used: info.ModTime()})
		total += info.Size()
	}
	if total <= budget {
		return
	}
	slices.SortFunc(files, func(a, b fileInfo) int { return a.used.Compare(b.used) })
	target := budget * 8 / 10
	for _, f := range files {
		if total <= target {
			break
		}
		if os.Remove(f.path) == nil {
			total -= f.size
		}
	}
}

// enableImageDiskCache 为测试提供可控目录的磁盘缓存初始化。
func (m *Manager) enableImageDiskCache(dir string) {
	m.imgMu.Lock()
	m.imgDisk = openImageDiskCacheAt(dir)
	m.imgMu.Unlock()
}

func (m *Manager) getDiskCache() *imageDiskCache {
	m.imgMu.Lock()
	d := m.imgDisk
	m.imgMu.Unlock()
	return d
}

func (m *Manager) dataDir() string {
	if m.env == nil {
		return ""
	}
	if m.env.Svc != nil && m.env.Svc.DataDir != "" {
		return m.env.Svc.DataDir
	}
	if m.env.Backend != nil {
		return m.env.Backend.DataDir()
	}
	return ""
}

// resolveLocalPath 解析图片路径：/local/... 映射到应用数据目录，http(s) 返回 isLocal=false，其余作为本地路径。
func (m *Manager) resolveLocalPath(src string) (path string, isLocal bool) {
	if i := strings.Index(src, "/local/"); i >= 0 {
		rel := strings.TrimPrefix(src[i:], "/local/")
		if j := strings.IndexAny(rel, "?#"); j >= 0 {
			rel = rel[:j]
		}
		rel = filepath.Clean(filepath.FromSlash(rel))
		if rel == "" || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			return "", true
		}
		if dir := m.dataDir(); dir != "" {
			return filepath.Join(dir, rel), true
		}
		return "", true
	}
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		return "", false
	}
	return src, true
}

func (m *Manager) peekCover(src string) *ui.Bitmap {
	if m.env == nil || m.env.Covers == nil {
		return nil
	}
	return m.env.Covers.Peek(src)
}

func (m *Manager) invalidateWindows() {
	if m.env == nil || m.env.Win == nil {
		return
	}
	m.env.Win.Update(func() {
		for _, w := range m.windows {
			if w.win != nil {
				w.win.Invalidate()
			}
		}
	})
}

func (m *Manager) getCachedBitmap(src string) *ui.Bitmap {
	m.imgMu.Lock()
	defer m.imgMu.Unlock()
	if el, ok := m.imgCache[src]; ok {
		m.imgLRU.MoveToFront(el)
		return el.Value.(*imgCacheEntry).bmp
	}
	return nil
}

func (m *Manager) putCachedBitmap(src string, bmp *ui.Bitmap) {
	w, h := bmp.Size()
	size := max(int64(1), int64(4)*int64(w)*int64(h))

	m.imgMu.Lock()
	defer m.imgMu.Unlock()

	if prev, ok := m.imgCache[src]; ok {
		pe := prev.Value.(*imgCacheEntry)
		m.imgBytes -= pe.bytes
		pe.bmp = bmp
		pe.bytes = size
		m.imgBytes += size
		m.imgLRU.MoveToFront(prev)
	} else {
		e := &imgCacheEntry{key: src, bmp: bmp, bytes: size}
		m.imgCache[src] = m.imgLRU.PushFront(e)
		m.imgBytes += size
	}
	delete(m.imgFailed, src)

	for m.imgBytes > pluginImageMemBudget && m.imgLRU.Len() > 1 {
		back := m.imgLRU.Back()
		if back == nil {
			break
		}
		be := m.imgLRU.Remove(back).(*imgCacheEntry)
		delete(m.imgCache, be.key)
		m.imgBytes -= be.bytes
	}
}

// isImageFailed 返回指定图片来源最近一次加载是否失败（用于 Image 的 onError 去重回调）。
func (m *Manager) isImageFailed(src string) bool {
	src = strings.TrimSpace(src)
	if src == "" {
		return false
	}
	m.imgMu.Lock()
	_, failed := m.imgFailed[src]
	m.imgMu.Unlock()
	return failed
}

// LoadImageSource 异步加载本地或远程图标/图片（支持 SVG、PNG、JPEG、GIF、WebP、AVIF）并缓存为 ui.ImageSource，绝不阻塞界面线程。
func (m *Manager) LoadImageSource(src string) ui.ImageSource {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil
	}
	lowerNoQuery := strings.ToLower(strings.SplitN(src, "?", 2)[0])
	if strings.HasSuffix(lowerNoQuery, ".svg") {
		m.imgMu.Lock()
		if svg, ok := m.svgCache[src]; ok {
			m.imgMu.Unlock()
			if svg != nil {
				return svg
			}
			return nil
		}
		if m.imgLoading[src] {
			m.imgMu.Unlock()
			return nil
		}
		if failedAt, ok := m.imgFailed[src]; ok && time.Since(failedAt) < pluginImageFailTTL {
			m.imgMu.Unlock()
			return nil
		}
		m.imgLoading[src] = true
		disk := m.imgDisk
		m.imgMu.Unlock()

		go func() {
			var data []byte
			var err error
			var saveToDisk func()

			localPath, isLocal := m.resolveLocalPath(src)
			if isLocal {
				if localPath == "" {
					err = os.ErrNotExist
				} else {
					data, err = os.ReadFile(localPath)
				}
			} else {
				if cached, hit := disk.loadBytes(src); hit {
					data = cached
				} else {
					data, err = m.fetchRemoteBytes(src)
					if err == nil && len(data) > 0 && disk != nil {
						raw := data
						saveToDisk = func() { disk.storeBytes(src, raw) }
					}
				}
			}

			var svg *ui.SVG
			if err == nil && len(data) > 0 {
				if parsed, perr := ui.ParseSVG(data); perr == nil {
					svg = parsed
				}
			}
			m.imgMu.Lock()
			delete(m.imgLoading, src)
			if svg != nil {
				if len(m.svgCache) >= 256 {
					clear(m.svgCache)
				}
				m.svgCache[src] = svg
				delete(m.imgFailed, src)
			} else {
				m.imgFailed[src] = time.Now()
			}
			m.imgMu.Unlock()

			m.invalidateWindows()
			if saveToDisk != nil && svg != nil {
				saveToDisk()
			}
		}()
		return nil
	}
	if bmp := m.loadBitmap(src); bmp != nil {
		return bmp
	}
	return nil
}

func (m *Manager) newHTTPClient() *http.Client {
	opts := httputils.ClientOptions{
		Timeout: 15 * time.Second,
	}
	if m.env != nil && (m.env.Svc != nil || m.env.Backend != nil) {
		opts.ProxyConfig = m.env.Config()
	}
	client, _, cerr := httputils.NewClient(opts)
	if cerr != nil || client == nil {
		return &http.Client{Timeout: 15 * time.Second}
	}
	return client
}

func (m *Manager) fetchRemoteBytes(src string) ([]byte, error) {
	client := m.newHTTPClient()
	req, err := http.NewRequest(http.MethodGet, src, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", version.UserAgent())
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, os.ErrNotExist
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, pluginImageMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > pluginImageMaxBytes {
		return nil, os.ErrInvalid
	}
	return data, nil
}

// loadBitmap 异步加载本地路径（含 /local/...）或 http(s) URL 图片并缓存为 *ui.Bitmap（内存 LRU + 磁盘持久化），
// 绝不阻塞界面线程。若封面缓存中已有同源位图，加载期间会先返回该位图避免首帧黑屏。
func (m *Manager) loadBitmap(src string) *ui.Bitmap {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil
	}
	if bmp := m.getCachedBitmap(src); bmp != nil {
		return bmp
	}
	peek := m.peekCover(src)

	m.imgMu.Lock()
	if m.imgLoading[src] {
		m.imgMu.Unlock()
		return peek
	}
	if failedAt, ok := m.imgFailed[src]; ok && time.Since(failedAt) < pluginImageFailTTL {
		m.imgMu.Unlock()
		return peek
	}
	m.imgLoading[src] = true
	disk := m.imgDisk
	m.imgMu.Unlock()

	go func() {
		var scaled image.Image
		var err error
		var saveToDisk func()

		localPath, isLocal := m.resolveLocalPath(src)
		if isLocal {
			if localPath == "" {
				err = os.ErrNotExist
			} else {
				f, ferr := os.Open(localPath)
				if ferr != nil {
					err = ferr
				} else {
					m.decodeSlots <- struct{}{}
					img, _, derr := image.Decode(f)
					_ = f.Close()
					if derr == nil && img != nil {
						scaled = assets.Downscale(img, pluginImageMaxDim)
					} else {
						err = derr
					}
					<-m.decodeSlots
				}
			}
		} else {
			var cachedImg image.Image
			var hit bool
			if disk != nil {
				m.decodeSlots <- struct{}{}
				cachedImg, hit = disk.loadBitmap(src)
				<-m.decodeSlots
			}
			if hit {
				scaled = cachedImg
			} else {
				var data []byte
				data, err = m.fetchRemoteBytes(src)
				if err == nil && len(data) > 0 {
					m.decodeSlots <- struct{}{}
					img, _, derr := image.Decode(bytes.NewReader(data))
					if derr == nil && img != nil {
						scaled = assets.Downscale(img, pluginImageMaxDim)
						if disk != nil {
							toSave := scaled
							saveToDisk = func() { disk.storeBitmap(src, toSave) }
						}
					} else {
						err = derr
					}
					<-m.decodeSlots
				}
			}
		}

		var bmp *ui.Bitmap
		if err == nil && scaled != nil {
			bmp = ui.NewBitmap(scaled)
		}
		if bmp != nil {
			m.putCachedBitmap(src, bmp)
		}
		m.imgMu.Lock()
		delete(m.imgLoading, src)
		if bmp == nil {
			m.imgFailed[src] = time.Now()
		}
		m.imgMu.Unlock()

		// 先触发重绘让界面立刻上屏，再把缩放后的图片落盘，不阻塞显示
		m.invalidateWindows()
		if saveToDisk != nil && bmp != nil {
			saveToDisk()
		}
	}()
	return peek
}
