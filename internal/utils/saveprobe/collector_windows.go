//go:build windows

package saveprobe

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

type Collector struct {
	gameDirectory string
	startedAt     time.Time
	store         *activityStore
	watchers      []*directoryWatcher
	stopOnce      sync.Once
	result        Result
}

func Supported() bool {
	return true
}

func Start(gameDirectory string) (*Collector, error) {
	gameDirectory = filepath.Clean(strings.TrimSpace(gameDirectory))
	if gameDirectory == "" || gameDirectory == "." {
		return nil, fmt.Errorf("game directory is unavailable")
	}

	collector := &Collector{
		gameDirectory: gameDirectory,
		startedAt:     time.Now(),
		store:         newActivityStore(),
	}
	var startErrors []error
	for _, root := range monitorRoots(gameDirectory) {
		watcher, err := startDirectoryWatcher(root, gameDirectory, collector.store)
		if err != nil {
			startErrors = append(startErrors, err)
			continue
		}
		collector.watchers = append(collector.watchers, watcher)
	}
	if len(collector.watchers) == 0 {
		return nil, fmt.Errorf("start save directory monitors: %v", startErrors)
	}
	return collector, nil
}

func (c *Collector) Stop() (Result, error) {
	c.stopOnce.Do(func() {
		for _, watcher := range c.watchers {
			_ = watcher.stop()
		}
		c.result = c.store.result(c.startedAt, time.Now(), c.gameDirectory)
	})
	return c.result, nil
}

func monitorRoots(gameDirectory string) []string {
	roots := []string{gameDirectory}
	knownFolders := []*windows.KNOWNFOLDERID{
		windows.FOLDERID_Profile,
		windows.FOLDERID_Documents,
		windows.FOLDERID_SavedGames,
		windows.FOLDERID_RoamingAppData,
		windows.FOLDERID_LocalAppData,
		windows.FOLDERID_LocalAppDataLow,
	}
	for _, folderID := range knownFolders {
		path, err := windows.KnownFolderPath(folderID, windows.KF_FLAG_DEFAULT)
		if err == nil && strings.TrimSpace(path) != "" {
			roots = append(roots, filepath.Clean(path))
		}
	}

	sort.SliceStable(roots, func(i, j int) bool {
		return len(roots[i]) < len(roots[j])
	})
	result := make([]string, 0, len(roots))
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		covered := false
		for _, existing := range result {
			if isUnderDirectory(root, existing) {
				covered = true
				break
			}
		}
		if !covered {
			result = append(result, root)
		}
	}
	return result
}
