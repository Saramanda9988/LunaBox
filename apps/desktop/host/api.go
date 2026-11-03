package host

import (
	"database/sql"
	"strings"

	"lunabox/backend/appconf"
	"lunabox/backend/bootstrap"
	"lunabox/desktop/api"
)

// Backend 是界面使用的后端：bootstrap 构造的服务、数据库与配置，加上把服务事件交给界面的 Events。
// Config 是应用配置（遮住了 Services.Config 这个 ConfigService，后者用 Services.Config 访问）。
type Backend struct {
	*bootstrap.Services
	DB      *sql.DB
	Config  *appconf.AppConfig
	DataDir string
	Events  *Events
}

// API 返回界面使用的接口，背后是同一组真实服务。
func (b *Backend) API() *api.Services {
	s := b.Services
	tz := ""
	if b.Config != nil && b.Config.TimeZone == "" {
		tz = SystemTimeZone() // 只有时区选择弹窗用到
	}
	return &api.Services{
		Config:  b.Config,
		DataDir: b.DataDir,
		Events:  b.Events,

		SystemTimeZone: tz,

		AppConfig:   s.Config,
		Home:        s.Home,
		Game:        s.Game,
		Tag:         s.Tag,
		Stats:       s.Stats,
		Template:    s.Template,
		AI:          s.AI,
		Session:     s.Session,
		Category:    s.Category,
		Backup:      s.Backup,
		Start:       s.Start,
		Integration: s.Integration,
		Progress:    s.GameProgress,
		Review:      s.GameReview,
		Bangumi:     s.Bangumi,
		Hikarinagi:  s.Hikarinagi,
		Kungal:      s.Kungal,
		Download:    s.Download,
		Version:     s.Version,
		Update:      s.Update,
		Portable:    s.PortableSetup,
		CloudSync:   s.CloudSync,
		Presets:     s.GameFilterPreset,
		Import:      s.Import,
	}
}

// SystemTimeZone 返回系统的 IANA 时区：用一个临时的内存 DuckDB 读它的默认时区（ICU 检测系统时区，Windows 上也是
// IANA 名称；Go 的 time.Local 只有“Local”）。应用的数据库不行：没有配置 time_zone 时 OpenDatabase 把它设成 UTC。
// 驱动由后端注册（bootstrap 打开数据库用的也是它）。读取失败时为空。
func SystemTimeZone() string {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return ""
	}
	defer db.Close()
	var tz string
	if err := db.QueryRow("SELECT current_setting('TimeZone')").Scan(&tz); err != nil {
		return ""
	}
	return strings.TrimSpace(tz)
}
