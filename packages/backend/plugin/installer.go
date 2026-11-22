package plugin

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/semver"

	"lunabox/backend/utils/apputils"
)

// ZipInstallPreview 描述一个待安装的 .zip 插件包校验结果（用于安装确认框展示）。
type ZipInstallPreview struct {
	ZipPath         string   `json:"zip_path"`
	Manifest        Manifest `json:"manifest"`
	DisplayName     string   `json:"display_name"`
	Description     string   `json:"description"`
	IsUpgrade       bool     `json:"is_upgrade"`
	ExistingVersion string   `json:"existing_version,omitempty"`
}

// InspectZip 解压 .zip 到临时目录并校验 plugin.json 与宿主兼容性，不修改已安装插件。
func (m *Manager) InspectZip(zipPath string) (ZipInstallPreview, error) {
	tmpDir, err := os.MkdirTemp("", "lunabox-plugin-inspect-*")
	if err != nil {
		return ZipInstallPreview{}, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	rootDir, err := extractPluginZip(zipPath, tmpDir)
	if err != nil {
		return ZipInstallPreview{}, err
	}
	manifest, err := LoadManifest(rootDir)
	if err != nil {
		return ZipInstallPreview{}, err
	}
	if err := manifest.Validate(m.hostVersion); err != nil {
		return ZipInstallPreview{}, err
	}
	entryPath := filepath.Join(rootDir, filepath.FromSlash(manifest.Main))
	if _, err := os.Stat(entryPath); err != nil {
		return ZipInstallPreview{}, fmt.Errorf("插件入口文件 %s 不存在", manifest.Main)
	}

	preview := ZipInstallPreview{
		ZipPath:     zipPath,
		Manifest:    manifest,
		DisplayName: manifest.DisplayName(m.currentLanguage()),
		Description: manifest.DisplayDescription(m.currentLanguage()),
	}

	m.mu.RLock()
	if existing, ok := m.plugins[manifest.ID]; ok {
		preview.IsUpgrade = true
		preview.ExistingVersion = existing.manifest.Version
	}
	m.mu.RUnlock()
	return preview, nil
}

// InstallFromZip 确认安装或升级一个 .zip 插件包，并立即启用加载。
func (m *Manager) InstallFromZip(zipPath string) (PluginInfo, error) {
	tmpDir, err := os.MkdirTemp("", "lunabox-plugin-install-*")
	if err != nil {
		return PluginInfo{}, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	rootDir, err := extractPluginZip(zipPath, tmpDir)
	if err != nil {
		return PluginInfo{}, err
	}
	manifest, err := LoadManifest(rootDir)
	if err != nil {
		return PluginInfo{}, err
	}
	if err := manifest.Validate(m.hostVersion); err != nil {
		return PluginInfo{}, err
	}

	// 若同 ID 已安装，检查版本并先卸载旧版
	m.mu.RLock()
	existing, exists := m.plugins[manifest.ID]
	existingVer := ""
	if exists {
		existingVer = existing.manifest.Version
	}
	m.mu.RUnlock()
	if exists && existingVer != "" {
		if semver.Compare(ensureV(manifest.Version), ensureV(existingVer)) < 0 {
			return PluginInfo{}, fmt.Errorf("不能降级插件 %s（当前 %s，安装包 %s）", manifest.ID, existingVer, manifest.Version)
		}
		m.unloadPlugin(manifest.ID)
	}

	targetDir := filepath.Join(m.PluginsDir(), manifest.ID)
	if err := os.MkdirAll(m.PluginsDir(), 0o755); err != nil {
		return PluginInfo{}, fmt.Errorf("创建插件目录失败: %w", err)
	}
	if err := os.RemoveAll(targetDir); err != nil {
		return PluginInfo{}, fmt.Errorf("清理旧版本目录失败: %w", err)
	}
	if err := apputils.CopyDir(rootDir, targetDir); err != nil {
		return PluginInfo{}, fmt.Errorf("复制插件文件失败: %w", err)
	}

	m.Discover()
	if err := m.Enable(manifest.ID); err != nil {
		// 即使加载报错，目录也已就位，返回当前信息以便界面展示错误原因
		for _, info := range m.List() {
			if info.ID == manifest.ID {
				return info, err
			}
		}
		return PluginInfo{}, err
	}
	for _, info := range m.List() {
		if info.ID == manifest.ID {
			return info, nil
		}
	}
	return PluginInfo{}, fmt.Errorf("安装完成后未找到插件 %s", manifest.ID)
}

// Uninstall 卸载插件：停止运行时、删除 plugins/<id>/，按 deleteData 选择是否删除 plugins-data/<id>/。
func (m *Manager) Uninstall(id string, deleteData bool) error {
	m.mu.RLock()
	mp, ok := m.plugins[id]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("未找到插件 %s", id)
	}
	isDev := mp.isDev
	dir := mp.dir
	dataDir := mp.dataDir
	m.mu.RUnlock()

	if isDev {
		return m.RemoveDevPath(dir)
	}

	m.unloadPlugin(id)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("删除插件目录失败: %w", err)
	}
	if deleteData && dataDir != "" {
		if err := os.RemoveAll(dataDir); err != nil {
			return fmt.Errorf("删除插件数据失败: %w", err)
		}
	}

	m.mu.Lock()
	delete(m.plugins, id)
	m.order = slices.DeleteFunc(m.order, func(x string) bool { return x == id })
	m.mu.Unlock()
	m.persistPluginConfig()
	m.notifyState()
	return nil
}

// extractPluginZip 安全解压 zip 到 destDir，并自动识别包含 plugin.json 的根目录（兼容带单层顶层目录的压缩包）。
func extractPluginZip(zipPath, destDir string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("打开插件压缩包失败: %w", err)
	}
	defer r.Close()

	cleanDest := filepath.Clean(destDir)
	for _, f := range r.File {
		name := filepath.FromSlash(f.Name)
		target := filepath.Join(cleanDest, name)
		rel, err := filepath.Rel(cleanDest, target)
		if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			return "", fmt.Errorf("压缩包包含非法路径: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", err
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return "", err
		}
		_, copyErr := io.Copy(out, io.LimitReader(rc, 256<<20))
		closeErr := out.Close()
		rc.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
	}

	if _, err := os.Stat(filepath.Join(cleanDest, "plugin.json")); err == nil {
		return cleanDest, nil
	}
	entries, err := os.ReadDir(cleanDest)
	if err == nil {
		var dirs []string
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "__MACOSX" {
				dirs = append(dirs, filepath.Join(cleanDest, e.Name()))
			}
		}
		if len(dirs) == 1 {
			if _, err := os.Stat(filepath.Join(dirs[0], "plugin.json")); err == nil {
				return dirs[0], nil
			}
		}
	}
	return "", fmt.Errorf("压缩包中未找到 plugin.json")
}
