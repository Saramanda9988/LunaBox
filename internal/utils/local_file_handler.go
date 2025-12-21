package utils

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// LocalFileHandler 处理本地文件请求的 HTTP Handler
type LocalFileHandler struct {
	appDir string
}

// NewLocalFileHandler 创建本地文件处理器
func NewLocalFileHandler() (*LocalFileHandler, error) {
	appDir, err := GetDataDir()
	if err != nil {
		return nil, err
	}
	return &LocalFileHandler{
		appDir: appDir,
	}, nil
}

// ServeHTTP 实现 http.Handler 接口
func (h *LocalFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 只处理 /local/ 开头的请求
	if !strings.HasPrefix(r.URL.Path, "/local/") {
		http.NotFound(w, r)
		return
	}

	// 移除 /local/ 前缀
	relativePath := strings.TrimPrefix(r.URL.Path, "/local/")

	// 构建完整路径
	fullPath := filepath.Join(h.appDir, relativePath)

	// 清理路径
	cleanPath := filepath.Clean(fullPath)

	// 检查文件是否存在
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	// 设置缓存头
	w.Header().Set("Cache-Control", "public, max-age=31536000")

	// 提供文件服务
	http.ServeFile(w, r, cleanPath)
}
