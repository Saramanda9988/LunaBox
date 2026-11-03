// Package host 把 lunabox/backend 接到 MyGo：用 MyGo 实现 appruntime.Runtime 与 bootstrap.Window，
// 把服务事件转给界面（Events），给界面提供 api.Services（Backend.API），以及开发运行的数据目录。
// 服务的构造、启动与关闭都在 lunabox/backend/bootstrap，与 Wails 版相同。
package host

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"lunabox/backend/utils"
	"lunabox/backend/utils/apputils"
	"lunabox/backend/version"
)

// seedEntries 是开发数据目录首次使用时从安装版数据目录复制过来的内容（日志与云同步身份不复制）。
var seedEntries = []string{"appconf.json", "lunabox.db", "lunabox.db.wal", "covers", "backgrounds", "avatars", "templates", "backups"}

// DataDirOptions 决定本次运行的数据目录。
type DataDirOptions struct {
	Dir    string // 指定的数据目录（-data-dir）；为空时按 Dev 决定
	Dev    bool   // 开发运行：用 DevDataDir，首次使用时从安装版复制
	Reseed bool   // 开发运行时重新从安装版复制，覆盖开发数据
}

// PrepareDataDir 返回本次运行的数据目录，必须在任何服务读取路径之前调用；seededFrom 是这次复制数据的来源（没有复制时为空）。
// 正式版与 Wails 版相同，由 version.BuildMode 决定：便携版是程序所在目录，安装版是 %APPDATA%/LunaBox。
// 开发运行用 DevDataDir（首次使用时从安装版复制），指定 Dir 时用它；两者都当作便携版的数据目录。
func PrepareDataDir(o DataDirOptions) (dir, seededFrom string, err error) {
	if o.Dir == "" && !o.Dev {
		dir, err = apputils.GetDataDir()
		return dir, "", err
	}
	dir = o.Dir
	if dir == "" {
		dir = DevDataDir()
	}
	if dir, err = UseDataDir(dir); err != nil || !o.Dev {
		return dir, "", err
	}
	if from, err := InstalledDataDir(); err == nil && filepath.Clean(from) != dir {
		seeded, err := Seed(dir, from, o.Reseed)
		if err != nil {
			return "", "", fmt.Errorf("seed data dir: %w", err)
		}
		if seeded {
			seededFrom = from
		}
	}
	return dir, seededFrom, nil
}

// DevProjectDir 是开发运行时的项目目录（apps/desktop-mygo）：可执行文件在 build/ 下时是它的上一级，
// 否则是当前目录（go run、mygo dev 在 apps/desktop-mygo 下执行）。
func DevProjectDir() string {
	if exe, err := os.Executable(); err == nil {
		if dir := filepath.Dir(exe); filepath.Base(dir) == "build" {
			return filepath.Dir(dir)
		}
	}
	return "."
}

// DevDataDir 是开发运行的数据目录：项目目录下的 build/data。
func DevDataDir() string {
	return filepath.Join(DevProjectDir(), "build", "data")
}

// LoadDevEnv 同 Wails 版的 wails3 dev：读取项目目录下的 .env.build 与 .env，
// 补上没有用 ldflags 注入的构建期凭据（OAuth Client ID 等）。只在开发运行时调用。
func LoadDevEnv() error {
	dir := DevProjectDir()
	if err := utils.LoadEnvFilesIfExists(filepath.Join(dir, ".env.build"), filepath.Join(dir, ".env")); err != nil {
		return err
	}
	utils.ApplyDevBuildEnvFallbacks()
	return nil
}

// InstalledDataDir 是 LunaBox 安装版的数据目录（%APPDATA%/LunaBox），用作开发数据的来源。
func InstalledDataDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "LunaBox"), nil
}

// UseDataDir 让 apputils 把 dir 当作便携版数据目录。必须在任何服务读取路径之前调用。
func UseDataDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return "", err
	}
	version.BuildMode = "portable"
	if err := os.Setenv("LUNABOX_PORTABLE_ROOT", abs); err != nil {
		return "", err
	}
	return abs, nil
}

// Seed 在 dir 还没有数据库（或 force）时，从 from 复制设置、数据库、封面与备份。只复制，不改动 from。
func Seed(dir, from string, force bool) (bool, error) {
	if _, err := os.Stat(filepath.Join(dir, "lunabox.db")); err == nil && !force {
		return false, nil
	}
	if _, err := os.Stat(filepath.Join(from, "lunabox.db")); err != nil {
		return false, nil // 没有安装版数据：从空数据库开始
	}
	if force {
		_ = os.Remove(filepath.Join(dir, "lunabox.db.wal"))
	}
	for _, name := range seedEntries {
		src, dst := filepath.Join(from, name), filepath.Join(dir, name)
		info, err := os.Stat(src)
		if err != nil {
			continue
		}
		if info.IsDir() {
			err = copyDir(src, dst)
		} else {
			err = copyFile(src, dst)
		}
		if err != nil {
			return false, fmt.Errorf("copy %s: %w", name, err)
		}
	}
	return true, nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
