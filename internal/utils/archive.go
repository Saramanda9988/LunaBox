package utils

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"

	"golift.io/xtractr"
)

// ExtractArchive 通用解压函数（支持 zip/rar/7z/tar 及常见压缩归档组合）
// 返回值 extracted 表示是否完整解压成功。
// extracted=false 且 err!=nil 表示可回退的解压失败（例如目标平台非法文件名），可由上层进入手动解压模式。
func ExtractArchive(source, target string) (extracted bool, err error) {
	if err := os.MkdirAll(target, 0755); err != nil {
		return false, fmt.Errorf("create target dir: %w", err)
	}

	xfile := &xtractr.XFile{
		FilePath:  source,
		OutputDir: target,
		FileMode:  0644,
		DirMode:   0755,
	}

	_, _, _, err = xtractr.ExtractFile(xfile)
	if err != nil {
		if isRecoverableExtractPathError(err) {
			return false, fmt.Errorf("extract archive entries: %w", err)
		}
		return true, fmt.Errorf("extract archive entries: %w", err)
	}

	return true, nil
}

func isRecoverableExtractPathError(err error) bool {
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) {
		return false
	}

	if errors.Is(pathErr.Err, syscall.EINVAL) {
		return true
	}

	if runtime.GOOS == "windows" {
		if errors.Is(pathErr.Err, syscall.Errno(123)) {
			return true
		}
	}

	return false
}
