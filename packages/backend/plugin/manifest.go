package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/mod/semver"
)

// SupportedAPIVersion 是当前宿主支持的插件 API 主版本。
const SupportedAPIVersion = 1

var pluginIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

// EnginesDecl 声明插件兼容的宿主版本范围。
type EnginesDecl struct {
	LunaBox string `json:"lunabox,omitempty"`
}

// Manifest 对应插件目录下的 plugin.json 清单。
type Manifest struct {
	ID          string            `json:"id"`
	Version     string            `json:"version"`
	APIVersion  int               `json:"apiVersion"`
	Engines     EnginesDecl       `json:"engines,omitempty"`
	Main        string            `json:"main"`
	Name        map[string]string `json:"name"`
	Description map[string]string `json:"description,omitempty"`
	Author      string            `json:"author,omitempty"`
	Homepage    string            `json:"homepage,omitempty"`
	Repository  string            `json:"repository,omitempty"`
	License     string            `json:"license,omitempty"`
	Icon        string            `json:"icon,omitempty"`
	IconBg      string            `json:"iconBg,omitempty"`
	Keywords    []string          `json:"keywords,omitempty"`
}

// LoadManifest 读取并解析插件目录下的 plugin.json。
func LoadManifest(dir string) (Manifest, error) {
	path := filepath.Join(dir, "plugin.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("读取 plugin.json 失败: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("解析 plugin.json 失败: %w", err)
	}
	m.ID = strings.TrimSpace(m.ID)
	m.Version = strings.TrimSpace(m.Version)
	m.Main = strings.TrimSpace(m.Main)
	if m.Main == "" {
		m.Main = "main.js"
	}
	return m, nil
}

// Validate 校验清单字段格式与当前宿主版本的兼容性。
func (m Manifest) Validate(hostVersion string) error {
	if !pluginIDPattern.MatchString(m.ID) {
		return fmt.Errorf("无效的插件 ID %q（必须为小写字母开头，仅含小写字母、数字与连字符）", m.ID)
	}
	if m.Version == "" || !semver.IsValid(ensureV(m.Version)) {
		return fmt.Errorf("无效的插件版本号 %q（必须为合法 semver）", m.Version)
	}
	if m.APIVersion != SupportedAPIVersion {
		return fmt.Errorf("不兼容的 apiVersion=%d（当前宿主支持 %d）", m.APIVersion, SupportedAPIVersion)
	}
	if m.Main == "" || strings.Contains(m.Main, "..") || filepath.IsAbs(m.Main) {
		return fmt.Errorf("无效的入口文件路径 %q", m.Main)
	}
	if icon := strings.TrimSpace(m.Icon); icon != "" {
		if strings.Contains(icon, "..") || filepath.IsAbs(icon) {
			return fmt.Errorf("无效的插件图标路径 %q", m.Icon)
		}
	}
	if len(m.Name) == 0 {
		return fmt.Errorf("插件清单缺少 name 字段")
	}
	if strings.TrimSpace(m.Engines.LunaBox) != "" {
		if ok, reason := CheckSemverRange(hostVersion, m.Engines.LunaBox); !ok {
			return fmt.Errorf("不兼容当前 LunaBox 版本 %s（要求 %s：%s）", hostVersion, m.Engines.LunaBox, reason)
		}
	}
	return nil
}

// DisplayName 按当前语言取插件名称，依次回退到 en-US、第一个值、插件 ID。
func (m Manifest) DisplayName(lang string) string {
	if v := pickLocalized(m.Name, lang); v != "" {
		return v
	}
	return m.ID
}

// DisplayDescription 按当前语言取插件描述。
func (m Manifest) DisplayDescription(lang string) string {
	return pickLocalized(m.Description, lang)
}

func pickLocalized(values map[string]string, lang string) string {
	if len(values) == 0 {
		return ""
	}
	lang = strings.TrimSpace(lang)
	if lang != "" {
		if v := strings.TrimSpace(values[lang]); v != "" {
			return v
		}
		if idx := strings.IndexAny(lang, "-_"); idx > 0 {
			prefix := strings.ToLower(lang[:idx])
			for k, v := range values {
				if strings.HasPrefix(strings.ToLower(k), prefix) && strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			}
		}
	}
	if v := strings.TrimSpace(values["en-US"]); v != "" {
		return v
	}
	if v := strings.TrimSpace(values["en"]); v != "" {
		return v
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if v := strings.TrimSpace(values[k]); v != "" {
			return v
		}
	}
	return ""
}

func ensureV(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

// normalizeSemver 把 "3" / "2.1" / "2.1.0" 补全成 golang.org/x/mod/semver 可比较的 "vMAJOR.MINOR.PATCH"。
func normalizeSemver(raw string) (string, bool) {
	s := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "v"))
	if s == "" {
		return "", false
	}
	// 剥离 prerelease/build 用于校验补全
	suffix := ""
	if idx := strings.IndexAny(s, "-+"); idx >= 0 {
		suffix = s[idx:]
		s = s[:idx]
	}
	parts := strings.Split(s, ".")
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	cand := "v" + strings.Join(parts[:3], ".") + suffix
	if !semver.IsValid(cand) {
		return "", false
	}
	return cand, true
}

// CheckSemverRange 校验 hostVersion 是否满足 expr（支持空格分隔的 AND 约束与 || 分隔的 OR 分支，
// 支持 >=, <=, >, <, =, ^, ~, * 等常见 npm/semver 范围语法）。
func CheckSemverRange(hostVersion, expr string) (bool, string) {
	expr = strings.TrimSpace(expr)
	if expr == "" || expr == "*" {
		return true, ""
	}
	hv, ok := normalizeSemver(hostVersion)
	if !ok {
		// 开发版本（如 "dev"）不阻塞兼容检查
		if strings.EqualFold(strings.TrimSpace(hostVersion), "dev") {
			return true, ""
		}
		return false, "宿主版本号格式无效"
	}
	// 比较时忽略宿主 prerelease 以便预发布版本也能匹配 >=x.y.z
	hvCanon := semver.Canonical(hv)
	if idx := strings.IndexByte(hvCanon, '-'); idx > 0 {
		hvCanon = hvCanon[:idx]
	}

	orBranches := strings.Split(expr, "||")
	for _, branch := range orBranches {
		branch = strings.TrimSpace(branch)
		if branch == "" {
			continue
		}
		tokens := strings.Fields(branch)
		allMatch := true
		for _, tok := range tokens {
			if !matchSemverComparator(hvCanon, tok) {
				allMatch = false
				break
			}
		}
		if allMatch {
			return true, ""
		}
	}
	return false, "版本不在允许范围内"
}

func matchSemverComparator(hv, comp string) bool {
	comp = strings.TrimSpace(comp)
	if comp == "" || comp == "*" {
		return true
	}
	switch {
	case strings.HasPrefix(comp, ">="):
		v, ok := normalizeSemver(comp[2:])
		return ok && semver.Compare(hv, v) >= 0
	case strings.HasPrefix(comp, "<="):
		v, ok := normalizeSemver(comp[2:])
		return ok && semver.Compare(hv, v) <= 0
	case strings.HasPrefix(comp, ">"):
		v, ok := normalizeSemver(comp[1:])
		return ok && semver.Compare(hv, v) > 0
	case strings.HasPrefix(comp, "<"):
		v, ok := normalizeSemver(comp[1:])
		return ok && semver.Compare(hv, v) < 0
	case strings.HasPrefix(comp, "="):
		v, ok := normalizeSemver(comp[1:])
		return ok && semver.Compare(hv, v) == 0
	case strings.HasPrefix(comp, "^"):
		v, ok := normalizeSemver(comp[1:])
		if !ok || semver.Compare(hv, v) < 0 {
			return false
		}
		maj := semver.Major(v)
		return semver.Major(hv) == maj
	case strings.HasPrefix(comp, "~"):
		v, ok := normalizeSemver(comp[1:])
		if !ok || semver.Compare(hv, v) < 0 {
			return false
		}
		return semver.MajorMinor(hv) == semver.MajorMinor(v)
	default:
		v, ok := normalizeSemver(comp)
		return ok && semver.Compare(hv, v) == 0
	}
}
