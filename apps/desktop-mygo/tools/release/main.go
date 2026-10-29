// release 构建 MyGo 版的发布包：用 mygo build 编译（Windows 的 exe 带图标、manifest 与版本信息，macOS 是 .app），
// 注入版本号与构建期凭据，再和 lunacli、7z 一起按 Wails 版的布局打包到 build/release。-mode 是包的种类：
//
//   - Windows portable（默认）：便携版 LunaBox-mygo-<版本>-windows-<架构>-portable.zip（旁边留一份解压好的目录），
//     对应 Wails 版的 apps/desktop/scripts/build.bat portable；
//   - Windows installer：NSIS 安装包 LunaBox-mygo-<版本>-windows-<架构>-setup.exe，对应 build.bat installer，
//     用 Wails 版的 build/windows/nsis/project.nsi（不装 WebView2、不带 LunaBoxUpdater）；需要 NSIS；
//   - Linux appimage（默认）：LunaBox-mygo-<版本>-linux-<架构>.AppImage（旁边留 .AppDir），对应 build.sh appimage，
//     BuildMode 是 appimage：数据在 ~/.config/LunaBox，与 Wails 版的 AppImage 相同；需要 appimagetool；
//   - Linux installer：LunaBox-mygo-<版本>-linux-<架构>.deb 与 .rpm（旁边留 .pkgroot），对应 build.sh installer，
//     布局同 Wails 版（主程序与 7zz 在 /usr/lib/lunabox），依赖 GTK 3 而不是 WebKitGTK；需要 nFPM；
//   - Linux all：installer 与 appimage 都打，对应 build.sh all；
//   - macOS installer（默认，也是唯一的）：LunaBox-mygo-<版本>-macos-<架构>.dmg（旁边留 DMG 的内容），
//     对应 build.sh installer：.app 里补上 Contents/Resources/bin 的 lunacli 与 7zz，ad-hoc 签名。
//
// 安装版（Windows 安装包、deb、rpm、DMG）的 BuildMode 是 installer，装到 Wails 版的位置，会替换已安装的 Wails 版。
//
//	go run ./tools/release                           # 版本取最近的 v* 标签，架构 amd64，包是默认的种类
//	go run ./tools/release -version 1.2.0 -arch arm64 -mode installer
//
// 在 apps/desktop-mygo 下运行，目标系统是当前系统（Windows、Linux 或 macOS）：DuckDB 需要 cgo，不能交叉编译。
// 凭据与 Wails 版相同：环境变量 LUNABOX_*，没有设置的从 .env.build、.env 读取（格式见 .env.build.example）。
// Windows arm64 与 build.bat 一样动态链接 apps/desktop/lib/winarm64 的 DuckDB。
//
// mygo build 的 -ldflags 是固定的（GOFLAGS 里的 -ldflags 会被它覆盖），所以版本号与凭据不用 -X 注入，
// 而是构建期间在 main 包里写一个 release_gen.go，在 init 里给 lunabox/backend/version 的变量赋值，构建完删除。
// 后端没有在包初始化时读取这些变量，效果与 -X 相同。
package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"lunabox/backend/utils"
)

// genFile 是构建期间写进 main 包的文件（.gitignore 忽略）。
const genFile = "release_gen.go"

// desktopDir 是 Wails 版的目录：lunacli 的源码、随包的 7z、Windows arm64 的 DuckDB 与安装包脚本都在这里。
var desktopDir = filepath.Join("..", "desktop")

// credential 是一项构建期凭据：环境变量 env 注入 version 包的变量 name。
type credential struct{ env, name string }

// credentialGroup 同 build.bat 的 configure_ldflags：required 要么都配置、要么都不配置，optional 只能和 required 一起配置。
type credentialGroup struct {
	label    string
	required []credential
	optional []credential
}

var credentialGroups = []credentialGroup{
	{label: "Update Service URL", required: []credential{{"LUNABOX_UPDATE_SERVICE_URL", "UpdateServiceURL"}}},
	{label: "Bangumi OAuth", required: []credential{
		{"LUNABOX_BANGUMI_CLIENT_ID", "BangumiOAuthClientID"},
		{"LUNABOX_BANGUMI_CLIENT_SECRET", "BangumiOAuthClientSecret"},
	}},
	{label: "Hikarinagi OAuth",
		required: []credential{{"LUNABOX_HIKARINAGI_CLIENT_ID", "HikarinagiOAuthClientID"}},
		optional: []credential{{"LUNABOX_HIKARINAGI_CLIENT_SECRET", "HikarinagiOAuthClientSecret"}},
	},
	{label: "Kungal OAuth", required: []credential{{"LUNABOX_KUNGAL_CLIENT_ID", "KungalOAuthClientID"}}},
	{label: "TouchGAL Token", required: []credential{{"LUNABOX_TOUCHGAL_TOKEN", "TouchGalAPIToken"}}},
	{label: "Umbra Registration", required: []credential{
		{"LUNABOX_UMBRA_CLIENT_ID", "UmbraOAuthClientID"},
		{"LUNABOX_UMBRA_REGISTRATION_TOKEN", "UmbraRegistrationToken"},
	}},
}

// assignment 是注入 version 包的一个变量。
type assignment struct{ name, value string }

// toolchain 是 cgo 的编译器与 Windows arm64 动态链接的 DuckDB。
type toolchain struct {
	cc, ccTarget string
	duckdbDLL    string // Windows arm64 时随包的 duckdb.dll，其余静态链接为空
}

// modes 是每个系统的 -mode，第一个是默认。
var modes = map[string][]string{
	"windows": {"portable", "installer"},
	"linux":   {"appimage", "installer", "all"},
	"darwin":  {"installer"},
}

// packageLabels 是每种包在输出里的名字，键是 <系统>/<种类>。种类也是注入的 version.BuildMode。
var packageLabels = map[string]string{
	"windows/portable":  "Windows Portable",
	"windows/installer": "Windows Installer",
	"linux/appimage":    "Linux AppImage",
	"linux/installer":   "Linux deb/rpm",
	"darwin/installer":  "macOS DMG",
}

// kindsOf 是 mode 要打的包的种类。每种单独 mygo build 一次（BuildMode 不同）；all 同 build.sh，先 installer 后 appimage。
func kindsOf(mode string) []string {
	if mode == "all" {
		return []string{"installer", "appimage"}
	}
	return []string{mode}
}

// target 是一个目标平台的构建差异。
type target struct {
	goos, arch string
	exeSuffix  string
	sevenZip   string // 随包的 7z：Windows 是目录（7z.exe、7z.dll），Linux、macOS 是 7zz
}

func newTarget(goos, arch string) target {
	switch goos {
	case "linux":
		return target{goos, arch, "", filepath.Join(desktopDir, "lib", "linux"+arch, "7z", "7zz")}
	case "darwin":
		// 同 build.sh：macOS 只有 lib/macarm64 的 7zz，两种架构都用它。
		return target{goos, arch, "", filepath.Join(desktopDir, "lib", "macarm64", "7z", "7zz")}
	}
	return target{goos, arch, ".exe", filepath.Join(desktopDir, "lib", "win"+arch, "7z")}
}

// sevenZipFiles 是随包的 7z 必须有的文件。
func (t target) sevenZipFiles() []string {
	if t.goos != "windows" {
		return []string{t.sevenZip}
	}
	return []string{filepath.Join(t.sevenZip, "7z.exe"), filepath.Join(t.sevenZip, "7z.dll")}
}

// sevenZipDest 是 7z 在主程序所在目录 dir 里的位置，即后端 archiveutils 找它的地方。
func (t target) sevenZipDest(dir string) string {
	if t.goos == "linux" {
		return filepath.Join(dir, "7zz")
	}
	return filepath.Join(dir, "7z")
}

// builtExe 是 mygo build 输出的主程序名：Windows 是 <name>.exe，Linux 是 slug 化的 name，macOS 是 <name>.app。
func (t target) builtExe(name string) string {
	switch t.goos {
	case "linux":
		return slugify(name)
	case "darwin":
		return name + ".app"
	}
	return name + ".exe"
}

// skipBuilt 报告 mygo build 的输出里不放进发布包的文件，exe 是输出的主程序名。发布包的内容同 Wails 版：
//   - icon.png：resources/icon.png 是图标，mygo build 当作资源复制到主程序旁边；exe 的图标资源、
//     AppImage 与 deb、rpm 的图标已经有了，窗口图标用 main.go 嵌入的那份，运行时不读它；
//   - Windows 的 NSIS 安装包（安装版用 Wails 版的安装包脚本另外打）；
//   - Linux 的 deb、tar.gz、install.sh 与桌面入口及其图标 <exe>.png（AppImage、deb、rpm 有自己的桌面入口与图标）。
func (t target) skipBuilt(e fs.DirEntry, exe string) bool {
	if e.IsDir() {
		return false
	}
	name := strings.ToLower(e.Name())
	if name == "icon.png" {
		return true
	}
	if t.goos == "linux" {
		return name == "install.sh" || strings.HasSuffix(name, ".deb") || strings.HasSuffix(name, ".tar.gz") ||
			strings.HasSuffix(name, ".desktop") || strings.HasSuffix(name, ".xml") || name == strings.ToLower(exe)+".png"
	}
	return filepath.Ext(name) == ".exe"
}

// slugRe 与 slugify 同 mygo 的 cmd/mygo/init.go：Linux 的主程序名。
var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.Trim(slugRe.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if s == "" {
		return "app"
	}
	return s
}

func main() {
	arch := flag.String("arch", "amd64", "目标架构：amd64 或 arm64")
	ver := flag.String("version", "", "版本号（如 1.2.0）；为空时取最近的 v* 标签")
	mode := flag.String("mode", "", "包的种类：Windows portable（默认）、installer；Linux appimage（默认）、installer、all；macOS installer")
	flag.Parse()
	// Ctrl+C 交给 mygo build 与 go build，等它们退出后再删除 release_gen.go。
	signal.Ignore(os.Interrupt)
	if err := run(*arch, *ver, *mode); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		fmt.Fprintln(os.Stderr, "ERROR: LunaBox MyGo build failed.")
		os.Exit(1)
	}
}

func run(arch, ver, mode string) error {
	switch arch {
	case "x64":
		arch = "amd64"
	case "aarch64":
		arch = "arm64"
	}
	if arch != "amd64" && arch != "arm64" {
		return fmt.Errorf("不支持的架构 %s（amd64 或 arm64）", arch)
	}
	supported, ok := modes[runtime.GOOS]
	if !ok {
		return fmt.Errorf("不支持在 %s 上构建（Windows、Linux 或 macOS）", runtime.GOOS)
	}
	if mode == "" {
		mode = supported[0]
	} else if !slices.Contains(supported, mode) {
		return fmt.Errorf("%s 不支持 -mode %s（%s）", runtime.GOOS, mode, strings.Join(supported, "、"))
	}
	t := newTarget(runtime.GOOS, arch)
	kinds := kindsOf(mode)
	cfg, err := readConfig()
	if err != nil {
		return err
	}
	envFiles := existingFiles(".env.build", ".env")
	if err := utils.LoadEnvFilesIfExists(envFiles...); err != nil {
		return err
	}
	ver, commit, buildTime := resolveVersion(ver, cfg.Version)
	// mygo build 用 mygo.json 的 version 写 exe 的文件版本（同 Wails 版的 build/windows/info.json），
	// 应用里显示的版本是 ver；两者不一致说明 mygo.json 没有随 apps/desktop/scripts/update-build-assets 更新。
	if cfg.Version != baseVersion(ver) {
		fmt.Fprintf(os.Stderr, "WARNING: mygo.json 的 version %s 与版本号 %s 不一致，exe 的文件版本会是 %s；运行 apps/desktop/scripts/update-build-assets 同步。\n", cfg.Version, ver, cfg.Version)
	}
	creds, status, err := credentials()
	if err != nil {
		return err
	}
	tc, err := configureToolchain(t.goos, arch)
	if err != nil {
		return err
	}
	for _, p := range t.sevenZipFiles() {
		if !fileExists(p) {
			return fmt.Errorf("缺少 %s", p)
		}
	}
	tools := map[string]string{}
	var labels []string
	for _, kind := range kinds {
		if tools[kind], err = findTool(t.goos, kind); err != nil {
			return err
		}
		labels = append(labels, packageLabels[t.goos+"/"+kind])
	}

	fmt.Println("========================================")
	fmt.Println("LunaBox MyGo " + strings.Join(labels, " + ") + " Build")
	fmt.Println("Target: " + t.goos + "/" + arch)
	fmt.Println("Version: " + ver)
	fmt.Println("Commit: " + commit)
	fmt.Printf("CGO Compiler: %s (%s)\n", tc.cc, tc.ccTarget)
	if len(envFiles) > 0 {
		fmt.Println("Build Env File: " + strings.Join(envFiles, ", "))
	}
	for _, g := range credentialGroups {
		fmt.Printf("%s Injection: %s\n", g.label, status[g.label])
	}
	if tc.duckdbDLL != "" {
		fmt.Println("DuckDB Dynamic DLL: " + tc.duckdbDLL)
	}
	fmt.Println("Bundled 7z: " + t.sevenZip)
	fmt.Println("========================================")
	fmt.Println()

	var pkgs, dirs []string
	for _, kind := range kinds {
		vars := append([]assignment{
			{"Version", ver},
			{"GitCommit", commit},
			{"BuildTime", buildTime},
			{"BuildMode", kind},
		}, creds...)
		p, d, err := buildPackage(cfg, t, tc, kind, tools[kind], ver, vars)
		if err != nil {
			return err
		}
		pkgs, dirs = append(pkgs, p...), append(dirs, d)
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Build completed successfully.")
	for _, p := range pkgs {
		fmt.Println("Package: " + p)
	}
	for _, d := range dirs {
		fmt.Println("Unpacked: " + d)
	}
	fmt.Println("========================================")
	return nil
}

// findTool 找打包 kind 要用的外部工具：AppImage 的 appimagetool、deb 与 rpm 的 nfpm、Windows 安装包的 makensis。
// macOS 用的 ditto、codesign、hdiutil 是系统自带的。
func findTool(goos, kind string) (string, error) {
	switch goos + "/" + kind {
	case "linux/appimage":
		p, err := exec.LookPath("appimagetool")
		if err != nil {
			return "", fmt.Errorf("打包 AppImage 需要 appimagetool（https://github.com/AppImage/appimagetool）：%w", err)
		}
		return p, nil
	case "linux/installer":
		p, err := exec.LookPath("nfpm")
		if err != nil {
			return "", fmt.Errorf("打包 deb、rpm 需要 nFPM（go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest）：%w", err)
		}
		return p, nil
	case "windows/installer":
		if p, err := exec.LookPath("makensis"); err == nil {
			return p, nil
		}
		for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles"} {
			if dir := os.Getenv(env); dir != "" && fileExists(filepath.Join(dir, "NSIS", "makensis.exe")) {
				return filepath.Join(dir, "NSIS", "makensis.exe"), nil
			}
		}
		return "", errors.New("打包安装版需要 NSIS（https://nsis.sourceforge.io/Download，或 choco install nsis）：找不到 makensis")
	}
	return "", nil
}

// buildPackage 编译 GUI 与 CLI（BuildMode 是 kind），再打成 kind 的包，返回包与解压好的目录。
func buildPackage(cfg config, t target, tc toolchain, kind, tool, ver string, vars []assignment) ([]string, string, error) {
	label := packageLabels[t.goos+"/"+kind]
	fmt.Printf("[%s 1/3] Building GUI (mygo build)...\n", label)
	if err := buildApp(t, vars); err != nil {
		return nil, "", err
	}

	fmt.Printf("[%s 2/3] Building CLI...\n", label)
	cli := filepath.Join(cfg.Out, "release", "lunacli-"+t.goos+"-"+t.arch+t.exeSuffix)
	if err := buildCLI(t, cli, vars[:4]); err != nil {
		return nil, "", err
	}

	release := filepath.Join(cfg.Out, "release")
	switch t.goos + "/" + kind {
	case "windows/portable":
		fmt.Printf("[%s 3/3] Creating ZIP...\n", label)
		name := fmt.Sprintf("LunaBox-mygo-%s-windows-%s-portable", ver, t.arch)
		pkg, dir := filepath.Join(release, name+".zip"), filepath.Join(release, name)
		if err := os.RemoveAll(dir); err != nil {
			return nil, "", err
		}
		for _, sub := range []string{"backups", "covers", "backgrounds", "logs"} {
			if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
				return nil, "", err
			}
		}
		if err := stageFiles(cfg, t, tc, cli, dir); err != nil {
			return nil, "", err
		}
		if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte(readme(ver)), 0o644); err != nil {
			return nil, "", err
		}
		return []string{pkg}, dir, writeZip(pkg, dir)

	case "windows/installer":
		fmt.Printf("[%s 3/3] Creating NSIS installer...\n", label)
		name := fmt.Sprintf("LunaBox-mygo-%s-windows-%s", ver, t.arch)
		pkg, dir := filepath.Join(release, name+"-setup.exe"), filepath.Join(release, name+"-installer")
		if err := os.RemoveAll(dir); err != nil {
			return nil, "", err
		}
		if err := stageFiles(cfg, t, tc, cli, dir); err != nil {
			return nil, "", err
		}
		return []string{pkg}, dir, writeInstaller(tool, cfg, t, dir, pkg)

	case "linux/appimage":
		fmt.Printf("[%s 3/3] Creating AppImage...\n", label)
		name := fmt.Sprintf("LunaBox-mygo-%s-linux-%s", ver, t.arch)
		pkg, dir := filepath.Join(release, name+".AppImage"), filepath.Join(release, name+".AppDir")
		if err := os.RemoveAll(dir); err != nil {
			return nil, "", err
		}
		if err := stageFiles(cfg, t, tc, cli, filepath.Join(dir, "usr", "bin")); err != nil {
			return nil, "", err
		}
		return []string{pkg}, dir, writeAppImage(tool, t, ver, dir, pkg)

	case "linux/installer":
		fmt.Printf("[%s 3/3] Creating deb and rpm packages...\n", label)
		name := fmt.Sprintf("LunaBox-mygo-%s-linux-%s", ver, t.arch)
		dir := filepath.Join(release, name+".pkgroot")
		if err := os.RemoveAll(dir); err != nil {
			return nil, "", err
		}
		if err := stageLinuxPackage(cfg, t, tc, cli, dir); err != nil {
			return nil, "", err
		}
		pkgs, err := writeLinuxPackages(tool, t, ver, dir, filepath.Join(release, name))
		return pkgs, dir, err

	case "darwin/installer":
		fmt.Printf("[%s 3/3] Creating DMG...\n", label)
		name := fmt.Sprintf("LunaBox-mygo-%s-macos-%s", ver, t.arch)
		pkg, dir := filepath.Join(release, name+".dmg"), filepath.Join(release, name)
		if err := os.RemoveAll(dir); err != nil {
			return nil, "", err
		}
		return []string{pkg}, dir, writeDMG(cfg, t, cli, dir, pkg)
	}
	return nil, "", fmt.Errorf("不支持的包 %s/%s", t.goos, kind)
}

// stageFiles 把主程序、lunacli、7z（与 Windows arm64 的 duckdb.dll）放进 dir。
// mygo build 的输出里主程序改名为 LunaBox(.exe)（lunacli protocol register 在旁边找它），
// 其余文件（资源）原样放在旁边，图标、安装包之类见 skipBuilt。
func stageFiles(cfg config, t target, tc toolchain, cli, dir string) error {
	built := filepath.Join(cfg.Out, t.goos+"-"+t.arch)
	entries, err := os.ReadDir(built)
	if err != nil {
		return err
	}
	builtExe := t.builtExe(cfg.Name)
	gui := filepath.Join(dir, "LunaBox"+t.exeSuffix)
	for _, e := range entries {
		src, dst := filepath.Join(built, e.Name()), filepath.Join(dir, e.Name())
		switch {
		case e.Name() == builtExe:
			dst = gui
		case t.skipBuilt(e, builtExe):
			continue
		}
		if err := copyPath(src, dst); err != nil {
			return err
		}
	}
	if !fileExists(gui) {
		return fmt.Errorf("mygo build 没有输出 %s", filepath.Join(built, builtExe))
	}
	if err := copyPath(cli, filepath.Join(dir, "lunacli"+t.exeSuffix)); err != nil {
		return err
	}
	if err := copyPath(t.sevenZip, t.sevenZipDest(dir)); err != nil {
		return err
	}
	if t.goos == "linux" {
		for _, p := range []string{gui, filepath.Join(dir, "lunacli"), t.sevenZipDest(dir)} {
			if err := os.Chmod(p, 0o755); err != nil {
				return err
			}
		}
	}
	if tc.duckdbDLL != "" {
		if err := copyPath(tc.duckdbDLL, filepath.Join(dir, "duckdb.dll")); err != nil {
			return err
		}
	}
	return nil
}

// writeInstaller 同 build.bat 的 build_installer_package：用 Wails 版的 NSIS 脚本把 dir（stageFiles 放好的
// LunaBox.exe、lunacli.exe、7z 与 duckdb.dll）打成 pkg。脚本装到 Wails 版的位置、写同一个卸载项，
// 所以会替换已安装的 Wails 版；LUNABOX_NO_WEBVIEW2、LUNABOX_NO_UPDATER 去掉 MyGo 用不到的 WebView2 运行时
// 与 LunaBoxUpdater.exe。安装包的版本取 mygo.json，与 exe 的文件版本相同。
func writeInstaller(tool string, cfg config, t target, dir, pkg string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	absPkg, err := filepath.Abs(pkg)
	if err != nil {
		return err
	}
	if err := os.Remove(pkg); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	cmd := exec.Command(tool,
		"/DARG_WAILS_"+strings.ToUpper(t.arch)+"_BINARY="+filepath.Join(absDir, "LunaBox.exe"),
		"/DINFO_PRODUCTVERSION="+cfg.Version,
		"/DLUNABOX_PAYLOAD_DIR="+absDir,
		"/DLUNABOX_OUTFILE="+absPkg,
		"/DLUNABOX_NO_WEBVIEW2",
		"/DLUNABOX_NO_UPDATER",
		"project.nsi",
	)
	cmd.Dir = filepath.Join(desktopDir, "build", "windows", "nsis")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("makensis：%w", err)
	}
	if !fileExists(pkg) {
		return fmt.Errorf("makensis 没有输出 %s", pkg)
	}
	return nil
}

// appID 是 Linux 桌面入口与图标的名字，同 Wails 版 build.sh 的 AppImage 与后端 protocol 包的 linuxDesktopID。
const appID = "io.github.saramanda9988.lunabox"

// writeAppImage 同 build.sh 的 AppImage 打包：在 AppDir（主程序等已在 usr/bin）里写 AppRun、桌面入口与图标，
// 再用 appimagetool 打成 pkg。AppRun 不设置 Wails 版给 WebKitGTK 的环境变量（MyGo 不用 WebView）。
func writeAppImage(tool string, t target, ver, dir, pkg string) error {
	if err := os.WriteFile(filepath.Join(dir, "AppRun"), []byte(appRun), 0o755); err != nil {
		return err
	}
	desktop := []byte(desktopEntry("LunaBox", "X-AppImage-Name=LunaBox\nX-AppImage-Version="+ver+"\n"))
	for _, p := range []string{
		filepath.Join(dir, appID+".desktop"),
		filepath.Join(dir, "usr", "share", "applications", appID+".desktop"),
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, desktop, 0o644); err != nil {
			return err
		}
	}
	for _, p := range []string{filepath.Join(dir, appID+".png"), linuxIconPath(dir)} {
		if err := copyPath(appIcon, p); err != nil {
			return err
		}
	}
	if err := os.Remove(pkg); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	appImageArch := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[t.arch]
	cmd := exec.Command(tool, dir, pkg)
	cmd.Env = append(os.Environ(), "ARCH="+appImageArch)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("appimagetool：%w", err)
	}
	return os.Chmod(pkg, 0o755)
}

// appIcon 是 AppImage、deb 与 rpm 的图标（与 Wails 版的 build/appicon.png 相同）。
var appIcon = filepath.Join("resources", "icon.png")

// linuxIconPath 是 root 下桌面入口用的图标，同 Wails 版。
func linuxIconPath(root string) string {
	return filepath.Join(root, "usr", "share", "icons", "hicolor", "512x512", "apps", appID+".png")
}

// appRun 同 build.sh 的 write_linux_appimage_apprun：告诉后端 AppImage 的路径（协议注册、CLI 包装脚本用），
// 第一个参数是 cli 或 lunacli 时运行 lunacli。
const appRun = `#!/usr/bin/env sh
set -eu
APP_RUN_PATH="$(readlink -f "$0" 2>/dev/null || printf '%s' "$0")"
APP_DIR="$(CDPATH= cd "$(dirname "$APP_RUN_PATH")" && pwd -P)"
if [ -n "${APPIMAGE:-}" ]; then
    export LUNABOX_APPIMAGE_PATH="$APPIMAGE"
fi
case "${1:-}" in
    cli|lunacli)
        shift
        exec "$APP_DIR/usr/bin/lunacli" "$@"
        ;;
esac
exec "$APP_DIR/usr/bin/LunaBox" "$@"
`

// linuxLauncher 是 deb、rpm 的 /usr/bin/LunaBox，同 build.sh 的 write_linux_installer_launcher，
// 不设置给 WebKitGTK 的环境变量：主程序在 /usr/lib/lunabox，旁边是后端找的 7zz。
const linuxLauncher = `#!/usr/bin/env sh
set -eu
exec /usr/lib/lunabox/LunaBox "$@"
`

// desktopEntry 同 build.sh 的 write_linux_appimage_desktop 与 Wails 版的 build/linux/desktop：exec 是启动的命令，
// extra 是附加的行。MyGo 不设置 GTK 的程序名，窗口的 WM_CLASS 取自可执行文件名 LunaBox。
func desktopEntry(exec, extra string) string {
	return `[Desktop Entry]
Version=1.0
Type=Application
Name=LunaBox
Comment=LunaBox game library manager
Exec=` + exec + ` %u
Terminal=false
Icon=` + appID + `
Categories=Game;
StartupWMClass=LunaBox
MimeType=x-scheme-handler/lunabox;
` + extra
}

// stageLinuxPackage 按 Wails 版 build/linux/nfpm/nfpm.yaml 的布局把 deb、rpm 的内容放进 root：
// 主程序与 7zz 在 usr/lib/lunabox，usr/bin 是启动脚本 LunaBox 与 lunacli，另有桌面入口与图标。
func stageLinuxPackage(cfg config, t target, tc toolchain, cli, root string) error {
	lib, bin := filepath.Join(root, "usr", "lib", "lunabox"), filepath.Join(root, "usr", "bin")
	if err := stageFiles(cfg, t, tc, cli, lib); err != nil {
		return err
	}
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	if err := os.Rename(filepath.Join(lib, "lunacli"), filepath.Join(bin, "lunacli")); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(bin, "LunaBox"), []byte(linuxLauncher), 0o755); err != nil {
		return err
	}
	desktop := filepath.Join(root, "usr", "share", "applications", appID+".desktop")
	if err := os.MkdirAll(filepath.Dir(desktop), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(desktop, []byte(desktopEntry("/usr/bin/LunaBox", "")), 0o644); err != nil {
		return err
	}
	return copyPath(appIcon, linuxIconPath(root))
}

// writeLinuxPackages 同 build.sh 的 installer：用 nfpm 把 root 打成 <base>.deb 与 <base>.rpm。
// 配置同 Wails 版的 nfpm.yaml（包名 lunabox，会替换已安装的 Wails 版），内容是 root 下的全部文件，
// 依赖换成 MyGo 运行时加载的 GTK 3（同 mygo build 的 deb）。配置写成 JSON，它也是 YAML。
func writeLinuxPackages(tool string, t target, ver, root, base string) ([]string, error) {
	var contents []map[string]any
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		src, err := filepath.Abs(p)
		if err != nil {
			return err
		}
		contents = append(contents, map[string]any{
			"src":       src,
			"dst":       "/" + filepath.ToSlash(rel),
			"file_info": map[string]any{"mode": info.Mode().Perm()},
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	scripts := filepath.Join(desktopDir, "build", "linux", "nfpm", "scripts")
	postinstall, err := filepath.Abs(filepath.Join(scripts, "postinstall.sh"))
	if err != nil {
		return nil, err
	}
	postremove, err := filepath.Abs(filepath.Join(scripts, "postremove.sh"))
	if err != nil {
		return nil, err
	}
	maintainer := os.Getenv("MAINTAINER")
	if maintainer == "" {
		maintainer = "LunaBox contributors"
	}
	data, err := json.MarshalIndent(map[string]any{
		"name":        "lunabox",
		"arch":        t.arch,
		"platform":    "linux",
		"version":     linuxPackageVersion(ver),
		"release":     "1",
		"section":     "utils",
		"priority":    "optional",
		"maintainer":  maintainer,
		"description": "LunaBox game library manager",
		"vendor":      "LunaBox",
		"homepage":    "https://github.com/Saramanda9988/LunaBox",
		"license":     "MIT",
		"contents":    contents,
		// MyGo 的 Linux 托盘用 AppIndicator（dlopen），没有它「关闭到托盘」不会生效。
		"depends":   []string{"libgtk-3-0 | libgtk-3-0t64", "libayatana-appindicator3-1", "xdg-utils"},
		"overrides": map[string]any{"rpm": map[string]any{"depends": []string{"gtk3", "libayatana-appindicator-gtk3", "xdg-utils"}}},
		"scripts":   map[string]string{"postinstall": postinstall, "postremove": postremove},
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	config := base + ".nfpm.yaml"
	if err := os.WriteFile(config, data, 0o644); err != nil {
		return nil, err
	}
	var pkgs []string
	for _, packager := range []string{"deb", "rpm"} {
		pkg := base + "." + packager
		if err := os.Remove(pkg); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		if err := runTool(tool, "pkg", "--config", config, "--packager", packager, "--target", pkg); err != nil {
			return nil, err
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}

// linuxPackageVersion 同 build.sh：deb 与 rpm 的版本把预发布的 - 换成 ~，排在正式版之前。
func linuxPackageVersion(ver string) string {
	return strings.Replace(ver, "-", "~", 1)
}

// writeDMG 同 build.sh 的 macOS 打包：mygo build 的 .app 里补上 Contents/Resources/bin 的 lunacli 与 7zz
// （后端在那里找它们），重新 ad-hoc 签名，再和 /Applications 的链接一起放进 dir，用 hdiutil 打成 pkg。
func writeDMG(cfg config, t target, cli, dir, pkg string) error {
	built := filepath.Join(cfg.Out, t.goos+"-"+t.arch, t.builtExe(cfg.Name))
	if info, err := os.Stat(built); err != nil || !info.IsDir() {
		return fmt.Errorf("mygo build 没有输出 %s", built)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	app := filepath.Join(dir, "LunaBox.app")
	if err := runTool("ditto", built, app); err != nil {
		return err
	}
	// icon.png 同 skipBuilt：.app 的图标是 mygo build 生成的 icns。
	if err := os.Remove(filepath.Join(app, "Contents", "Resources", "icon.png")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	bin := filepath.Join(app, "Contents", "Resources", "bin")
	for src, name := range map[string]string{cli: "lunacli", t.sevenZip: "7zz"} {
		if err := copyPath(src, filepath.Join(bin, name)); err != nil {
			return err
		}
		if err := os.Chmod(filepath.Join(bin, name), 0o755); err != nil {
			return err
		}
	}
	if err := runTool("codesign", "--force", "--deep", "--sign", "-", app); err != nil {
		return err
	}
	if err := runTool("codesign", "--verify", "--deep", "--strict", "--verbose=2", app); err != nil {
		return err
	}
	if err := os.Symlink("/Applications", filepath.Join(dir, "Applications")); err != nil {
		return err
	}
	var kb int64
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		kb += (info.Size() + 1023) / 1024
		return nil
	})
	if err != nil {
		return err
	}
	if err := os.Remove(pkg); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	// 同 build.sh：映像的容量是内容的两倍再加 64 MiB。
	size := fmt.Sprintf("%dm", (kb+1023)/1024*2+64)
	return runTool("hdiutil", "create", "-volname", "LunaBox", "-srcfolder", dir, "-size", size,
		"-fs", "HFS+", "-ov", "-format", "UDZO", pkg)
}

// runTool 运行打包用的外部工具，输出直接打印。
func runTool(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s：%w", filepath.Base(name), err)
	}
	return nil
}

// config 是 mygo.json 里用到的字段：mygo build 把主程序输出到 <out>/<系统>-<架构>/。
type config struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Out     string `json:"out"`
}

// baseVersion 是版本号的 X.Y.Z 部分（去掉 -dev.N+commit 之类的后缀）。
func baseVersion(v string) string {
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		return v[:i]
	}
	return v
}

func readConfig() (config, error) {
	data, err := os.ReadFile("mygo.json")
	if err != nil {
		return config{}, fmt.Errorf("读取 mygo.json（要在 apps/desktop-mygo 下运行）：%w", err)
	}
	var c config
	if err := json.Unmarshal(data, &c); err != nil {
		return config{}, fmt.Errorf("mygo.json：%w", err)
	}
	if c.Out == "" {
		c.Out = "build"
	}
	return c, nil
}

// resolveVersion 同 build.bat 的 resolve_version：版本号去掉开头的 v。
// 拿不到 git 标签时依次退回 mygo.json 的版本与 1.0.0（容器里挂载 worktree、没装 git 时会走到这里）。
func resolveVersion(ver, fallback string) (v, commit, buildTime string) {
	if ver == "" {
		ver = gitOutput("describe", "--tags", "--abbrev=0", "--match", "v[0-9]*")
	}
	if ver == "" {
		ver = fallback
	}
	if ver == "" {
		ver = "1.0.0"
	}
	commit = gitOutput("rev-parse", "--short", "HEAD")
	if commit == "" {
		commit = "unknown"
	}
	return strings.TrimPrefix(ver, "v"), commit, time.Now().Format("2006-01-02 15:04:05")
}

func gitOutput(args ...string) string {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// credentials 返回要注入的凭据与每组的状态（enabled、disabled）。
func credentials() ([]assignment, map[string]string, error) {
	var vars []assignment
	status := map[string]string{}
	for _, g := range credentialGroups {
		var set, missing []string
		for _, c := range g.required {
			if v := strings.TrimSpace(os.Getenv(c.env)); v != "" {
				set = append(set, c.env)
				vars = append(vars, assignment{c.name, v})
			} else {
				missing = append(missing, c.env)
			}
		}
		if len(set) > 0 && len(missing) > 0 {
			return nil, nil, fmt.Errorf("%s 必须同时配置（缺少 %s）", strings.Join(append(set, missing...), " 与 "), strings.Join(missing, "、"))
		}
		for _, c := range g.optional {
			if v := strings.TrimSpace(os.Getenv(c.env)); v != "" {
				if len(set) == 0 {
					return nil, nil, fmt.Errorf("%s 需要同时配置 %s", c.env, g.required[0].env)
				}
				vars = append(vars, assignment{c.name, v})
			}
		}
		status[g.label] = "disabled"
		if len(set) > 0 {
			status[g.label] = "enabled"
		}
	}
	return vars, status, nil
}

// configureToolchain 同 build.bat 的 configure_architecture：设置 cgo 的环境变量（mygo build 与 go build 继承），
// Windows arm64 用 MSYS2 的 clang 并动态链接 apps/desktop/lib/winarm64 的 DuckDB（duckdb_use_lib）；
// Linux 与 build.sh 相同，用系统的 gcc 静态链接 DuckDB；macOS 同 build.sh，用 Xcode 的 clang，最低 macOS 12.0。
func configureToolchain(goos, arch string) (toolchain, error) {
	var tc toolchain
	os.Setenv("CGO_ENABLED", "1")
	if goos == "windows" && arch == "arm64" {
		src := filepath.Join(desktopDir, "lib", "winarm64")
		dst, err := filepath.Abs(filepath.Join("build", "duckdb", "winarm64"))
		if err != nil {
			return tc, err
		}
		for _, name := range []string{"duckdb.dll", "duckdb.lib"} {
			if !fileExists(filepath.Join(src, name)) {
				return tc, fmt.Errorf("缺少 %s", filepath.Join(src, name))
			}
		}
		if err := copyPath(filepath.Join(src, "duckdb.dll"), filepath.Join(dst, "duckdb.dll")); err != nil {
			return tc, err
		}
		if err := copyPath(filepath.Join(src, "duckdb.lib"), filepath.Join(dst, "libduckdb.dll.a")); err != nil {
			return tc, err
		}
		const triple = "aarch64-w64-windows-gnu"
		var bin string
		for _, root := range []string{os.Getenv("MSYS2_LOCATION"), `C:\msys64`} {
			if root != "" && fileExists(filepath.Join(root, "clangarm64", "bin", "clang.exe")) {
				bin = filepath.Join(root, "clangarm64", "bin")
				break
			}
		}
		if bin != "" {
			os.Setenv("CC", filepath.Join(bin, "clang.exe")+" --target="+triple)
			if fileExists(filepath.Join(bin, "clang++.exe")) {
				os.Setenv("CXX", filepath.Join(bin, "clang++.exe")+" --target="+triple)
			}
			prependPath(bin)
		} else if os.Getenv("CC") == "" {
			if _, err := exec.LookPath("aarch64-w64-mingw32-gcc"); err == nil {
				os.Setenv("CC", "aarch64-w64-mingw32-gcc")
			}
		}
		os.Setenv("CGO_LDFLAGS", "-L"+dst+" -lduckdb")
		// mygo build 把 GOFLAGS 里的 -tags 和它自己的合在一起。
		os.Setenv("GOFLAGS", strings.TrimSpace(os.Getenv("GOFLAGS")+" -tags=duckdb_use_lib"))
		prependPath(dst)
		tc.duckdbDLL = filepath.Join(dst, "duckdb.dll")
	}
	if goos == "darwin" {
		// mygo build 写进 Info.plist 的 LSMinimumSystemVersion 也是 12.0。
		os.Setenv("MACOSX_DEPLOYMENT_TARGET", "12.0")
		for _, name := range []string{"CGO_CFLAGS", "CGO_LDFLAGS"} {
			os.Setenv(name, strings.TrimSpace(os.Getenv(name)+" -mmacosx-version-min=12.0"))
		}
	}
	if os.Getenv("CC") == "" {
		cc := "gcc"
		if goos == "darwin" {
			cc = "clang"
		}
		if _, err := exec.LookPath(cc); err == nil {
			os.Setenv("CC", cc)
		}
	}
	tc.cc = os.Getenv("CC")
	if tc.cc == "" {
		switch goos {
		case "linux":
			return tc, fmt.Errorf("linux/%s 的 cgo 构建需要 C 编译器：安装 gcc（如 build-essential），或设置 CC", arch)
		case "darwin":
			return tc, fmt.Errorf("darwin/%s 的 cgo 构建需要 C 编译器：安装 Xcode 命令行工具（xcode-select --install），或设置 CC", arch)
		}
		return tc, fmt.Errorf("windows/%s 的 cgo 构建需要对应架构的 C 编译器：安装 MSYS2 与它的 %s 工具链，或设置 CC", arch, arch)
	}
	fields := strings.Fields(tc.cc)
	out, err := exec.Command(fields[0], append(fields[1:], "-dumpmachine")...).Output()
	if err != nil {
		return tc, fmt.Errorf("无法检查 C 编译器的目标平台：%s：%w", tc.cc, err)
	}
	tc.ccTarget = strings.TrimSpace(string(out))
	want := []string{"x86_64", "amd64"}
	if arch == "arm64" {
		want = []string{"aarch64", "arm64"}
	}
	if !strings.Contains(tc.ccTarget, want[0]) && !strings.Contains(tc.ccTarget, want[1]) {
		return tc, fmt.Errorf("C 编译器的目标平台 %s 与 %s 不符（CC=%s）", tc.ccTarget, arch, tc.cc)
	}
	return tc, nil
}

func prependPath(dir string) {
	os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// buildApp 写出 release_gen.go 后运行 mygo build，构建完（包括失败、中断）删除它。
// macOS 不让 mygo build 打 DMG（补上 lunacli 与 7zz 后由 writeDMG 打）。
func buildApp(t target, vars []assignment) error {
	var b strings.Builder
	b.WriteString("// Code generated by tools/release; DO NOT EDIT.\n\npackage main\n\nimport \"lunabox/backend/version\"\n\nfunc init() {\n")
	for _, v := range vars {
		fmt.Fprintf(&b, "\tversion.%s = %s\n", v.name, strconv.Quote(v.value))
	}
	b.WriteString("}\n")
	if err := os.WriteFile(genFile, []byte(b.String()), 0o644); err != nil {
		return err
	}
	defer os.Remove(genFile)
	args := []string{"tool", "mygo", "build", "-platform", t.goos + "/" + t.arch}
	if t.goos == "darwin" {
		args = append(args, "-skip-dmg", "-skip-notarize")
	}
	cmd := exec.Command("go", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mygo build：%w", err)
	}
	return nil
}

// buildCLI 同 build.bat 的 build_cli：用 Wails 版模块里的 cmd/lunacli，注入版本号与 BuildMode。
func buildCLI(t target, out string, vars []assignment) error {
	abs, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	ldflags := "-s -w"
	for _, v := range vars {
		ldflags += " -X " + ldflagsQuote("lunabox/backend/version."+v.name+"="+v.value)
	}
	cmd := exec.Command("go", "-C", desktopDir, "build", "-trimpath", "-buildvcs=false", "-ldflags", ldflags, "-o", abs, "./cmd/lunacli")
	cmd.Env = append(os.Environ(), "GOOS="+t.goos, "GOARCH="+t.arch)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("构建 lunacli：%w", err)
	}
	return nil
}

// ldflagsQuote 给 -ldflags 里含空格的参数加引号（go build 按空格拆分 -ldflags，引号里不转义）。
func ldflagsQuote(s string) string {
	if !strings.ContainsAny(s, " \t'\"") {
		return s
	}
	if strings.Contains(s, "'") {
		return `"` + s + `"`
	}
	return "'" + s + "'"
}

func readme(ver string) string {
	return strings.Join([]string{
		"LunaBox (MyGo) Portable v" + ver,
		"",
		"This package contains:",
		"  - LunaBox.exe  : GUI version (double-click to launch)",
		"  - lunacli.exe  : CLI version (use in a terminal)",
		"",
		"Data (database, settings, covers, logs) is stored next to LunaBox.exe.",
		"",
		"CLI usage:",
		"  lunacli list",
		"  lunacli start <game-id>",
		"  lunacli protocol register",
		"  lunacli protocol unregister",
		"  lunacli help",
		"",
	}, "\r\n")
}

// writeZip 把 dir 打成 zip，里面是以 dir 的名字命名的顶层目录（同 build.bat 的 Compress-Archive）。
func writeZip(path, dir string) (err error) {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	zw := zip.NewWriter(f)
	parent := filepath.Dir(dir)
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(parent, p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		h.Name = filepath.ToSlash(rel)
		if d.IsDir() {
			h.Name += "/"
			_, err = zw.CreateHeader(h)
			return err
		}
		h.Method = zip.Deflate
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		src, err := os.Open(p)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(w, src)
		return err
	})
	if err != nil {
		return err
	}
	return zw.Close()
}

// copyPath 复制文件或目录（目录递归复制），需要时创建目标所在的目录。
func copyPath(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(p, target)
	})
}

// copyFile 复制一个文件，保留它的权限（Linux 的可执行位）。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func existingFiles(names ...string) []string {
	var out []string
	for _, n := range names {
		if fileExists(n) {
			out = append(out, n)
		}
	}
	return out
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
