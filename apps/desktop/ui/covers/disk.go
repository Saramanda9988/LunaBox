package covers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// diskBudget 是磁盘缓存的上限（字节）：卡片封面约 30–60KB，首页大图约 100–200KB。
const diskBudget = 200 << 20

// diskCache 把 AVIF 封面缩小后的图存在系统缓存目录（Windows 是 %LOCALAPPDATA%\LunaBox\covers，Linux 是
// ~/.cache/LunaBox/covers），下次直接读小图：AVIF 只能用 WASM 解码，一张 1400×1000 的要一百多毫秒，
// 第一次还要先花约 0.8 秒编译解码器；其他格式解码本来就快，不缓存。缓存目录可以随时删掉，删了只是重新解码。
//
// 文件名是来源与尺寸的哈希：本地封面的来源包括文件大小与修改时间，换了封面（地址不变）就是新的文件；
// 远程封面是地址本身。读到时更新修改时间，超出 diskBudget 时从最久没用的删起（prune）。
type diskCache struct {
	dir string
}

// openDiskCache 打开缓存目录；拿不到系统缓存目录或建不了目录时返回 nil（不缓存）。
func openDiskCache() *diskCache {
	base, err := os.UserCacheDir()
	if err != nil {
		return nil
	}
	dir := filepath.Join(base, "LunaBox", "covers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("covers: disk cache: %v", err)
		return nil
	}
	return &diskCache{dir: dir}
}

func (d *diskCache) path(id string, width, height int) string {
	sum := sha256.Sum256(fmt.Appendf(nil, "%s\x00%d\x00%d", id, width, height))
	return filepath.Join(d.dir, hex.EncodeToString(sum[:16]))
}

// load 返回缓存的小图，没有或读不了时返回 nil。
func (d *diskCache) load(id string, width, height int) image.Image {
	if d == nil {
		return nil
	}
	path := d.path(id, width, height)
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	now := time.Now()
	os.Chtimes(path, now, now) // 最近用过，prune 时留下
	return img
}

// store 写入缩小后的图：不透明的存 JPEG（质量 92），带透明的存 PNG。先写临时文件再改名，读的一方不会读到一半。
func (d *diskCache) store(id string, width, height int, img image.Image) {
	if d == nil {
		return
	}
	path := d.path(id, width, height)
	f, err := os.CreateTemp(d.dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return
	}
	if o, ok := img.(interface{ Opaque() bool }); ok && o.Opaque() {
		err = jpeg.Encode(f, img, &jpeg.Options{Quality: 92})
	} else {
		err = (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(f, img)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		os.Remove(f.Name())
		log.Printf("covers: disk cache: %v", err)
	}
}

// prune 在总大小超出 budget 时，从最久没用的删起，删到 budget 的 80%；顺带删掉写到一半留下的临时文件。
func (d *diskCache) prune(budget int64) {
	if d == nil {
		return
	}
	entries, err := os.ReadDir(d.dir)
	if err != nil {
		return
	}
	type file struct {
		path string
		size int64
		used time.Time
	}
	var files []file
	var total int64
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		path := filepath.Join(d.dir, e.Name())
		if filepath.Ext(e.Name()) == ".tmp" {
			if time.Since(info.ModTime()) > time.Hour {
				os.Remove(path)
			}
			continue
		}
		files = append(files, file{path, info.Size(), info.ModTime()})
		total += info.Size()
	}
	if total <= budget {
		return
	}
	slices.SortFunc(files, func(a, b file) int { return a.used.Compare(b.used) })
	for _, f := range files {
		if total <= budget*8/10 {
			break
		}
		if os.Remove(f.path) == nil {
			total -= f.size
		}
	}
}
