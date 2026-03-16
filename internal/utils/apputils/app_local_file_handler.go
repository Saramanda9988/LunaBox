package apputils

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
	if !strings.HasPrefix(r.URL.Path, "/local/") {
		http.NotFound(w, r)
		return
	}

	relativePath := strings.TrimPrefix(r.URL.Path, "/local/")
	baseDir, err := filepath.Abs(h.appDir)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	fullPath := filepath.Join(baseDir, filepath.FromSlash(relativePath))
	cleanPath, err := filepath.Abs(filepath.Clean(fullPath))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if cleanPath != baseDir && !strings.HasPrefix(cleanPath, baseDir+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}

	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000")
	http.ServeFile(w, r, cleanPath)
}
