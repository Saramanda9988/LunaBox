package bootstrap

import (
	"context"
	"database/sql"

	"lunabox/backend/appconf"
	"lunabox/backend/applog"
	"lunabox/backend/appruntime"
	"lunabox/backend/plugin"
	"lunabox/backend/service"
)

// Services 是桌面端的全部服务。Wails 版把它们绑定给前端，MyGo 版的界面通过接口调用它们。
type Services struct {
	Startup          *service.StartupService
	Game             *service.GameService
	Bangumi          *service.BangumiService
	Hikarinagi       *service.HikarinagiService
	Kungal           *service.KungalService
	AI               *service.AiService
	AIStats          *service.AIStatsBuilder
	Backup           *service.BackupService
	CloudSync        *service.CloudSyncService
	Home             *service.HomeService
	Stats            *service.StatsService
	Start            *service.StartService
	Integration      *service.IntegrationService
	Category         *service.CategoryService
	Config           *service.ConfigService
	Import           *service.ImportService
	Version          *service.VersionService
	Template         *service.TemplateService
	Update           *service.UpdateService
	Session          *service.SessionService
	Download         *service.DownloadService
	GameProgress     *service.GameProgressService
	GameReview       *service.GameReviewService
	Tag              *service.TagService
	GameFilterPreset *service.GameFilterPresetService
	MCPRead          *service.MCPReadService
	MCPServer        *service.MCPServerService
	PortableSetup    *service.PortableSetupService
	Plugin           *plugin.Manager

	disableMCP bool
}

// ServiceOptions 是构造服务时的参数。
type ServiceOptions struct {
	// QuitForUpdate 在应用内更新下载完成、需要退出以替换程序时调用。
	QuitForUpdate func()
	// DisableMCP 为 true 时保存配置不按配置启动 MCP 服务（开发运行与测试，以免占用安装版的端口）。
	DisableMCP bool
}

// NewServices 构造服务。Wails 要在创建应用时绑定服务，所以构造与 Init 分开。
func NewServices(o ServiceOptions) *Services {
	var quitHandlers []func()
	if o.QuitForUpdate != nil {
		quitHandlers = append(quitHandlers, o.QuitForUpdate)
	}
	return &Services{
		Startup:          service.NewStartupService(),
		Game:             service.NewGameService(),
		Bangumi:          service.NewBangumiService(),
		Hikarinagi:       service.NewHikarinagiService(),
		Kungal:           service.NewKungalService(),
		AI:               service.NewAiService(),
		AIStats:          service.NewAIStatsBuilder(),
		Backup:           service.NewBackupService(),
		CloudSync:        service.NewCloudSyncService(),
		Home:             service.NewHomeService(),
		Stats:            service.NewStatsService(),
		Start:            service.NewStartService(),
		Integration:      service.NewIntegrationService(),
		Category:         service.NewCategoryService(),
		Config:           service.NewConfigService(),
		Import:           service.NewImportService(),
		Version:          service.NewVersionService(),
		Template:         service.NewTemplateService(),
		Update:           service.NewUpdateService(quitHandlers...),
		Session:          service.NewSessionService(),
		Download:         service.NewDownloadService(),
		GameProgress:     service.NewGameProgressService(),
		GameReview:       service.NewGameReviewService(),
		Tag:              service.NewTagService(),
		GameFilterPreset: service.NewGameFilterPresetService(),
		MCPRead:          service.NewMCPReadService(),
		MCPServer:        service.NewMCPServerService(),
		PortableSetup:    service.NewPortableSetupService(),
		Plugin:           plugin.NewManager(),
		disableMCP:       o.DisableMCP,
	}
}

// Init 用打开的数据库与配置初始化服务，并把服务相互注入。
func (s *Services) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.Config.Init(ctx, db, config)
	s.Download.Init(ctx, db, config)
	s.Game.Init(ctx, db, config)
	s.Bangumi.Init(ctx, db, config)
	s.Hikarinagi.Init(ctx, db, config)
	s.Kungal.Init(ctx, db, config)
	s.Tag.Init(ctx, db, config)
	s.GameFilterPreset.Init(ctx, db, config)
	s.AI.Init(ctx, db, config)
	s.AIStats.Init(ctx, db, config)
	s.Backup.Init(ctx, db, config)
	s.CloudSync.Init(ctx, db, config)
	s.Home.Init(ctx, db, config)
	s.Stats.Init(ctx, db, config)
	s.Session.Init(ctx, db, config)
	s.Start.Init(ctx, db, config)
	s.Integration.Init(ctx, db, config)
	s.Category.Init(ctx, db, config)
	s.Import.Init(ctx, db, config)
	s.Version.Init(ctx)
	s.Template.Init(ctx, db, config)
	s.Update.Init(ctx)
	s.GameProgress.Init(ctx, db, config)
	s.GameReview.Init(ctx, db, config)
	s.MCPRead.Init(ctx, db, config)
	s.MCPServer.Init(ctx)
	s.PortableSetup.Init(ctx)

	s.Start.SetBackupService(s.Backup)
	s.Start.SetGameService(s.Game)
	s.Start.SetIntegrationService(s.Integration)
	s.Start.SetSessionService(s.Session)
	s.Download.SetGameService(s.Game)
	s.Config.SetDownloadService(s.Download)
	s.Game.SetImageDownloadTaskStarter(s.Download.StartCoverImageDownloadTask)
	s.Game.SetTagService(s.Tag)
	s.Game.SetBangumiService(s.Bangumi)
	s.Game.SetHikarinagiService(s.Hikarinagi)
	s.Game.SetKungalService(s.Kungal)
	s.GameReview.SetBangumiService(s.Bangumi)
	s.GameReview.SetHikarinagiService(s.Hikarinagi)
	s.Import.SetGameService(s.Game)
	s.Integration.SetGameService(s.Game)
	s.Import.SetBangumiService(s.Bangumi)
	s.Import.SetHikarinagiService(s.Hikarinagi)
	s.Import.SetKungalService(s.Kungal)
	s.Import.SetSessionService(s.Session)
	s.Update.SetConfigService(s.Config)
	s.MCPRead.SetGameService(s.Game)
	s.MCPRead.SetStartService(s.Start)
	s.MCPRead.SetSessionService(s.Session)
	s.MCPRead.SetGameProgressService(s.GameProgress)
	s.MCPRead.SetTagService(s.Tag)
	s.MCPRead.SetStatsProvider(s.AIStats)
	s.MCPServer.SetReadService(s.MCPRead)

	// 初始化插件管理器，并把 hooks.Bus 注入业务 service。
	s.Plugin.Init(ctx, plugin.Dependencies{
		Config:        config,
		ConfigService: s.Config,
		Game:          s.Game,
		Tag:           s.Tag,
		Category:      s.Category,
		Start:         s.Start,
		Download:      s.Download,
		Import:        s.Import,
		AI:            s.AI,
		Hikarinagi:    s.Hikarinagi,
		Session:       s.Session,
		Progress:      s.GameProgress,
		Review:        s.GameReview,
	})
	hookBus := s.Plugin.HookBus()
	s.Game.SetHookBus(hookBus)
	s.Import.SetHookBus(hookBus)
	s.Start.SetHookBus(hookBus)

	s.Config.SetConfigUpdateHook(func(updatedConfig appconf.AppConfig) error {
		s.Plugin.NotifyLanguageChanged(updatedConfig.Language)
		if !s.disableMCP {
			if err := s.MCPServer.ApplyConfig(updatedConfig); err != nil {
				return err
			}
		}
		if err := s.Backup.EnforceLocalDBBackupRetention(); err != nil {
			applog.LogWarningf(ctx, "failed to enforce local database backup retention: %v", err)
		}
		return nil
	})
}

// SetRuntime 把对话框、事件、打开网址与开机自启交给桌面端的 rt。
func (s *Services) SetRuntime(rt appruntime.Runtime) {
	s.Backup.SetRuntime(rt)
	s.Bangumi.SetRuntime(rt)
	s.Hikarinagi.SetRuntime(rt)
	s.Kungal.SetRuntime(rt)
	s.CloudSync.SetRuntime(rt)
	s.Config.SetRuntime(rt)
	s.Download.SetRuntime(rt)
	s.Game.SetRuntime(rt)
	s.Import.SetRuntime(rt)
	s.Start.SetRuntime(rt)
	s.Stats.SetRuntime(rt)
	s.Template.SetRuntime(rt)
	s.Update.SetRuntime(rt)
	s.Plugin.SetRuntime(rt)
}
