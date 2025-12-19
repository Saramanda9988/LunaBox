package potatovn

import (
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

// Galgame PotatoVN 导出的游戏数据结构
type Galgame struct {
	Uuid                 string                      `json:"Uuid"`
	HeaderImageUrl       *string                     `json:"HeaderImageUrl"`
	PlayedTime           map[string]int              `json:"PlayedTime"`
	KeyMappings          []interface{}               `json:"KeyMappings"`
	Ids                  []string                    `json:"Ids"`
	ProcessName          *string                     `json:"ProcessName"`
	TextPath             *string                     `json:"TextPath"`
	PvnUpdate            bool                        `json:"PvnUpdate"`
	PvnUploadProperties  int                         `json:"PvnUploadProperties"`
	AutoFetchStatus      AutoFetchStatus             `json:"AutoFetchStatus"`
	Path                 string                      `json:"Path"`
	RssType              RssType                     `json:"RssType"`
	SavePath             *string                     `json:"SavePath"`
	ImagePath            LockableProperty[string]    `json:"ImagePath"`
	HeaderImagePath      LockableProperty[*string]   `json:"HeaderImagePath"`
	Name                 LockableProperty[string]    `json:"Name"`
	CnName               string                      `json:"CnName"`
	OriginalName         LockableProperty[string]    `json:"OriginalName"`
	ChineseName          LockableProperty[string]    `json:"ChineseName"`
	Description          LockableProperty[string]    `json:"Description"`
	Developer            LockableProperty[string]    `json:"Developer"`
	LastPlayTime         time.Time                   `json:"LastPlayTime"`
	ExpectedPlayTime     LockableProperty[string]    `json:"ExpectedPlayTime"`
	Rating               LockableProperty[float64]   `json:"Rating"`
	ReleaseDate          LockableProperty[time.Time] `json:"ReleaseDate"`
	LastFetchInfoTime    time.Time                   `json:"LastFetchInfoTime"`
	AddTime              time.Time                   `json:"AddTime"`
	Characters           []Character                 `json:"Characters"`
	SavePosition         string                      `json:"SavePosition"`
	PlayCount            int                         `json:"PlayCount"`
	ExePath              *string                     `json:"ExePath"`
	ExeArguments         *string                     `json:"ExeArguments"`
	Tags                 LockableProperty[[]string]  `json:"Tags"`
	TotalPlayTime        int                         `json:"TotalPlayTime"`
	RunAsAdmin           bool                        `json:"RunAsAdmin"`
	RunInLocaleEmulator  bool                        `json:"RunInLocaleEmulator"`
	HighDpi              bool                        `json:"HighDpi"`
	EnableMagpie         bool                        `json:"EnableMagpie"`
	MuteInBackground     bool                        `json:"MuteInBackground"`
	KeyReMap             bool                        `json:"KeyReMap"`
	DetectedSavePosition *string                     `json:"DetectedSavePosition"`
	PlayType             int                         `json:"PlayType"`
	Comment              string                      `json:"Comment"`
	MyRate               int                         `json:"MyRate"`
	PrivateComment       bool                        `json:"PrivateComment"`
}

// GetSourceID 根据 RssType 获取对应的数据源 ID
func (g *Galgame) GetSourceID() string {
	if len(g.Ids) == 0 {
		return ""
	}

	// 根据 RssType 获取对应位置的 ID
	index := int(g.RssType)
	if index < len(g.Ids) && g.Ids[index] != "" {
		return g.Ids[index]
	}

	// 如果当前源没有 ID，尝试从其他位置获取
	for _, id := range g.Ids {
		if id != "" {
			return id
		}
	}
	return ""
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
		return *g.ExePath
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
	ID         string `json:"Id"`
	SourceType int    `json:"SourceType"`
	Path       string `json:"Path"`
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
