package potatovn

import (
	"fmt"
	"strings"
	"time"
)

// RssType 对应 PotatoVN 的数据源类型
type RssType int

const (
	RssTypeVndb     RssType = 0
	RssTypeBangumi  RssType = 1
	RssTypeMixed    RssType = 2
	RssTypeNone     RssType = 3
	RssTypePotatoVn RssType = 4
	RssTypeYmgal    RssType = 5
	RssTypeCngal    RssType = 6
	RssTypeSteam    RssType = 7
)

// DefaultImagePath PotatoVN 默认图标路径
const DefaultImagePath = "ms-appx:///Assets/WindowIcon.ico"

// LockableProperty 可锁定属性
type LockableProperty[T any] struct {
	Value  T    `json:"Value"`
	IsLock bool `json:"IsLock"`
}

// Character 角色信息
type Character struct {
	Ids              []string `json:"Ids"`
	Name             string   `json:"Name"`
	Relation         string   `json:"Relation"`
	PreviewImagePath string   `json:"PreviewImagePath"`
	ImagePath        string   `json:"ImagePath"`
	Summary          string   `json:"Summary"`
	Gender           int      `json:"Gender"`
	BirthYear        *int     `json:"BirthYear"`
	BirthMon         *int     `json:"BirthMon"`
	BirthDay         *int     `json:"BirthDay"`
	BirthDate        *string  `json:"BirthDate"`
	BloodType        *string  `json:"BloodType"`
	Height           *string  `json:"Height"`
	Weight           *string  `json:"Weight"`
	BWH              *string  `json:"BWH"`
}

// AutoFetchStatus 自动获取状态
type AutoFetchStatus struct {
	HeaderImage bool `json:"HeaderImage"`
	Staff       bool `json:"Staff"`
}

// FlexibleTime handles various time formats exported by PotatoVN
type FlexibleTime time.Time

// UnmarshalJSON accepts RFC3339, RFC3339Nano, and timezone-less timestamps like "2006-01-02T15:04:05" or "2006-01-02"
func (f *FlexibleTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s == "" || s == "null" {
		*f = FlexibleTime(time.Time{})
		return nil
	}

	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			*f = FlexibleTime(t)
			return nil
		}
	}

	return fmt.Errorf("cannot parse time: %s", s)
}

// MarshalJSON formats the time in RFC3339, or null for zero time
func (f FlexibleTime) MarshalJSON() ([]byte, error) {
	t := time.Time(f)
	if t.IsZero() {
		return []byte("null"), nil
	}
	return []byte("\"" + t.Format(time.RFC3339) + "\""), nil
}

// ToTime converts FlexibleTime to time.Time
func (f FlexibleTime) ToTime() time.Time {
	return time.Time(f)
}

// Galgame PotatoVN 导出的游戏数据结构
type Galgame struct {
	Uuid                 string                         `json:"Uuid"`
	HeaderImageUrl       *string                        `json:"HeaderImageUrl"`
	PlayedTime           map[string]int                 `json:"PlayedTime"`
	KeyMappings          []interface{}                  `json:"KeyMappings"`
	Ids                  []string                       `json:"Ids"`
	ProcessName          *string                        `json:"ProcessName"`
	TextPath             *string                        `json:"TextPath"`
	PvnUpdate            bool                           `json:"PvnUpdate"`
	PvnUploadProperties  int                            `json:"PvnUploadProperties"`
	AutoFetchStatus      AutoFetchStatus                `json:"AutoFetchStatus"`
	Path                 string                         `json:"Path"`
	RssType              RssType                        `json:"RssType"`
	SavePath             *string                        `json:"SavePath"`
	ImagePath            LockableProperty[string]       `json:"ImagePath"`
	HeaderImagePath      LockableProperty[*string]      `json:"HeaderImagePath"`
	Name                 LockableProperty[string]       `json:"Name"`
	CnName               string                         `json:"CnName"`
	OriginalName         LockableProperty[string]       `json:"OriginalName"`
	ChineseName          LockableProperty[string]       `json:"ChineseName"`
	Description          LockableProperty[string]       `json:"Description"`
	Developer            LockableProperty[string]       `json:"Developer"`
	LastPlayTime         FlexibleTime                   `json:"LastPlayTime"`
	ExpectedPlayTime     LockableProperty[string]       `json:"ExpectedPlayTime"`
	Rating               LockableProperty[float64]      `json:"Rating"`
	ReleaseDate          LockableProperty[FlexibleTime] `json:"ReleaseDate"`
	LastFetchInfoTime    FlexibleTime                   `json:"LastFetchInfoTime"`
	AddTime              FlexibleTime                   `json:"AddTime"`
	Characters           []Character                    `json:"Characters"`
	SavePosition         string                         `json:"SavePosition"`
	PlayCount            int                            `json:"PlayCount"`
	ExePath              *string                        `json:"ExePath"`
	ExeArguments         *string                        `json:"ExeArguments"`
	Tags                 LockableProperty[[]string]     `json:"Tags"`
	TotalPlayTime        int                            `json:"TotalPlayTime"`
	RunAsAdmin           bool                           `json:"RunAsAdmin"`
	RunInLocaleEmulator  bool                           `json:"RunInLocaleEmulator"`
	HighDpi              bool                           `json:"HighDpi"`
	EnableMagpie         bool                           `json:"EnableMagpie"`
	MuteInBackground     bool                           `json:"MuteInBackground"`
	KeyReMap             bool                           `json:"KeyReMap"`
	DetectedSavePosition *string                        `json:"DetectedSavePosition"`
	PlayType             int                            `json:"PlayType"`
	Comment              string                         `json:"Comment"`
	MyRate               int                            `json:"MyRate"`
	PrivateComment       bool                           `json:"PrivateComment"`
}

// IDForRssType 返回 Ids 数组中指定数据源槽位的 ID
func (g *Galgame) IDForRssType(rssType RssType) string {
	index := int(rssType)
	if index < 0 || index >= len(g.Ids) {
		return ""
	}
	return strings.TrimSpace(g.Ids[index])
}

// MixedIDs 解析 Mixed 槽位的复合 ID 串（形如 "bgm:586274,vndb:59409,ymgal:56847,steam:null"），
// 跳过值为 null 的源
func (g *Galgame) MixedIDs() map[string]string {
	raw := g.IDForRssType(RssTypeMixed)
	if raw == "" {
		return nil
	}
	ids := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		key, value, found := strings.Cut(pair, ":")
		if !found {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if key == "" || value == "" || strings.EqualFold(value, "null") {
			continue
		}
		ids[key] = value
	}
	return ids
}

// GetDisplayName 获取显示名称，优先中文名
func (g *Galgame) GetDisplayName() string {
	if g.ChineseName.Value != "" {
		return g.ChineseName.Value
	}
	if g.CnName != "" {
		return g.CnName
	}
	return g.Name.Value
}

// GetExePath 获取可执行文件路径
func (g *Galgame) GetExePath() string {
	if g.ExePath != nil {
		return strings.TrimSpace(*g.ExePath)
	}
	return ""
}

// GetProcessName 获取手动指定的实际监控进程名
func (g *Galgame) GetProcessName() string {
	if g.ProcessName != nil {
		return strings.TrimSpace(*g.ProcessName)
	}
	return ""
}

// GetSavePath 获取存档路径
func (g *Galgame) GetSavePath() string {
	if g.DetectedSavePosition != nil {
		return *g.DetectedSavePosition
	}
	if g.SavePath != nil {
		return *g.SavePath
	}
	return ""
}

// GalgameSource 游戏数据源信息
type GalgameSource struct {
	ID         string               `json:"Id"`
	SourceType int                  `json:"SourceType"`
	Path       string               `json:"Path"`
	Galgames   []GalgameSourceEntry `json:"Galgames"`
}

// GalgameSourceEntry 记录游戏与 PotatoVN 扫描目录的关联。
type GalgameSourceEntry struct {
	Galgame string `json:"Galgame"`
	Path    string `json:"Path"`
}

// DataStatus 导出数据状态信息
type DataStatus struct {
	Version      string `json:"Version"`
	ExportTime   string `json:"ExportTime"`
	GalgameCount int    `json:"GalgameCount"`
}

// CategoryGroup 分类组
type CategoryGroup struct {
	ID         string   `json:"Id"`
	Name       string   `json:"Name"`
	Categories []string `json:"Categories"`
}

// Staff 制作人员信息
type Staff struct {
	ID       string `json:"Id"`
	Name     string `json:"Name"`
	Role     string `json:"Role"`
	GameUuid string `json:"GameUuid"`
}
