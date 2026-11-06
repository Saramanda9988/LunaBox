package gamehelper

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lunabox/backend/common/vo"
	"lunabox/backend/models"
)

const (
	pathCheckWorkers     = 8
	volumeCheckTimeout   = 3 * time.Second // 断开的网络共享上 os.Stat 可能卡很久
	missingPathCheckSkip = "://"           // URL 形式的路径不是本地文件
)

// CheckGamePaths 检查每个游戏的启动路径（game.Path）是否还存在。
//
// 路径为空或是 URL 的游戏跳过，不计入 Checked。所在磁盘或挂载点不可用（移动硬盘没插、网络共享断开）时
// 无法判断文件是否真的不在了，归入 Unavailable 而不是 Missing，避免被当成失效游戏删掉。
// 除“不存在”以外的错误（如没有权限）视为存在。
func CheckGamePaths(ctx context.Context, games []models.Game) vo.MissingPathScanResult {
	result := vo.MissingPathScanResult{
		Missing:     []vo.MissingPathGame{},
		Unavailable: []vo.MissingPathGame{},
	}

	type job struct {
		index int
		game  vo.MissingPathGame
		clean string
	}
	jobs := make([]job, 0, len(games))
	for _, g := range games {
		path := strings.TrimSpace(g.Path)
		if path == "" || strings.Contains(path, missingPathCheckSkip) {
			continue
		}
		jobs = append(jobs, job{
			index: len(jobs),
			game:  vo.MissingPathGame{GameID: g.ID, Name: g.Name, Path: path},
			clean: filepath.Clean(path),
		})
	}
	result.Checked = len(jobs)
	if len(jobs) == 0 {
		return result
	}

	volumes := newVolumeChecker()
	states := make([]pathState, len(jobs))
	work := make(chan job)
	var wg sync.WaitGroup
	for range min(pathCheckWorkers, len(jobs)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range work {
				if !volumes.available(volumeRoot(j.clean)) {
					states[j.index] = pathVolumeUnavailable
					continue
				}
				if _, err := os.Stat(j.clean); errors.Is(err, fs.ErrNotExist) {
					states[j.index] = pathMissing
				}
			}
		}()
	}
	for _, j := range jobs {
		if ctx.Err() != nil {
			break
		}
		work <- j
	}
	close(work)
	wg.Wait()

	for _, j := range jobs {
		switch states[j.index] {
		case pathMissing:
			result.Missing = append(result.Missing, j.game)
		case pathVolumeUnavailable:
			result.Unavailable = append(result.Unavailable, j.game)
		}
	}
	return result
}

type pathState uint8

const (
	pathOK pathState = iota
	pathMissing
	pathVolumeUnavailable
)

// volumeChecker 对每个磁盘根目录或挂载点只检查一次，并给检查加上超时。
type volumeChecker struct {
	mu    sync.Mutex
	cache map[string]*volumeCheck
}

type volumeCheck struct {
	once sync.Once
	ok   bool
}

func newVolumeChecker() *volumeChecker {
	return &volumeChecker{cache: map[string]*volumeCheck{}}
}

func (v *volumeChecker) available(root string) bool {
	if root == "" {
		return true
	}
	v.mu.Lock()
	check, ok := v.cache[root]
	if !ok {
		check = &volumeCheck{}
		v.cache[root] = check
	}
	v.mu.Unlock()

	check.once.Do(func() {
		done := make(chan bool, 1)
		go func() { done <- volumeReachable(root) }()
		select {
		case check.ok = <-done:
		case <-time.After(volumeCheckTimeout):
			check.ok = false
		}
	})
	return check.ok
}

// volumeReachable 报告根目录存在且不是空目录：Linux 上没挂载的 /mnt/x 通常是一个空目录。
func volumeReachable(root string) bool {
	entries, err := os.ReadDir(root)
	return err == nil && len(entries) > 0
}

// volumeRoot 返回路径所在的磁盘根目录或挂载点；在系统盘上（或无法判断）时返回空字符串，表示不需要检查。
//
//   - Windows：盘符根目录（D:\）或网络共享（\\server\share\）
//   - macOS：/Volumes/<名称>
//   - Linux：/media/<用户>/<名称>、/run/media/<用户>/<名称>、/mnt/<名称>
func volumeRoot(path string) string {
	if vol := filepath.VolumeName(path); vol != "" {
		return vol + string(filepath.Separator)
	}
	if !strings.HasPrefix(path, "/") {
		return ""
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	depth := 0
	switch {
	case len(parts) >= 1 && parts[0] == "Volumes", len(parts) >= 1 && parts[0] == "mnt":
		depth = 2
	case len(parts) >= 1 && parts[0] == "media":
		depth = 3
	case len(parts) >= 2 && parts[0] == "run" && parts[1] == "media":
		depth = 4
	}
	if depth == 0 || len(parts) < depth {
		return ""
	}
	return "/" + strings.Join(parts[:depth], "/")
}
