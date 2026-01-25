package utils

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FindExecutables 在指定目录下查找可执行文件
// 注意：不包含 .lnk 快捷方式，因为无法直接启动
func FindExecutables(folderPath string, excludeKeywords []string) []string {
	var executables []string

	// 仅扫描一级目录
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return executables
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		lowerName := strings.ToLower(name)

		// 检查是否是可执行文件（不包含 .lnk 快捷方式）
		if !strings.HasSuffix(lowerName, ".exe") &&
			!strings.HasSuffix(lowerName, ".bat") {
			continue
		}

		// 检查是否应该排除
		excluded := false
		for _, keyword := range excludeKeywords {
			if strings.Contains(lowerName, keyword) {
				excluded = true
				break
			}
		}

		if !excluded {
			executables = append(executables, filepath.Join(folderPath, name))
		}
	}

	return executables
}

// SelectBestExecutable 选择最佳可执行文件
func SelectBestExecutable(executables []string, folderName string) string {
	if len(executables) == 0 {
		return ""
	}
	if len(executables) == 1 {
		return executables[0]
	}

	lowerFolderName := strings.ToLower(folderName)

	// 优先选择与文件夹名相似的
	for _, exe := range executables {
		exeName := strings.ToLower(filepath.Base(exe))
		exeName = strings.TrimSuffix(exeName, filepath.Ext(exeName))
		if strings.Contains(exeName, lowerFolderName) || strings.Contains(lowerFolderName, exeName) {
			return exe
		}
	}

	// 否则按文件大小排序，选择最大的
	type exeInfo struct {
		path string
		size int64
	}
	var exeInfos []exeInfo

	for _, exe := range executables {
		info, err := os.Stat(exe)
		if err == nil {
			exeInfos = append(exeInfos, exeInfo{path: exe, size: info.Size()})
		}
	}

	if len(exeInfos) > 0 {
		sort.Slice(exeInfos, func(i, j int) bool {
			return exeInfos[i].size > exeInfos[j].size
		})
		return exeInfos[0].path
	}

	return executables[0]
}
